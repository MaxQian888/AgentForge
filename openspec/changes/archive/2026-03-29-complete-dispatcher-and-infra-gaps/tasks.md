## 1. Queue Management Service Layer

- [x] 1.1 Add `CancelQueueEntry(ctx, projectID, entryID, reason)` method to AgentService that validates entry status is `queued`, calls `CompleteQueuedEntry` with `cancelled` status, emits `agent.queue.cancelled` WebSocket event, and broadcasts updated pool stats
- [x] 1.2 Add `ListQueueEntries(ctx, projectID, statusFilter)` method to AgentService that delegates to `ListQueuedByProject` with optional status filtering and returns entries in priority order

## 2. Queue Management HTTP Handler

- [x] 2.1 Create `internal/handler/queue_management_handler.go` with `QueueManagementHandler` struct, constructor, `List` method (GET), and `Cancel` method (DELETE)
- [x] 2.2 Wire `QueueManagementHandler` into routes.go: `GET /api/v1/projects/:pid/queue` → List, `DELETE /api/v1/projects/:pid/queue/:entryId` → Cancel

## 3. Budget Query Service Layer

- [x] 3.1 Add `GetProjectBudgetSummary(ctx, projectID)` method to BudgetGovernanceService that aggregates project-level allocation/spend, active sprint budget state, and task-level metrics into a `ProjectBudgetSummary` response struct
- [x] 3.2 Add `GetSprintBudgetDetail(ctx, sprintID)` method to BudgetGovernanceService that returns sprint allocated budget, total spend, per-task breakdown, and threshold status

## 4. Budget Query HTTP Handler

- [x] 4.1 Create `internal/handler/budget_query_handler.go` with `BudgetQueryHandler` struct, constructor, `ProjectSummary` method, and `SprintDetail` method
- [x] 4.2 Wire `BudgetQueryHandler` into routes.go: `GET /api/v1/projects/:pid/budget/summary` → ProjectSummary, `GET /api/v1/sprints/:sid/budget` → SprintDetail

## 5. WebSocket Event Registration

- [x] 5.1 Add `EventAgentQueueCancelled` constant to WebSocket event types (ws package) and ensure the event payload includes entry ID, task ID, member ID, project ID, and cancellation reason

## 6. Response Models

- [x] 6.1 Add `ProjectBudgetSummary` and `SprintBudgetDetail` DTOs to model package with fields for allocated, spent, remaining, threshold status, scope breakdowns, and per-task entries
- [x] 6.2 Add `QueueEntryDTO` response type if not already exposed, ensuring all fields from the spec (entry ID, task ID, member ID, runtime, provider, model, role ID, priority, budget USD, reason, status, timestamps) are included

## 7. Testing

- [x] 7.1 Add unit tests for `CancelQueueEntry` covering: successful cancel, cancel of promoted entry (409), cancel of non-existent entry (404), pool stats broadcast after cancel
- [x] 7.2 Add unit tests for `GetProjectBudgetSummary` and `GetSprintBudgetDetail` covering: active budgets, no budgets configured, real-time spend accuracy
- [x] 7.3 Add handler tests for all new endpoints: queue list, queue cancel, project budget summary, sprint budget detail — covering success, auth required, and error cases
