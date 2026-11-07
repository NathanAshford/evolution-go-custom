package message_service

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/proto/waCommon"
	"go.mau.fi/whatsmeow/proto/waMmsRetry"
	"go.mau.fi/whatsmeow/store"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
)

type mediaDownloadClient interface {
	Download(context.Context, whatsmeow.DownloadableMessage) ([]byte, error)
	DownloadMediaWithPath(context.Context, string, []byte, []byte, []byte, whatsmeow.MediaType, string, bool) ([]byte, error)
	AddEventHandler(whatsmeow.EventHandler) uint32
	RemoveEventHandler(uint32) bool
	SendMediaRetryReceipt(context.Context, *types.MessageInfo, []byte) error
}

func downloadMediaWithRecovery(ctx context.Context, client mediaDownloadClient, lids store.LIDStore, media whatsmeow.DownloadableMessage, key *waCommon.MessageKey) ([]byte, error) {
	data, downloadErr := client.Download(ctx, media)
	if downloadErr == nil || !recoverableMediaDownload(downloadErr, media.GetDirectPath(), time.Now()) {
		return data, downloadErr
	}
	if key == nil || key.FromMe == nil || key.GetID() == "" || len(key.GetID()) > 128 || len(media.GetMediaKey()) != 32 {
		return nil, downloadErr
	}
	chat, err := types.ParseJID(key.GetRemoteJID())
	if err != nil || chat.User == "" || (chat.Server != types.DefaultUserServer && chat.Server != types.HiddenUserServer && chat.Server != types.GroupServer) {
		return nil, downloadErr
	}
	info := &types.MessageInfo{MessageSource: types.MessageSource{Chat: chat.ToNonAD(), IsFromMe: key.GetFromMe(), IsGroup: chat.Server == types.GroupServer}, ID: key.GetID()}
	if info.IsGroup {
		info.Sender, err = types.ParseJID(key.GetParticipant())
		if err != nil || info.Sender.User == "" || (info.Sender.Server != types.DefaultUserServer && info.Sender.Server != types.HiddenUserServer) {
			return nil, downloadErr
		}
		info.Sender = info.Sender.ToNonAD()
	}
	recoveryCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	if recoveryCtx.Err() != nil {
		return nil, fmt.Errorf("%w: media recovery canceled", downloadErr)
	}
	chatAlias, err := mediaRecoveryAlternateJID(recoveryCtx, lids, info.Chat)
	if err != nil {
		return nil, fmt.Errorf("%w: media recovery identity lookup failed", downloadErr)
	}
	var senderAlias types.JID
	if info.IsGroup {
		senderAlias, err = mediaRecoveryAlternateJID(recoveryCtx, lids, info.Sender)
		if err != nil {
			return nil, fmt.Errorf("%w: media recovery identity lookup failed", downloadErr)
		}
	}
	notifications := make(chan any, 1)
	handlerID := client.AddEventHandler(func(raw any) {
		switch evt := raw.(type) {
		case *events.MediaRetry:
			if evt == nil || evt.MessageID != info.ID || evt.FromMe != info.IsFromMe {
				return
			}
			chat := evt.ChatID.ToNonAD()
			if chat != info.Chat && (chatAlias.IsEmpty() || chat != chatAlias) {
				return
			}
			if info.IsGroup && evt.SenderID.ToNonAD() != info.Sender && (senderAlias.IsEmpty() || evt.SenderID.ToNonAD() != senderAlias) {
				return
			}
		case *events.Disconnected, *events.LoggedOut:
		default:
			return
		}
		select {
		case notifications <- raw:
		default:
		}
	})
	// Remove outside the callback: the SDK dispatch holds a read lock on handlers.
	defer client.RemoveEventHandler(handlerID)
	if err = client.SendMediaRetryReceipt(recoveryCtx, info, media.GetMediaKey()); err != nil {
		return nil, fmt.Errorf("%w: media recovery request failed", downloadErr)
	}
	select {
	case raw := <-notifications:
		notification, ok := raw.(*events.MediaRetry)
		if !ok {
			return nil, fmt.Errorf("%w: media recovery disconnected", downloadErr)
		}
		recovered, decryptErr := whatsmeow.DecryptMediaRetryNotification(notification, media.GetMediaKey())
		if decryptErr != nil || recovered.GetResult() != waMmsRetry.MediaRetryNotification_SUCCESS || !validMediaRecoveryPath(recovered.GetDirectPath()) {
			return nil, fmt.Errorf("%w: media recovery unavailable", downloadErr)
		}
		data, err = client.DownloadMediaWithPath(recoveryCtx, recovered.GetDirectPath(), media.GetFileEncSHA256(), media.GetFileSHA256(), media.GetMediaKey(), whatsmeow.GetMediaType(media), "", false)
		if err != nil {
			return nil, fmt.Errorf("%w: media recovery download failed", downloadErr)
		}
		return data, nil
	case <-recoveryCtx.Done():
		return nil, fmt.Errorf("%w: media recovery canceled", downloadErr)
	}
}

func mediaRecoveryAlternateJID(ctx context.Context, lids store.LIDStore, jid types.JID) (types.JID, error) {
	if lids == nil {
		return types.JID{}, nil
	}
	switch jid.Server {
	case types.DefaultUserServer:
		return lids.GetLIDForPN(ctx, jid)
	case types.HiddenUserServer:
		return lids.GetPNForLID(ctx, jid)
	default:
		return types.JID{}, nil
	}
}

func validMediaRecoveryPath(path string) bool {
	parsed, err := url.Parse(path)
	return err == nil && strings.HasPrefix(path, "/") && parsed.Host == "" && parsed.Scheme == "" && parsed.Fragment == ""
}

func recoverableMediaDownload(err error, path string, now time.Time) bool {
	if errors.Is(err, whatsmeow.ErrMediaDownloadFailedWith404) || errors.Is(err, whatsmeow.ErrMediaDownloadFailedWith410) {
		return true
	}
	if !errors.Is(err, whatsmeow.ErrMediaDownloadFailedWith403) || !validMediaRecoveryPath(path) {
		return false
	}
	parsed, _ := url.Parse(path)
	query, parseErr := url.ParseQuery(parsed.RawQuery)
	expiry := query["oe"]
	if parseErr != nil || len(expiry) != 1 || len(expiry[0]) == 0 || len(expiry[0]) > 16 {
		return false
	}
	for _, digit := range expiry[0] {
		if !strings.ContainsRune("0123456789abcdefABCDEF", digit) {
			return false
		}
	}
	expires, parseErr := strconv.ParseInt(expiry[0], 16, 64)
	// shortcut: oe is an undocumented expiry hint; revisit if WhatsApp changes CDN links.
	return parseErr == nil && expires > 0 && expires <= now.Unix()
}
