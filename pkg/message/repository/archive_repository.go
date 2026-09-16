package message_repository

import (
	"strings"

	message_model "github.com/EvolutionAPI/evolution-go/pkg/message/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	defaultSearchLimit = 50
	maxSearchLimit     = 500
)

// ArchiveMessage stores a message for later searching. Re-delivered messages
// (WhatsApp retries, history sync) update the existing row instead of
// duplicating it.
func (m *messageRepository) ArchiveMessage(msg message_model.ArchivedMessage) error {
	if msg.InstanceID == "" || msg.MessageID == "" {
		// Nothing searchable to key on — skip rather than write a junk row.
		return nil
	}

	return m.db.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "instance_id"}, {Name: "message_id"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"content", "message_type", "push_name", "timestamp",
		}),
	}).Create(&msg).Error
}

// SearchMessages returns archived messages matching the filter, newest first.
func (m *messageRepository) SearchMessages(filter message_model.MessageSearchFilter) ([]message_model.ArchivedMessage, int64, error) {
	query := m.db.Model(&message_model.ArchivedMessage{})

	if filter.InstanceID != "" {
		query = query.Where("instance_id = ?", filter.InstanceID)
	}
	if q := strings.TrimSpace(filter.Query); q != "" {
		// ILIKE keeps the search case-insensitive on Postgres; the wildcards make
		// it a substring match so "boleto" finds "segue o boleto".
		query = query.Where("content ILIKE ?", "%"+escapeLike(q)+"%")
	}
	if filter.ChatJID != "" {
		// Partial match so callers can pass a bare number instead of a full JID.
		query = query.Where("chat_jid ILIKE ?", "%"+escapeLike(filter.ChatJID)+"%")
	}
	if filter.SenderJID != "" {
		query = query.Where("sender_jid ILIKE ?", "%"+escapeLike(filter.SenderJID)+"%")
	}
	if filter.MessageType != "" {
		query = query.Where("message_type = ?", filter.MessageType)
	}
	if filter.FromMe != nil {
		query = query.Where("from_me = ?", *filter.FromMe)
	}
	if filter.IsGroup != nil {
		query = query.Where("is_group = ?", *filter.IsGroup)
	}
	if filter.StartDate != nil {
		query = query.Where("timestamp >= ?", *filter.StartDate)
	}
	if filter.EndDate != nil {
		query = query.Where("timestamp <= ?", *filter.EndDate)
	}

	// Total is computed before paging so callers can tell how much was truncated.
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	limit := filter.Limit
	if limit <= 0 {
		limit = defaultSearchLimit
	}
	if limit > maxSearchLimit {
		limit = maxSearchLimit
	}

	var results []message_model.ArchivedMessage
	err := query.
		Order("timestamp DESC").
		Limit(limit).
		Offset(filter.Offset).
		Find(&results).Error
	if err != nil {
		return nil, 0, err
	}

	return results, total, nil
}

// GetArchivedMessage looks up a single archived message by WhatsApp message id.
// instanceID may be empty to search across all instances.
func (m *messageRepository) GetArchivedMessage(instanceID, messageID string) (*message_model.ArchivedMessage, error) {
	query := m.db.Where("message_id = ?", messageID)
	if instanceID != "" {
		query = query.Where("instance_id = ?", instanceID)
	}

	var msg message_model.ArchivedMessage
	if err := query.First(&msg).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}

	return &msg, nil
}

// ListChats returns the most recently active chats for an instance.
func (m *messageRepository) ListChats(instanceID string, limit int) ([]message_model.ChatSummary, error) {
	if limit <= 0 {
		limit = defaultSearchLimit
	}
	if limit > maxSearchLimit {
		limit = maxSearchLimit
	}

	// DISTINCT ON picks the newest row per chat in a single pass; the outer
	// ordering then sorts those winners by recency.
	const q = `
		SELECT chat_jid, push_name, is_group, content AS last_message,
		       from_me AS last_from_me, timestamp, total
		FROM (
			SELECT DISTINCT ON (chat_jid)
			       chat_jid, push_name, is_group, content, from_me, timestamp,
			       COUNT(*) OVER (PARTITION BY chat_jid) AS total
			FROM archived_messages
			WHERE instance_id = ?
			ORDER BY chat_jid, timestamp DESC
		) latest
		ORDER BY timestamp DESC
		LIMIT ?`

	var chats []message_model.ChatSummary
	if err := m.db.Raw(q, instanceID, limit).Scan(&chats).Error; err != nil {
		return nil, err
	}

	return chats, nil
}

// escapeLike neutralizes LIKE wildcards so user input is matched literally.
func escapeLike(s string) string {
	r := strings.NewReplacer("\\", "\\\\", "%", "\\%", "_", "\\_")
	return r.Replace(s)
}
