## Context

Employee attribution is already partially working for DAG workflows: `workflow/nodetypes/llm_agent.go:62-67,93` accepts an `employeeId` on a single node's config and forwards it to `SpawnAgentPayload`. The spawner then sets `AgentRun.EmployeeID` correctly. This works for a DAG user who manually ties one node to an employee. Three seams break the story:

1. **Legacy step router** (`workflow_step_router.go`) — processes `agent` / `review` / `task` step actions and reads `memberId` from the trigger payload only (`grep memberId workflow_step_router.go` → multiple; `grep employeeId workflow_step_router.go` → zero). Any workflow plugin calling `agent` through this router spawns runs with `employee_id = NULL`.
2. **Trigger router** (`trigger/router.go`) — `Router.Route` constructs `StartOptions{Seed, TriggeredBy: &triggerID}` but has no concept of "acting employee." The trigger row records only `WorkflowTrigger.TriggeredBy *uuid.UUID` which is the trigger id itself, not an operator identity.
3. **Run record columns** — neither `workflow_executions` nor `workflow_plugin_run` carries an `acting_employee_id` column. Consumers cannot query "all runs acting as employee X" without joining through every agent_run row.

Once bridge `bridge-trigger-dispatch-unification` lands, triggers can fire both engines. Employee attribution must flow through both paths symmetrically, or the two engines will diverge in attribution quality.

## Goals / Non-Goals

**Goals:**
- Any agent run spawned through a trigger-initiated workflow (DAG or plugin) or through a plugin workflow's `agent` / `review` / `task` step carries the correct `employee_id` when the workflow run or step declares one.
- Attribution flows hierarchically: trigger-level `acting_employee_id` becomes the default on the started run record; step-level `employee_id` overrides; explicit `null` clears.
- Run records themselves expose the acting employee so queries like "list runs acting as employee X" need no agent_run JOIN.
- Cross-project references are rejected at author / trigger-fire time — an employee from project A cannot appear on a workflow in project B.

**Non-Goals:**
- Adding per-node RBAC around employee invocation. That is change `bridge-employee-rbac` (out of scope here; separately scoped if needed later).
- Back-filling historical `agent_runs.employee_id` for already-completed runs. New attribution only applies to runs spawned after this change lands.
- Cost aggregation per employee (a view on top of attribution; separate change if wanted).
- Creating an "acting as employee" experience in the UI for manual workflow starts — the UI gain here is limited to the Triggers tab. Manual start flows can pass `employeeId` through the existing DAG node config.

## Decisions

### Decision 1: Two-level attribution (run-level default + step-level override)

**Chosen**: A run record carries a single `acting_employee_id` column. Steps read from `node.config.employeeId` if present, else fall back to the run's `acting_employee_id`. If both are absent, `agent_runs.employee_id` stays NULL (current behavior).

**Alternatives considered**:
- *Step-level only* — forces every node config to repeat the employee id; brittle and duplicative.
- *Trigger-level only* — no way to run a workflow where most steps act as one employee but one escalation step acts as another.

**Rationale**: Matches how operators think ("this whole run is employee A's work, except the review step which is employee B"). Cheap to persist (one extra column per run table).

### Decision 2: `acting_employee_id` validated against project scope, not against employee.state

**Chosen**: At author time (trigger sync / node config save) and at dispatch time, the system MUST verify `employee.project_id == workflow.project_id` (or the corresponding plugin's project). `employee.state` is NOT checked at dispatch — a `paused` employee can still be an attribution target. Only `archived` state blocks use, because archived means "this identity should not be reused."

**Alternatives considered**:
- *Block paused employees* — conflates "this employee is on pause for autonomous runs" with "this employee cannot be attributed." The former is a scheduling concern owned by the agent pool; the latter is an identity concern.

**Rationale**: Lets an operator run a workflow "as" an employee even if that employee's autonomous scheduler is paused — attribution is about identity, not availability.

### Decision 3: Extend `workflow_step_router` input contract, not action list

**Chosen**: The router's existing `agent`, `review`, `task` actions gain an optional `employee_id` field in their trigger payload. No new action type is added.

**Alternatives considered**:
- *New `employee_dispatch` action* — duplicates `agent` action semantics with one extra field. Rejected as redundant.
- *Wrap every run in an employee-aware proxy* — indirection without benefit.

**Rationale**: Keeps the router's five-action contract (see `workflow-engine/spec.md`) intact. The modification is pure input extension, not a new capability.

### Decision 4: Dispatch-time cross-project guard returns structured rejection

**Chosen**: If a step or trigger references an employee whose `project_id` differs from the workflow's `project_id`, dispatch returns a structured non-success outcome (same shape used by `task-triggered-workflow-automation`) and no run is started.

**Rationale**: Silent fallbacks to NULL would cover up serious author-time mistakes (copy-paste between projects, stale trigger pointing at deleted employee). Structured rejection is louder and more debuggable.

## Risks / Trade-offs

- **Risk**: A trigger's `actingEmployeeId` could become stale if the employee is archived after the trigger is saved. → **Mitigation**: Dispatch-time validation rejects archived-employee targets; operators see a structured reason on the trigger outcome and can re-bind.
- **Risk**: Existing DAG workflows that already set `employeeId` on a `llm_agent` node might behave differently if a run-level `acting_employee_id` is also set. → **Mitigation**: Node-level config always wins over run-level default. This is spec-enforced in the new capability.
- **Trade-off**: This change adds `acting_employee_id` columns to both run tables, duplicating the concept across engines. We accept the duplication because merging run tables is explicitly out of scope (see `bridge-unified-run-view`).
- **Trade-off**: No back-fill means historical attribution queries will continue to see NULLs for older runs. Acceptable given pre-1.0 internal testing status — operators can re-run recent workflows if they need complete attribution.

## Migration Plan

1. Add migration adding `acting_employee_id UUID NULL REFERENCES employees(id)` to `workflow_triggers`, `workflow_executions`, and the legacy plugin-run table.
2. Extend `model.WorkflowTrigger`, `model.WorkflowExecution`, legacy run model, and their repo layers.
3. Extend `trigger.Router` to read `acting_employee_id` off the trigger row and pass it through into the engine adapter's start call.
4. Extend DAG + plugin start flows to persist `acting_employee_id` on the run record.
5. Extend `workflow_step_router.go` action payload parsing to accept `employee_id` and thread it into the spawned run / review / task.
6. Extend `llm_agent` node handler (and any other node type that spawns runs) to fall back to `run.acting_employee_id` when node config does not override.
7. Add cross-project and archived-state validation in `employee.Service` (or a small `employee.AttributionGuard` helper) and call it from trigger sync, node-config save, and dispatch.
8. Update trigger-store and Triggers tab UI to surface `actingEmployeeId` selection.
9. Add tests per "Verification impact" in the proposal.

Rollback is trivial because every column is nullable and every behavior change is additive for unset inputs.

## Open Questions

- Do we also want `acting_employee_id` to show up in the workflow execution WebSocket broadcast payload so the frontend can render "acting as @employee" badges without a second fetch? (Expected: yes, but can be added as a small follow-up — not a blocker for the core change.)
- Is there a use case for multiple acting employees per run (shared attribution)? (Not in any current flow. Skipping unless evidence emerges.)
