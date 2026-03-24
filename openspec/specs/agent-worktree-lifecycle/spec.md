# agent-worktree-lifecycle Specification

## Purpose
TBD - created by archiving change improve-agent-worktree-lifecycle-support. Update Purpose after archive.
## Requirements
### Requirement: Managed task worktrees use a canonical workspace contract
The system SHALL manage at most one canonical Git worktree per task under the configured worktree base path. For each task workspace, the manager MUST derive the canonical workspace path from the project slug and task identifier and MUST associate it with the canonical managed agent branch for that task.

#### Scenario: First allocation creates the canonical task workspace
- **WHEN** a task without an existing managed workspace is prepared for agent execution
- **THEN** the system creates the worktree at `<worktree-base>/<project-slug>/<task-id>`
- **THEN** the system checks out the canonical managed branch for that task
- **THEN** the allocation result identifies the workspace as managed for that task

#### Scenario: Repeated allocation reuses the existing managed workspace
- **WHEN** the same task is prepared again and its managed workspace already exists on disk and in Git worktree metadata
- **THEN** the system returns the existing workspace instead of creating a second worktree
- **THEN** the allocation result preserves the same canonical workspace path and managed branch

### Requirement: Managed worktree admission enforces capacity and ownership guardrails
The system SHALL reject new managed worktree creation when the configured active limit for managed task worktrees has been reached. The system MUST also reject canonical path conflicts when an existing directory or Git worktree entry does not belong to the managed workspace for the requested task.

#### Scenario: Active limit blocks a new task workspace
- **WHEN** the number of managed task worktrees already equals the configured active limit and a different task requests a workspace
- **THEN** the system returns a capacity error for worktree allocation
- **THEN** the system does not create a new branch or worktree for that task

#### Scenario: Path ownership conflict is rejected
- **WHEN** the canonical task workspace path already exists but is not a valid managed worktree for the requested task
- **THEN** the system returns a path-conflict or stale-state error
- **THEN** the system does not attach the conflicting path to the task

### Requirement: Managed worktree cleanup is idempotent and repair-oriented
The system SHALL provide a managed cleanup path for task worktrees that tolerates partially removed directories and stale Git metadata. Cleanup MUST remove the managed worktree checkout, prune stale Git worktree metadata, and only remove the canonical managed branch that belongs to the target task.

#### Scenario: Normal cleanup removes the managed workspace and branch
- **WHEN** cleanup is invoked for a healthy managed task workspace
- **THEN** the system removes the worktree checkout from the filesystem
- **THEN** the system prunes the corresponding Git worktree metadata
- **THEN** the system removes the canonical managed branch for that task

#### Scenario: Cleanup succeeds for a partially missing workspace
- **WHEN** cleanup is invoked for a task whose managed worktree directory is already missing but stale Git metadata remains
- **THEN** the system treats the cleanup as successful repair instead of failing on the missing directory
- **THEN** the system prunes stale Git metadata and clears the managed workspace from its view

### Requirement: The system can identify stale managed worktree state
The system SHALL provide inspection or garbage-collection support that can identify managed worktree state that is stale relative to Git metadata or runtime ownership. A stale workspace result MUST include enough task and path information for the backend or an operator to clean it up safely.

#### Scenario: Inspection marks a workspace as stale
- **WHEN** the system inspects managed worktree state for a task and finds a missing workspace directory or mismatched Git metadata
- **THEN** the inspection result marks the workspace as stale
- **THEN** the result includes the task identifier and canonical workspace path for cleanup

#### Scenario: Garbage collection repairs stale managed state
- **WHEN** garbage collection is invoked for a stale managed workspace
- **THEN** the system removes any remaining managed Git worktree metadata for that task
- **THEN** the system leaves no managed workspace entry for that stale task state

