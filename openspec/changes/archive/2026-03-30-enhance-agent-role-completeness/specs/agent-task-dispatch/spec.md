## MODIFIED Requirements

### Requirement: Assignment and dispatch outcomes are visible to synchronous and realtime consumers

The agent detail page SHALL display dispatch context including the dispatch outcome (started, queued, blocked, skipped), preflight summary, and budget metadata. This context SHALL be shown as a dedicated section on the agent detail page so operators can understand how the agent was dispatched without navigating away.

#### Scenario: Agent detail shows dispatch outcome
- **WHEN** operator views an agent's detail page
- **THEN** a Dispatch Context section displays the dispatch outcome (started/queued/blocked/skipped)
- **AND** shows the preflight summary including budget status and pool state at dispatch time

#### Scenario: Agent detail shows dispatch history for the task
- **WHEN** operator views an agent's detail page for a task with multiple dispatch attempts
- **THEN** the Dispatch Context section shows the dispatch history for that task
- **AND** each attempt shows outcome, timestamp, and reason if blocked

#### Scenario: Agent without dispatch context shows minimal info
- **WHEN** operator views an agent that was spawned manually without dispatch
- **THEN** the Dispatch Context section shows "Manual spawn" with the spawn timestamp
- **AND** no preflight or guardrail data is displayed
