package message_model

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// ArchivedMessage stores the searchable content of every message that passes
// through an instance, in both directions.
//
// The existing Message model only tracks delivery bookkeeping (id/status/source)
// and deliberately holds no text, so it cannot answer "which messages mention X"
// or "what was said last Tuesday". This table backs the MCP search tools.
// Column names are pinned explicitly: GORM's default naming would turn ChatJID
// into "chat_j_id", which the raw SQL in the repository does not expect.
type ArchivedMessage struct {
	Id string `json:"id" gorm:"type:uuid;primaryKey"`
	// InstanceID leads every index, including the unique one — a WhatsApp
	// message id is only unique within an instance, so two instances may
	// legitimately hold the same id.
	InstanceID string `json:"instanceId" gorm:"column:instance_id;size:64;uniqueIndex:idx_arch_instance_msg,priority:1;index:idx_arch_instance_ts,priority:1;index:idx_arch_instance_chat,priority:1"`
	MessageID  string `json:"messageId" gorm:"column:message_id;size:128;uniqueIndex:idx_arch_instance_msg,priority:2"`
	ChatJID    string `json:"chatJid" gorm:"column:chat_jid;size:128;index:idx_arch_instance_chat,priority:2"`
	SenderJID  string `json:"senderJid" gorm:"column:sender_jid;size:128"`
	PushName   string `json:"pushName" gorm:"column:push_name;size:255"`
	FromMe     bool   `json:"fromMe" gorm:"column:from_me"`
	IsGroup    bool   `json:"isGroup" gorm:"column:is_group"`
	// MessageType is the normalized kind (text, image, audio, ...), not the raw
	// protobuf field name.
	MessageType string    `json:"messageType" gorm:"column:message_type;size:64"`
	Content     string    `json:"content" gorm:"column:content;type:text"`
	Timestamp   time.Time `json:"timestamp" gorm:"column:timestamp;index:idx_arch_instance_ts,priority:2"`
	CreatedAt   time.Time `json:"createdAt" gorm:"column:created_at;autoCreateTime"`
}

func (ArchivedMessage) TableName() string { return "archived_messages" }

func (m *ArchivedMessage) BeforeCreate(tx *gorm.DB) (err error) {
	if m.Id == "" {
		m.Id = uuid.New().String()
	}
	return
}

// MessageSearchFilter describes a message archive query. All fields are
// optional; zero values mean "no constraint on this dimension".
type MessageSearchFilter struct {
	InstanceID  string
	Query       string // case-insensitive substring match against Content
	ChatJID     string // partial match, so a bare phone number works
	SenderJID   string
	MessageType string
	FromMe      *bool
	IsGroup     *bool
	StartDate   *time.Time
	EndDate     *time.Time
	Limit       int
	Offset      int
}

// ChatSummary is one row of the "recent chats" listing: the chat plus its most
// recent message.
//
// The column tags matter here as well: this struct is filled by a raw query, and
// GORM would otherwise look for "chat_j_id" instead of the selected "chat_jid".
type ChatSummary struct {
	ChatJID     string    `json:"chatJid" gorm:"column:chat_jid"`
	PushName    string    `json:"pushName" gorm:"column:push_name"`
	IsGroup     bool      `json:"isGroup" gorm:"column:is_group"`
	LastMessage string    `json:"lastMessage" gorm:"column:last_message"`
	LastFromMe  bool      `json:"lastFromMe" gorm:"column:last_from_me"`
	Timestamp   time.Time `json:"timestamp" gorm:"column:timestamp"`
	Total       int64     `json:"totalMessages" gorm:"column:total"`
}
