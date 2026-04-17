## MODIFIED Requirements

### Requirement: `/im` SHALL expose one authoritative IM Bridge operator console
The `/im` workspace SHALL load the canonical IM provider catalog, IM Bridge runtime snapshot, configured channels, recent delivery history, and subscribable event types from backend APIs and present them as one operator console instead of isolated status widgets. The console MUST show summary cards for provider count, pending deliveries, recent failures, and average settled latency derived from the canonical snapshot, while still allowing operators to inspect configured providers whose runtime is not currently registered.

#### Scenario: Operator opens a healthy console
- **WHEN** an authenticated operator navigates to `/im` and the backend reports a healthy IM Bridge snapshot
- **THEN** the frontend requests the canonical IM provider catalog together with `/api/v1/im/bridge/status`, `/api/v1/im/channels`, `/api/v1/im/deliveries`, and `/api/v1/im/event-types`
- **AND** the page renders summary metrics, provider cards, channel configuration, and activity history from those canonical responses

#### Scenario: Operator opens a degraded console
- **WHEN** the backend reports one provider as degraded or disconnected
- **THEN** the console highlights the affected provider and overall health state
- **AND** the operator can still inspect last-known diagnostics and recent delivery history without losing the rest of the console

#### Scenario: Catalog-known provider appears without active runtime registration
- **WHEN** the canonical provider catalog includes `email` or `wechat` but no active bridge instance for that provider is currently registered
- **THEN** `/im` still renders the provider in operator context
- **AND** the console makes the missing runtime state explicit instead of hiding the provider entirely

### Requirement: Provider cards SHALL expose diagnostics and configuration drill-through
The operator console SHALL render one provider card per provider truth exposed to the operator, combining canonical catalog metadata with any live runtime snapshot when available. Each provider card MUST show transport or interaction class, callback or delivery surface, queue backlog when known, recent failure or fallback signal, and last-known diagnostics metadata. Each provider card MUST expose a configure action that reuses the existing channel configuration surface within `/im` with that provider context preselected.

#### Scenario: Operator drills into provider configuration
- **WHEN** the operator clicks the configure action on the Slack provider card
- **THEN** the console switches to the existing channel configuration surface
- **AND** the Slack platform context is preselected for edit or new-channel creation

#### Scenario: Provider has no diagnostics snapshot
- **WHEN** a provider does not report optional diagnostics metadata
- **THEN** the provider card displays diagnostics as unavailable
- **AND** the console still renders transport, callback, capability, or interaction-class information for that provider

#### Scenario: Delivery-only provider card stays truthful
- **WHEN** the operator views the Email provider card
- **THEN** the card labels Email as delivery-only
- **AND** it exposes configuration and delivery-oriented health information without fabricating slash-command or callback interaction parity

### Requirement: Operator console SHALL consume authoritative event inventory and configured test targets
The `/im` operator console SHALL derive its provider list, event inventory, and test-target affordances from authoritative backend data rather than stale hardcoded assumptions. Provider availability and configuration schema MUST come from the canonical provider catalog, event subscription choices MUST come from the backend event inventory, and test-send platform/channel choices MUST reflect configured channels whose providers advertise test-send support.

#### Scenario: Provider catalog refreshes operator choices
- **WHEN** the backend provider catalog adds or updates a provider such as `wechat` or `email`
- **THEN** `/im` refreshes its provider choices from the canonical catalog
- **AND** the operator does not need a separate frontend hardcoded platform update to see the change

#### Scenario: Event subscription choices refresh from backend inventory
- **WHEN** the backend adds or removes an event type from the authoritative IM event inventory
- **THEN** `/im` refreshes its event subscription choices from `GET /api/v1/im/event-types`
- **AND** the operator does not need a frontend hardcoded list update to see the new inventory

#### Scenario: Test-send defaults to configured channel context
- **WHEN** the operator opens `/im` with configured channels for Feishu and QQ Bot
- **THEN** the test-send form offers those configured platform/channel targets as the current project truth
- **AND** it does not invent an operator test target that is absent from the configured channels

#### Scenario: Delivery-only provider remains eligible for delivery smoke tests
- **WHEN** the operator has a configured Email channel and the provider catalog marks Email as supporting test-send
- **THEN** the test-send form can offer that Email target as a delivery smoke test
- **AND** the surrounding UI keeps the provider labeled as delivery-only rather than interactive chat
