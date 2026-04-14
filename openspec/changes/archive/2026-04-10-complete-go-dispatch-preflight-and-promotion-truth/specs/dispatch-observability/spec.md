## MODIFIED Requirements

### Requirement: Dispatch metrics endpoint exposes operational statistics
The system SHALL expose a project-scoped dispatch statistics endpoint that returns aggregated metrics about dispatch outcomes, queue performance, and promotion lifecycle truth for an optional requested time window.

#### Scenario: Dispatch stats returns outcome distribution for the requested window
- **WHEN** an authenticated caller requests dispatch stats for a project with optional time-window filters
- **THEN** the response includes counts of dispatches by outcome (`started`, `queued`, `blocked`, `skipped`) for the requested window
- **THEN** the response includes the blocked-reason distribution grouped by machine-readable guardrail classification
- **THEN** the response remains valid when the caller omits the optional filters and relies on the default window

#### Scenario: Dispatch stats returns promotion lifecycle metrics
- **WHEN** an authenticated caller requests dispatch stats for a project that has had queued dispatches and promotion rechecks
- **THEN** the response includes current queue depth, median wait time from queued to promoted, and promotion success rate
- **THEN** the response includes the count of queued entries cancelled without promotion and the count of terminal promotion failures
- **THEN** the response does not require consumers to infer promotion outcomes from queue roster snapshots alone

#### Scenario: Dispatch stats returns empty metrics for inactive projects
- **WHEN** an authenticated caller requests dispatch stats for a project with no dispatch activity in the requested window
- **THEN** the response returns zero counts and null averages rather than an error
- **THEN** promotion lifecycle counters also return zero values rather than being omitted unpredictably

### Requirement: Dispatch history is queryable per task
The system SHALL expose a per-task dispatch history that records each dispatch attempt with its outcome, timestamp, and canonical dispatch context, including promotion rechecks that materially change queue or runtime truth.

#### Scenario: Task dispatch history lists chronological attempts
- **WHEN** an authenticated caller requests dispatch history for a specific task
- **THEN** the response lists all dispatch attempts for that task ordered by timestamp descending
- **THEN** each entry includes the dispatch outcome, trigger source (assignment, manual, workflow, IM, promotion), member targeted, resolved runtime tuple, and reason if blocked or queued
- **THEN** queue-linked attempts preserve queue identity and any machine-readable guardrail classification needed for diagnosis

#### Scenario: Promotion rechecks appear as distinct history verdicts
- **WHEN** a queued dispatch is revalidated during promotion and is re-queued, terminally failed, or successfully started
- **THEN** the task dispatch history records that promotion verdict as a separate chronological entry
- **THEN** the recorded entry preserves linkage back to the originating queue request when such linkage exists
- **THEN** operators can understand how a queued request evolved without inferring that evolution only from current queue state

#### Scenario: Task dispatch history is empty for undispatched tasks
- **WHEN** an authenticated caller requests dispatch history for a task that has never produced a dispatch verdict
- **THEN** the response returns an empty list rather than an error
