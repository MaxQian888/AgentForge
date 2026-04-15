# im-action-execution Specification

## Purpose
Define the canonical backend contract for executing shared IM actions against task dispatch, task decomposition, and review workflows with truthful started, completed, blocked, or failed outcomes.
## Requirements
### Requirement: Shared IM actions SHALL execute canonical backend workflows
The system SHALL treat normalized IM actions as executable backend workflow requests instead of placeholder acknowledgements. When the Bridge submits a supported shared action through `/api/v1/im/action`, the backend MUST execute the corresponding task, agent, or review operation, and it MUST return a canonical action result that truthfully reports whether the operation started, completed, blocked, or failed.

#### Scenario: Assign-agent action dispatches the real task workflow
- **WHEN** the Bridge submits an `assign-agent` action for a valid task with a preserved reply target
- **THEN** the backend assigns the task through the existing task-dispatch or agent-spawn workflow instead of returning a placeholder acknowledgement
- **AND** the action result reports the resulting task identity, dispatch status, and run identity when an Agent run is created

#### Scenario: Decompose action executes task decomposition
- **WHEN** the Bridge submits a `decompose` action for a task that can be decomposed
- **THEN** the backend invokes the task decomposition workflow
- **AND** the action result reports the parent task plus the created or updated decomposition outcome instead of only confirming receipt

#### Scenario: Review action updates the real review state
- **WHEN** the Bridge submits `approve` or `request-changes` for a valid review entity
- **THEN** the backend updates the persisted review outcome through the existing review workflow
- **AND** the returned action result truthfully reflects the new review state

### Requirement: Action outcomes SHALL remain explicit when execution is blocked or invalid
If a normalized IM action cannot be executed because the entity is missing, the transition is no longer valid, the binding is stale, or a downstream workflow refuses the request, the system MUST return an explicit blocked or failed outcome. It MUST NOT report success for operations that were not actually performed.

#### Scenario: Stale review action returns an explicit terminal failure
- **WHEN** a user clicks an IM review action for a review that is already completed in an incompatible state
- **THEN** the backend returns a terminal blocked or failed action outcome
- **AND** the user-visible response explains that the requested transition was not applied

#### Scenario: Missing action entity is rejected without fake success
- **WHEN** the Bridge submits an action whose task, run, or review entity cannot be resolved
- **THEN** the backend rejects the action explicitly
- **AND** the returned action outcome does not claim that assignment, decomposition, or approval succeeded

#### Scenario: Action result preserves reply-target-aware completion context
- **WHEN** an executable or blocked action result is returned to the Bridge
- **THEN** the result preserves the canonical reply target and metadata needed for follow-up delivery
- **AND** the Bridge can render the terminal outcome back into the originating conversation without guessing a new destination

### Requirement: IM action execution SHALL preserve bridge binding and reply-target lineage
When IM Bridge submits an executable action through the backend, the backend SHALL preserve the originating bridge binding and reply-target lineage needed for follow-up progress and terminal delivery. The backend MUST return enough canonical context for the IM Bridge or control plane to deliver the eventual outcome back to the originating conversation without inventing a new destination.

#### Scenario: Assign-agent action keeps the originating bridge binding
- **WHEN** an IM-originated `assign-agent` action starts a real backend dispatch workflow
- **THEN** the backend action result preserves the bridge binding or reply-target context associated with that originating IM conversation
- **THEN** later progress and terminal updates can be routed through the control plane to the same conversation

#### Scenario: Review action terminal result returns to the same conversation
- **WHEN** an IM-originated review action completes successfully or is blocked
- **THEN** the backend action result preserves the reply-target-aware completion context
- **THEN** the IM Bridge can render the final result in the originating conversation without resolving a new reply target

### Requirement: Workflow success and delivery settlement SHALL remain distinct
The backend SHALL distinguish between successful execution of a workflow and successful delivery of the follow-up IM message. If a workflow completes but the terminal IM delivery cannot be settled, the action result and diagnostics MUST preserve that distinction instead of reporting a fully successful end-to-end IM completion.

#### Scenario: Workflow succeeds but terminal delivery is blocked
- **WHEN** an IM action starts or completes the requested backend workflow but the bound bridge instance is unavailable for the terminal response
- **THEN** the backend records the workflow outcome and the delivery settlement failure separately
- **THEN** operators can see that the action logic succeeded even though the user-facing IM reply did not settle

### Requirement: Message conversion actions SHALL execute canonical wiki and task workflows
The system SHALL treat `save-as-doc` and `create-task` as executable shared IM actions backed by the existing wiki and task creation workflows. When the Bridge submits either action through `/api/v1/im/action`, the backend MUST create the corresponding wiki page or task instead of returning a placeholder acknowledgement, and it MUST return a canonical action result containing the created entity reference needed for follow-up delivery.

#### Scenario: Save-as-doc action creates a wiki page through the canonical backend workflow
- **WHEN** the Bridge submits `save-as-doc` with a valid project entity and source message metadata
- **THEN** the backend resolves the project's wiki space and creates a wiki page through the existing wiki creation workflow
- **AND** the returned IM action result includes a link or identifier for the created page

#### Scenario: Create-task action creates a backlog task through the canonical backend workflow
- **WHEN** the Bridge submits `create-task` with a valid project entity and source message metadata
- **THEN** the backend creates a task through the existing task creation workflow instead of an IM-only shortcut path
- **AND** the returned IM action result includes the created task identity and task link

### Requirement: Message conversion action results SHALL preserve source context for IM follow-up delivery
The backend SHALL preserve reply-target lineage and message-derived metadata when completing message conversion actions so the Bridge can render the final outcome back into the originating IM conversation without inventing a new destination or losing the source content summary.

#### Scenario: Save-as-doc result returns to the originating reply target
- **WHEN** a `save-as-doc` action completes successfully for a message that originated from Slack thread context
- **THEN** the IM action result preserves that reply-target-aware completion context
- **AND** the Bridge can post the resulting page link back into the same Slack thread

#### Scenario: Create-task failure remains source-aware
- **WHEN** a `create-task` action fails because task creation workflow is unavailable or rejects the request
- **THEN** the backend returns an explicit failed IM action outcome
- **AND** the result still preserves the originating reply target and source message metadata needed for a truthful user-visible failure response

### Requirement: Task transition actions SHALL execute canonical task lifecycle workflows
The system SHALL treat canonical task transition actions submitted through `/api/v1/im/action` as executable backend task lifecycle requests. When the Bridge submits a supported task transition action such as `transition-task`, the backend MUST execute the corresponding task status transition through the canonical task workflow surface and MUST return a truthful action result containing the updated task state or the explicit blocked or failed reason.

#### Scenario: Transition-task action updates the real task state
- **WHEN** the Bridge submits `transition-task` for a valid task together with a supported target status and preserved reply target
- **THEN** the backend transitions that task through the canonical task status workflow instead of returning a placeholder acknowledgement
- **AND** the action result reports the updated task identity and resulting workflow status

#### Scenario: Invalid task transition returns an explicit non-success outcome
- **WHEN** the Bridge submits `transition-task` for a missing task, an unsupported target status, or an invalid status transition
- **THEN** the backend returns an explicit blocked or failed action outcome
- **AND** the result does not claim that the task state changed when no canonical transition was applied

#### Scenario: Transition-task result preserves reply-target lineage for later follow-up
- **WHEN** an IM-originated task transition action succeeds or is blocked
- **THEN** the returned IM action result preserves the canonical reply target and task identity associated with that originating conversation
- **AND** later task or workflow follow-up delivery can reuse that lineage without inventing a new destination

