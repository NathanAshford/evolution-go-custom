package whatsmeow_service

import (
	"strings"
	"time"

	message_model "github.com/EvolutionAPI/evolution-go/pkg/message/model"
	voip_registry "github.com/EvolutionAPI/evolution-go/pkg/voip/registry"
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/types/events"
)

// maxArchivedContent caps stored text so a pathological payload (e.g. a huge
// base64 caption) can't bloat the archive table.
const maxArchivedContent = 16000

// describeMessage reduces a protobuf message to a searchable (type, text) pair.
//
// Media messages contribute their caption when present, so "search for the
// message where I sent the invoice" works even though the bytes aren't stored.
func describeMessage(msg *waE2E.Message) (msgType string, content string) {
	if msg == nil {
		return "unknown", ""
	}

	// Unwrap the containers WhatsApp uses to future-proof new message types.
	if v := msg.GetEphemeralMessage().GetMessage(); v != nil {
		return describeMessage(v)
	}
	if v := msg.GetViewOnceMessage().GetMessage(); v != nil {
		return describeMessage(v)
	}
	if v := msg.GetViewOnceMessageV2().GetMessage(); v != nil {
		return describeMessage(v)
	}
	if v := msg.GetDocumentWithCaptionMessage().GetMessage(); v != nil {
		return describeMessage(v)
	}

	switch {
	case msg.GetConversation() != "":
		return "text", msg.GetConversation()
	case msg.GetExtendedTextMessage() != nil:
		return "text", msg.GetExtendedTextMessage().GetText()
	case msg.GetImageMessage() != nil:
		return "image", msg.GetImageMessage().GetCaption()
	case msg.GetVideoMessage() != nil:
		return "video", msg.GetVideoMessage().GetCaption()
	case msg.GetAudioMessage() != nil:
		return "audio", ""
	case msg.GetDocumentMessage() != nil:
		doc := msg.GetDocumentMessage()
		text := doc.GetCaption()
		if title := doc.GetTitle(); title != "" {
			text = strings.TrimSpace(title + " " + text)
		}
		return "document", text
	case msg.GetStickerMessage() != nil:
		return "sticker", ""
	case msg.GetContactMessage() != nil:
		return "contact", msg.GetContactMessage().GetDisplayName()
	case msg.GetLocationMessage() != nil:
		return "location", msg.GetLocationMessage().GetName()
	case msg.GetReactionMessage() != nil:
		return "reaction", msg.GetReactionMessage().GetText()
	case msg.GetPollCreationMessage() != nil:
		return "poll", msg.GetPollCreationMessage().GetName()
	case msg.GetButtonsMessage() != nil:
		b := msg.GetButtonsMessage()
		return "buttons", strings.TrimSpace(b.GetContentText() + " " + b.GetFooterText())
	case msg.GetListMessage() != nil:
		l := msg.GetListMessage()
		return "list", strings.TrimSpace(l.GetTitle() + " " + l.GetDescription())
	case msg.GetInteractiveMessage() != nil:
		i := msg.GetInteractiveMessage()
		return "interactive", strings.TrimSpace(
			i.GetHeader().GetTitle() + " " + i.GetBody().GetText(),
		)
	case msg.GetTemplateMessage() != nil:
		return "template", ""
	case msg.GetProtocolMessage() != nil:
		return "protocol", ""
	}

	return "unknown", ""
}

func truncateContent(s string) string {
	if len(s) > maxArchivedContent {
		return s[:maxArchivedContent]
	}
	return s
}

// archiveIncomingMessage records a received message so it can be searched later.
// Failures are logged and swallowed: archiving must never block delivery or
// webhook dispatch.
func (mycli *MyClient) archiveIncomingMessage(evt *events.Message) {
	if evt == nil || mycli.messageRepository == nil {
		return
	}

	msgType, content := describeMessage(evt.Message)

	// Protocol/system messages carry nothing a human would search for.
	if msgType == "protocol" {
		return
	}

	chatJID := evt.Info.Chat.String()
	record := message_model.ArchivedMessage{
		InstanceID:  mycli.userID,
		MessageID:   evt.Info.ID,
		ChatJID:     chatJID,
		SenderJID:   evt.Info.Sender.String(),
		PushName:    evt.Info.PushName,
		FromMe:      evt.Info.IsFromMe,
		IsGroup:     strings.Contains(chatJID, "@g.us"),
		MessageType: msgType,
		Content:     truncateContent(content),
		Timestamp:   evt.Info.Timestamp,
	}

	if record.Timestamp.IsZero() {
		record.Timestamp = time.Now()
	}

	if err := mycli.messageRepository.ArchiveMessage(record); err != nil {
		mycli.loggerWrapper.GetLogger(mycli.userID).LogWarn(
			"[%s] Failed to archive incoming message %s: %v", mycli.userID, evt.Info.ID, err,
		)
	}
}

// ArchiveOutgoingMessage records a message sent through the API. Outgoing sends
// do not produce an events.Message on this client, so the send path calls this
// explicitly to keep both directions searchable.
func (w *whatsmeowService) ArchiveOutgoingMessage(instanceId, messageID, chatJID string, msg *waE2E.Message, timestamp time.Time) {
	if w.messageRepository == nil || instanceId == "" || messageID == "" {
		return
	}

	msgType, content := describeMessage(msg)
	if msgType == "protocol" {
		return
	}

	if timestamp.IsZero() {
		timestamp = time.Now()
	}

	var senderJID string
	if client := w.clients.Get(instanceId); client != nil && client.Store != nil && client.Store.ID != nil {
		senderJID = client.Store.ID.String()
	}

	record := message_model.ArchivedMessage{
		InstanceID:  instanceId,
		MessageID:   messageID,
		ChatJID:     chatJID,
		SenderJID:   senderJID,
		FromMe:      true,
		IsGroup:     strings.Contains(chatJID, "@g.us"),
		MessageType: msgType,
		Content:     truncateContent(content),
		Timestamp:   timestamp,
	}

	if err := w.messageRepository.ArchiveMessage(record); err != nil {
		w.loggerWrapper.GetLogger(instanceId).LogWarn(
			"[%s] Failed to archive outgoing message %s: %v", instanceId, messageID, err,
		)
	}
}

// CallRegistry exposes the VoIP call registry to the call service.
func (w *whatsmeowService) CallRegistry() *voip_registry.Registry {
	return w.callRegistry
}

// GetClient returns the live whatsmeow client of an instance, or nil when the
// instance is not started.
func (w *whatsmeowService) GetClient(instanceId string) *whatsmeow.Client {
	return w.clients.Get(instanceId)
}

// SearchArchivedMessages exposes the message archive to callers that only hold
// the whatsmeow service (the MCP server).
func (w *whatsmeowService) SearchArchivedMessages(filter message_model.MessageSearchFilter) ([]message_model.ArchivedMessage, int64, error) {
	return w.messageRepository.SearchMessages(filter)
}

// GetArchivedMessage returns one archived message by WhatsApp message id.
func (w *whatsmeowService) GetArchivedMessage(instanceId, messageID string) (*message_model.ArchivedMessage, error) {
	return w.messageRepository.GetArchivedMessage(instanceId, messageID)
}

// ListArchivedChats returns the most recently active chats for an instance.
func (w *whatsmeowService) ListArchivedChats(instanceId string, limit int) ([]message_model.ChatSummary, error) {
	return w.messageRepository.ListChats(instanceId, limit)
}
