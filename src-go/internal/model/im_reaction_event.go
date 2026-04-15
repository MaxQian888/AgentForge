package model

import (
	"time"

	"github.com/google/uuid"
)

const (
	IMReactionEventTypeCreated = "created"
	IMReactionEventTypeDeleted = "deleted"
)

type IMReactionEvent struct {
	ID         uuid.UUID `json:"id"`
	Platform   string    `json:"platform"`
	ChatID     string    `json:"chatId"`
	MessageID  string    `json:"messageId"`
	UserID     string    `json:"userId"`
	Emoji      string    `json:"emoji"`
	EventType  string    `json:"eventType"`
	RawPayload []byte    `json:"-"`
	CreatedAt  time.Time `json:"createdAt"`
}
