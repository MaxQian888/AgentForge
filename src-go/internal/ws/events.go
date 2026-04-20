// Package ws provides a WebSocket hub for real-time event broadcasting.
package ws

import (
	"encoding/json"

	"github.com/react-go-quick-starter/server/internal/model"
)

// Event types broadcast to connected clients.
const (
	EventTaskCreated                 = "task.created"
	EventTaskUpdated                 = "task.updated"
	EventTaskTransitioned            = "task.transitioned"
	EventTaskAssigned                = "task.assigned"
	EventTaskDispatchBlocked         = "task.dispatch_blocked"
	EventTaskDeleted                 = "task.deleted"
	EventAgentStarted                = "agent.started"
	EventAgentQueued                 = "agent.queued"
	EventAgentQueueCancelled         = "agent.queue.cancelled"
	EventAgentQueuePromoted          = "agent.queue.promoted"
	EventAgentQueueFailed            = "agent.queue.failed"
	EventAgentPoolUpdated            = "agent.pool.updated"
	EventAgentProgress               = "agent.progress"
	EventAgentCompleted              = "agent.completed"
	EventAgentFailed                 = "agent.failed"
	EventAgentOutput                 = "agent.output"
	EventAgentCostUpdate             = "agent.cost_update"
	EventReviewCreated               = "review.created"
	EventReviewCompleted             = "review.completed"
	EventReviewUpdated               = "review.updated"
	EventReviewPendingHuman          = "review.pending_human"
	EventReviewFixRequested          = "review.fix_requested"
	EventNotification                = "notification"
	EventBudgetWarning               = "budget.warning"
	EventBudgetExceeded              = "budget.exceeded"
	EventTaskProgressUpdated         = "task.progress.updated"
	EventTaskProgressAlerted         = "task.progress.alerted"
	EventTaskProgressRecovered       = "task.progress.recovered"
	EventSprintCreated               = "sprint.created"
	EventSprintUpdated               = "sprint.updated"
	EventSprintTransitioned          = "sprint.transitioned"
	EventTaskDependencyResolved      = "task.dependency_resolved"
	EventWorkflowTriggerFired        = "workflow.trigger_fired"
	EventTeamCreated                 = "team.created"
	EventTeamPlanning                = "team.planning"
	EventTeamExecuting               = "team.executing"
	EventTeamReviewing               = "team.reviewing"
	EventTeamCompleted               = "team.completed"
	EventTeamFailed                  = "team.failed"
	EventTeamCancelled               = "team.cancelled"
	EventTeamCostUpdate              = "team.cost_update"
	EventTeamArtifactCreated         = "team.artifact.created"
	EventPluginLifecycle             = "plugin.lifecycle"
	EventSchedulerJobUpdated         = "scheduler.job.updated"
	EventSchedulerRunStarted         = "scheduler.run.started"
	EventSchedulerRunCancelRequested = "scheduler.run.cancel_requested"
	EventSchedulerRunCompleted       = "scheduler.run.completed"
	EventLogCreated                  = "log.created"
	EventWikiPageCreated             = "wiki.page.created"
	EventWikiPageUpdated             = "wiki.page.updated"
	EventWikiPageMoved               = "wiki.page.moved"
	EventWikiPageDeleted             = "wiki.page.deleted"
	EventWikiCommentCreated          = "wiki.comment.created"
	EventWikiCommentResolved         = "wiki.comment.resolved"
	EventWikiVersionPublished        = "wiki.version.published"
	EventLinkCreated                 = "link.created"
	EventLinkDeleted                 = "link.deleted"
	EventTaskCommentCreated          = "task_comment.created"
	EventTaskCommentResolved         = "task_comment.resolved"
	EventAgentPermissionRequest      = "agent.permission_request"
	EventAgentToolCall               = "agent.tool_call"
	EventAgentToolResult             = "agent.tool_result"
	EventAgentReasoning              = "agent.reasoning"
	EventAgentFileChange             = "agent.file_change"
	EventAgentTodoUpdate             = "agent.todo_update"
	EventAgentRateLimit              = "agent.rate_limit"
	EventAgentPartialMessage         = "agent.partial_message"
	EventAgentSnapshot               = "agent.snapshot"

	// Knowledge asset events (replaces wiki.page.* and knowledge.comment.*)
	EventKnowledgeAssetCreated        = "knowledge.asset.created"
	EventKnowledgeAssetUpdated        = "knowledge.asset.updated"
	EventKnowledgeAssetMoved          = "knowledge.asset.moved"
	EventKnowledgeAssetDeleted        = "knowledge.asset.deleted"
	EventKnowledgeAssetContentChanged = "knowledge.asset.content_changed"
	EventKnowledgeCommentCreated      = "knowledge.comment.created"
	EventKnowledgeCommentResolved     = "knowledge.comment.resolved"
	EventKnowledgeCommentDeleted      = "knowledge.comment.deleted"
	EventKnowledgeIngestStatusChanged = "knowledge.ingest.status_changed"

	// Workflow DAG execution events
	EventWorkflowExecutionStarted   = "workflow.execution.started"
	EventWorkflowExecutionAdvanced  = "workflow.execution.advanced"
	EventWorkflowExecutionCompleted = "workflow.execution.completed"
	EventWorkflowExecutionPaused    = "workflow.execution.paused"
	EventWorkflowNodeCompleted      = "workflow.node.completed"
	EventWorkflowNodeWaiting        = "workflow.node.waiting"
	EventWorkflowReviewRequested    = "workflow.review.requested"
	EventWorkflowReviewResolved     = "workflow.review.resolved"

	// Emitted by outbound_dispatcher after retry exhaustion when it could
	// not deliver the default reply card to the IM Bridge. Payload:
	//   {executionId: string, lastError: string, attempts: int}
	EventOutboundDeliveryFailed = "workflow.outbound_delivery.failed"

	// Unified cross-engine workflow-run events (bridge-unified-run-view).
	// Emitted alongside the engine-native channels for the workflow workspace
	// UI. Payload is service.UnifiedRunRow — keyed on the shared envelope.
	EventWorkflowRunStatusChanged = "workflow.run.status_changed"
	EventWorkflowRunTerminal      = "workflow.run.terminal"

	// VCS outbound events (spec2 §5 S2-B). Forwarded to WS clients so the
	// frontend can surface delivery-failed badges.
	EventVCSDeliveryFailed = "vcs.delivery.failed"
	EventVCSAuthExpired    = "vcs.auth.expired"
)

// Event types pushed from the TS bridge into Go orchestration.
const (
	BridgeEventOutput            = "output"
	BridgeEventToolCall          = "tool_call"
	BridgeEventToolResult        = "tool_result"
	BridgeEventStatusChange      = "status_change"
	BridgeEventCostUpdate        = "cost_update"
	BridgeEventBudgetAlert       = "budget_alert"
	BridgeEventError             = "error"
	BridgeEventSnapshot          = "snapshot"
	BridgeEventReasoning         = "reasoning"
	BridgeEventFileChange        = "file_change"
	BridgeEventTodoUpdate        = "todo_update"
	BridgeEventProgress          = "progress"
	BridgeEventRateLimit         = "rate_limit"
	BridgeEventPartialMessage    = "partial_message"
	BridgeEventPermissionRequest = "permission_request"
	BridgeEventToolStatusChange  = "tool.status_change"
	BridgeEventToolCallLog       = "tool.call_log"
)

// BridgeAgentEvent is the canonical runtime event envelope emitted by the TS bridge.
type BridgeAgentEvent struct {
	TaskID      string          `json:"task_id"`
	SessionID   string          `json:"session_id"`
	TimestampMS int64           `json:"timestamp_ms"`
	Type        string          `json:"type"`
	Data        json.RawMessage `json:"data"`
}

type BridgeEventStatusChangeData struct {
	OldStatus        string          `json:"old_status"`
	NewStatus        string          `json:"new_status"`
	Reason           string          `json:"reason,omitempty"`
	StructuredOutput json.RawMessage `json:"structured_output,omitempty"`
}

type BridgeEventCostUpdateData struct {
	InputTokens         int64                         `json:"input_tokens"`
	OutputTokens        int64                         `json:"output_tokens"`
	CacheReadTokens     int64                         `json:"cache_read_tokens"`
	CacheCreationTokens int64                         `json:"cache_creation_tokens"`
	CostUSD             float64                       `json:"cost_usd"`
	BudgetRemainingUSD  float64                       `json:"budget_remaining_usd"`
	TurnNumber          int                           `json:"turn_number,omitempty"`
	CostAccounting      *model.CostAccountingSnapshot `json:"cost_accounting,omitempty"`
}

type BridgeEventBudgetAlertData struct {
	CostUSD            float64 `json:"cost_usd"`
	BudgetRemainingUSD float64 `json:"budget_remaining_usd"`
	ThresholdRatio     float64 `json:"threshold_ratio"`
	ThresholdPercent   int     `json:"threshold_percent"`
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

type BridgeEventPermissionRequestData struct {
	RequestID       string `json:"request_id"`
	ToolName        string `json:"tool_name,omitempty"`
	Context         any    `json:"context,omitempty"`
	ElicitationType string `json:"elicitation_type,omitempty"`
	Fields          []any  `json:"fields,omitempty"`
	MCPServerID     string `json:"mcp_server_id,omitempty"`
}

type BridgeEventToolCallData struct {
	ToolName   string `json:"tool_name"`
	ToolCallID string `json:"tool_call_id,omitempty"`
	Input      any    `json:"input,omitempty"`
	TurnNumber int    `json:"turn_number,omitempty"`
}

type BridgeEventToolResultData struct {
	ToolName   string `json:"tool_name"`
	ToolCallID string `json:"tool_call_id,omitempty"`
	Output     any    `json:"output,omitempty"`
	IsError    bool   `json:"is_error,omitempty"`
	TurnNumber int    `json:"turn_number,omitempty"`
}

type BridgeEventFileChangeData struct {
	Files []BridgeFileChangeEntry `json:"files"`
}

type BridgeFileChangeEntry struct {
	Path       string `json:"path"`
	ChangeType string `json:"change_type,omitempty"`
}

type BridgeEventReasoningData struct {
	Content string `json:"content"`
}

type BridgeEventProgressData struct {
	ToolName      string `json:"tool_name,omitempty"`
	ProgressText  string `json:"progress_text,omitempty"`
	ItemType      string `json:"item_type,omitempty"`
	PartialOutput any    `json:"partial_output,omitempty"`
}

type BridgeEventRateLimitData struct {
	Utilization float64 `json:"utilization,omitempty"`
	ResetAt     any     `json:"reset_at,omitempty"`
}

type BridgeEventPartialMessageData struct {
	Content    string `json:"content"`
	IsComplete bool   `json:"is_complete"`
}

type BridgeEventTodoUpdateData struct {
	Todos []BridgeTodoEntry `json:"todos"`
}

type BridgeTodoEntry struct {
	ID      string `json:"id,omitempty"`
	Content string `json:"content,omitempty"`
	Status  string `json:"status,omitempty"`
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
