package model

import (
	"time"

	"github.com/google/uuid"
)

const (
	TaskProgressHealthHealthy = "healthy"
	TaskProgressHealthWarning = "warning"
	TaskProgressHealthStalled = "stalled"
)

const (
	TaskProgressSourceTaskCreated    = "task_created"
	TaskProgressSourceTaskUpdated    = "task_updated"
	TaskProgressSourceTaskAssigned   = "task_assigned"
	TaskProgressSourceTaskTransition = "task_transition"
	TaskProgressSourceAgentStarted   = "agent_started"
	TaskProgressSourceAgentHeartbeat = "agent_heartbeat"
	TaskProgressSourceAgentStatus    = "agent_status"
	TaskProgressSourceReviewCreated  = "review_created"
	TaskProgressSourceReviewComplete = "review_complete"
	TaskProgressSourceDetector       = "detector"
)

const (
	TaskProgressReasonNone           = ""
	TaskProgressReasonNoAssignee     = "no_assignee"
	TaskProgressReasonAwaitingReview = "awaiting_review"
	TaskProgressReasonNoRecentUpdate = "no_recent_update"
)

type TaskProgressSnapshot struct {
	TaskID             uuid.UUID  `db:"task_id"`
	LastActivityAt     time.Time  `db:"last_activity_at"`
	LastActivitySource string     `db:"last_activity_source"`
	LastTransitionAt   time.Time  `db:"last_transition_at"`
	HealthStatus       string     `db:"health_status"`
	RiskReason         string     `db:"risk_reason"`
	RiskSinceAt        *time.Time `db:"risk_since_at"`
	LastAlertState     string     `db:"last_alert_state"`
	LastAlertAt        *time.Time `db:"last_alert_at"`
	LastRecoveredAt    *time.Time `db:"last_recovered_at"`
	CreatedAt          time.Time  `db:"created_at"`
	UpdatedAt          time.Time  `db:"updated_at"`
}

type TaskProgressSnapshotDTO struct {
	LastActivityAt     string  `json:"lastActivityAt"`
	LastActivitySource string  `json:"lastActivitySource"`
	LastTransitionAt   string  `json:"lastTransitionAt"`
	HealthStatus       string  `json:"healthStatus"`
	RiskReason         string  `json:"riskReason"`
	RiskSinceAt        *string `json:"riskSinceAt,omitempty"`
	LastAlertState     string  `json:"lastAlertState"`
	LastAlertAt        *string `json:"lastAlertAt,omitempty"`
	LastRecoveredAt    *string `json:"lastRecoveredAt,omitempty"`
}

func (s *TaskProgressSnapshot) ToDTO() *TaskProgressSnapshotDTO {
	if s == nil {
		return nil
	}

	dto := &TaskProgressSnapshotDTO{
		LastActivityAt:     s.LastActivityAt.Format(time.RFC3339),
		LastActivitySource: s.LastActivitySource,
		LastTransitionAt:   s.LastTransitionAt.Format(time.RFC3339),
		HealthStatus:       s.HealthStatus,
		RiskReason:         s.RiskReason,
		LastAlertState:     s.LastAlertState,
	}
	if s.RiskSinceAt != nil {
		value := s.RiskSinceAt.Format(time.RFC3339)
		dto.RiskSinceAt = &value
	}
	if s.LastAlertAt != nil {
		value := s.LastAlertAt.Format(time.RFC3339)
		dto.LastAlertAt = &value
	}
	if s.LastRecoveredAt != nil {
		value := s.LastRecoveredAt.Format(time.RFC3339)
		dto.LastRecoveredAt = &value
	}
	return dto
}
