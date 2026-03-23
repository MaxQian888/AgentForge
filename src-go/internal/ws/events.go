// Package ws provides a WebSocket hub for real-time event broadcasting.
package ws

import "encoding/json"

// Event types broadcast to connected clients.
const (
	EventTaskCreated       = "task.created"
	EventTaskUpdated       = "task.updated"
	EventTaskTransitioned  = "task.transitioned"
	EventTaskAssigned      = "task.assigned"
	EventTaskDeleted       = "task.deleted"
	EventAgentStarted      = "agent.started"
	EventAgentProgress     = "agent.progress"
	EventAgentCompleted    = "agent.completed"
	EventAgentFailed       = "agent.failed"
	EventAgentCostUpdate   = "agent.cost_update"
	EventReviewCreated     = "review.created"
	EventReviewCompleted   = "review.completed"
	EventNotification      = "notification"
	EventBudgetWarning     = "budget.warning"
	EventBudgetExceeded    = "budget.exceeded"
)

// Event is a message sent over WebSocket connections.
type Event struct {
	Type      string `json:"type"`
	ProjectID string `json:"projectId,omitempty"`
	Payload   any    `json:"payload"`
}

// JSON serializes the event.
func (e *Event) JSON() []byte {
	data, _ := json.Marshal(e)
	return data
}
