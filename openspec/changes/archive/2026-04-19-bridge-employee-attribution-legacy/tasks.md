## 1. Schema migration

- [x] 1.1 Add migration `src-go/migrations/0NN_workflow_acting_employee.up.sql` + `.down.sql` adding `acting_employee_id UUID NULL REFERENCES employees(id)` to `workflow_triggers`, `workflow_executions`, and the legacy workflow plugin run table
- [x] 1.2 Extend `model.WorkflowTrigger`, `model.WorkflowExecution`, and the legacy plugin run struct with `ActingEmployeeID *uuid.UUID`
- [x] 1.3 Extend corresponding repositories (read/write/list) to round-trip the column; add list filter `ListByActingEmployee`

## 2. Attribution guard helper

- [x] 2.1 Add `employee.AttributionGuard` (or equivalent helper on `employee.Service`) exposing `ValidateForProject(ctx, employeeID, projectID)` and `ValidateNotArchived(ctx, employeeID)`
- [x] 2.2 Unit-test guard against: cross-project, unknown id, archived, active, paused
- [x] 2.3 Export the guard through the appropriate package so trigger/registrar, DAG service, and step router can call it without cyclic imports

## 3. Trigger router propagation

- [x] 3.1 Update `trigger.Router.Route` to read `trigger.ActingEmployeeID` and pass it into the engine adapter's `Start` call (both DAG and plugin adapters)
- [x] 3.2 Update DAG adapter to set the new column on `WorkflowExecution.ActingEmployeeID` on start
- [x] 3.3 Update plugin adapter to set the new column on the legacy plugin run on start
- [x] 3.4 Invoke `AttributionGuard.ValidateNotArchived` at dispatch time; on failure, emit structured non-success outcome and skip dispatch (idempotency key NOT consumed)
- [x] 3.5 Extend router tests: archived target, paused target, null target, unknown id, cross-project reference

## 4. Registrar author-time validation

- [x] 4.1 Extend `trigger/registrar.go` sync logic to accept `acting_employee_id` on incoming configs
- [x] 4.2 Invoke `AttributionGuard.ValidateForProject` at sync time; on failure, persist `enabled=false` with structured `disabled_reason`
- [x] 4.3 Extend `registrar_test.go` to cover cross-project and unknown-employee cases

## 5. DAG step executor fallback

- [x] 5.1 Update `workflow/nodetypes/llm_agent.go` to resolve an effective employee id as: node config `employeeId` > run-level `acting_employee_id` > null
- [x] 5.2 Pass effective employee id into `SpawnAgentPayload` as today
- [x] 5.3 Extend nodetypes tests to cover fallback precedence: override, default, absent

## 6. Legacy step router propagation

- [x] 6.1 Extend `workflow_step_router.go` `agent` action payload struct to include `EmployeeID *uuid.UUID`; forward it into the spawned `AgentRun`
- [x] 6.2 Extend `review` action payload to accept and forward `EmployeeID` into the triggered review record
- [x] 6.3 Extend `task` action payload to accept and forward `EmployeeID` into the dispatched task's spawned runs
- [x] 6.4 In each action, when `EmployeeID` is absent on the step payload, consult the workflow run's `acting_employee_id`; otherwise leave null
- [x] 6.5 Update `workflow_step_router_test.go` to cover: step-level override, run-level fallback, both absent (existing behavior preserved)

## 7. API + frontend surface

- [x] 7.1 Extend workflow run read DTOs (DAG + plugin) to expose `actingEmployeeId`
- [x] 7.2 Extend trigger create/update DTOs to accept `actingEmployeeId`; extend list/read DTOs to expose it plus `disabledReason`
- [x] 7.3 Update `lib/stores/workflow-trigger-store.ts` types and CRUD calls to round-trip `actingEmployeeId`
- [x] 7.4 Update `components/workflow/workflow-triggers-section.tsx` with an employee selector (loads from `lib/stores/employee-store.ts`) and a disabled-reason warning surface
- [x] 7.5 Extend `workflow-trigger-store.test.ts` to cover `actingEmployeeId` round-trip

## 8. Integration verification

- [x] 8.1 Add Go integration test: register one DAG trigger and one plugin trigger sharing a single IM command, both with `actingEmployeeId = E`; fire an IM event; assert both run records persist `acting_employee_id = E` and every spawned `agent_run` row has `employee_id = E`
- [x] 8.2 Add Go integration test: registrar rejects cross-project `actingEmployeeId`; dispatch rejects archived target
- [x] 8.3 Run `go test ./internal/employee/... ./internal/trigger/... ./internal/service/... ./internal/workflow/nodetypes/...` scoped to touched packages; record any pre-existing unrelated failures

## 9. Spec reconciliation

- [x] 9.1 Confirm deltas under `specs/employee-runtime-attribution`, `specs/workflow-engine`, and `specs/workflow-trigger-dispatch` are merged at archive time
- [x] 9.2 Validate no conflict with `bridge-trigger-dispatch-unification` deltas (this change depends on that one landing first)
