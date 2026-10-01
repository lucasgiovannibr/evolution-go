package message_repository

import (
	message_model "github.com/evolution-foundation/evolution-go/pkg/message/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type MessageRepository interface {
	InsertMessage(message message_model.Message) error
	GetMessageByID(instanceID, messageID string) (*message_model.Message, error)
}

type messageRepository struct {
	db *gorm.DB
}

func messageUpdateColumns(message message_model.Message) []string {
	updates := []string{"timestamp", "status", "source"}
	if len(message.Referral) > 0 {
		updates = append(updates, "referral")
	}

	return updates
}

func (m *messageRepository) InsertMessage(message message_model.Message) error {
	return m.db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "instance_id"}, {Name: "message_id"}},
		DoUpdates: clause.AssignmentColumns(messageUpdateColumns(message)),
	}).Create(&message).Error
}

// GetMessageByID looks a message up inside one instance: it used to search every
// instance's rows, so any instance token could read the status of another one's messages.
func (m *messageRepository) GetMessageByID(instanceID, messageID string) (*message_model.Message, error) {
	var message message_model.Message
	err := m.db.Where("instance_id = ? AND message_id = ?", instanceID, messageID).First(&message).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}

	return &message, nil
}

func NewMessageRepository(db *gorm.DB) MessageRepository {
	return &messageRepository{db: db}
}
