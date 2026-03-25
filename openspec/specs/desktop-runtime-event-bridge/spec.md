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
The desktop event bridge SHALL emit normalized notification outcome events for native desktop delivery attempts. Outcome events MUST distinguish at least delivered, suppressed, and failed results, MUST include the related business notification identifier plus summary metadata when available, and MUST remain additive to the existing backend notification API and websocket business events instead of replacing them.

#### Scenario: Native delivery emits a delivered outcome event
- **WHEN** the desktop bridge successfully shows a native notification for a business notification
- **THEN** the desktop event bridge emits a normalized delivered outcome event that frontend subscribers can consume for diagnostics or UI feedback

#### Scenario: Suppressed delivery still emits an observable outcome
- **WHEN** the desktop bridge suppresses a native notification because the documented foreground policy applies
- **THEN** the desktop event bridge emits a normalized suppressed outcome event instead of leaving the frontend unable to tell whether delivery was skipped or broken

#### Scenario: Failed native delivery does not replace business notification truth
- **WHEN** native desktop delivery fails for a business notification
- **THEN** the desktop event bridge emits a normalized failed outcome event
- **AND** the same notification continues to exist through the standard backend notification API and in-app notification store

