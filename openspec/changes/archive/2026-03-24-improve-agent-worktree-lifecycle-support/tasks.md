## 1. Worktree Lifecycle Contract

- [x] 1.1 Expand `src-go/internal/worktree` to derive canonical managed branch/path metadata for a task and return structured allocation results instead of only a raw path string.
- [x] 1.2 Add managed allocation guardrails for active-capacity limits, existing healthy workspace reuse, and canonical-path ownership conflicts.
- [x] 1.3 Implement idempotent cleanup plus stale Git metadata pruning and managed-branch removal, with focused unit tests for healthy and partially missing workspace cases.
- [x] 1.4 Add inspection or garbage-collection helpers that identify stale managed worktree state and cover the stale-state paths with tests.

## 2. Spawn Integration

- [x] 2.1 Update `src-go/internal/service/agent_service.go` and related interfaces so spawn acquires worktrees through the lifecycle-aware manager and uses the canonical branch/worktree metadata it returns.
- [x] 2.2 Map worktree capacity, ownership, and stale-state failures to clean spawn outcomes that do not leave task runtime metadata or active runs in an ambiguous state.
- [x] 2.3 Ensure cancellation and other terminal cleanup paths release managed worktrees through the same lifecycle contract only when the runtime actually owns them.

## 3. Verification And Wiring

- [x] 3.1 Thread the relevant config-driven limits and paths through server composition without changing the public spawn request payloads.
- [x] 3.2 Add or update focused Go tests covering spawn success with workspace reuse, spawn rejection on worktree denial, and end-to-end cleanup behavior.


