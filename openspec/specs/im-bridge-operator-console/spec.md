# im-bridge-operator-console Specification

## Purpose
Define the operator-facing IM Bridge console contract so `/im` exposes one canonical workspace for runtime summary, provider diagnostics, delivery operations, and test-send workflows.
## Requirements
### Requirement: `/im` SHALL expose one authoritative IM Bridge operator console

The `/im` workspace SHALL load the canonical IM Bridge runtime snapshot, configured channels, recent delivery history, and subscribable event types from backend APIs and present them as one operator console instead of isolated status widgets. The console MUST show summary cards for provider count, pending deliveries, recent failures, and average settled latency derived from the canonical snapshot.

#### Scenario: Operator opens a healthy console
- **WHEN** an authenticated operator navigates to `/im` and the backend reports a healthy IM Bridge snapshot
- **THEN** the frontend requests `/api/v1/im/bridge/status`, `/api/v1/im/channels`, `/api/v1/im/deliveries`, and `/api/v1/im/event-types`
- **THEN** the page renders summary metrics, provider cards, channel configuration, and activity history from those canonical responses

#### Scenario: Operator opens a degraded console
- **WHEN** the backend reports one provider as degraded or disconnected
- **THEN** the console highlights the affected provider and overall health state
- **THEN** the operator can still inspect last-known diagnostics and recent delivery history without losing the rest of the console

### Requirement: Provider cards SHALL expose diagnostics and configuration drill-through

The operator console SHALL render one provider card per registered or configured IM platform, including transport mode, callback surface, queue backlog, recent failure or fallback signal, and last-known diagnostics metadata. Each provider card MUST expose a configure action that reuses the existing channel configuration surface within `/im` with that provider context preselected.

#### Scenario: Operator drills into provider configuration
- **WHEN** the operator clicks the configure action on the Slack provider card
- **THEN** the console switches to the existing channel configuration surface
- **THEN** the Slack platform context is preselected for edit or new-channel creation

#### Scenario: Provider has no diagnostics snapshot
- **WHEN** a provider does not report optional diagnostics metadata
- **THEN** the provider card displays diagnostics as unavailable
- **THEN** the console still renders transport, callback, capability, and queue information for that provider

### Requirement: Operator console SHALL support canonical test-send workflows

The operator console SHALL let an operator send a test message for a selected platform/channel through the canonical IM delivery pipeline. The result MUST surface the delivery identifier and report whether the test settled as delivered, failed, or remained pending within the bounded wait window.

#### Scenario: Test send settles successfully
- **WHEN** the operator submits a test message to a configured Feishu channel and the bridge settles that delivery within the wait window
- **THEN** the backend returns the delivery id, terminal status, processed timestamp, and measured latency
- **THEN** the console displays success feedback and refreshes the activity history and summary cards

#### Scenario: Test send stays pending
- **WHEN** the operator submits a test message and no terminal settlement arrives before the wait window expires
- **THEN** the backend returns a pending result with the delivery id
- **THEN** the console tells the operator that the delivery is still pending and keeps that delivery discoverable in the history view

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

