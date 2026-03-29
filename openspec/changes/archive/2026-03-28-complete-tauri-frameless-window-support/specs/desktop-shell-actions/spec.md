## MODIFIED Requirements

### Requirement: Desktop shell exposes normalized window control actions
The system SHALL expose main-window control through a shared desktop shell action contract instead of requiring frontend pages to call raw Tauri window APIs. The supported window actions MUST include showing and focusing the main window, minimizing it, toggling maximized versus restored state, restoring it from a minimized or hidden state, and closing the main window when the operator explicitly requests it from the shared custom chrome. Every action MUST return a normalized result and MUST preserve enough action metadata for the frameless chrome and shell coordinator to reconcile visible control state safely.

#### Scenario: Frontend focuses the main window in desktop mode
- **WHEN** an authenticated desktop-aware surface requests the `focus_main_window` shell action
- **THEN** the desktop shell restores or shows the main window if needed
- **AND** the main window receives focus
- **AND** the caller receives a normalized success result

#### Scenario: Frameless titlebar toggles maximize and restore
- **WHEN** the shared custom titlebar requests the documented maximize-toggle shell action in desktop mode
- **THEN** the desktop shell maximizes the main window if it is currently restored or restores it if it is already maximized
- **AND** the caller receives a normalized success result that the shared chrome can reconcile with its current state

#### Scenario: Window control is requested outside desktop mode
- **WHEN** the same shell action is requested in a non-Tauri session
- **THEN** the shared shell action contract returns an explicit unsupported or not-applicable result
- **AND** the caller does not need to import any raw platform API
