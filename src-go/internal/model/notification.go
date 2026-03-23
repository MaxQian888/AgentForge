package model

import (
	"time"

	"github.com/google/uuid"
)

type Notification struct {
	ID        uuid.UUID `db:"id"`
	TargetID  uuid.UUID `db:"target_id"` // user or member who receives this
	Type      string    `db:"type"`
	Title     string    `db:"title"`
	Body      string    `db:"body"`
	Data      string    `db:"data"` // JSON string with extra context
	IsRead    bool      `db:"is_read"`
	CreatedAt time.Time `db:"created_at"`
}

const (
	NotificationTypeTaskCreated     = "task_created"
	NotificationTypeTaskAssigned    = "task_assigned"
	NotificationTypeAgentStarted    = "agent_started"
	NotificationTypeAgentCompleted  = "agent_completed"
	NotificationTypeAgentFailed     = "agent_failed"
	NotificationTypeReviewCompleted = "review_completed"
	NotificationTypeBudgetWarning   = "budget_warning"
)

type NotificationDTO struct {
	ID        string `json:"id"`
	TargetID  string `json:"targetId"`
	Type      string `json:"type"`
	Title     string `json:"title"`
	Body      string `json:"body"`
	Data      string `json:"data"`
	IsRead    bool   `json:"isRead"`
	CreatedAt string `json:"createdAt"`
}

func (n *Notification) ToDTO() NotificationDTO {
	return NotificationDTO{
		ID:        n.ID.String(),
		TargetID:  n.TargetID.String(),
		Type:      n.Type,
		Title:     n.Title,
		Body:      n.Body,
		Data:      n.Data,
		IsRead:    n.IsRead,
		CreatedAt: n.CreatedAt.Format(time.RFC3339),
	}
}
