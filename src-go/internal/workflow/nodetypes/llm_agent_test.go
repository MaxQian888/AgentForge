package nodetypes

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/agentforge/server/internal/model"
	"github.com/google/uuid"
)

func TestLLMAgentHandler_HappyPath(t *testing.T) {
	h := LLMAgentHandler{}
	taskID := uuid.New()
	memberID := uuid.New()
	exec := &model.WorkflowExecution{ID: uuid.New(), TaskID: &taskID}
	node := &model.WorkflowNode{ID: "agent-1"}

	result, err := h.Execute(context.Background(), &NodeExecRequest{
		Execution: exec,
		Node:      node,
		Config: map[string]any{
			"runtime":   "claude_code",
			"provider":  "anthropic",
			"model":     "claude-sonnet-4-6",
			"roleId":    "coder",
			"memberId":  memberID.String(),
			"budgetUsd": 5.0,
		},
	})
	if err != nil {
		t.Fatalf("Execute() returned error: %v", err)
	}
	if result == nil {
		t.Fatal("Execute() returned nil result")
	}
	if result.Result != nil {
		t.Errorf("result.Result = %v, want nil", result.Result)
	}
	if len(result.Effects) != 1 {
		t.Fatalf("expected 1 effect, got %d", len(result.Effects))
	}
	eff := result.Effects[0]
	if eff.Kind != EffectSpawnAgent {
		t.Errorf("effect kind = %s, want %s", eff.Kind, EffectSpawnAgent)
	}

	var payload SpawnAgentPayload
	if err := json.Unmarshal(eff.Payload, &payload); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}
	if payload.Runtime != "claude_code" {
		t.Errorf("Runtime = %q, want claude_code", payload.Runtime)
	}
	if payload.Provider != "anthropic" {
		t.Errorf("Provider = %q, want anthropic", payload.Provider)
	}
	if payload.Model != "claude-sonnet-4-6" {
		t.Errorf("Model = %q, want claude-sonnet-4-6", payload.Model)
	}
	if payload.RoleID != "coder" {
		t.Errorf("RoleID = %q, want coder", payload.RoleID)
	}
	if payload.MemberID != memberID.String() {
		t.Errorf("MemberID = %q, want %q", payload.MemberID, memberID.String())
	}
	if payload.BudgetUsd != 5.0 {
		t.Errorf("BudgetUsd = %v, want 5.0", payload.BudgetUsd)
	}

	caps := h.Capabilities()
	if len(caps) != 1 || caps[0] != EffectSpawnAgent {
		t.Errorf("Capabilities() = %v, want [%s]", caps, EffectSpawnAgent)
	}
	assertCapsCoverEffects(t, h, result.Effects)
}

func TestLLMAgentHandler_NilTaskID_Errors(t *testing.T) {
	h := LLMAgentHandler{}
	exec := &model.WorkflowExecution{ID: uuid.New(), TaskID: nil}
	node := &model.WorkflowNode{ID: "agent-1"}

	_, err := h.Execute(context.Background(), &NodeExecRequest{
		Execution: exec,
		Node:      node,
		Config:    map[string]any{"runtime": "claude_code"},
	})
	if err == nil {
		t.Fatal("expected error when TaskID is nil, got nil")
	}
	want := "llm_agent requires a task ID in the execution context"
	if err.Error() != want {
		t.Errorf("error message = %q, want %q", err.Error(), want)
	}
}

func TestLLMAgentHandler_DefaultBudget(t *testing.T) {
	h := LLMAgentHandler{}
	taskID := uuid.New()
	exec := &model.WorkflowExecution{ID: uuid.New(), TaskID: &taskID}
	node := &model.WorkflowNode{ID: "agent-1"}

	cases := []struct {
		name   string
		config map[string]any
	}{
		{"missing", map[string]any{}},
		{"zero", map[string]any{"budgetUsd": 0.0}},
		{"negative", map[string]any{"budgetUsd": -1.5}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := h.Execute(context.Background(), &NodeExecRequest{
				Execution: exec,
				Node:      node,
				Config:    tc.config,
			})
			if err != nil {
				t.Fatalf("Execute() returned error: %v", err)
			}
			var payload SpawnAgentPayload
			_ = json.Unmarshal(result.Effects[0].Payload, &payload)
			if payload.BudgetUsd != 5.0 {
				t.Errorf("BudgetUsd = %v, want 5.0 default", payload.BudgetUsd)
			}
		})
	}
}

func TestLLMAgentHandler_InvalidMemberID(t *testing.T) {
	h := LLMAgentHandler{}
	taskID := uuid.New()
	exec := &model.WorkflowExecution{ID: uuid.New(), TaskID: &taskID}
	node := &model.WorkflowNode{ID: "agent-1"}

	cases := []struct {
		name   string
		config map[string]any
	}{
		{"missing", map[string]any{}},
		{"not-a-uuid", map[string]any{"memberId": "not-a-uuid"}},
		{"empty", map[string]any{"memberId": ""}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := h.Execute(context.Background(), &NodeExecRequest{
				Execution: exec,
				Node:      node,
				Config:    tc.config,
			})
			if err != nil {
				t.Fatalf("Execute() returned error: %v", err)
			}
			var payload SpawnAgentPayload
			_ = json.Unmarshal(result.Effects[0].Payload, &payload)
			// MemberID is serialized as uuid.Nil.String() ("00000000-0000-0000-0000-000000000000"),
			// or omitted (omitempty) when empty. Either way the parsed payload must equal
			// the string form of uuid.Nil or the empty string.
			if payload.MemberID != uuid.Nil.String() && payload.MemberID != "" {
				t.Errorf("MemberID = %q, want uuid.Nil string or empty", payload.MemberID)
			}
		})
	}
}

func TestLLMAgentHandler_ValidMemberID(t *testing.T) {
	h := LLMAgentHandler{}
	taskID := uuid.New()
	memberID := uuid.New()
	exec := &model.WorkflowExecution{ID: uuid.New(), TaskID: &taskID}
	node := &model.WorkflowNode{ID: "agent-1"}

	result, err := h.Execute(context.Background(), &NodeExecRequest{
		Execution: exec,
		Node:      node,
		Config:    map[string]any{"memberId": memberID.String()},
	})
	if err != nil {
		t.Fatalf("Execute() returned error: %v", err)
	}
	var payload SpawnAgentPayload
	_ = json.Unmarshal(result.Effects[0].Payload, &payload)
	if payload.MemberID != memberID.String() {
		t.Errorf("MemberID = %q, want %q", payload.MemberID, memberID.String())
	}
}

func TestLLMAgentHandler_ConfigSchema(t *testing.T) {
	h := LLMAgentHandler{}
	schema := h.ConfigSchema()
	if len(schema) == 0 {
		t.Fatal("ConfigSchema() returned empty")
	}
	if !json.Valid(schema) {
		t.Errorf("ConfigSchema() returned invalid JSON: %s", schema)
	}
}

func TestAgentDispatchHandler_DelegatesToLLMAgent(t *testing.T) {
	h := AgentDispatchHandler{}
	taskID := uuid.New()
	memberID := uuid.New()
	exec := &model.WorkflowExecution{ID: uuid.New(), TaskID: &taskID}
	node := &model.WorkflowNode{ID: "agent-1"}

	result, err := h.Execute(context.Background(), &NodeExecRequest{
		Execution: exec,
		Node:      node,
		Config: map[string]any{
			"runtime":   "claude_code",
			"provider":  "anthropic",
			"model":     "claude-sonnet-4-6",
			"roleId":    "coder",
			"memberId":  memberID.String(),
			"budgetUsd": 7.5,
		},
	})
	if err != nil {
		t.Fatalf("Execute() returned error: %v", err)
	}
	if len(result.Effects) != 1 {
		t.Fatalf("expected 1 effect, got %d", len(result.Effects))
	}
	eff := result.Effects[0]
	if eff.Kind != EffectSpawnAgent {
		t.Errorf("effect kind = %s, want %s", eff.Kind, EffectSpawnAgent)
	}
	var payload SpawnAgentPayload
	_ = json.Unmarshal(eff.Payload, &payload)
	if payload.Runtime != "claude_code" || payload.Provider != "anthropic" ||
		payload.Model != "claude-sonnet-4-6" || payload.RoleID != "coder" ||
		payload.MemberID != memberID.String() || payload.BudgetUsd != 7.5 {
		t.Errorf("AgentDispatchHandler did not preserve LLMAgentHandler payload: %+v", payload)
	}

	caps := h.Capabilities()
	if len(caps) != 1 || caps[0] != EffectSpawnAgent {
		t.Errorf("Capabilities() = %v, want [%s]", caps, EffectSpawnAgent)
	}
	assertCapsCoverEffects(t, h, result.Effects)
}

func TestAgentDispatchHandler_ConfigSchema(t *testing.T) {
	h := AgentDispatchHandler{}
	schema := h.ConfigSchema()
	if len(schema) == 0 {
		t.Fatal("ConfigSchema() returned empty")
	}
	if !json.Valid(schema) {
		t.Errorf("ConfigSchema() returned invalid JSON: %s", schema)
	}
}

func TestLLMAgentHandler_EmployeeIDPropagated(t *testing.T) {
	empID := uuid.New()
	taskID := uuid.New()
	req := &NodeExecRequest{
		Execution: &model.WorkflowExecution{TaskID: &taskID},
		Config: map[string]any{
			"runtime":    "claude_code",
			"roleId":     "code-reviewer",
			"employeeId": empID.String(),
		},
	}
	res, err := LLMAgentHandler{}.Execute(context.Background(), req)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if len(res.Effects) != 1 {
		t.Fatalf("expected 1 effect, got %d", len(res.Effects))
	}
	var p SpawnAgentPayload
	if err := json.Unmarshal(res.Effects[0].Payload, &p); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}
	if p.EmployeeID != empID.String() {
		t.Errorf("expected EmployeeID=%s, got %q", empID, p.EmployeeID)
	}
}

func TestLLMAgentHandler_EmployeeIDInvalidIgnored(t *testing.T) {
	taskID := uuid.New()
	req := &NodeExecRequest{
		Execution: &model.WorkflowExecution{TaskID: &taskID},
		Config: map[string]any{
			"runtime":    "claude_code",
			"roleId":     "code-reviewer",
			"employeeId": "not-a-uuid",
		},
	}
	res, err := LLMAgentHandler{}.Execute(context.Background(), req)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	var p SpawnAgentPayload
	_ = json.Unmarshal(res.Effects[0].Payload, &p)
	if p.EmployeeID != "" {
		t.Errorf("expected empty EmployeeID for invalid input, got %q", p.EmployeeID)
	}
}

// ---------------------------------------------------------------------------
// Section 5.3 — fallback precedence tests for acting_employee_id
// (change bridge-employee-attribution-legacy).
// ---------------------------------------------------------------------------

// Node-level override beats run-level acting_employee_id.
func TestLLMAgentHandler_NodeConfigOverridesRunLevelActingEmployee(t *testing.T) {
	taskID := uuid.New()
	runEmp := uuid.New()
	nodeEmp := uuid.New()
	req := &NodeExecRequest{
		Execution: &model.WorkflowExecution{
			TaskID:           &taskID,
			ActingEmployeeID: &runEmp,
		},
		Config: map[string]any{
			"runtime":    "claude_code",
			"roleId":     "code-reviewer",
			"employeeId": nodeEmp.String(),
		},
	}
	res, err := LLMAgentHandler{}.Execute(context.Background(), req)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	var p SpawnAgentPayload
	_ = json.Unmarshal(res.Effects[0].Payload, &p)
	if p.EmployeeID != nodeEmp.String() {
		t.Errorf("expected node override EmployeeID=%s, got %q", nodeEmp, p.EmployeeID)
	}
}

// Run-level default applies when node config has no employeeId.
func TestLLMAgentHandler_FallsBackToRunLevelActingEmployee(t *testing.T) {
	taskID := uuid.New()
	runEmp := uuid.New()
	req := &NodeExecRequest{
		Execution: &model.WorkflowExecution{
			TaskID:           &taskID,
			ActingEmployeeID: &runEmp,
		},
		Config: map[string]any{
			"runtime": "claude_code",
			"roleId":  "code-reviewer",
			// no employeeId
		},
	}
	res, err := LLMAgentHandler{}.Execute(context.Background(), req)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	var p SpawnAgentPayload
	_ = json.Unmarshal(res.Effects[0].Payload, &p)
	if p.EmployeeID != runEmp.String() {
		t.Errorf("expected run-level EmployeeID=%s, got %q", runEmp, p.EmployeeID)
	}
}

// Both absent → EmployeeID remains empty, preserving current behavior.
func TestLLMAgentHandler_BothAbsentEmployeeIDRemainsEmpty(t *testing.T) {
	taskID := uuid.New()
	req := &NodeExecRequest{
		Execution: &model.WorkflowExecution{TaskID: &taskID},
		Config: map[string]any{
			"runtime": "claude_code",
			"roleId":  "code-reviewer",
		},
	}
	res, err := LLMAgentHandler{}.Execute(context.Background(), req)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	var p SpawnAgentPayload
	_ = json.Unmarshal(res.Effects[0].Payload, &p)
	if p.EmployeeID != "" {
		t.Errorf("expected empty EmployeeID, got %q", p.EmployeeID)
	}
}

// Invalid node-config employeeId with run-level fallback available:
// invalid node value is ignored, run-level wins.
func TestLLMAgentHandler_InvalidNodeIDFallsBackToRunLevel(t *testing.T) {
	taskID := uuid.New()
	runEmp := uuid.New()
	req := &NodeExecRequest{
		Execution: &model.WorkflowExecution{
			TaskID:           &taskID,
			ActingEmployeeID: &runEmp,
		},
		Config: map[string]any{
			"runtime":    "claude_code",
			"roleId":     "code-reviewer",
			"employeeId": "not-a-uuid",
		},
	}
	res, err := LLMAgentHandler{}.Execute(context.Background(), req)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	var p SpawnAgentPayload
	_ = json.Unmarshal(res.Effects[0].Payload, &p)
	if p.EmployeeID != runEmp.String() {
		t.Errorf("expected fallback EmployeeID=%s, got %q", runEmp, p.EmployeeID)
	}
}
