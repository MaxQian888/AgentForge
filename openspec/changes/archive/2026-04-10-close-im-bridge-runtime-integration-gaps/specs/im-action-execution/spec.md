## ADDED Requirements

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
