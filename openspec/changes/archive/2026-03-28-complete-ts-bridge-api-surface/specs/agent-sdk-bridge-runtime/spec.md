## ADDED Requirements

### Requirement: Go agent service performs post-spawn status verification
After dispatching an execute request to Bridge, Go agent service SHALL verify execution started by either receiving a WS `agent.started` event or falling back to a `GET /bridge/status/:id` call within a 5-second window.

#### Scenario: WS event confirms start
- **WHEN** Bridge sends `agent.started` WS event within 5 seconds of spawn
- **THEN** agent record status is updated to `running` from the WS event (no status poll needed)

#### Scenario: Fallback status poll on missing WS event
- **WHEN** no `agent.started` WS event arrives within 5 seconds
- **THEN** Go service calls `bridge.GetStatus(taskID)` and updates agent record from response state

### Requirement: Bridge test coverage includes all active and pool endpoints
Bridge server tests SHALL cover `/bridge/active` and `/bridge/pool` endpoints with comprehensive scenarios.

#### Scenario: Active endpoint returns running agents
- **WHEN** 2 agents are running and client calls `GET /bridge/active`
- **THEN** response contains array of 2 agent summaries with task_id, runtime, state, spent_usd

#### Scenario: Pool endpoint returns slot allocation
- **WHEN** pool has 3 active and 2 warm slots
- **THEN** `GET /bridge/pool` returns `{"active": 3, "available": N, "warm": 2, "queued": 0}`
