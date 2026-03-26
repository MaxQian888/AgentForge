# review-state-transitions Specification

## Purpose
TBD - created by archiving change close-review-pipeline-loop. Update Purpose after archive.
## Requirements
### Requirement: Human approval transition appends a decision record without modifying review evidence
The system SHALL provide a dedicated `ApproveReview` operation that records a human approval decision as an appended `ReviewDecision` entry in the review's `executionMetadata.decisions` array. This operation MUST update `status` and `recommendation` to reflect approval, but SHALL NOT overwrite `findings`, `summary`, `costUSD`, or any per-plugin provenance data that was written by the automated bridge result.

#### Scenario: Operator approves a pending_human review
- **WHEN** an authenticated operator submits an approval for a review in `pending_human` state
- **THEN** the system appends a `ReviewDecision` record (actor, action: approved, timestamp, optional comment) to the review's decisions array
- **THEN** `status` transitions to the terminal approved state and `recommendation` is updated accordingly
- **THEN** the original `findings`, `summary`, `costUSD`, and plugin provenance remain intact and queryable

#### Scenario: Approval on a non-pending_human review is rejected
- **WHEN** an authenticated caller attempts to approve a review that is not in `pending_human` state
- **THEN** the backend returns an error indicating the transition is not valid for the current state
- **THEN** the review record is unchanged

### Requirement: Request-changes transition records actionable feedback without overwriting evidence
The system SHALL provide a dedicated `RequestChangesReview` operation that appends a `ReviewDecision` entry (action: request_changes, comment) and transitions the review to the request-changes state without touching original automated evidence.

#### Scenario: Reviewer requests changes on a pending_human review
- **WHEN** an authenticated reviewer submits a request-changes action with a comment for a review in `pending_human` state
- **THEN** the system appends a `ReviewDecision` record with the reviewer's identity, action `request_changes`, comment text, and timestamp
- **THEN** `status` transitions to the request-changes state
- **THEN** `findings`, `summary`, `costUSD`, and plugin provenance are unchanged

#### Scenario: Request-changes without comment is still valid
- **WHEN** a reviewer submits a request-changes transition with an empty comment field
- **THEN** the transition is accepted; the appended `ReviewDecision` records an empty comment
- **THEN** the review state transitions correctly

### Requirement: False-positive marking records a dismissal decision without removing findings
The system SHALL provide a `MarkFalsePositive` operation that appends a `ReviewDecision` record (action: false_positive, reason, optional finding references) and transitions the review or finding to a dismissed state. Original finding content SHALL remain queryable for audit purposes.

#### Scenario: Operator dismisses a finding as a false positive
- **WHEN** an authenticated operator marks one or more findings as false positives with an optional reason
- **THEN** the system appends a `ReviewDecision` record identifying the dismissed finding IDs, the reason, and the actor
- **THEN** the dismissed findings are marked with a `dismissed: true` flag but their content is preserved
- **THEN** a `review.updated` event is emitted

#### Scenario: False-positive mark does not change overall review status
- **WHEN** findings are marked as false positives on a completed review
- **THEN** the review's overall `status` and terminal state are unchanged
- **THEN** only the per-finding `dismissed` flag and the decisions array are updated

### Requirement: All human transition operations emit review events
The system SHALL emit a typed WebSocket event after each successful human transition (approve, request-changes, false-positive) so that connected frontends and IM bridge instances can update their review state without polling.

#### Scenario: Approve transition emits review.completed event
- **WHEN** an `ApproveReview` transition succeeds
- **THEN** the backend emits a `review.completed` WebSocket event containing the updated review identifier and final state

#### Scenario: Request-changes transition emits review.updated event
- **WHEN** a `RequestChangesReview` transition succeeds
- **THEN** the backend emits a `review.updated` WebSocket event with the review identifier and new state

