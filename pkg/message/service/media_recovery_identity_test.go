package message_service

import (
	"context"
	"net/http"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"go.mau.fi/whatsmeow"
	waBinary "go.mau.fi/whatsmeow/binary"
	"go.mau.fi/whatsmeow/store"
	"go.mau.fi/whatsmeow/store/sqlstore"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
	"google.golang.org/protobuf/proto"
	_ "modernc.org/sqlite"
)

func mediaRecoveryLIDStore(t *testing.T) *sqlstore.Container {
	t.Helper()
	c, err := sqlstore.New(context.Background(), "sqlite", "file:"+filepath.Join(t.TempDir(), "mapping.db")+"?_pragma=foreign_keys(1)", nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { c.Close() })
	pn := types.NewJID("15550000001", types.DefaultUserServer)
	lid := types.NewJID("100000000000002", types.HiddenUserServer)
	if err = c.LIDMap.PutLIDMapping(context.Background(), lid, pn); err != nil {
		t.Fatal(err)
	}
	actual, err := c.LIDMap.GetLIDForPN(context.Background(), pn)
	if err != nil || actual != lid {
		t.Fatalf("stored alias was not read back: %v", err)
	}
	return c
}

func dispatchNativeMediaRetry(t *testing.T, handler whatsmeow.EventHandler, evt *events.MediaRetry) {
	t.Helper()
	client := whatsmeow.NewClient(&store.Device{}, nil)
	client.AddEventHandler(handler)
	client.DangerousInternals().HandleMediaRetryNotification(context.Background(), &waBinary.Node{
		Tag: "notification", Attrs: waBinary.Attrs{"id": evt.MessageID, "t": strconv.FormatInt(time.Now().Unix(), 10)},
		Content: []waBinary.Node{
			{Tag: "rmr", Attrs: waBinary.Attrs{"jid": evt.ChatID, "from_me": strconv.FormatBool(evt.FromMe), "participant": evt.SenderID}},
			{Tag: "encrypt", Content: []waBinary.Node{{Tag: "enc_p", Content: evt.Ciphertext}, {Tag: "enc_iv", Content: evt.IV}}},
		},
	})
}

func TestMediaRecoveryUsesStoredAliasesAndNativeNotification(t *testing.T) {
	for _, scenario := range []string{"pn-to-lid", "owner-pn-to-lid", "lid-to-pn", "group-participant", "no-map", "wrong-chat", "wrong-id", "wrong-direction", "wrong-participant", "closed-store"} {
		t.Run(scenario, func(t *testing.T) {
			c := mediaRecoveryLIDStore(t)
			media, key := mediaRecoveryInput()
			if scenario == "owner-pn-to-lid" {
				key.FromMe = proto.Bool(true)
			}
			pn := types.NewJID("15550000001", types.DefaultUserServer)
			lid := types.NewJID("100000000000002", types.HiddenUserServer)
			if scenario == "lid-to-pn" {
				key.RemoteJID = proto.String(lid.String())
			}
			if scenario == "group-participant" || scenario == "wrong-participant" {
				key.RemoteJID = proto.String("12345@g.us")
				key.Participant = proto.String(pn.String())
			}
			if scenario == "no-map" {
				key.RemoteJID = proto.String("15550000002@s.whatsapp.net")
			}
			if scenario == "closed-store" {
				key.RemoteJID = proto.String("15550000009@s.whatsapp.net")
				c.Close()
			}
			f := &mediaDownloadFixture{initialError: whatsmeow.DownloadHTTPError{Response: &http.Response{StatusCode: 403}}}
			f.onReceipt = func(_ context.Context, info *types.MessageInfo, mediaKey []byte) error {
				evt := encryptedMediaRecovery(t, mediaKey, info)
				if info.IsGroup {
					evt.SenderID = lid
				} else {
					evt.ChatID = lid
				}
				switch scenario {
				case "lid-to-pn":
					evt.ChatID = pn
				case "wrong-chat":
					evt.ChatID = types.NewJID("100000000000003", types.HiddenUserServer)
				case "wrong-id":
					evt.MessageID = "OTHER-SYNTHETIC"
				case "wrong-direction":
					evt.FromMe = !info.IsFromMe
				case "wrong-participant":
					evt.SenderID = types.NewJID("100000000000003", types.HiddenUserServer)
				}
				dispatchNativeMediaRetry(t, f.handler, evt)
				return nil
			}
			ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
			defer cancel()
			data, err := downloadMediaWithRecovery(ctx, f, c.LIDMap, media, key)
			wantSuccess := scenario == "pn-to-lid" || scenario == "owner-pn-to-lid" || scenario == "lid-to-pn" || scenario == "group-participant"
			if wantSuccess {
				if err != nil || string(data) != "recovered media" || f.refreshed != 1 {
					t.Fatalf("stored alias did not recover: %v", err)
				}
			} else if err == nil || f.refreshed != 0 {
				t.Fatalf("untrusted or unreadable alias accepted: %v", err)
			}
			if scenario == "closed-store" {
				if f.sent != 0 || f.removed != 0 {
					t.Fatal("lookup failure sent a receipt or leaked handler")
				}
			} else if f.sent != 1 || f.removed != 1 || f.handler != nil {
				t.Fatal("receipt bound or handler cleanup changed")
			}
		})
	}
}

func TestMediaRecoveryAcceptsPhoneResponseAfterFifteenSeconds(t *testing.T) {
	c := mediaRecoveryLIDStore(t)
	media, key := mediaRecoveryInput()
	f := &mediaDownloadFixture{initialError: whatsmeow.DownloadHTTPError{Response: &http.Response{StatusCode: 403}}}
	f.onReceipt = func(ctx context.Context, info *types.MessageInfo, mediaKey []byte) error {
		deadline, ok := ctx.Deadline()
		if !ok || time.Until(deadline) > 30*time.Second {
			t.Fatal("unbounded recovery")
		}
		evt := encryptedMediaRecovery(t, mediaKey, info)
		evt.ChatID = types.NewJID("100000000000002", types.HiddenUserServer)
		handler := f.handler
		go func() {
			select {
			case <-time.After(16 * time.Second):
				handler(evt)
			case <-ctx.Done():
			}
		}()
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 22*time.Second)
	defer cancel()
	data, err := downloadMediaWithRecovery(ctx, f, c.LIDMap, media, key)
	if err != nil || string(data) != "recovered media" || f.sent != 1 || f.removed != 1 {
		t.Fatalf("late phone response lost: %v", err)
	}
}
