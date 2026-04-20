package strategy

// ParsedSpecSchemaVersion is bumped only when the on-disk JSON shape stored in
// qianchuan_strategies.parsed_spec changes incompatibly. Plan 3D's runtime
// inspects this before consuming a row.
const ParsedSpecSchemaVersion = 1

// ParsedSpec is the runtime-optimized form persisted to
// qianchuan_strategies.parsed_spec. Field order in the struct determines the
// JSON key order encoding/json emits, which keeps stored payloads stable.
type ParsedSpec struct {
	SchemaVersion   int           `json:"schema_version"`
	ScheduleSeconds int           `json:"schedule_seconds"`
	Inputs          []ParsedInput `json:"inputs"`
	Rules           []ParsedRule  `json:"rules"`
}

// ParsedInput is the compiled input declaration; window durations from the
// YAML are pre-resolved to seconds.
type ParsedInput struct {
	Metric        string   `json:"metric"`
	Dimensions    []string `json:"dimensions"`
	WindowSeconds int      `json:"window_seconds"`
}

// ParsedRule is a single rule in compiled form. The condition expression is
// stored raw because nodetypes.EvaluateExpression is fast and stateless;
// precomputing an AST gains nothing here.
type ParsedRule struct {
	Name         string         `json:"name"`
	ConditionRaw string         `json:"condition_raw"`
	Actions      []ParsedAction `json:"actions"`
}

// ParsedAction lifts the target's ad_id_expr to a top-level field for cheaper
// runtime access; everything else lives in Params.
type ParsedAction struct {
	Type     string         `json:"type"`
	AdIDExpr string         `json:"ad_id_expr,omitempty"`
	Params   map[string]any `json:"params"`
}
