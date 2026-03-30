## Context

The Go dispatch control plane, agent pool, and budget governance are fully implemented for core dispatch lifecycle (assign → spawn → pool admission → queue → promote). Two operator-facing API gaps remain:

1. **Queue cancellation**: The repository layer supports `CompleteQueuedEntry()` with `cancelled` status, but no HTTP endpoint exposes this. Operators and automation rules cannot cancel queued dispatch entries externally.
2. **Budget query API**: Budget state is only queryable via dashboard widget endpoints (`/dashboard/widgets/budget_consumption`). The spec requires budget data to feed consumers and operators — dedicated endpoints make this data accessible without requiring a dashboard widget configuration.
3. **Queue listing**: While `ListQueuedByProject` exists in the repository, no dedicated operator-facing queue list endpoint exists (pool summary includes counts but not individual entries).

## Goals / Non-Goals

**Goals:**
- Add queue cancellation endpoint with proper authorization and event broadcasting
- Add queue listing endpoint for operator visibility
- Add budget summary endpoint for direct budget consumption queries
- Maintain consistency with existing dispatch patterns (handler → service → repository)
- Emit appropriate WebSocket events for queue state changes

**Non-Goals:**
- Frontend integration (future change)
- Budget allocation management (setting/modifying budgets)
- Queue reordering or priority modification after enqueue
- Batch queue operations

## Decisions

### Queue cancellation as service method on AgentService

The `AgentService` already owns queue admission and promotion. Adding `CancelQueueEntry(projectID, entryID)` keeps queue lifecycle in one place. The method calls the existing `CompleteQueuedEntry()` repository method with status `cancelled`, then broadcasts a WebSocket event.

**Alternative considered**: Separate QueueManagementService — rejected because it would duplicate the pool/queue context already held by AgentService.

### Dedicated handler files for new endpoints

- `queue_management_handler.go` for queue list and cancel operations
- `budget_query_handler.go` for budget summary queries

This follows the existing pattern where each concern gets its own handler file (e.g., `dispatch_preflight_handler.go`, `dispatch_observability_handler.go`).

### Budget summary aggregates from existing services

The `BudgetGovernanceService` already has `CheckSprintBudget` and `CheckProjectBudget`. The new summary endpoint calls these existing methods plus a new `GetProjectBudgetSummary()` method that aggregates across scopes. No new data model needed — it composes existing budget check results.

### Route placement under project group

All new routes go under the authenticated project group:
- `GET /api/v1/projects/:pid/queue` — list queued entries
- `DELETE /api/v1/projects/:pid/queue/:entryId` — cancel queued entry
- `GET /api/v1/projects/:pid/budget/summary` — project budget overview
- `GET /api/v1/sprints/:sid/budget` — sprint budget detail

This matches the existing route structure where dispatch endpoints live under the project scope.

## Risks / Trade-offs

- **Race condition on cancel**: A queue entry might be promoted between the cancel request and the repository update. Mitigation: `CompleteQueuedEntry` checks current status is still `queued` before updating; if already promoted, return 409 Conflict.
- **Budget summary performance**: Aggregating across all tasks in a project could be slow for large projects. Mitigation: Use existing indexed queries; consider caching if needed in future.
- **WebSocket event proliferation**: Adding `agent.queue.cancelled` is a new event type. Mitigation: Follows the same pattern as existing queue events; frontend consumers can subscribe selectively.
