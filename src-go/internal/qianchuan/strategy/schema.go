// Package strategy implements the declarative Qianchuan strategy schema:
// YAML manifests authored by users, parsed and validated into an in-memory
// Strategy form, and compiled into a runtime-friendly ParsedSpec persisted
// alongside the YAML in the qianchuan_strategies table.
//
// The action allowlist (ActionTypes) is the contract Plan 3D's runtime
// consumes — adding or removing entries is a cross-plan change.
package strategy

// Strategy is the in-memory form of a strategy YAML manifest.
type Strategy struct {
	Name        string           `yaml:"name"`
	Description string           `yaml:"description,omitempty"`
	Triggers    StrategyTriggers `yaml:"triggers"`
	Inputs      []StrategyInput  `yaml:"inputs"`
	Rules       []StrategyRule   `yaml:"rules"`
}

// StrategyTriggers carries scheduling configuration. Schedule must parse via
// time.ParseDuration and fall in [10s, 1h].
type StrategyTriggers struct {
	Schedule string `yaml:"schedule"`
}

// StrategyInput declares the metric inputs a strategy consumes. The runtime
// (Plan 3A/3B) materializes these into snapshot.metrics.* fields.
type StrategyInput struct {
	Metric     string   `yaml:"metric"`
	Dimensions []string `yaml:"dimensions"`
	Window     string   `yaml:"window"`
}

// StrategyRule is a single condition+actions pair evaluated each tick.
type StrategyRule struct {
	Name      string           `yaml:"name"`
	Condition string           `yaml:"condition"`
	Actions   []StrategyAction `yaml:"actions"`
}

// StrategyAction is a single side effect emitted when the parent rule fires.
// Type must be one of ActionTypes; params shape is enforced per-type by
// ValidateAction.
type StrategyAction struct {
	Type   string         `yaml:"type"`
	Target StrategyTarget `yaml:"target"`
	Params map[string]any `yaml:"params"`
}

// StrategyTarget identifies the entity an action operates on. v1 only
// supports targeting a single ad via expression resolution at runtime.
type StrategyTarget struct {
	AdIDExpr string `yaml:"ad_id_expr,omitempty"`
}

// ActionTypes is the load-bearing allowlist. Plan 3D's runtime ships against
// it. Order is preserved for deterministic test assertions.
var ActionTypes = []string{
	"adjust_bid",
	"adjust_budget",
	"pause_ad",
	"resume_ad",
	"apply_material",
	"notify_im",
	"record_event",
}

// IsValidActionType reports whether t is in the action allowlist.
func IsValidActionType(t string) bool {
	for _, a := range ActionTypes {
		if a == t {
			return true
		}
	}
	return false
}
