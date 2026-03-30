## Why

The Go dispatcher control plane, agent pool queue, and budget governance systems are functionally complete for the core dispatch lifecycle. However, two operator-facing gaps remain: queued entries cannot be cancelled via API, and budget consumption lacks dedicated query endpoints outside the dashboard widget system. These gaps prevent operators from fully managing dispatch queues and budget state as required by the agent-pool-control-plane and dispatch-budget-governance specs.

## What Changes

- **Queue cancellation endpoint**: Add `DELETE /api/v1/projects/:pid/queue/:entryId` to allow operators and automation rules to cancel queued dispatch entries. Emits `agent.queue.cancelled` WebSocket event and updates queue entry status to `cancelled`.
- **Budget query endpoints**: Add `GET /api/v1/projects/:pid/budget/summary` returning current budget consumption across task, sprint, and project scopes with threshold status. Add `GET /api/v1/sprints/:sid/budget` for sprint-scoped budget detail.
- **Queue list endpoint for operators**: Add `GET /api/v1/projects/:pid/queue` to list current queued entries with priority ordering, exposing the data already available in the repository layer.

## Capabilities

### New Capabilities

- `dispatch-queue-management`: Operator-facing queue cancellation, listing, and lifecycle control for queued dispatch entries.
- `budget-query-api`: Dedicated budget consumption query endpoints outside dashboard widgets, providing real-time budget state for operators and automation consumers.

### Modified Capabilities

- `agent-pool-control-plane`: Add queue cancellation and operator queue listing to the existing pool control plane requirements.
- `dispatch-budget-governance`: Add dedicated budget query API surface to the existing budget governance requirements.

## Impact

- **Go handlers**: New handler files for queue management and budget query.
- **Go routes**: New routes in `routes.go` under project group.
- **Go service layer**: Extend `AgentService` with cancel-queue-entry method; extend `BudgetGovernanceService` with summary query.
- **WebSocket events**: New `agent.queue.cancelled` event type.
- **Frontend**: Consumers can integrate queue cancel and budget query once available (not in scope for this change).
