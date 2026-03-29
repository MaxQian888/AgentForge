package model

import (
	"time"

	"github.com/google/uuid"
)

type DispatchAttempt struct {
	ID             uuid.UUID  `db:"id"`
	ProjectID      uuid.UUID  `db:"project_id"`
	TaskID         uuid.UUID  `db:"task_id"`
	MemberID       *uuid.UUID `db:"member_id"`
	Outcome        string     `db:"outcome"`
	TriggerSource  string     `db:"trigger_source"`
	Reason         string     `db:"reason"`
	GuardrailType  string     `db:"guardrail_type"`
	GuardrailScope string     `db:"guardrail_scope"`
	CreatedAt      time.Time  `db:"created_at"`
}

type DispatchAttemptDTO struct {
	ID             string  `json:"id"`
	ProjectID      string  `json:"projectId"`
	TaskID         string  `json:"taskId"`
	MemberID       *string `json:"memberId,omitempty"`
	Outcome        string  `json:"outcome"`
	TriggerSource  string  `json:"triggerSource"`
	Reason         string  `json:"reason,omitempty"`
	GuardrailType  string  `json:"guardrailType,omitempty"`
	GuardrailScope string  `json:"guardrailScope,omitempty"`
	CreatedAt      string  `json:"createdAt"`
}

func (a *DispatchAttempt) ToDTO() DispatchAttemptDTO {
	dto := DispatchAttemptDTO{
		ID:             a.ID.String(),
		ProjectID:      a.ProjectID.String(),
		TaskID:         a.TaskID.String(),
		Outcome:        a.Outcome,
		TriggerSource:  a.TriggerSource,
		Reason:         a.Reason,
		GuardrailType:  a.GuardrailType,
		GuardrailScope: a.GuardrailScope,
		CreatedAt:      a.CreatedAt.Format(time.RFC3339),
	}
	if a.MemberID != nil {
		memberID := a.MemberID.String()
		dto.MemberID = &memberID
	}
	return dto
}
