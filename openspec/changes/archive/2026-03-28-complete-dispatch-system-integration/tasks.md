## 1. Budget Governance Integration

- [x] 1.1 Add `DispatchBudgetChecker` interface to `task_dispatch_service.go` with `CheckBudget(ctx, projectID, sprintID, requestedUsd) (*BudgetCheckResult, error)` method
- [x] 1.2 Implement `DispatchBudgetChecker` adapter in `budget_governance_service.go` that delegates to `CheckSprintBudget` and `CheckProjectBudget` in sequence
- [x] 1.3 Add `WithBudgetChecker()` setter to `TaskDispatchService` and call budget check in `spawnForTask()` before runtime spawn or queue admission
- [x] 1.4 Return structured `blocked` outcome with `guardrailType: budget` and `guardrailScope: sprint|project|task` when budget check fails
- [x] 1.5 Wire `BudgetGovernanceService` into `TaskDispatchService` in `routes.go` / `main.go` initialization
- [x] 1.6 Add budget re-check in `AgentService.promoteQueuedAdmission()` before promoting a queued entry
- [x] 1.7 Emit `budget.warning` WebSocket event when dispatch crosses 80% threshold but proceeds
- [x] 1.8 Write tests for budget-blocked dispatch paths in `task_dispatch_service_test.go` (sprint blocked, project blocked, warning-but-proceed)
- [x] 1.9 Write tests for budget re-check during queue promotion in `agent_service_test.go`

## 2. Priority Queue

- [x] 2.1 Add `Priority int` field to `AgentPoolQueueEntry` model and define constants `PriorityLow=0`, `PriorityNormal=10`, `PriorityHigh=20`, `PriorityCritical=30`
- [x] 2.2 Create database migration adding `priority INT NOT NULL DEFAULT 0` column to `agent_pool_queue_entries` and update composite index to `(project_id, status, priority DESC, created_at ASC)`
- [x] 2.3 Update `QueueAgentAdmission()` in `agent_pool_queue_repo.go` to accept and persist priority
- [x] 2.4 Update `ReserveNextQueuedByProject()` to order by `priority DESC, created_at ASC` (both DB and in-memory paths)
- [x] 2.5 Update `ListAllQueued()` and `ListQueuedByProject()` to return entries in priority order
- [x] 2.6 Add `Priority` field to `QueueAgentAdmissionInput` / `QueueAdmissionInput` / `DispatchSpawnInput` structs
- [x] 2.7 Write tests for priority-ordered reservation (higher first, FIFO tiebreaker, default=0)

## 3. Dispatch Preflight API

- [x] 3.1 Create `DispatchPreflightHandler` in `src-go/internal/handler/dispatch_preflight_handler.go` with `GET /api/v1/projects/:pid/dispatch/preflight` endpoint
- [x] 3.2 Implement preflight logic: validate task+member, check budget readiness via `BudgetGovernanceService`, check pool availability via `AgentService.PoolStats()`
- [x] 3.3 Define `PreflightResponse` struct with fields: `admissionLikely`, `budgetWarning`, `budgetBlocked`, `poolActive`, `poolAvailable`, `poolQueued`, `dispatchOutcomeHint`
- [x] 3.4 Register preflight route in `routes.go`
- [x] 3.5 Write tests for preflight handler (eligible, budget-warning, budget-blocked, non-agent-member)

## 4. Dispatch Observability API

- [x] 4.1 Create `DispatchStatsHandler` in `src-go/internal/handler/dispatch_stats_handler.go` with `GET /api/v1/projects/:pid/dispatch/stats` endpoint
- [x] 4.2 Implement stats aggregation queries: count by outcome, blocked-reason distribution, queue depth, median wait time from `agent_pool_queue_entries` and `agent_runs`
- [x] 4.3 Create `DispatchHistoryHandler` with `GET /api/v1/tasks/:tid/dispatch/history` endpoint returning chronological dispatch attempts per task
- [x] 4.4 Add `DispatchAttempt` model to record each dispatch attempt (outcome, trigger source, member, reason, timestamp) — persist via existing task progress service or new lightweight table
- [x] 4.5 Record dispatch attempts in `TaskDispatchService.Assign()` and `Spawn()` for all outcome branches
- [x] 4.6 Register stats and history routes in `routes.go`
- [x] 4.7 Write tests for stats and history handlers

## 5. Dispatch Outcome Contract Enhancement

- [x] 5.1 Add `GuardrailType` and `GuardrailScope` fields to `DispatchOutcome` model for machine-readable budget/pool/worktree classification
- [x] 5.2 Add optional `BudgetWarning` field to `DispatchOutcome` for warning-but-proceed scenarios
- [x] 5.3 Add `Priority` field to `DispatchOutcome.Queue` so consumers see the queued entry's priority
- [x] 5.4 Update `blockedResult()` to populate `GuardrailType` and `GuardrailScope` based on block reason
- [x] 5.5 Update IM action executor (`im_action_execution.go`) to render budget-blocked and priority-queued outcomes in IM messages
- [x] 5.6 Update workflow step router (`workflow_step_router.go`) to pass priority from workflow trigger config to dispatch input

## 6. Assignment Recommender Tests

- [x] 6.1 Write `assignment_recommender_test.go` covering: basic scoring with role/skill match, load penalty calculation, agent type bonus
- [x] 6.2 Add edge case tests: no members, all inactive, no matching skills, empty project
- [x] 6.3 Add tie-breaking test: verify deterministic ordering when scores are equal
- [x] 6.4 Add test for top-N limit (currently hardcoded to 3)

## 7. Frontend: Dispatch Status Surfaces

- [x] 7.1 Add `dispatchStatus` and `guardrailType` fields to the agent run type in `lib/stores/agent-store.ts`
- [x] 7.2 Add dispatch status badge column to agent run table in `app/(dashboard)/agents/page.tsx` using `event-badge-list` component pattern
- [x] 7.3 Add tooltip on blocked/queued badges showing machine-readable reason and guardrail scope
- [x] 7.4 Add priority column to queue table in agent monitor, displaying semantic label (low/normal/high/critical)
- [x] 7.5 Add preflight API call to `lib/stores/agent-store.ts` (`fetchDispatchPreflight(taskId, memberId)`)
- [x] 7.6 Create dispatch preflight confirmation dialog reusing `confirm-dialog` component — shows budget remaining, pool availability, admission likelihood
- [x] 7.7 Wire preflight dialog into task assignment flow (task detail panel assign button)

## 8. Frontend: Dispatch History & Stats

- [x] 8.1 Add `fetchDispatchHistory(taskId)` and `fetchDispatchStats(projectId)` to agent store
- [x] 8.2 Create dispatch history panel component reusing task-context-rail layout pattern — shows chronological dispatch attempts per task
- [x] 8.3 Add dispatch stats cards to agent monitor page (outcome distribution, queue depth, median wait time)
- [x] 8.4 Add WebSocket listener for `budget.warning` events to show toast notification with budget scope details
- [x] 8.5 Add i18n strings for dispatch status labels, guardrail types, and priority levels in `messages/en/` and `messages/zh-CN/`

## 9. Integration & Verification

- [x] 9.1 Run full Go test suite (`cd src-go && go test ./...`) and verify all new and existing tests pass
- [ ] 9.2 Run frontend test suite (`pnpm test`) and verify no regressions
- [x] 9.3 Verify dispatch preflight → assign → budget-blocked flow end-to-end with manual testing
- [x] 9.4 Verify priority queue ordering with multiple queued entries at different priority levels
