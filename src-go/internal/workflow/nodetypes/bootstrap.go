package nodetypes

import (
	"fmt"
)

// BuiltinDeps bundles runtime dependencies required by dep-bearing built-in
// handlers. Fields may be nil — only ConditionHandler and LoopHandler consume
// them today. LoopHandler requires DefRepo to be non-nil at runtime (it
// returns an error from Execute if nil), so callers that register loop nodes
// must supply it.
type BuiltinDeps struct {
	TaskRepo ConditionTaskResolver // optional — enables task.status in condition expressions
	DefRepo  LoopDefResolver       // required for loop nodes to compute reset topology
}

// RegisterBuiltins registers the 14 built-in node-type handlers into r.
//
// Returns the first error encountered. Callers typically invoke r.LockGlobal()
// afterward to prevent further built-in registration in the running process.
func RegisterBuiltins(r *NodeTypeRegistry, deps BuiltinDeps) error {
	entries := []struct {
		name    string
		handler NodeTypeHandler
	}{
		{"trigger", TriggerHandler{}},
		{"gate", GateHandler{}},
		{"parallel_split", ParallelSplitHandler{}},
		{"parallel_join", ParallelJoinHandler{}},
		{"sub_workflow", SubWorkflowHandler{}},
		{"function", FunctionHandler{}},
		{"condition", ConditionHandler{TaskRepo: deps.TaskRepo}},
		{"notification", NotificationHandler{}},
		{"status_transition", StatusTransitionHandler{}},
		{"loop", LoopHandler{DefRepo: deps.DefRepo}},
		{"llm_agent", LLMAgentHandler{}},
		{"agent_dispatch", AgentDispatchHandler{}},
		{"human_review", HumanReviewHandler{}},
		{"wait_event", WaitEventHandler{}},
	}

	for _, e := range entries {
		if err := r.RegisterBuiltin(e.name, e.handler); err != nil {
			return fmt.Errorf("register builtin %q: %w", e.name, err)
		}
	}
	return nil
}
