## Why

The Go dispatch system has its core service layer (`TaskDispatchService`, `AgentService`, pool/admission controllers) in place, but several critical integration points remain disconnected: budget governance is defined in spec and service code but never called during dispatch; the assignment recommender has zero test coverage; the frontend lacks dispatch-specific status surfaces for individual agent runs; and queue promotion has no priority mechanism. These gaps mean users can silently exceed budgets, dispatch failures are invisible in the UI, and scoring bugs in recommendation go undetected. Completing these integrations now prevents compounding drift as more consumers (IM, workflow, scheduler) depend on the dispatch contract.

## What Changes

- **Integrate budget governance into the dispatch flow**: wire `BudgetGovernanceService` checks into `TaskDispatchService.Assign()`, `Spawn()`, and queue promotion paths so dispatch respects task/sprint/project budget limits before starting a runtime.
- **Add budget pre-check API endpoint**: expose a `GET /api/v1/projects/:pid/dispatch/preflight` endpoint that returns remaining budget, estimated cost, and admission readiness before a caller commits to dispatch.
- **Expose dispatch status in the frontend agent monitor**: add a dispatch status column to the agent run table and a dispatch history panel showing blocked/queued/started outcomes with machine-readable reasons.
- **Implement priority-aware queue admission**: extend `AgentPoolQueueRepository` with a priority field and update `ReserveNextQueuedByProject()` to respect priority ordering, enabling urgent tasks to bypass the FIFO queue.
- **Add assignment recommender test coverage**: write comprehensive tests for `AssignmentRecommender.Recommend()` covering scoring, edge cases, tie-breaking, and empty-member scenarios.
- **Add dispatch metrics endpoint**: expose `GET /api/v1/projects/:pid/dispatch/stats` returning blocked rate, average queue depth, median wait time, and dispatch success rate.
- **Frontend dispatch preflight dialog**: add a confirmation dialog before agent dispatch that shows budget remaining, pool availability, and estimated cost.
- **Reuse existing components**: leverage shared `platform-badge`, `event-badge-list`, `confirm-dialog`, and task-context-rail patterns for dispatch UI rather than building new components.

## Capabilities

### New Capabilities
- `dispatch-preflight-api`: Backend preflight endpoint returning budget readiness, pool availability, and cost estimation before dispatch commitment.
- `dispatch-observability`: Dispatch metrics, history, and status surfaces for operators — API endpoints and frontend panels.
- `dispatch-priority-queue`: Priority-aware admission queue replacing FIFO ordering, with configurable priority levels per task urgency.

### Modified Capabilities
- `agent-task-dispatch`: Integrate budget governance checks into the Assign/Spawn paths; add budget-blocked as a first-class guardrail classification in dispatch outcomes.
- `dispatch-budget-governance`: Wire existing budget governance service into live dispatch flow; add preflight budget check before admission.
- `agent-pool-control-plane`: Add priority field to queue entries; update promotion logic to respect priority ordering.

## Impact

- **Backend (src-go)**: `task_dispatch_service.go`, `agent_service.go`, `agent_handler.go`, `task_handler.go`, `budget_governance_service.go`, `agent_pool_queue_repo.go`, `pool/admission.go`, `routes.go`, `main.go` — new handler/service wiring.
- **Frontend**: `app/(dashboard)/agents/page.tsx`, new `components/agents/dispatch-*` components, `lib/stores/agent-store.ts` — dispatch status displays and preflight dialog.
- **API Surface**: Two new endpoints (`dispatch/preflight`, `dispatch/stats`), modified dispatch outcome contract to include budget guardrail metadata.
- **Database**: Migration to add `priority` column to `agent_pool_queue_entries` table.
- **Tests**: New test files for `assignment_recommender`, `dispatch_preflight`, `dispatch_stats`; extended tests for budget-integrated dispatch paths.
