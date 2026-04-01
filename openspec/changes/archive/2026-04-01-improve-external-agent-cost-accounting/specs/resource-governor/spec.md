## ADDED Requirements

### Requirement: Budget tracking consumes cumulative runtime accounting snapshots
The system SHALL treat each processed runtime cost update as the latest cumulative accounting snapshot for that run. Persisted run totals and downstream task-spend recalculation MUST be based on those latest cumulative snapshots rather than additive per-event deltas, and any runtime whose spend cannot be truthfully priced in USD MUST surface a budget-coverage gap instead of silently behaving like zero spend.

#### Scenario: Repeated updates for one run replace the latest tracked totals
- **WHEN** Go receives multiple cost updates for the same active run as that run accumulates more spend and tokens
- **THEN** the run's persisted token and cost totals are replaced by the latest cumulative snapshot from that run
- **THEN** task-level spend is recalculated from persisted run totals without double-counting earlier updates from the same run

#### Scenario: Budget warning and hard-stop logic use normalized cumulative spend
- **WHEN** a cumulative runtime accounting snapshot causes the task's recomputed spend to cross the warning threshold or hard limit
- **THEN** the resource governor emits the same warning or hard-stop behavior defined by this capability
- **THEN** those budget decisions are based on the normalized run totals rather than on duplicated intermediate deltas

#### Scenario: Unpriced run creates an explicit budget coverage gap
- **WHEN** a run is marked as unpriced or plan-included because its billing surface cannot be truthfully expressed as USD
- **THEN** the system records that the task or project budget view has incomplete runtime-cost coverage
- **THEN** the system SHALL NOT silently treat that run as zero authoritative spend
