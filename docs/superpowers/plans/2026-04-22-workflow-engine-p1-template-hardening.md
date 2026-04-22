# Workflow Engine P1 — Built-in Template Hardening Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Fix every real bug and mis-declared data contract in the 7 built-in workflow templates, plus the single engine-side change P1 depends on (`ConditionHandler.onFalse` policy). No other engine or DB changes.

**Architecture:** Test-first. Each template gets a structural test asserting its config shape, node contracts, and data references resolve; then the template is fixed; then the test is green. `ConditionHandler` gains `onFalse ∈ {error, skip_downstream, proceed}` with default `error` (preserves today's behaviour for every existing caller). `CustomerService` template then opts into `skip_downstream` so non-urgent tickets stop crashing. All other P1 fixes are config-key corrections or prompt/schema formalisations.

**Tech Stack:** Go 1.22+, existing `workflow_templates.go` + `nodetypes/condition.go`, standard `testing` package, `stretchr/testify/require`.

**Source spec:** `docs/superpowers/specs/2026-04-22-workflow-engine-completion-design.md` — §6 Phase P1, plus §7.7 Condition `onFalse` Policy (pulled forward into P1).

---

## File Structure

**Modified files (2):**
- `src-go/internal/workflow/nodetypes/condition.go` — add `onFalse` config branch
- `src-go/internal/service/workflow_templates.go` — fix all 7 built-in templates

**Modified test files (1):**
- `src-go/internal/workflow/nodetypes/condition_test.go` — cover the 3 onFalse variants

**New test file (1):**
- `src-go/internal/service/workflow_templates_test.go` — structural tests for all 7 built-in templates (template-config schema invariants, references resolve, required fields present)

**Not touched in P1:** DB migrations, `dag_workflow_service.go`, `llm_agent.go`, `http_call.go`, `status_transition.go`, any handler other than condition, any frontend store. Those belong to P2/P3/P4.

**Scope guardrail:** if a task tempts you to edit anything outside the four files above, stop and re-read the spec — that task likely belongs to P2 or later.

---

## Task 1: Add `onFalse` policy to `ConditionHandler`

**Files:**
- Modify: `src-go/internal/workflow/nodetypes/condition.go`
- Modify: `src-go/internal/workflow/nodetypes/condition_test.go`

- [ ] **Step 1.1: Read the current handler to understand existing surface**

Run: Read `src-go/internal/workflow/nodetypes/condition.go` in full.

Confirm what you see: `ConditionHandler.Execute` returns `error("condition not met: <expr>")` when the expression is false; `ConfigSchema()` declares only `expression`; `Capabilities()` returns nil.

- [ ] **Step 1.2: Write the failing test — `onFalse: skip_downstream` returns no error**

Add this test at the end of `condition_test.go`:

```go
func TestConditionHandler_OnFalseSkipDownstream(t *testing.T) {
    h := ConditionHandler{}
    req := &NodeExecRequest{
        Node:   &model.WorkflowNode{ID: "gate"},
        Config: map[string]any{
            "expression": "1 == 2",
            "onFalse":    "skip_downstream",
        },
    }
    res, err := h.Execute(context.Background(), req)
    require.NoError(t, err)
    require.NotNil(t, res)
    require.Equal(t, "skip", res.Output["_route"])
}
```

- [ ] **Step 1.3: Run the test to verify it fails**

Run: `cd src-go && go test ./internal/workflow/nodetypes/ -run TestConditionHandler_OnFalseSkipDownstream -v`

Expected: FAIL — either the `_route` assertion fails or the handler returns an error on the false expression.

- [ ] **Step 1.4: Write the failing test — `onFalse: proceed` succeeds with no route signal**

```go
func TestConditionHandler_OnFalseProceed(t *testing.T) {
    h := ConditionHandler{}
    req := &NodeExecRequest{
        Node:   &model.WorkflowNode{ID: "gate"},
        Config: map[string]any{"expression": "1 == 2", "onFalse": "proceed"},
    }
    res, err := h.Execute(context.Background(), req)
    require.NoError(t, err)
    require.NotNil(t, res)
    require.Empty(t, res.Output["_route"])
}
```

- [ ] **Step 1.5: Write the failing test — `onFalse: error` (default) preserves today's behaviour**

```go
func TestConditionHandler_OnFalseErrorIsDefault(t *testing.T) {
    h := ConditionHandler{}
    req := &NodeExecRequest{
        Node:   &model.WorkflowNode{ID: "gate"},
        Config: map[string]any{"expression": "1 == 2"}, // no onFalse
    }
    _, err := h.Execute(context.Background(), req)
    require.Error(t, err)
    require.Contains(t, err.Error(), "condition not met")
}
```

- [ ] **Step 1.6: Run all three tests together — all should fail (1, 2) or pass (3)**

Run: `cd src-go && go test ./internal/workflow/nodetypes/ -run 'TestConditionHandler_OnFalse' -v`

Expected: Two FAIL (skip_downstream, proceed), one PASS (error default).

- [ ] **Step 1.7: Implement `onFalse` branch in `ConditionHandler.Execute`**

Replace the body after the expression evaluation in `condition.go`:

```go
if !EvaluateCondition(ctx, exec, expression, dataStore, h.TaskRepo) {
    onFalse, _ := config["onFalse"].(string)
    switch onFalse {
    case "skip_downstream":
        return &NodeExecResult{Output: map[string]any{"_route": "skip"}}, nil
    case "proceed":
        return &NodeExecResult{}, nil
    case "", "error":
        return nil, fmt.Errorf("condition not met: %s", expression)
    default:
        return nil, fmt.Errorf("condition node %s has invalid onFalse %q (want error|skip_downstream|proceed)", req.Node.ID, onFalse)
    }
}
return &NodeExecResult{}, nil
```

Also update `ConfigSchema()` to include `onFalse`:

```go
return json.RawMessage(`{
  "type": "object",
  "properties": {
    "expression": {"type": "string"},
    "onFalse": {"type": "string", "enum": ["error", "skip_downstream", "proceed"], "default": "error"}
  }
}`)
```

- [ ] **Step 1.8: Run the three tests — all pass**

Run: `cd src-go && go test ./internal/workflow/nodetypes/ -run 'TestConditionHandler_OnFalse' -v`

Expected: three PASS.

- [ ] **Step 1.9: Run the whole nodetypes package test to catch regressions**

Run: `cd src-go && go test ./internal/workflow/nodetypes/ -v`

Expected: all existing tests still pass.

- [ ] **Step 1.10: Commit**

```bash
git add src-go/internal/workflow/nodetypes/condition.go src-go/internal/workflow/nodetypes/condition_test.go
git commit -m "feat(workflow): ConditionHandler gains onFalse policy (error|skip_downstream|proceed)"
```

---

## Task 2: Scaffold `workflow_templates_test.go`

**Files:**
- Create: `src-go/internal/service/workflow_templates_test.go`

- [ ] **Step 2.1: Create the test file with a minimal structural suite**

```go
package service

import (
    "encoding/json"
    "testing"

    "github.com/agentforge/server/internal/model"
    "github.com/stretchr/testify/require"
)

// helper: unmarshal a template's nodes into typed slices
func unmarshalNodes(t *testing.T, def *model.WorkflowDefinition) []model.WorkflowNode {
    t.Helper()
    var nodes []model.WorkflowNode
    require.NoError(t, json.Unmarshal(def.Nodes, &nodes))
    return nodes
}

func findNode(t *testing.T, nodes []model.WorkflowNode, id string) model.WorkflowNode {
    t.Helper()
    for _, n := range nodes {
        if n.ID == id {
            return n
        }
    }
    t.Fatalf("node %q not found", id)
    return model.WorkflowNode{}
}

func configOf(t *testing.T, n model.WorkflowNode) map[string]any {
    t.Helper()
    var cfg map[string]any
    require.NoError(t, json.Unmarshal(n.Config, &cfg))
    return cfg
}

func TestAllSystemTemplatesLoadable(t *testing.T) {
    templates := AllSystemTemplates()
    require.NotEmpty(t, templates)
    for _, def := range templates {
        require.NotEmpty(t, def.Name, "template missing name")
        require.NotEmpty(t, def.Nodes, "template %q missing nodes", def.Name)
        require.NotEmpty(t, def.Edges, "template %q missing edges", def.Name)
    }
}
```

- [ ] **Step 2.2: Run the smoke test**

Run: `cd src-go && go test ./internal/service/ -run TestAllSystemTemplatesLoadable -v`

Expected: PASS.

- [ ] **Step 2.3: Commit**

```bash
git add src-go/internal/service/workflow_templates_test.go
git commit -m "test(workflow): scaffold structural test file for built-in templates"
```

---

## Task 3: Fix `SystemCodeReview` — `finalize` targetStatus bug

**Files:**
- Modify: `src-go/internal/service/workflow_templates.go` (`SystemCodeReviewTemplate`)
- Modify: `src-go/internal/service/workflow_templates_test.go`

- [ ] **Step 3.1: Write failing test that `finalize.config.targetStatus` references the decision output**

```go
func TestSystemCodeReviewTemplate_FinalizeUsesTargetStatus(t *testing.T) {
    def := SystemCodeReviewTemplate()
    nodes := unmarshalNodes(t, def)
    finalize := findNode(t, nodes, "finalize")
    cfg := configOf(t, finalize)

    target, ok := cfg["targetStatus"].(string)
    require.True(t, ok, "finalize.config.targetStatus must be a string")
    require.Equal(t, "{{decision.output.decision}}", target)

    _, hasFrom := cfg["from"]
    _, hasTo := cfg["to"]
    require.False(t, hasFrom, "finalize.config must not carry legacy 'from' key")
    require.False(t, hasTo, "finalize.config must not carry legacy 'to' key")
}

func TestSystemCodeReviewTemplate_DecisionSchemaDeclared(t *testing.T) {
    def := SystemCodeReviewTemplate()
    nodes := unmarshalNodes(t, def)
    decision := findNode(t, nodes, "decision")
    cfg := configOf(t, decision)

    schema, ok := cfg["decision_schema"].(map[string]any)
    require.True(t, ok, "decision node must declare decision_schema")
    require.Equal(t, "bool", schema["approved"])
    require.Equal(t, "string", schema["comment"])
}
```

- [ ] **Step 3.2: Run — expect fail**

Run: `cd src-go && go test ./internal/service/ -run TestSystemCodeReviewTemplate_ -v`

Expected: FAIL on both tests.

- [ ] **Step 3.3: Patch `SystemCodeReviewTemplate()` in `workflow_templates.go`**

Change the `decision` node's config to add `decision_schema`:

```go
{ID: "decision", Type: model.NodeTypeHumanReview, Label: "Approve or request changes", Position: model.WorkflowPos{X: 600, Y: 200},
    Config: buildConfig(map[string]any{
        "prompt":         "Review the automated analysis and approve or request changes. You MUST emit a decision object with fields {approved: bool, comment: string}.",
        "decision_field": "decision",
        "decision_schema": map[string]any{
            "approved": "bool",
            "comment":  "string",
        },
    })},
```

Change the `finalize` node's config to use `targetStatus`:

```go
{ID: "finalize", Type: model.NodeTypeStatusTransition, Label: "Finalize review", Position: model.WorkflowPos{X: 900, Y: 200},
    Config: buildConfig(map[string]any{
        "targetStatus": "{{decision.output.decision}}",
        "reason":       "{{decision.output.comment}}",
    })},
```

- [ ] **Step 3.4: Run both tests — all pass**

Run: `cd src-go && go test ./internal/service/ -run TestSystemCodeReviewTemplate_ -v`

Expected: two PASS.

- [ ] **Step 3.5: Run the whole service package test to confirm no regression**

Run: `cd src-go && go test ./internal/service/ -v 2>&1 | tail -30`

Expected: all tests pass.

- [ ] **Step 3.6: Commit**

```bash
git add src-go/internal/service/workflow_templates.go src-go/internal/service/workflow_templates_test.go
git commit -m "fix(workflow): SystemCodeReview finalize config uses targetStatus key"
```

---

## Task 4: Fix `CustomerService` — discrete urgency_band + condition onFalse

**Files:**
- Modify: `src-go/internal/service/workflow_templates.go` (`CustomerServiceTemplate`)
- Modify: `src-go/internal/service/workflow_templates_test.go`

- [ ] **Step 4.1: Write failing test — classify prompt must produce `urgency_band`**

```go
func TestCustomerServiceTemplate_ClassifyEmitsUrgencyBand(t *testing.T) {
    def := CustomerServiceTemplate()
    nodes := unmarshalNodes(t, def)
    classify := findNode(t, nodes, "classify")
    cfg := configOf(t, classify)

    prompt, _ := cfg["prompt"].(string)
    require.Contains(t, prompt, "urgency_band", "classify prompt must instruct the LLM to emit urgency_band")
    require.Contains(t, prompt, `"urgent"`, "prompt should show the discrete urgent value")
    require.Contains(t, prompt, `"normal"`, "prompt should show the discrete normal value")
}

func TestCustomerServiceTemplate_UrgentCheckUsesDiscreteBand(t *testing.T) {
    def := CustomerServiceTemplate()
    nodes := unmarshalNodes(t, def)
    check := findNode(t, nodes, "urgent_check")
    cfg := configOf(t, check)

    expr, _ := cfg["expression"].(string)
    require.Equal(t, `{{classify.output.urgency_band}} == "urgent"`, expr)
    require.Equal(t, "skip_downstream", cfg["onFalse"], "urgent_check must skip downstream on non-urgent to avoid the default-error crash")
}

func TestCustomerServiceTemplate_EdgesUseDiscreteBand(t *testing.T) {
    def := CustomerServiceTemplate()
    var edges []model.WorkflowEdge
    require.NoError(t, json.Unmarshal(def.Edges, &edges))

    foundHumanRouting := false
    foundAutoReplyRouting := false
    for _, e := range edges {
        if e.Source == "urgent_check" && e.Target == "human_review" {
            require.Equal(t, `{{classify.output.urgency_band}} == "urgent"`, e.Condition)
            foundHumanRouting = true
        }
        if e.Source == "urgent_check" && e.Target == "auto_reply" {
            require.Equal(t, `{{classify.output.urgency_band}} == "normal"`, e.Condition)
            foundAutoReplyRouting = true
        }
    }
    require.True(t, foundHumanRouting && foundAutoReplyRouting, "both urgency branches must be wired with discrete-band conditions")
}
```

- [ ] **Step 4.2: Run tests — expect fail**

Run: `cd src-go && go test ./internal/service/ -run TestCustomerServiceTemplate_ -v`

Expected: three FAIL.

- [ ] **Step 4.3: Patch `CustomerServiceTemplate()` in `workflow_templates.go`**

Update `classify` prompt, `urgent_check` expression + onFalse, and both edges. Concrete patched nodes:

```go
{ID: "classify", Type: model.NodeTypeLLMAgent, Label: "Classify & Analyze", Position: model.WorkflowPos{X: 250, Y: 200},
    Config: buildConfig(map[string]any{
        "prompt":    `Classify the customer inquiry. Input: {{trigger.output}}. You MUST emit {category: string, urgency_band: "urgent" | "normal"} (discrete, not a float score).`,
        "runtime":   "{{runtime}}",
        "provider":  "{{provider}}",
        "model":     "{{model}}",
        "budgetUsd": 0.5,
    })},
{ID: "urgent_check", Type: model.NodeTypeCondition, Label: "Urgent?", Position: model.WorkflowPos{X: 500, Y: 200},
    Config: buildConfig(map[string]any{
        "expression": `{{classify.output.urgency_band}} == "urgent"`,
        "onFalse":    "skip_downstream",
    })},
```

Update edges:

```go
{ID: "e3", Source: "urgent_check", Target: "human_review", Condition: `{{classify.output.urgency_band}} == "urgent"`, Label: "Urgent"},
{ID: "e4", Source: "urgent_check", Target: "auto_reply",  Condition: `{{classify.output.urgency_band}} == "normal"`, Label: "Normal"},
```

- [ ] **Step 4.4: Run tests — pass**

Run: `cd src-go && go test ./internal/service/ -run TestCustomerServiceTemplate_ -v`

Expected: three PASS.

- [ ] **Step 4.5: Commit**

```bash
git add src-go/internal/service/workflow_templates.go src-go/internal/service/workflow_templates_test.go
git commit -m "fix(workflow): CustomerService discrete urgency_band + condition skip_downstream"
```

---

## Task 5: Fix `ContentCreation` — editor output schema & explicit references

**Files:**
- Modify: `src-go/internal/service/workflow_templates.go` (`ContentCreationTemplate`)
- Modify: `src-go/internal/service/workflow_templates_test.go`

- [ ] **Step 5.1: Write failing test — editor prompt mandates `{approved: bool, feedback: string}`**

```go
func TestContentCreationTemplate_EditorOutputSchema(t *testing.T) {
    def := ContentCreationTemplate()
    nodes := unmarshalNodes(t, def)
    editor := findNode(t, nodes, "editor")
    cfg := configOf(t, editor)

    prompt, _ := cfg["prompt"].(string)
    require.Contains(t, prompt, "approved", "editor must emit approved:bool so edit_loop.exit_condition works")
    require.Contains(t, prompt, "feedback")
    require.Contains(t, prompt, "bool")
}

func TestContentCreationTemplate_LoopExitConditionReferencesApproved(t *testing.T) {
    def := ContentCreationTemplate()
    nodes := unmarshalNodes(t, def)
    loop := findNode(t, nodes, "edit_loop")
    cfg := configOf(t, loop)
    require.Equal(t, "{{editor.output.approved}} == true", cfg["exit_condition"])
}
```

- [ ] **Step 5.2: Run — expect fail**

Run: `cd src-go && go test ./internal/service/ -run TestContentCreationTemplate_ -v`

- [ ] **Step 5.3: Patch editor node prompt**

```go
{ID: "editor", Type: model.NodeTypeLLMAgent, Label: "Editor", Position: model.WorkflowPos{X: 1000, Y: 200},
    Config: buildConfig(map[string]any{
        "prompt":    `Edit and improve the draft: {{writer.output}}. You MUST emit {approved: bool, feedback: string} — approved=true ends the revision loop; feedback is the actionable next-draft guidance.`,
        "runtime":   "{{runtime}}",
        "provider":  "{{provider}}",
        "model":     "{{model}}",
        "budgetUsd": 1.0,
    })},
```

- [ ] **Step 5.4: Run — pass**

Run: `cd src-go && go test ./internal/service/ -run TestContentCreationTemplate_ -v`

- [ ] **Step 5.5: Commit**

```bash
git add src-go/internal/service/workflow_templates.go src-go/internal/service/workflow_templates_test.go
git commit -m "fix(workflow): ContentCreation editor declares {approved,feedback} output"
```

---

## Task 6: Fix `PlanCodeReview` / `Swarm` — `fan_out` / `split` subtasks schema

**Files:**
- Modify: `src-go/internal/service/workflow_templates.go` (both templates)
- Modify: `src-go/internal/service/workflow_templates_test.go`

- [ ] **Step 6.1: Write failing test — planner prompt declares the subtasks schema**

```go
func TestPlanCodeReviewTemplate_PlannerDeclaresSubtasksSchema(t *testing.T) {
    def := PlanCodeReviewTemplate()
    nodes := unmarshalNodes(t, def)
    planner := findNode(t, nodes, "planner")
    cfg := configOf(t, planner)

    prompt, _ := cfg["prompt"].(string)
    require.Contains(t, prompt, "subtasks")
    require.Contains(t, prompt, "id")
    require.Contains(t, prompt, "title")
    require.Contains(t, prompt, "prompt")
    require.Contains(t, prompt, "budget_usd")
}

func TestPlanCodeReviewTemplate_FanOutExpressionReferencesSubtasks(t *testing.T) {
    def := PlanCodeReviewTemplate()
    nodes := unmarshalNodes(t, def)
    fanOut := findNode(t, nodes, "fan_out")
    cfg := configOf(t, fanOut)
    require.Equal(t, "{{planner.output.subtasks}}", cfg["expression"])
}

func TestSwarmTemplate_PlannerDeclaresSubtasksSchema(t *testing.T) {
    def := SwarmTemplate()
    nodes := unmarshalNodes(t, def)
    planner := findNode(t, nodes, "planner")
    cfg := configOf(t, planner)
    prompt, _ := cfg["prompt"].(string)
    require.Contains(t, prompt, "subtasks")
    require.Contains(t, prompt, "id")
    require.Contains(t, prompt, "budget_usd")
}
```

- [ ] **Step 6.2: Run — expect fail**

Run: `cd src-go && go test ./internal/service/ -run 'TestPlanCodeReviewTemplate_|TestSwarmTemplate_' -v`

- [ ] **Step 6.3: Patch planner prompts in both templates**

For `PlanCodeReviewTemplate`:

```go
"prompt": `Analyze the task and create a structured plan with subtasks. You MUST emit {subtasks: [{id: string, title: string, prompt: string, budget_usd: number}]}. Each subtask becomes an independent coder invocation.`,
```

For `SwarmTemplate`:

```go
"prompt": `Break down the task into independent subtasks for maximum parallelism. You MUST emit {subtasks: [{id: string, title: string, prompt: string, budget_usd: number}]}.`,
```

- [ ] **Step 6.4: Run — pass**

Run: `cd src-go && go test ./internal/service/ -run 'TestPlanCodeReviewTemplate_|TestSwarmTemplate_' -v`

- [ ] **Step 6.5: Commit**

```bash
git add src-go/internal/service/workflow_templates.go src-go/internal/service/workflow_templates_test.go
git commit -m "fix(workflow): PlanCodeReview/Swarm planner declares subtasks schema"
```

---

## Task 7: Fix `Pipeline` — explicit `coder.inputs`

**Files:**
- Modify: `src-go/internal/service/workflow_templates.go` (`PipelineTemplate`)
- Modify: `src-go/internal/service/workflow_templates_test.go`

- [ ] **Step 7.1: Write failing test**

```go
func TestPipelineTemplate_CoderReferencesPlannerPlanExplicitly(t *testing.T) {
    def := PipelineTemplate()
    nodes := unmarshalNodes(t, def)
    coder := findNode(t, nodes, "coder")
    cfg := configOf(t, coder)
    inputs, ok := cfg["inputs"].(map[string]any)
    require.True(t, ok, "coder.config.inputs must be declared")
    require.Equal(t, "{{planner.output.plan}}", inputs["plan"])
}
```

- [ ] **Step 7.2: Run — expect fail**

- [ ] **Step 7.3: Patch `PipelineTemplate()` — planner declares `plan` output and coder references it**

Update planner prompt:

```go
"prompt": `Analyze the task and create an ordered list of implementation steps. You MUST emit {plan: [{step: string, rationale: string}]}.`,
```

Add `inputs` key to coder config:

```go
"inputs": map[string]any{
    "plan": "{{planner.output.plan}}",
},
```

- [ ] **Step 7.4: Run — pass**

- [ ] **Step 7.5: Commit**

```bash
git add src-go/internal/service/workflow_templates.go src-go/internal/service/workflow_templates_test.go
git commit -m "fix(workflow): Pipeline coder.inputs references planner.output.plan explicitly"
```

---

## Task 8: Fix `CodeFixer` — `has_prebaked` expression + validate function

**Files:**
- Modify: `src-go/internal/workflow/system/code_fixer_dag.go` (the CodeFixer template lives here, not in `workflow_templates.go`)
- Modify: `src-go/internal/service/workflow_templates_test.go` (reaches into the system package's definition)

- [ ] **Step 8.1: Read the current template to confirm file location**

Run: Read `src-go/internal/workflow/system/code_fixer_dag.go` — confirm it exports `CodeFixerDefinition()`.

- [ ] **Step 8.2: Verify whether the `function` node type has a registered handler**

Run: Grep `src-go/internal/workflow/nodetypes/registry.go` for `NodeTypeFunction`.

If the function type is unregistered, this task decomposes further: either (a) remove the `validate` function node and replace with a direct condition, or (b) register a minimal function handler. Prefer (a) for P1 to keep scope minimal.

Capture the finding before writing the test.

- [ ] **Step 8.3: Write failing test — `has_prebaked` references `$event`, not undefined `input`**

```go
func TestCodeFixerTemplate_HasPrebakedReferencesEvent(t *testing.T) {
    def := system.CodeFixerDefinition()
    nodes := unmarshalNodes(t, def)
    gate := findNode(t, nodes, "has_prebaked")
    cfg := configOf(t, gate)
    expr, _ := cfg["expression"].(string)
    require.Equal(t, "{{$event.suggested_patch}} != null", expr)
}
```

Add the `system` package import to the test file:

```go
import (
    ...
    "github.com/agentforge/server/internal/workflow/system"
)
```

- [ ] **Step 8.4: Run — expect fail**

Run: `cd src-go && go test ./internal/service/ -run TestCodeFixerTemplate_ -v`

- [ ] **Step 8.5: Patch `code_fixer_dag.go` — replace `input.suggested_patch != null` with `{{$event.suggested_patch}} != null`**

Also document at the trigger node that the payload schema declares `suggested_patch` explicitly. If the `validate` function node has no handler (from Step 8.2), remove it and wire the edge direct from `generate` to `decide`; add a comment referring to P3's retry node as the future home for validate-retry semantics.

- [ ] **Step 8.6: Run — pass**

- [ ] **Step 8.7: Commit**

```bash
git add src-go/internal/workflow/system/code_fixer_dag.go src-go/internal/service/workflow_templates_test.go
git commit -m "fix(workflow): CodeFixer has_prebaked references \$event.suggested_patch"
```

---

## Task 9: LLM node selection-priority documentation

**Files:**
- Modify: `src-go/internal/workflow/nodetypes/llm_agent.go` — doc comment at the top of the file + above the Execute method
- Modify: `src-go/internal/workflow/nodetypes/llm_agent_test.go` — golden test for the priority

- [ ] **Step 9.1: Read the existing llm_agent handler to locate the selection logic**

Run: Read `src-go/internal/workflow/nodetypes/llm_agent.go` and note where `employeeId`, `roleId`, `runtime`, `provider`, `model` are consumed.

- [ ] **Step 9.2: Write a failing test locking in the priority order**

```go
func TestLLMAgent_SelectionPriority_EmployeeBeatsRoleBeatsTemplateVars(t *testing.T) {
    // Three cases: employeeId only → employeeId wins;
    //              no employeeId, roleId + runtime → roleId wins;
    //              no employeeId, no roleId, only runtime/provider/model → templateVars win.
    // Assert by inspecting the effect produced or the spawn invocation argument.
}
```

Implementation detail: depending on the llm_agent effect shape, may need to stub an agent spawner and assert which code path was taken. Study the handler carefully and write a test that asserts the documented priority is the behaviour.

- [ ] **Step 9.3: Run — it may already pass if priority is correct**

Run: `cd src-go && go test ./internal/workflow/nodetypes/ -run TestLLMAgent_SelectionPriority_ -v`

If PASS: the code already implements the priority; only documentation is missing.
If FAIL: fix the handler to honour the documented priority.

- [ ] **Step 9.4: Add doc comments to `llm_agent.go`**

```go
// LLMAgentHandler implements the "llm_agent" node type.
//
// Runtime/provider/model selection priority (documented as the P1 contract):
//  1. Node-level employeeId — if set, the employee's own runtime/provider/model overrides everything.
//  2. Node-level roleId — if set, the role's default runtime/provider/model is used.
//  3. Node-level runtime/provider/model config — the direct config on the node.
//  4. templateVars (definition-scoped) — baked at clone time.
//
// Lower-priority fields are only consulted when higher-priority fields are empty.
```

- [ ] **Step 9.5: Run — still pass**

- [ ] **Step 9.6: Commit**

```bash
git add src-go/internal/workflow/nodetypes/llm_agent.go src-go/internal/workflow/nodetypes/llm_agent_test.go
git commit -m "docs(workflow): document llm_agent selection priority + lock it in a test"
```

---

## Task 10: Final P1 acceptance — whole-package regression sweep

- [ ] **Step 10.1: Run the full Go test suite**

Run: `cd src-go && go test ./... 2>&1 | tail -50`

Expected: all tests green. If any fail, triage: is it a real regression, or an unrelated flaky test? Fix real regressions in-scope for P1 only.

- [ ] **Step 10.2: Grep for any remaining legacy `from`/`to` keys in templates**

Run: `cd src-go && grep -n '"from"\|"to"' internal/service/workflow_templates.go internal/workflow/system/code_fixer_dag.go`

Expected: zero matches referencing status_transition config. If any remain, they belong to a template P1 missed — fix.

- [ ] **Step 10.3: Grep for `urgency` to confirm no float threshold survives**

Run: `cd src-go && grep -n 'urgency' internal/service/workflow_templates.go`

Expected: only discrete `urgency_band` references. No `urgency > 0.7` or similar fragments.

- [ ] **Step 10.4: Inspect each template visually in `AllSystemTemplates()`**

Open `src-go/internal/service/workflow_templates.go` and the `CodeFixerDefinition` function; for each template, re-read the node list and confirm nothing from the §6 P1 table is missing.

- [ ] **Step 10.5: Create a P1 closeout commit**

```bash
git commit --allow-empty -m "chore(workflow): P1 template hardening complete (7 templates, condition.onFalse)"
```

This creates a discoverable anchor commit for the P1 boundary.

---

## Definition of Done (P1)

- All tests in `src-go/internal/workflow/nodetypes/condition_test.go` and `src-go/internal/service/workflow_templates_test.go` pass.
- `go test ./...` green for the whole module.
- Grep sanity:
  - No `"from"`/`"to"` keys on `status_transition` nodes in any built-in template
  - No float-threshold `urgency` comparison in `CustomerService`
  - `{{$event.suggested_patch}}` used in `CodeFixer` `has_prebaked`
- `ConditionHandler.Execute` handles three `onFalse` values + a default.
- `llm_agent.go` documents the selection priority and a test locks it in.
- P1 closeout commit created.

## Explicit Non-Goals (guardrail for this plan only)

- DO NOT add `idempotency_key` — that is P2.
- DO NOT add any new node type (`retry`, `timeout`, `switch`, `map`, `reduce`) — that is P3.
- DO NOT rewrite any template using `map`/`switch` — that is P4.
- DO NOT modify the DB schema — no P2/P3/P4 migrations in this plan.
- DO NOT touch `dag_workflow_service.go`, `status_transition.go`, `http_call.go`, or any other handler outside `condition.go` and doc-only `llm_agent.go`.

If you are tempted to stretch past these lines, stop, reference the spec, and open a follow-up plan.
