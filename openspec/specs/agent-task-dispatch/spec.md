# agent-task-dispatch Specification

## Purpose
Define the backend requirements for task-centered agent assignment dispatch, dispatch preflight validation, layered synchronous dispatch outcomes, and truthful IM feedback for assignment-triggered starts.
## Requirements
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

### Requirement: Dispatch preflight validates the target before runtime startup
The system MUST validate task dispatch targets before attempting to start agent runtime execution. The dispatch preflight MUST reject or block startup when the task does not exist, the assignee is not an active agent member for the task's project, or the task already has an active agent run.

#### Scenario: Assignment targets a non-agent or inactive member
- **WHEN** a caller attempts to trigger agent dispatch for a member that is inactive, not part of the task's project, or not typed as `agent`
- **THEN** the system MUST NOT start an agent run
- **THEN** the system returns a dispatch outcome of `blocked`
- **THEN** the outcome explains why the target cannot be dispatched

#### Scenario: Task already has an active agent run
- **WHEN** a caller assigns or dispatches a task that already has an agent run in `starting`, `running`, or `paused` state
- **THEN** the system MUST NOT create a duplicate run
- **THEN** the system returns a dispatch outcome of `blocked`
- **THEN** the outcome explains that the task already has an active agent run

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

### Requirement: IM task assignment commands reflect dispatch outcomes truthfully
The system MUST make IM-triggered task assignment reuse the same task dispatch workflow and outcome semantics as the canonical backend API.

#### Scenario: IM task assignment starts an agent
- **WHEN** an IM command assigns a task to an eligible agent target and dispatch starts successfully
- **THEN** the IM command path uses the same backend dispatch workflow as the HTTP assignment path
- **THEN** the user receives a success message that confirms both assignment and agent startup

#### Scenario: IM task assignment is blocked before startup
- **WHEN** an IM command assignment succeeds but dispatch is blocked by validation or runtime preflight
- **THEN** the user receives a result that confirms the task assignment outcome
- **THEN** the same result explains why agent startup did not begin
- **THEN** the command path MUST NOT claim that an agent was started

### Requirement: Assignment-triggered dispatch outcomes remain canonical across consumer contracts
The system SHALL expose assignment-triggered dispatch through one canonical outcome contract for synchronous API callers, realtime consumers, and IM bridge clients. The contract MUST preserve explicit `started`, `queued`, `blocked`, and `skipped` branches, together with any queue references and machine-readable guardrail details required to explain non-started outcomes.

#### Scenario: Queued assignment preserves queue metadata across surfaces
- **WHEN** assigning a task to an eligible agent member results in a queued dispatch outcome
- **THEN** the synchronous assignment response includes the authoritative queue reference and dispatch metadata for that request
- **THEN** the corresponding realtime and IM consumer contracts expose the same `queued` branch instead of collapsing it to generic success or idle text
- **THEN** consumers can identify that assignment succeeded even though runtime startup has not begun yet

#### Scenario: Human assignment remains an explicit skipped outcome
- **WHEN** a task assignment targets a human member and therefore does not attempt agent startup
- **THEN** the synchronous response returns a `skipped` dispatch outcome
- **THEN** IM and other consumer contracts preserve that `skipped` branch explicitly
- **THEN** consumers MUST NOT infer a human assignment indirectly from the absence of run or queue data

#### Scenario: Blocked assignment preserves a machine-readable dispatch reason
- **WHEN** task assignment succeeds but dispatch preflight blocks runtime startup
- **THEN** the synchronous response and realtime dispatch payload include the blocked outcome together with a machine-readable reason classification
- **THEN** IM consumer output identifies that assignment completed while startup did not begin
- **THEN** consumers can distinguish budget, pool, worktree, and target-validation blocks without relying on free-form reason strings alone

