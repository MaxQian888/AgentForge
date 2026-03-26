// Package ws provides a WebSocket hub for real-time event broadcasting.
package ws

import "encoding/json"

// Event types broadcast to connected clients.
const (
	EventTaskCreated            = "task.created"
	EventTaskUpdated            = "task.updated"
	EventTaskTransitioned       = "task.transitioned"
	EventTaskAssigned           = "task.assigned"
	EventTaskDispatchBlocked    = "task.dispatch_blocked"
	EventTaskDeleted            = "task.deleted"
	EventAgentStarted           = "agent.started"
	EventAgentQueued            = "agent.queued"
	EventAgentQueuePromoted     = "agent.queue.promoted"
	EventAgentQueueFailed       = "agent.queue.failed"
	EventAgentPoolUpdated       = "agent.pool.updated"
	EventAgentProgress          = "agent.progress"
	EventAgentCompleted         = "agent.completed"
	EventAgentFailed            = "agent.failed"
	EventAgentOutput            = "agent.output"
	EventAgentCostUpdate        = "agent.cost_update"
	EventReviewCreated          = "review.created"
	EventReviewCompleted        = "review.completed"
	EventReviewPendingHuman     = "review.pending_human"
	EventReviewFixRequested     = "review.fix_requested"
	EventNotification           = "notification"
	EventBudgetWarning          = "budget.warning"
	EventBudgetExceeded         = "budget.exceeded"
	EventTaskProgressUpdated    = "task.progress.updated"
	EventTaskProgressAlerted    = "task.progress.alerted"
	EventTaskProgressRecovered  = "task.progress.recovered"
	EventSprintCreated          = "sprint.created"
	EventSprintUpdated          = "sprint.updated"
	EventSprintTransitioned     = "sprint.transitioned"
	EventTaskDependencyResolved = "task.dependency_resolved"
	EventWorkflowTriggerFired   = "workflow.trigger_fired"
	EventTeamCreated            = "team.created"
	EventTeamPlanning           = "team.planning"
	EventTeamExecuting          = "team.executing"
	EventTeamReviewing          = "team.reviewing"
	EventTeamCompleted          = "team.completed"
	EventTeamFailed             = "team.failed"
	EventTeamCancelled          = "team.cancelled"
	EventTeamCostUpdate         = "team.cost_update"
	EventPluginLifecycle        = "plugin.lifecycle"
	EventSchedulerJobUpdated    = "scheduler.job.updated"
	EventSchedulerRunStarted    = "scheduler.run.started"
	EventSchedulerRunCompleted  = "scheduler.run.completed"
	EventWikiPageCreated        = "wiki.page.created"
	EventWikiPageUpdated        = "wiki.page.updated"
	EventWikiPageMoved          = "wiki.page.moved"
	EventWikiPageDeleted        = "wiki.page.deleted"
	EventWikiCommentCreated     = "wiki.comment.created"
	EventWikiCommentResolved    = "wiki.comment.resolved"
	EventWikiVersionPublished   = "wiki.version.published"
	EventLinkCreated            = "link.created"
	EventLinkDeleted            = "link.deleted"
	EventTaskCommentCreated     = "task_comment.created"
	EventTaskCommentResolved    = "task_comment.resolved"
)

// Event types pushed from the TS bridge into Go orchestration.
const (
	BridgeEventOutput       = "output"
	BridgeEventToolCall     = "tool_call"
	BridgeEventToolResult   = "tool_result"
	BridgeEventStatusChange = "status_change"
	BridgeEventCostUpdate   = "cost_update"
	BridgeEventError        = "error"
	BridgeEventSnapshot     = "snapshot"
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

// BridgeAgentEvent is the canonical runtime event envelope emitted by the TS bridge.
type BridgeAgentEvent struct {
	TaskID      string          `json:"task_id"`
	SessionID   string          `json:"session_id"`
	TimestampMS int64           `json:"timestamp_ms"`
	Type        string          `json:"type"`
	Data        json.RawMessage `json:"data"`
}

type BridgeEventStatusChangeData struct {
	OldStatus string `json:"old_status"`
	NewStatus string `json:"new_status"`
	Reason    string `json:"reason,omitempty"`
}

type BridgeEventCostUpdateData struct {
	InputTokens        int64   `json:"input_tokens"`
	OutputTokens       int64   `json:"output_tokens"`
	CacheReadTokens    int64   `json:"cache_read_tokens"`
	CostUSD            float64 `json:"cost_usd"`
	BudgetRemainingUSD float64 `json:"budget_remaining_usd"`
	TurnNumber         int     `json:"turn_number,omitempty"`
}

type BridgeEventOutputData struct {
	Content     string `json:"content"`
	ContentType string `json:"content_type,omitempty"`
	TurnNumber  int    `json:"turn_number,omitempty"`
}

type BridgeEventErrorData struct {
	Code      string `json:"code"`
	Message   string `json:"message"`
	Retryable bool   `json:"retryable"`
}

// Backward-compatible aliases used by existing service tests/handlers.
type BridgeStatusChangeData = BridgeEventStatusChangeData
type BridgeCostUpdateData = BridgeEventCostUpdateData
type BridgeOutputData = BridgeEventOutputData

func (e *BridgeAgentEvent) DecodeData(target any) error {
	if e == nil || len(e.Data) == 0 {
		return nil
	}
	return json.Unmarshal(e.Data, target)
}
