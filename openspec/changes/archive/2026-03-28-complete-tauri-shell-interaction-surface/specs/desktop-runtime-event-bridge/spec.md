## ADDED Requirements

### Requirement: Desktop event bridge exposes plugin lifecycle projections additively
The desktop event bridge SHALL expose normalized plugin lifecycle events as an additive desktop-aware projection over the existing backend websocket or store truth. The projection MUST preserve the plugin identifier, lifecycle event type, timestamp, and any summary fields required by desktop-aware plugin surfaces, and it MUST NOT replace the existing backend API or websocket path as the authoritative source of plugin state.

#### Scenario: Backend plugin lifecycle event appears in the desktop event stream
- **WHEN** the authenticated frontend receives a `plugin.lifecycle` event from the existing backend realtime path during a desktop session
- **THEN** the desktop event bridge emits a normalized desktop event for that plugin lifecycle transition
- **AND** desktop-aware plugin surfaces can consume it through the same desktop event subscription API used for native shell events

#### Scenario: Plugin lifecycle projection is unavailable
- **WHEN** the desktop shell is available but the additive plugin lifecycle projection is not currently connected
- **THEN** plugin pages continue to load and mutate plugins through the existing backend API and websocket flows
- **AND** the desktop event bridge reports that the plugin lifecycle projection is unavailable instead of failing the full page

### Requirement: Shell interaction result events are exposed to the frontend
The desktop event bridge SHALL emit normalized shell interaction result events for tray actions, native menu actions, window control actions, and notification activations. These events MUST preserve `source`, `actionId`, status, timestamp, and any available route or entity context needed by frontend coordinators and tests.

#### Scenario: Native shell action emits a result event
- **WHEN** the desktop shell processes a supported tray, menu, or window action
- **THEN** it emits a normalized shell action event that includes the action identifier, source, and result status
- **AND** frontend subscribers can observe that outcome without polling native logs

#### Scenario: Notification activation emits a shell interaction result
- **WHEN** an operator activates a supported desktop notification
- **THEN** the desktop event bridge emits the corresponding shell action event with the related notification identifier and target context
- **AND** the event remains additive to the existing business notification truth
