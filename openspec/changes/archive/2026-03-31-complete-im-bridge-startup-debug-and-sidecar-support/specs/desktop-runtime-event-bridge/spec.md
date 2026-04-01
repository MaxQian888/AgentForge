## MODIFIED Requirements

### Requirement: Desktop runtime status and events are exposed to the frontend
The Tauri shell SHALL expose a current desktop runtime status snapshot and SHALL publish normalized desktop runtime events to the frontend. The snapshot and events MUST include per-runtime state for backend, bridge, and IM Bridge and MUST distinguish starting, ready, degraded, and stopped-style conditions for each managed runtime.

#### Scenario: Frontend requests the current runtime snapshot
- **WHEN** the frontend requests current desktop runtime status
- **THEN** Tauri returns the latest known backend, bridge, IM Bridge, and overall runtime state in one payload

#### Scenario: A managed runtime changes state
- **WHEN** a managed backend, bridge, or IM Bridge runtime transitions between states
- **THEN** Tauri emits a desktop runtime event that frontend subscribers can consume without polling logs
