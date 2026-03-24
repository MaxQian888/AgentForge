## ADDED Requirements

### Requirement: Manual spawn reuses task assignment context
The system SHALL allow explicit agent spawn requests to reuse the task's current agent assignment context instead of requiring every caller to provide redundant member metadata. When a spawn request identifies a task but omits explicit member identity, the system MUST derive the dispatch target from the task's current assignee if that assignee is an active agent member for the same project.

#### Scenario: Task-scoped spawn resolves the assigned agent member
- **WHEN** a caller requests agent spawn for a task without explicitly providing `memberId`
- **THEN** the system reads the task's current assignee context before startup
- **THEN** the system starts runtime execution with that assigned agent member if the assignee is a valid active agent target
- **THEN** the request reuses the same startup and compensation semantics as other spawn flows

#### Scenario: Task-scoped spawn rejects tasks without a valid agent assignee
- **WHEN** a caller requests agent spawn for a task that is not currently assigned to an active agent member
- **THEN** the system MUST reject the request before runtime startup
- **THEN** the system MUST NOT create a new agent run for that request
- **THEN** the response explains that the task has no valid agent dispatch target
