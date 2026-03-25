## MODIFIED Requirements

### Requirement: Agent assignment can trigger a real task dispatch workflow
The system SHALL treat assignment of a task to an active agent member as a task-centered dispatch request instead of only updating assignee fields. When an authenticated caller assigns a task to an agent member, the system MUST persist the assignment, keep task workflow state aligned with the assignment, evaluate AgentPool admission readiness for that task/member/project combination, and return a structured dispatch outcome for the same request.

#### Scenario: Assigning a task to an agent starts dispatch immediately
- **WHEN** an authenticated caller assigns an existing task to an active agent member in the same project, the task has no active agent run, and AgentPool admission has an immediate slot
- **THEN** the system persists the task assignee as that agent member
- **THEN** the system returns the updated task together with a dispatch outcome of `started`
- **THEN** the dispatch outcome includes the started agent run or an equivalent runtime reference for that task

#### Scenario: Assigning a task to an agent queues dispatch
- **WHEN** an authenticated caller assigns an existing task to an active agent member in the same project, the task has no active agent run, and AgentPool admission has no immediate slot
- **THEN** the system persists the task assignee as that agent member
- **THEN** the system returns a dispatch outcome of `queued`
- **THEN** the dispatch outcome includes an admission or queue reference that identifies the queued request for that task

#### Scenario: Assigning a task to a human skips dispatch
- **WHEN** an authenticated caller assigns an existing task to a human member
- **THEN** the system persists the assignment normally
- **THEN** the system returns a dispatch outcome of `skipped`
- **THEN** the system MUST NOT create or start an agent run for that assignment

### Requirement: Assignment and dispatch outcomes are visible to synchronous and realtime consumers
The system SHALL expose assignment and dispatch as a layered result, so API clients, WebSocket consumers, notifications, and IM command handlers can distinguish between assignment success, queued admission, and runtime startup success. Successful dispatch MUST continue to emit runtime lifecycle signals, while queued or blocked dispatch attempts MUST emit explicit results that consumers can render without inferring from missing runtime events.

#### Scenario: Successful agent assignment emits assignment and runtime feedback
- **WHEN** an assignment request dispatches an agent successfully
- **THEN** the system emits the task assignment signal for the relevant task/project scope
- **THEN** the system emits the runtime-started lifecycle signal for the same task/project scope
- **THEN** synchronous callers receive the same dispatch outcome reflected in the API response

#### Scenario: Queued assignment emits an explicit non-started result
- **WHEN** an assignment request succeeds but AgentPool admission queues the task before runtime startup
- **THEN** the system keeps the task assignment result distinguishable from runtime startup
- **THEN** the system emits an explicit queued dispatch signal or notification for the relevant task/project scope
- **THEN** synchronous callers receive the same queued outcome in the API response

#### Scenario: Blocked dispatch emits an explicit non-started result
- **WHEN** an assignment request updates the task assignee but startup is blocked before a runtime begins
- **THEN** the system keeps the task assignment result distinguishable from runtime startup
- **THEN** the system emits an explicit blocked dispatch signal or notification for the relevant task/project scope
- **THEN** consumers MUST NOT need to infer the blocked state solely from the absence of `agent.started`
