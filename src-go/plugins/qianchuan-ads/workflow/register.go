package qcworkflow

import (
	"fmt"

	nodetypes "github.com/agentforge/server/internal/workflow/nodetypes"
)

// Handlers returns the qianchuan node-type handlers in the canonical
// (name, handler) form expected by NodeTypeRegistry. It lets the
// plugin's install function register its contributions without the
// core bootstrap touching plugin-owned types.
func Handlers() []struct {
	Name    string
	Handler nodetypes.NodeTypeHandler
} {
	return []struct {
		Name    string
		Handler nodetypes.NodeTypeHandler
	}{
		{"qianchuan_metrics_fetcher", QianchuanMetricsFetcherHandler{}},
		{"qianchuan_strategy_runner", QianchuanStrategyRunnerHandler{}},
		{"qianchuan_action_executor", QianchuanActionExecutorHandler{}},
	}
}

// RegisterAll registers every plugin-provided node-type handler into
// the shared NodeTypeRegistry. Call before LockGlobal() so the
// qianchuan handlers land in the global scope alongside core builtins.
func RegisterAll(r *nodetypes.NodeTypeRegistry) error {
	for _, entry := range Handlers() {
		if err := r.RegisterBuiltin(entry.Name, entry.Handler); err != nil {
			return fmt.Errorf("register qianchuan handler %q: %w", entry.Name, err)
		}
	}
	return nil
}
