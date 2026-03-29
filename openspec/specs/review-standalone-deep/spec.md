# review-standalone-deep Specification

## Purpose
Define how operators and IM users can trigger a deep review directly from a PR URL, even when no task binding exists yet, while still surfacing the result through the shared dashboard review workspace.

## Requirements
### Requirement: Deep review can be initiated from a bare PR URL without a pre-existing task binding
The system SHALL allow an authenticated caller to create a deep review by supplying only a `prUrl` (and optionally a `projectId`), with no `taskId` required. The system SHALL create a detached review record, return the new `reviewId` synchronously, and initiate the bridge deep-review run asynchronously. A detached review SHALL emit all standard review lifecycle events, SHALL NOT attempt to update any task state, and MUST remain queryable through the same dashboard review workspace contract used for task-bound reviews.

#### Scenario: Caller submits a standalone deep review request with only a PR URL
- **WHEN** an authenticated caller creates a review request containing a valid `prUrl` and no `taskId`
- **THEN** the backend creates a new review record in pending state with the supplied PR URL, assigns a `reviewId`, and returns it in the response
- **THEN** the bridge deep-review run is triggered asynchronously for the new review
- **THEN** no task state update is performed at any point in the review lifecycle

#### Scenario: Standalone deep review completes and emits events without task update
- **WHEN** a detached review (no taskId) completes through the bridge
- **THEN** the backend persists findings, executionMetadata, and recommendation normally
- **THEN** a `review.completed` or `review.pending_human` event is emitted
- **THEN** no task-related side effects are triggered

#### Scenario: Standalone deep review with unknown PR URL is still accepted
- **WHEN** a caller submits a standalone review request with a PR URL that does not correspond to any known task in the system
- **THEN** the backend accepts the request, creates a detached review record, and proceeds
- **THEN** the response includes the new `reviewId` for tracking

#### Scenario: Detached review appears in the shared dashboard detail surface
- **WHEN** a detached review is listed in the review backlog or opened from a direct review link
- **THEN** the dashboard renders it through the same shared review detail contract used for task-bound reviews
- **THEN** missing task metadata does not prevent operators from viewing findings, execution metadata, or decision history

### Requirement: IM /review deep command triggers a standalone deep review
The system SHALL accept a `/review deep <pr-url>` command from an IM user, create a standalone deep review for the supplied PR URL, and reply with a status card containing the review ID and a link to the shared review detail surface in the dashboard.

#### Scenario: IM user requests deep review by PR URL
- **WHEN** an IM user sends `/review deep <pr-url>` to the bridge
- **THEN** the bridge calls the standalone deep review creation API with the supplied URL
- **THEN** the bridge replies with a card showing the review ID, initial status (pending), and a link to view progress in the shared review workspace

#### Scenario: IM deep review command with invalid PR URL returns an error card
- **WHEN** an IM user sends `/review deep` with a malformed or empty URL
- **THEN** the bridge replies with an error card describing the invalid input without creating a review record
