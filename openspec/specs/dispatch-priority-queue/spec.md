# dispatch-priority-queue Specification

## Purpose
Define the priority-aware queue admission contract for dispatch so queued work is ordered predictably by urgency and FIFO tiebreaking.
## Requirements
### Requirement: Queue entries carry a priority level that influences admission ordering
The system SHALL support a priority field on agent pool queue entries that determines promotion order within the same project, with higher priority entries promoted before lower priority entries and FIFO ordering as a tiebreaker within the same priority level.

#### Scenario: Higher priority entry is promoted before lower priority entry
- **WHEN** two queue entries exist for the same project with different priority levels and a pool slot becomes available
- **THEN** the system promotes the entry with the higher priority value
- **THEN** the lower priority entry remains queued

#### Scenario: Equal priority entries follow FIFO ordering
- **WHEN** two queue entries exist for the same project with the same priority level and a pool slot becomes available
- **THEN** the system promotes the entry with the earlier `created_at` timestamp
- **THEN** the later entry remains queued

#### Scenario: Default priority preserves backward compatibility
- **WHEN** a dispatch queues an entry without specifying a priority
- **THEN** the entry is created with priority 0 (PriorityLow)
- **THEN** existing queue entries created before priority support also have priority 0

### Requirement: Priority levels have semantic constants
The system SHALL define canonical priority constants that callers can use to express task urgency without relying on arbitrary integer values.

#### Scenario: Priority constants cover standard urgency levels
- **WHEN** a caller specifies a priority level for a queued dispatch
- **THEN** the system accepts any of the predefined levels: `low` (0), `normal` (10), `high` (20), `critical` (30)
- **THEN** the system also accepts any integer value for custom priority levels between or beyond the predefined constants

#### Scenario: Queue roster displays priority level in operator views
- **WHEN** an operator views the queue roster in the agent monitor
- **THEN** each queue entry displays its priority level using the semantic label when it matches a predefined constant
- **THEN** entries are visually sorted by priority descending, then creation time ascending
