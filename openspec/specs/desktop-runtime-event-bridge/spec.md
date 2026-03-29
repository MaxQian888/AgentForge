# desktop-runtime-event-bridge Specification

## Purpose
Define how AgentForge desktop mode exposes runtime snapshots, additive desktop events, and read-only plugin status projections to the frontend without replacing the existing backend control path.
## Requirements
### Requirement: Desktop runtime status and events are exposed to the frontend
The Tauri shell SHALL expose a current desktop runtime status snapshot and SHALL publish normalized desktop runtime events to the frontend. The snapshot and events MUST include per-runtime state for backend and bridge and MUST distinguish starting, ready, degraded, and stopped-style conditions.

#### Scenario: Frontend requests the current runtime snapshot
- **WHEN** the frontend requests current desktop runtime status
- **THEN** Tauri returns the latest known backend, bridge, and overall runtime state in one payload

#### Scenario: A managed runtime changes state
- **WHEN** a managed runtime transitions between states
- **THEN** Tauri emits a desktop runtime event that frontend subscribers can consume without polling logs

### Requirement: Desktop plugin and system events stay additive to the main business path
The desktop event bridge SHALL forward plugin-status and desktop-enhanced system events in a normalized format, but it MUST NOT become the only supported source of plugin business state. Frontend pages MUST remain able to function via the existing backend API when the desktop event bridge is unavailable.

#### Scenario: A plugin or desktop system event is forwarded
- **WHEN** a desktop-managed plugin event or system event is observed by Tauri
- **THEN** Tauri emits a normalized event to the frontend with event source, event type, timestamp, and payload summary

#### Scenario: Desktop event forwarding is unavailable
- **WHEN** desktop event forwarding is unavailable but the backend API is still reachable
- **THEN** plugin and runtime pages continue to load core data from the backend API and show the desktop event bridge as unavailable rather than failing the entire page

### Requirement: Desktop plugin status commands are read-only projections
Any Tauri command that returns plugin status or runtime summaries SHALL behave as a read-only projection over authoritative backend or runtime state. These commands MUST NOT introduce plugin lifecycle mutations that bypass the existing backend control plane.

#### Scenario: Frontend queries plugin status through a desktop helper
- **WHEN** the frontend queries plugin status through a Tauri desktop helper
- **THEN** the returned data is derived from backend or runtime state and does not mutate plugin lifecycle

#### Scenario: Plugin lifecycle actions remain on the backend control path
- **WHEN** an operator enables, disables, activates, or restarts a plugin
- **THEN** the frontend uses the existing backend control path rather than a Tauri-only mutation command

### Requirement: Desktop notification delivery outcomes are exposed additively to the frontend
The desktop event bridge SHALL emit normalized notification outcome events for native desktop delivery attempts. Outcome events MUST distinguish at least delivered, suppressed, and failed results, MUST include the related business notification identifier, notification type, title, and any available `href` or policy metadata needed for frontend coordination, and MUST remain additive to the existing backend notification API and websocket business events instead of replacing them.

#### Scenario: Native delivery emits a delivered outcome event
- **WHEN** the desktop bridge successfully shows a native notification for a business notification
- **THEN** the desktop event bridge emits a normalized delivered outcome event that includes the business notification identifier and summary metadata
- **AND** frontend subscribers can consume it for diagnostics or UI feedback without re-fetching a second notification source

#### Scenario: Foreground suppression emits an observable outcome
- **WHEN** the desktop bridge suppresses a native notification because the documented foreground policy applies
- **THEN** the desktop event bridge emits a normalized suppressed outcome event that includes the related business notification identifier and suppression context
- **AND** the frontend remains able to distinguish intentional suppression from broken delivery

#### Scenario: Failed native delivery does not replace business notification truth
- **WHEN** native desktop delivery fails for a business notification
- **THEN** the desktop event bridge emits a normalized failed outcome event with the related business notification identifier
- **AND** the same notification continues to exist through the standard backend notification API and in-app notification store

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
The desktop event bridge SHALL emit normalized shell interaction result events for tray actions, native menu actions, window control actions, notification activations, and frameless window-state projections that must be observed by the shared desktop frame. These events MUST preserve `source`, `actionId`, status, timestamp, and any available route or entity context needed by frontend coordinators and tests. When the main window changes between restored, maximized, minimized, visible, or focused states through shell actions or native desktop gestures, the shared desktop subscription surface MUST expose a normalized state projection without requiring page-specific native listeners.

#### Scenario: Native shell action emits a result event
- **WHEN** the desktop shell processes a supported tray, menu, or window action
- **THEN** it emits a normalized shell action event that includes the action identifier, source, and result status
- **AND** frontend subscribers can observe that outcome without polling native logs

#### Scenario: Frameless chrome receives a window-state projection
- **WHEN** the main window changes state through the custom titlebar or a native desktop gesture such as maximizing or restoring from the drag region
- **THEN** the shared desktop subscription surface emits or refreshes a normalized window-state projection
- **AND** the shared frameless chrome can update its controls without wiring page-specific native listeners

#### Scenario: Notification activation emits a shell interaction result
- **WHEN** an operator activates a supported desktop notification
- **THEN** the desktop event bridge emits the corresponding shell action event with the related notification identifier and target context
- **AND** the event remains additive to the existing business notification truth
