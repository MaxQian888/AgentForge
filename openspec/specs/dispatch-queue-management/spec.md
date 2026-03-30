# dispatch-queue-management Specification

## Purpose
Define operator-facing queue roster and cancellation requirements for queued dispatch entries, including lifecycle updates and realtime visibility.
## Requirements
### Requirement: Operators can cancel queued dispatch entries via API
The system SHALL expose an authenticated endpoint to cancel a specific queued dispatch entry within a project scope. Cancellation MUST transition the entry to `cancelled` status, emit a realtime event, and return the updated entry state. Cancellation MUST fail gracefully if the entry has already been promoted or is in a terminal state.

#### Scenario: Successful queue entry cancellation
- **WHEN** an authenticated operator sends a DELETE request to cancel a queued entry that is currently in `queued` status
- **THEN** the system transitions the entry status from `queued` to `cancelled`
- **THEN** the system emits an `agent.queue.cancelled` WebSocket event for the project scope
- **THEN** the synchronous response returns the updated queue entry with `cancelled` status and a cancellation reason

#### Scenario: Cancellation of already-promoted entry returns conflict
- **WHEN** an authenticated operator sends a DELETE request to cancel a queue entry that has already been promoted to `admitted` or `promoted` status
- **THEN** the system returns HTTP 409 Conflict
- **THEN** the response body includes the current entry status and a message indicating the entry can no longer be cancelled
- **THEN** the entry status is NOT modified

#### Scenario: Cancellation of non-existent entry returns not found
- **WHEN** an authenticated operator sends a DELETE request with an entry ID that does not exist in the project's queue
- **THEN** the system returns HTTP 404 Not Found

#### Scenario: Pool stats broadcast after cancellation
- **WHEN** a queue entry is successfully cancelled
- **THEN** the system broadcasts an updated pool stats event for the project
- **THEN** the queued count in the pool summary reflects the removal

### Requirement: Operators can list queued entries for a project
The system SHALL expose an authenticated endpoint to list all queued dispatch entries for a project, ordered by priority descending then creation time ascending. The response MUST include each entry's task identity, member identity, runtime tuple, priority, enqueue time, queue reason, and current status.

#### Scenario: List queued entries for a project with entries
- **WHEN** an authenticated operator requests the queue list for a project that has queued entries
- **THEN** the system returns entries ordered by priority descending, then `created_at` ascending
- **THEN** each entry includes: entry ID, task ID, member ID, runtime, provider, model, role ID, priority, budget USD, reason, status, created at, and updated at

#### Scenario: List queued entries for a project with no entries
- **WHEN** an authenticated operator requests the queue list for a project with no queued entries
- **THEN** the system returns an empty array with HTTP 200

#### Scenario: Queue list supports status filtering
- **WHEN** an authenticated operator requests the queue list with a `status` query parameter (e.g., `?status=queued`)
- **THEN** the system returns only entries matching the specified status
- **THEN** ordering remains priority descending, then creation time ascending
