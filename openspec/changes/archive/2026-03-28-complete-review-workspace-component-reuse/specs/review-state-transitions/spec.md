## MODIFIED Requirements

### Requirement: All human transition operations emit review events
The system SHALL emit a typed WebSocket event after each successful human transition (approve, request-changes, false-positive) so that connected frontends and IM bridge instances can update their review state without polling. The emitted payload MUST contain the updated review DTO needed for the shared review workspace to reconcile backlog and task-level views through one store update path.

#### Scenario: Approve transition emits review.completed event
- **WHEN** an `ApproveReview` transition succeeds
- **THEN** the backend emits a `review.completed` WebSocket event containing the updated review identifier and final state
- **THEN** the payload includes the updated review data required for shared dashboard surfaces to reflect the approval without an immediate refetch

#### Scenario: Request-changes transition emits review.updated event
- **WHEN** a `RequestChangesReview` transition succeeds
- **THEN** the backend emits a `review.updated` WebSocket event with the review identifier and new state
- **THEN** the payload includes the updated decision history and unchanged automated evidence so shared dashboard surfaces can update in place

#### Scenario: False-positive transition updates reusable review surfaces without polling
- **WHEN** one or more findings are marked as false positives successfully
- **THEN** the backend emits the corresponding updated review payload
- **THEN** backlog and task-level review surfaces consuming the shared workspace contract can reflect dismissed findings through the same store update path
