package model

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

const (
	AutomationEventTaskStatusChanged      = "task.status_changed"
	AutomationEventTaskAssigneeChanged    = "task.assignee_changed"
	AutomationEventTaskDueDateApproach    = "task.due_date_approaching"
	AutomationEventTaskFieldChanged       = "task.field_changed"
	AutomationEventReviewCompleted        = "review.completed"
	AutomationEventBudgetThresholdReached = "budget.threshold_reached"
)

const (
	AutomationLogStatusSuccess = "success"
	AutomationLogStatusFailed  = "failed"
	AutomationLogStatusSkipped = "skipped"
)

type AutomationRule struct {
	ID         uuid.UUID  `db:"id"`
	ProjectID  uuid.UUID  `db:"project_id"`
	Name       string     `db:"name"`
	Enabled    bool       `db:"enabled"`
	EventType  string     `db:"event_type"`
	Conditions string     `db:"conditions"`
	Actions    string     `db:"actions"`
	CreatedBy  uuid.UUID  `db:"created_by"`
	CreatedAt  time.Time  `db:"created_at"`
	UpdatedAt  time.Time  `db:"updated_at"`
	DeletedAt  *time.Time `db:"deleted_at"`
}

type AutomationLog struct {
	ID          uuid.UUID  `db:"id"`
	RuleID      uuid.UUID  `db:"rule_id"`
	TaskID      *uuid.UUID `db:"task_id"`
	EventType   string     `db:"event_type"`
	TriggeredAt time.Time  `db:"triggered_at"`
	Status      string     `db:"status"`
	Detail      string     `db:"detail"`
}

type AutomationLogListQuery struct {
	EventType string
	Status    string
	Page      int
	Limit     int
}

type AutomationRuleDTO struct {
	ID         string          `json:"id"`
	ProjectID  string          `json:"projectId"`
	Name       string          `json:"name"`
	Enabled    bool            `json:"enabled"`
	EventType  string          `json:"eventType"`
	Conditions json.RawMessage `json:"conditions"`
	Actions    json.RawMessage `json:"actions"`
	CreatedBy  string          `json:"createdBy"`
	CreatedAt  string          `json:"createdAt"`
	UpdatedAt  string          `json:"updatedAt"`
	DeletedAt  *string         `json:"deletedAt,omitempty"`
}

type AutomationLogDTO struct {
	ID          string          `json:"id"`
	RuleID      string          `json:"ruleId"`
	TaskID      *string         `json:"taskId,omitempty"`
	EventType   string          `json:"eventType"`
	TriggeredAt string          `json:"triggeredAt"`
	Status      string          `json:"status"`
	Detail      json.RawMessage `json:"detail"`
}

type CreateAutomationRuleRequest struct {
	Name       string          `json:"name" validate:"required,min=1,max=120"`
	Enabled    *bool           `json:"enabled"`
	EventType  string          `json:"eventType" validate:"required"`
	Conditions json.RawMessage `json:"conditions"`
	Actions    json.RawMessage `json:"actions"`
}

type UpdateAutomationRuleRequest struct {
	Name       *string         `json:"name"`
	Enabled    *bool           `json:"enabled"`
	EventType  *string         `json:"eventType"`
	Conditions json.RawMessage `json:"conditions"`
	Actions    json.RawMessage `json:"actions"`
}

func (r *AutomationRule) ToDTO() AutomationRuleDTO {
	dto := AutomationRuleDTO{
		ID:         r.ID.String(),
		ProjectID:  r.ProjectID.String(),
		Name:       r.Name,
		Enabled:    r.Enabled,
		EventType:  r.EventType,
		Conditions: normalizeJSONRawMessage(r.Conditions, []byte("[]")),
		Actions:    normalizeJSONRawMessage(r.Actions, []byte("[]")),
		CreatedBy:  r.CreatedBy.String(),
		CreatedAt:  r.CreatedAt.Format(time.RFC3339),
		UpdatedAt:  r.UpdatedAt.Format(time.RFC3339),
	}
	if r.DeletedAt != nil {
		value := r.DeletedAt.Format(time.RFC3339)
		dto.DeletedAt = &value
	}
	return dto
}

func (l *AutomationLog) ToDTO() AutomationLogDTO {
	dto := AutomationLogDTO{
		ID:          l.ID.String(),
		RuleID:      l.RuleID.String(),
		EventType:   l.EventType,
		TriggeredAt: l.TriggeredAt.Format(time.RFC3339),
		Status:      l.Status,
		Detail:      normalizeJSONRawMessage(l.Detail, []byte("{}")),
	}
	if l.TaskID != nil {
		value := l.TaskID.String()
		dto.TaskID = &value
	}
	return dto
}
