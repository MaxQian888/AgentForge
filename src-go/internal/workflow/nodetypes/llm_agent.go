package nodetypes

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
)

// LLMAgentHandler implements the "llm_agent" node type. It validates the
// execution has a bound task ID, extracts runtime/provider/model/role/member
// config and emits a single EffectSpawnAgent effect. The applier performs the
// actual Bridge spawn, run-mapping persistence, and any broadcasting.
//
// The handler is pure: it does not call the agent spawner, does not touch
// repositories, and does not broadcast. All side effects are represented as
// effects.
type LLMAgentHandler struct{}

// Execute builds a spawn_agent effect from the node config.
//
// Config fields (all optional except where noted):
//   - runtime   (string): Bridge runtime slug, e.g. "claude_code".
//   - provider  (string): LLM provider, e.g. "anthropic".
//   - model     (string): Model name, e.g. "claude-sonnet-4-6".
//   - roleId    (string): role to bind to the spawned run.
//   - memberId  (string UUID): team member to attribute the run to;
//     malformed or missing values fall back to uuid.Nil (preserving the
//     current service-layer behaviour).
//   - budgetUsd (number): per-run budget cap; defaults to 5.0 when missing,
//     zero, or negative.
//
// The execution must have a non-nil TaskID; otherwise Execute returns the
// error `"llm_agent requires a task ID in the execution context"` to mirror
// the service implementation.
func (LLMAgentHandler) Execute(_ context.Context, req *NodeExecRequest) (*NodeExecResult, error) {
	if req == nil || req.Execution == nil {
		return nil, fmt.Errorf("llm_agent requires execution context")
	}
	if req.Execution.TaskID == nil {
		return nil, fmt.Errorf("llm_agent requires a task ID in the execution context")
	}

	runtime, _ := req.Config["runtime"].(string)
	provider, _ := req.Config["provider"].(string)
	modelName, _ := req.Config["model"].(string)
	roleID, _ := req.Config["roleId"].(string)

	budgetUsd := 5.0
	if b, ok := req.Config["budgetUsd"].(float64); ok && b > 0 {
		budgetUsd = b
	}

	memberID := uuid.Nil
	if mid, ok := req.Config["memberId"].(string); ok && mid != "" {
		if parsed, err := uuid.Parse(mid); err == nil {
			memberID = parsed
		}
	}

	// Effective employee id resolution (change bridge-employee-attribution-legacy):
	// precedence is node-config override > run-level acting_employee_id > null.
	// An explicit node-config override always wins, even if the run record
	// carries a different acting employee. When the node config is absent or
	// malformed, fall back to the run's run-level default.
	employeeID := ""
	if eid, ok := req.Config["employeeId"].(string); ok && eid != "" {
		if _, err := uuid.Parse(eid); err == nil {
			employeeID = eid
		}
	}
	if employeeID == "" && req.Execution != nil && req.Execution.ActingEmployeeID != nil {
		employeeID = req.Execution.ActingEmployeeID.String()
	}

	payload, _ := json.Marshal(SpawnAgentPayload{
		Runtime:    runtime,
		Provider:   provider,
		Model:      modelName,
		RoleID:     roleID,
		MemberID:   memberID.String(),
		EmployeeID: employeeID,
		BudgetUsd:  budgetUsd,
	})
	return &NodeExecResult{
		Effects: []Effect{{Kind: EffectSpawnAgent, Payload: payload}},
	}, nil
}

// ConfigSchema describes the llm_agent node configuration.
func (LLMAgentHandler) ConfigSchema() json.RawMessage {
	return json.RawMessage(`{
  "type": "object",
  "properties": {
    "runtime":   {"type": "string"},
    "provider":  {"type": "string"},
    "model":     {"type": "string"},
    "roleId":    {"type": "string"},
    "memberId":  {"type": "string", "format": "uuid"},
    "employeeId": {"type": "string", "format": "uuid"},
    "budgetUsd": {"type": "number", "minimum": 0}
  }
}`)
}

// Capabilities declares the spawn_agent effect this handler emits.
func (LLMAgentHandler) Capabilities() []EffectKind {
	return []EffectKind{EffectSpawnAgent}
}

// AgentDispatchHandler is a semantic alias for LLMAgentHandler — the existing
// DAG service registers the same logic under two node-type names
// ("llm_agent" and "agent_dispatch"). Keeping the alias as a thin delegating
// type avoids divergence while preserving the two distinct registry entries
// Task 15 will wire up.
type AgentDispatchHandler struct{}

// Execute delegates to LLMAgentHandler.
func (AgentDispatchHandler) Execute(ctx context.Context, req *NodeExecRequest) (*NodeExecResult, error) {
	return LLMAgentHandler{}.Execute(ctx, req)
}

// ConfigSchema delegates to LLMAgentHandler.
func (AgentDispatchHandler) ConfigSchema() json.RawMessage {
	return LLMAgentHandler{}.ConfigSchema()
}

// Capabilities delegates to LLMAgentHandler.
func (AgentDispatchHandler) Capabilities() []EffectKind {
	return LLMAgentHandler{}.Capabilities()
}
