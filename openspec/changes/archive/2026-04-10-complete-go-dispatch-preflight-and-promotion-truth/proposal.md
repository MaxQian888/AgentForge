## Why

The Go dispatch control-plane already handles assignment, manual spawn, queue admission, and queue promotion, but the operator-facing truth is still inconsistent across preflight, promotion revalidation, and observability. Preflight does not mirror the full canonical admission checks, promotion verdicts do not consistently flow into dispatch history, and realtime or stats consumers still need to infer too much from thin payloads.

## What Changes

- Canonicalize the read-only dispatch preflight contract so it reflects the same guardrail truth used by real dispatch decisions, including task-level budget pressure, current non-start verdicts, and advisory pool readiness without reserving resources.
- Route queued promotion revalidation through the same dispatch truth model used by immediate dispatch so recoverable re-queues, terminal failures, and successful promotions preserve consistent machine-readable verdicts.
- Persist promotion rechecks into dispatch observability so task history shows the latest queued, blocked, failed, and promoted outcomes instead of only the original admission attempt.
- Expand dispatch observability and realtime payloads to expose promotion success or failure metrics, queue lifecycle counts, and promoted run linkage without forcing consumers to reconstruct state from free-form reason strings.
- Tighten queue and history consumer contracts so operator APIs and realtime clients can distinguish queued, recoverable, promoted, and terminal queue states consistently.

## Capabilities

### New Capabilities
- None.

### Modified Capabilities
- `dispatch-preflight-api`: make advisory preflight mirror canonical dispatch guardrails and return truthful non-start verdict metadata.
- `agent-spawn-orchestration`: make queued promotion revalidation reuse canonical dispatch truth and record promotion verdicts consistently.
- `dispatch-observability`: extend stats and per-task history to include promotion outcomes and richer queue lifecycle diagnostics.
- `agent-pool-control-plane`: preserve promoted queue linkage and latest guardrail verdicts across queue lifecycle events and operator-facing payloads.

## Impact

- Go dispatch services: `src-go/internal/service/task_dispatch_service.go`, `src-go/internal/service/agent_service.go`
- Go handlers and contracts: `src-go/internal/handler/dispatch_preflight_handler.go`, `src-go/internal/handler/dispatch_observability_handler.go`
- Models and persistence: `src-go/internal/model/dispatch_attempt.go`, `src-go/internal/model/agent_pool.go`, `src-go/internal/repository/dispatch_attempt_repo.go`, `src-go/internal/repository/agent_pool_queue_repo.go`
- Realtime and API contracts: WebSocket event payload shaping plus `docs/api/openapi.yaml` and `docs/api/asyncapi.yaml`
