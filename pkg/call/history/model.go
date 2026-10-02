// Package call_history keeps a record of every call the call engine followed: who, when
// and how it ended. Never audio or any content of a call.
//
// It is opt-in (CALL_HISTORY) and expires on its own (CALL_HISTORY_RETENTION_DAYS): the
// peer's number is personal data, and a table that only grows is how data ends up kept
// for years by accident.
package call_history

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Record is one call. It is the row of the call_records table and, as is, one item of the
// answer of GET /call/history.
type Record struct {
	Id string `json:"id" gorm:"type:uuid;primaryKey"`

	// InstanceID and CallID identify the call: the same call can be seen by two
	// instances (a group call), so the pair is the key.
	InstanceID string `json:"-" gorm:"type:text;not null;uniqueIndex:idx_call_records_instance_call,priority:1;index:idx_call_records_instance_started,priority:1"`
	CallID     string `json:"callId" gorm:"type:text;not null;uniqueIndex:idx_call_records_instance_call,priority:2"`

	// Peer is the other side as WhatsApp reports it, often a @lid; PeerPhone is its phone
	// number (digits only) when the account could resolve it.
	Peer      string `json:"peer" gorm:"type:text"`
	PeerPhone string `json:"peerPhone,omitempty" gorm:"type:text"`

	Direction string `json:"direction" gorm:"type:text"` // incoming | outgoing
	Video     bool   `json:"video"`
	Outcome   string `json:"outcome" gorm:"type:text"` // answered, missed, rejected, cancelled, unanswered, busy, failed
	Reason    string `json:"reason" gorm:"type:text"`  // the technical reason the call ended

	StartedAt  time.Time  `json:"startedAt" gorm:"not null;index:idx_call_records_instance_started,priority:2,sort:desc;index:idx_call_records_started"`
	AnsweredAt *time.Time `json:"answeredAt,omitempty"`
	EndedAt    time.Time  `json:"endedAt"`

	// TalkSeconds is how long the conversation lasted, RingSeconds how long it rang.
	TalkSeconds int `json:"talkSeconds"`
	RingSeconds int `json:"ringSeconds"`
}

func (Record) TableName() string { return "call_records" }

func (r *Record) BeforeCreate(tx *gorm.DB) error {
	if r.Id == "" {
		r.Id = uuid.New().String()
	}
	return nil
}
