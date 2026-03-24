## ADDED Requirements

### Requirement: Agent spawn respects managed worktree guardrails
The spawn flow SHALL acquire task workspaces through the managed worktree lifecycle before bridge execution begins. If worktree allocation is denied because of capacity, path ownership, or unrecoverable stale state, the system MUST fail the spawn without leaving ambiguous task runtime metadata.

#### Scenario: Spawn surfaces worktree allocation denial
- **WHEN** an authenticated spawn request reaches worktree allocation and the manager returns a capacity, ownership, or stale-state error
- **THEN** the system does not mark the related task runtime as active
- **THEN** the system does not persist branch, worktree, or session metadata for the failed allocation attempt
- **THEN** the caller receives an error that reflects the worktree allocation failure

#### Scenario: Spawn reuses the healthy managed workspace for the task
- **WHEN** a spawn request targets a task whose canonical managed workspace already exists and is healthy
- **THEN** the system reuses that managed workspace instead of creating another checkout
- **THEN** the bridge execute request receives the canonical workspace path and branch for that task
