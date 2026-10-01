package message_model

import (
	"encoding/json"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Message struct {
	Id string `json:"id" gorm:"type:uuid;primaryKey"`
	// InstanceID owns the row. WhatsApp message ids are not unique across accounts (two
	// instances in the same group receive the same message), so the key is the pair.
	// Rows written before this column existed have it empty.
	InstanceID string          `json:"instance_id" gorm:"type:text;uniqueIndex:idx_messages_instance_message,priority:1"`
	MessageID  string          `json:"message_id" gorm:"uniqueIndex:idx_messages_instance_message,priority:2"`
	Timestamp  string          `json:"timestamp"`
	Status     string          `json:"status"`
	Source     string          `json:"source"`
	Referral   json.RawMessage `json:"referral,omitempty" gorm:"type:jsonb"`
}

func (m *Message) BeforeCreate(tx *gorm.DB) (err error) {
	m.Id = uuid.New().String()
	return
}
