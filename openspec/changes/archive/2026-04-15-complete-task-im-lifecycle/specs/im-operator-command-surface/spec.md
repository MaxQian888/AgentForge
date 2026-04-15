## MODIFIED Requirements

### Requirement: Task and queue commands expose lightweight orchestration control
The IM command surface SHALL let users perform lightweight task workflow control and queue management through existing backend APIs. The `/task` family MUST keep `create`, `list`, `status`, `assign`, `decompose`, `move`, and `delete`, with `/task move` remaining the canonical task status-transition subcommand. Task command results MUST expose canonical task identity and enough readable follow-up guidance for the next supported lifecycle step, while any richer interactive affordance remains subject to provider readiness. The `/queue` family MUST support project-scoped queue listing and cancellation with readable results and preserved error semantics for non-cancellable entries.

#### Scenario: Task move transitions workflow status
- **WHEN** an IM user sends `/task move task-123 done`
- **THEN** the Bridge calls the canonical task status-transition endpoint for `task-123` with target status `done`
- **AND** the reply confirms the updated workflow state and includes the task identity needed for follow-up actions

#### Scenario: Task delete removes the task through the canonical backend workflow
- **WHEN** an IM user sends `/task delete task-123`
- **THEN** the Bridge calls the canonical task deletion endpoint for `task-123`
- **AND** the reply confirms which task was removed instead of returning a generic success message without task identity

#### Scenario: Queue list returns project-scoped admission summaries
- **WHEN** an IM user sends `/queue list queued`
- **THEN** the Bridge calls the canonical project queue list endpoint with the requested filter
- **AND** the reply summarizes matching queued entries with task, member or runtime identity, priority, and reason in a compact IM format

#### Scenario: Queue cancel preserves backend conflict semantics
- **WHEN** an IM user sends `/queue cancel entry-123`
- **THEN** the Bridge calls the canonical project queue cancel endpoint for `entry-123`
- **AND** the IM reply distinguishes successful cancellation from already-completed or invalid-entry conflicts instead of flattening all outcomes into generic success text
