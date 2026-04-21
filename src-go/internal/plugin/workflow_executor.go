package plugin

import (
	"context"

	"github.com/agentforge/server/internal/model"
)

// WorkflowPlan is the resolved workflow definition passed to an executor.
type WorkflowPlan struct {
	PluginID string
	Spec     *model.WorkflowPluginSpec
	Input    map[string]any
}

// WorkflowEvent is emitted by executors during execution. The Type field
// names a coarse phase: "step_started" | "step_completed" | "step_failed" |
// "completed" | "failed". StepID identifies the step the event refers to
// (empty for terminal "completed"/"failed" events). Payload carries
// arbitrary structured data; Err is set on failure events.
type WorkflowEvent struct {
	Type    string
	StepID  string
	Payload map[string]any
	Err     error
}

// WorkflowExecutor orchestrates a workflow according to its process mode.
// Each mode (sequential, hierarchical, event-driven) has its own
// implementation; WorkflowExecutionService routes to the right one via
// Spec.Process. Execute returns a channel of events that closes when the
// workflow terminates, so callers can stream progress without polling.
type WorkflowExecutor interface {
	// Mode returns the WorkflowProcessMode this executor handles.
	Mode() model.WorkflowProcessMode

	// Execute starts the workflow and streams events. The returned channel
	// is closed when the workflow terminates (either successfully via
	// "completed" or with a final "failed" event). Callers must drain it.
	Execute(ctx context.Context, plan WorkflowPlan) (<-chan WorkflowEvent, error)

	// Cancel requests cancellation of a running workflow instance.
	// For executors that already honor ctx cancellation this can be a no-op.
	Cancel(ctx context.Context, instanceID string) error
}
