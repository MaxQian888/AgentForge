## ADDED Requirements

### Requirement: Queue entries support priority-ordered admission
The system SHALL store a priority level on each agent pool queue entry and use priority as the primary sort key when selecting the next entry for promotion, with creation time as the secondary sort key within equal priority levels.

#### Scenario: ReserveNextQueuedByProject respects priority ordering
- **WHEN** the system promotes the next queued entry for a project and multiple entries are waiting
- **THEN** the entry with the highest priority value is selected
- **THEN** among entries with equal priority, the entry with the earliest `created_at` timestamp is selected
- **THEN** the selected entry transitions from `queued` to `admitted` status

#### Scenario: ListQueuedByProject returns entries in priority order
- **WHEN** an operator requests the queue roster for a project
- **THEN** entries are returned ordered by priority descending, then `created_at` ascending
- **THEN** each entry includes its priority value and semantic label

#### Scenario: Database migration adds priority column with backward-compatible default
- **WHEN** the migration runs on an existing database with queue entries
- **THEN** a `priority INT NOT NULL DEFAULT 0` column is added to `agent_pool_queue_entries`
- **THEN** existing entries receive priority 0 (PriorityLow)
- **THEN** the composite index on `(project_id, status, created_at)` is updated to `(project_id, status, priority DESC, created_at ASC)`

### Requirement: Queue admission accepts an optional priority parameter
The system SHALL accept an optional priority parameter when creating a queue entry, defaulting to 0 (PriorityLow) when not specified.

#### Scenario: Queue entry created with explicit priority
- **WHEN** a dispatch queues an entry with priority set to `high` (20)
- **THEN** the entry is created with priority 20
- **THEN** the entry is promoted before entries with priority less than 20

#### Scenario: Queue entry created without priority defaults to low
- **WHEN** a dispatch queues an entry without specifying a priority
- **THEN** the entry is created with priority 0
- **THEN** the entry follows standard FIFO ordering among other priority-0 entries
