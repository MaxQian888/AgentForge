## ADDED Requirements

### Requirement: Desktop shell exposes normalized window control actions
The system SHALL expose main-window control through a shared desktop shell action contract instead of requiring frontend pages to call raw Tauri window APIs. The supported first-wave actions MUST include showing and focusing the main window, minimizing it, and restoring it from a minimized or hidden state. Every action MUST return a normalized result and MUST report unsupported states explicitly outside desktop mode.

#### Scenario: Frontend focuses the main window in desktop mode
- **WHEN** an authenticated desktop-aware surface requests the `focus_main_window` shell action
- **THEN** the desktop shell restores or shows the main window if needed
- **AND** the main window receives focus
- **AND** the caller receives a normalized success result

#### Scenario: Window control is requested outside desktop mode
- **WHEN** the same shell action is requested in a non-Tauri session
- **THEN** the shared shell action contract returns an explicit unsupported or not-applicable result
- **AND** the caller does not need to import any raw platform API

### Requirement: Tray, menu, and notification activations resolve through one shell action registry
The system SHALL normalize native tray selections, native menu selections, and desktop notification activations into one shell action registry. Each emitted shell action MUST include a stable `actionId`, an action `source`, and any available route or entity context needed by the frontend to continue the interaction without parsing Tauri-specific payloads.

#### Scenario: Tray or menu action opens a supported route
- **WHEN** an operator chooses a supported tray or native menu action such as opening the plugins or reviews surface
- **THEN** the desktop shell emits a normalized shell action event with the matching `actionId`
- **AND** the event includes the route context required for frontend navigation

#### Scenario: Notification activation enters the same shell action registry
- **WHEN** the operator activates a delivered desktop notification that preserved target context
- **THEN** the desktop shell emits the corresponding shell action event through the same registry used for tray or menu actions
- **AND** the event includes the related notification identifier and any available `href`

### Requirement: Shell-triggered plugin shortcuts stay on the existing backend control path
If the desktop shell exposes plugin-related quick actions, those actions SHALL resolve to existing frontend store actions or backend APIs rather than a Tauri-only mutation path. The shell layer MUST remain an additive trigger surface and MUST NOT become the authoritative plugin lifecycle control plane.

#### Scenario: Shell shortcut triggers a supported plugin action
- **WHEN** an operator selects a supported shell shortcut that targets a plugin lifecycle task
- **THEN** the frontend resolves that shortcut through the existing backend control path
- **AND** the shell layer only reports the trigger and outcome metadata

#### Scenario: Unsupported plugin shortcut is rejected safely
- **WHEN** a shell shortcut refers to a stale or unsupported plugin action
- **THEN** the shell action contract returns a stable failed or unsupported result
- **AND** the desktop shell does not mutate plugin state directly
