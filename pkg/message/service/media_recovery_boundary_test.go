package message_service

import (
	"bytes"
	"context"
	"fmt"
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/proto/waCommon"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/proto/waMmsRetry"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
	"go.mau.fi/whatsmeow/util/gcmutil"
	"go.mau.fi/whatsmeow/util/hkdfutil"
	"google.golang.org/protobuf/proto"
	"net/http"
	"testing"
	"time"
)

type mediaDownloadFixture struct {
	initialError   error
	handler        whatsmeow.EventHandler
	sent           int
	removed        int
	initial        int
	refreshed      int
	refreshedPath  string
	refreshedError error
	onReceipt      func(context.Context, *types.MessageInfo, []byte) error
}

func (f *mediaDownloadFixture) Download(context.Context, whatsmeow.DownloadableMessage) ([]byte, error) {
	f.initial++
	if f.initialError != nil {
		return nil, f.initialError
	}
	return []byte("original media"), nil
}
func (f *mediaDownloadFixture) DownloadMediaWithPath(_ context.Context, path string, encHash, hash, key []byte, _ whatsmeow.MediaType, _ string, _ bool) ([]byte, error) {
	f.refreshed++
	f.refreshedPath = path
	if len(encHash) != 32 || len(hash) != 32 || len(key) != 32 {
		return nil, fmt.Errorf("wrong media crypto metadata")
	}
	if f.refreshedError != nil {
		return nil, f.refreshedError
	}
	return []byte("recovered media"), nil
}
func (f *mediaDownloadFixture) AddEventHandler(handler whatsmeow.EventHandler) uint32 {
	f.handler = handler
	return 1
}
func (f *mediaDownloadFixture) RemoveEventHandler(uint32) bool {
	f.removed++
	f.handler = nil
	return true
}
func (f *mediaDownloadFixture) SendMediaRetryReceipt(ctx context.Context, info *types.MessageInfo, key []byte) error {
	f.sent++
	return f.onReceipt(ctx, info, key)
}

func mediaRecoveryInput() (*waE2E.AudioMessage, *waCommon.MessageKey) {
	return &waE2E.AudioMessage{DirectPath: proto.String("/v/old.enc?oe=1"), MediaKey: bytes.Repeat([]byte{1}, 32), FileEncSHA256: bytes.Repeat([]byte{2}, 32), FileSHA256: bytes.Repeat([]byte{3}, 32)}, &waCommon.MessageKey{RemoteJID: proto.String("15550000001@s.whatsapp.net"), FromMe: proto.Bool(false), ID: proto.String("SYNTHETIC-MEDIA")}
}
func encryptedMediaRecovery(t *testing.T, key []byte, info *types.MessageInfo) *events.MediaRetry {
	t.Helper()
	payload, err := proto.Marshal(&waMmsRetry.MediaRetryNotification{Result: waMmsRetry.MediaRetryNotification_SUCCESS.Enum(), DirectPath: proto.String("/v/new.enc?oe=ffffffff")})
	if err != nil {
		t.Fatal(err)
	}
	iv := bytes.Repeat([]byte{4}, 12)
	ciphertext, err := gcmutil.Encrypt(hkdfutil.SHA256(key, nil, []byte("WhatsApp Media Retry Notification"), 32), iv, payload, []byte(info.ID))
	if err != nil {
		t.Fatal(err)
	}
	return &events.MediaRetry{MessageID: info.ID, ChatID: info.Chat, FromMe: info.IsFromMe, Ciphertext: ciphertext, IV: iv}
}

func TestMediaDownloadKeepsSuccessfulSDKDownload(t *testing.T) {
	media, key := mediaRecoveryInput()
	f := &mediaDownloadFixture{}
	data, err := downloadMediaWithRecovery(context.Background(), f, nil, media, key)
	if err != nil || string(data) != "original media" || f.initial != 1 || f.sent != 0 || f.removed != 0 {
		t.Fatalf("successful media download changed: %q %v", data, err)
	}
}

func TestMediaDownloadRecoversGoneFileWithPinnedSDKNotification(t *testing.T) {
	media, key := mediaRecoveryInput()
	f := &mediaDownloadFixture{initialError: whatsmeow.DownloadHTTPError{Response: &http.Response{StatusCode: 410}}}
	f.onReceipt = func(ctx context.Context, info *types.MessageInfo, mediaKey []byte) error {
		deadline, ok := ctx.Deadline()
		if !ok || time.Until(deadline) > 30*time.Second {
			t.Fatal("recovery is not bounded")
		}
		if info.ID != "SYNTHETIC-MEDIA" || info.Chat.String() != "15550000001@s.whatsapp.net" || info.IsFromMe {
			t.Fatal("wrong native message identity")
		}
		f.handler(encryptedMediaRecovery(t, mediaKey, info))
		return nil
	}
	data, err := downloadMediaWithRecovery(context.Background(), f, nil, media, key)
	if err != nil || string(data) != "recovered media" {
		t.Fatalf("gone media was not recovered: %q %v", data, err)
	}
	if f.sent != 1 || f.refreshed != 1 || f.removed != 1 || f.handler != nil {
		t.Fatalf("unbounded or uncleared recovery: %+v", f)
	}
	if media.GetDirectPath() != "/v/old.enc?oe=1" || f.refreshedPath != "/v/new.enc?oe=ffffffff" {
		t.Fatal("original metadata mutated or refreshed path lost")
	}
}

func TestMediaDownloadRecoversExpiredForbiddenFile(t *testing.T) {
	media, key := mediaRecoveryInput()
	f := &mediaDownloadFixture{initialError: whatsmeow.DownloadHTTPError{Response: &http.Response{StatusCode: 403}}}
	f.onReceipt = func(ctx context.Context, info *types.MessageInfo, mediaKey []byte) error {
		f.handler(encryptedMediaRecovery(t, mediaKey, info))
		return nil
	}
	data, err := downloadMediaWithRecovery(context.Background(), f, nil, media, key)
	if err != nil || string(data) != "recovered media" || f.sent != 1 || f.removed != 1 {
		t.Fatalf("expired forbidden media not recovered: %q %v", data, err)
	}
}

func TestMediaDownloadDoesNotRecoverUnprovenForbiddenOrOtherFailures(t *testing.T) {
	for _, test := range []struct {
		name, path string
		status     int
	}{
		{"missing-expiry", "/v/old.enc?oh=synthetic", 403},
		{"future-expiry", "/v/old.enc?oe=ffffffffffff", 403},
		{"duplicate-expiry", "/v/old.enc?oe=1&oe=2", 403},
		{"invalid-expiry", "/v/old.enc?oe=0x1", 403},
		{"signed-expiry", "/v/old.enc?oe=+1", 403},
		{"invalid-query", "/v/old.enc?oe=1&oh=%zz", 403},
		{"overflow-expiry", "/v/old.enc?oe=ffffffffffffffff", 403},
		{"absolute-path", "https://example.invalid/media?oe=1", 403},
		{"empty-expiry", "/v/old.enc?oe=", 403},
		{"zero-expiry", "/v/old.enc?oe=0", 403},
		{"rate-limit", "/v/old.enc?oe=1", 429},
		{"upstream-error", "/v/old.enc?oe=1", 500},
	} {
		t.Run(test.name, func(t *testing.T) {
			media, key := mediaRecoveryInput()
			media.DirectPath = proto.String(test.path)
			original := whatsmeow.DownloadHTTPError{Response: &http.Response{StatusCode: test.status}}
			f := &mediaDownloadFixture{initialError: original}
			_, err := downloadMediaWithRecovery(context.Background(), f, nil, media, key)
			if err != original || f.sent != 0 || f.handler != nil || f.refreshed != 0 {
				t.Fatalf("unproven failure triggered recovery: %v", err)
			}
		})
	}
	media, key := mediaRecoveryInput()
	f := &mediaDownloadFixture{initialError: whatsmeow.ErrInvalidMediaSHA256}
	_, err := downloadMediaWithRecovery(context.Background(), f, nil, media, key)
	if err != whatsmeow.ErrInvalidMediaSHA256 || f.sent != 0 {
		t.Fatal("integrity failure triggered recovery")
	}
}

func TestMediaDownloadLegacyOrInvalidKeyDoesNotRequestRecovery(t *testing.T) {
	for _, kind := range []string{"legacy", "missing-fromme", "missing-id", "invalid-chat", "missing-media-key", "group-without-sender"} {
		t.Run(kind, func(t *testing.T) {
			media, key := mediaRecoveryInput()
			switch kind {
			case "legacy":
				key = nil
			case "missing-fromme":
				key.FromMe = nil
			case "missing-id":
				key.ID = nil
			case "invalid-chat":
				key.RemoteJID = proto.String("bad@unsupported.invalid")
			case "missing-media-key":
				media.MediaKey = nil
			case "group-without-sender":
				key.RemoteJID = proto.String("123-456@g.us")
			}
			f := &mediaDownloadFixture{initialError: whatsmeow.ErrMediaDownloadFailedWith410}
			_, err := downloadMediaWithRecovery(context.Background(), f, nil, media, key)
			if err == nil || f.sent != 0 || f.handler != nil {
				t.Fatal("invalid identity caused recovery")
			}
		})
	}
}

func TestMediaDownloadIgnoresOtherMessageNotifications(t *testing.T) {
	media, key := mediaRecoveryInput()
	f := &mediaDownloadFixture{initialError: whatsmeow.ErrMediaDownloadFailedWith404}
	f.onReceipt = func(ctx context.Context, info *types.MessageInfo, mediaKey []byte) error {
		for _, kind := range []string{"id", "chat", "direction"} {
			wrong := encryptedMediaRecovery(t, mediaKey, info)
			switch kind {
			case "id":
				wrong.MessageID = "OTHER"
			case "chat":
				wrong.ChatID = types.NewJID("15550000002", types.DefaultUserServer)
			case "direction":
				wrong.FromMe = !info.IsFromMe
			}
			f.handler(wrong)
		}
		f.handler(encryptedMediaRecovery(t, mediaKey, info))
		return nil
	}
	data, err := downloadMediaWithRecovery(context.Background(), f, nil, media, key)
	if err != nil || string(data) != "recovered media" || f.sent != 1 || f.refreshed != 1 || f.removed != 1 {
		t.Fatalf("mismatched event won recovery: %v", err)
	}
}

func TestMediaDownloadRecoveryFailureIsBoundedAndReleasesHandler(t *testing.T) {
	for _, kind := range []string{"send-failed", "phone-unavailable", "bad-ciphertext", "disconnect", "cancel"} {
		t.Run(kind, func(t *testing.T) {
			media, key := mediaRecoveryInput()
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			f := &mediaDownloadFixture{initialError: whatsmeow.ErrMediaDownloadFailedWith410}
			f.onReceipt = func(ctx context.Context, info *types.MessageInfo, mediaKey []byte) error {
				switch kind {
				case "send-failed":
					return fmt.Errorf("upstream secret must not escape")
				case "phone-unavailable":
					f.handler(&events.MediaRetry{MessageID: info.ID, ChatID: info.Chat, FromMe: info.IsFromMe, Error: &events.MediaRetryError{Code: 2}})
				case "bad-ciphertext":
					evt := encryptedMediaRecovery(t, mediaKey, info)
					evt.Ciphertext[0] ^= 1
					f.handler(evt)
				case "disconnect":
					f.handler(&events.Disconnected{})
				case "cancel":
					cancel()
				}
				return nil
			}
			data, err := downloadMediaWithRecovery(ctx, f, nil, media, key)
			if err == nil || data != nil || f.sent != 1 || f.refreshed != 0 || f.removed != 1 || f.handler != nil {
				t.Fatalf("unsafe recovery failure: %q %v", data, err)
			}
			if bytes.Contains([]byte(err.Error()), []byte("secret")) {
				t.Fatal("raw upstream error escaped")
			}
		})
	}
}

func TestMediaDownloadCanceledRequestDoesNotSendReceipt(t *testing.T) {
	media, key := mediaRecoveryInput()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	f := &mediaDownloadFixture{initialError: whatsmeow.ErrMediaDownloadFailedWith410}
	_, err := downloadMediaWithRecovery(ctx, f, nil, media, key)
	if err == nil || f.sent != 0 || f.removed != 0 {
		t.Fatal("canceled request sent recovery")
	}
}

func TestMediaDownloadRecoveryHonorsEarlierRequestDeadline(t *testing.T) {
	media, key := mediaRecoveryInput()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	f := &mediaDownloadFixture{initialError: whatsmeow.ErrMediaDownloadFailedWith410, onReceipt: func(context.Context, *types.MessageInfo, []byte) error { return nil }}
	start := time.Now()
	_, err := downloadMediaWithRecovery(ctx, f, nil, media, key)
	if err == nil || f.sent != 1 || f.removed != 1 || time.Since(start) > time.Second {
		t.Fatalf("request deadline or handler cleanup ignored: %v", err)
	}
}

func TestMediaDownloadRecoveredDownloadFailureNeverReportsSuccess(t *testing.T) {
	media, key := mediaRecoveryInput()
	f := &mediaDownloadFixture{initialError: whatsmeow.ErrMediaDownloadFailedWith410, refreshedError: fmt.Errorf("https://example.invalid/private-token")}
	f.onReceipt = func(ctx context.Context, info *types.MessageInfo, mediaKey []byte) error {
		f.handler(encryptedMediaRecovery(t, mediaKey, info))
		return nil
	}
	data, err := downloadMediaWithRecovery(context.Background(), f, nil, media, key)
	if err == nil || data != nil || f.sent != 1 || f.refreshed != 1 || f.removed != 1 {
		t.Fatal("failed recovered download reported success or retried")
	}
	if bytes.Contains([]byte(err.Error()), []byte("private-token")) {
		t.Fatal("recovered download URL escaped")
	}
}

func TestMediaDownloadGroupRecoveryMatchesParticipant(t *testing.T) {
	media, key := mediaRecoveryInput()
	key.RemoteJID = proto.String("123-456@g.us")
	key.Participant = proto.String("15550000001@s.whatsapp.net")
	f := &mediaDownloadFixture{initialError: whatsmeow.ErrMediaDownloadFailedWith410}
	f.onReceipt = func(ctx context.Context, info *types.MessageInfo, mediaKey []byte) error {
		if !info.IsGroup || info.Sender.String() != "15550000001@s.whatsapp.net" {
			t.Fatal("group sender lost")
		}
		wrong := encryptedMediaRecovery(t, mediaKey, info)
		wrong.SenderID = types.NewJID("15550000002", types.DefaultUserServer)
		f.handler(wrong)
		matching := encryptedMediaRecovery(t, mediaKey, info)
		matching.SenderID = info.Sender
		f.handler(matching)
		return nil
	}
	data, err := downloadMediaWithRecovery(context.Background(), f, nil, media, key)
	if err != nil || string(data) != "recovered media" || f.sent != 1 || f.removed != 1 {
		t.Fatalf("group identity mismatch: %v", err)
	}
}
