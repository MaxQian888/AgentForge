## 1. Automation action contract

- [x] 1.1 Extend automation rule parsing and validation so `start_workflow` is a supported action type, requires a workflow plugin identifier, and rejects malformed workflow-start configs before rule persistence.
- [x] 1.2 Update automation authoring consumers and request/response contracts so project admins can configure `start_workflow` without relying on unsupported raw action values.

## 2. Canonical workflow-start execution

- [x] 2.1 Add a dedicated automation workflow-start seam that reuses `WorkflowExecutionService.Start(...)` and canonical trigger payload shaping instead of generic plugin invocation.
- [x] 2.2 Add duplicate-scope guards and structured action verdict shaping for automation-triggered workflow starts, including plugin identity, run identity, and machine-readable reason metadata.

## 3. Scheduler-backed due-date truth

- [x] 3.1 Refactor due-date automation evaluation to return structured summary counts for evaluated tasks, matched rules, and downstream workflow start outcomes.
- [x] 3.2 Update `automation-due-date-detector` scheduler handling and run payload shaping so scheduler summaries and metrics expose downstream orchestration truth instead of scan-only status.

## 4. Observability and consumer alignment

- [x] 4.1 Align automation log persistence, API/store payloads, and relevant consumers so workflow-start outcomes are available as structured detail rather than free-form text inference.
- [x] 4.2 Update adjacent docs and lightweight operator-facing surfaces so `start_workflow` is documented as the canonical automation path for workflow orchestration while `invoke_plugin` remains generic.

## 5. Verification

- [x] 5.1 Add focused Go tests for workflow-start rule validation, canonical workflow run creation, duplicate blocking, and structured automation log detail.
- [x] 5.2 Add or update scheduler and frontend/store tests covering due-date summary metrics and automation authoring/log consumers for `start_workflow`.
- [x] 5.3 Run targeted verification for affected Go automation/scheduler packages plus touched frontend/store tests, then record any remaining out-of-scope gaps truthfully.
