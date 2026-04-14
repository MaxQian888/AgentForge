## ADDED Requirements

### Requirement: Operator test-send SHALL use live sender wiring and truthful settlement outcomes
The operator console SHALL execute test sends through the same backend sender wiring and canonical IM delivery pipeline used by real IM delivery. If no live sender or routable target is available, the backend MUST return an explicit operator-visible failure instead of reporting a fake pending or successful result.

#### Scenario: Test send uses live sender and returns settlement details
- **WHEN** an operator submits a test send from `/im` to a configured Slack channel
- **THEN** the backend routes the request through the live sender path used by canonical IM delivery
- **AND** the response includes the delivery id plus delivered, failed, or pending settlement information from the bounded wait window

#### Scenario: Sender unavailable returns an explicit failure
- **WHEN** an operator submits a test send while the backend has no available IM sender wiring for that environment
- **THEN** the backend returns an explicit unavailable or failed result to `/im`
- **AND** the operator console does not display the test send as pending or successful

### Requirement: Operator console SHALL consume authoritative event inventory and configured test targets
The `/im` operator console SHALL derive its event inventory and test-target affordances from authoritative backend data rather than stale hardcoded assumptions. Event subscription choices MUST come from the backend event inventory, and test-send platform/channel choices MUST reflect the configured channels that are actually available to the current project.

#### Scenario: Event subscription choices refresh from backend inventory
- **WHEN** the backend adds or removes an event type from the authoritative IM event inventory
- **THEN** `/im` refreshes its event subscription choices from `GET /api/v1/im/event-types`
- **AND** the operator does not need a frontend hardcoded list update to see the new inventory

#### Scenario: Test-send defaults to configured channel context
- **WHEN** the operator opens `/im` with configured channels for Feishu and QQ Bot
- **THEN** the test-send form offers those configured platform/channel targets as the current project truth
- **AND** it does not invent an operator test target that is absent from the configured channels
