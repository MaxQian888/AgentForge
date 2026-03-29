## MODIFIED Requirements

### Requirement: Shared platform capability facade exposes window control and shell action APIs
The shared platform capability facade SHALL expose desktop shell actions, frameless-window control flows, and read-only window chrome state alongside the existing file, notification, tray, shortcut, and updater APIs. Supported desktop pages and shared frame components MUST consume these capabilities through the shared facade instead of importing raw Tauri window, menu, or tray APIs directly. The facade MUST normalize minimize, maximize or restore, close, current window-state snapshot, and window-state subscriptions, and it MUST keep non-desktop sessions on an explicit unsupported or not-applicable path.

#### Scenario: Desktop shell action succeeds through the shared facade
- **WHEN** a desktop-aware surface requests minimize, maximize or restore, close, or another supported shell action through the shared platform capability facade
- **THEN** the facade routes the request through the documented desktop path
- **AND** it returns a normalized success or failure payload without exposing raw Tauri window or menu APIs to the caller

#### Scenario: Window chrome state is read through the shared facade
- **WHEN** the shared desktop frame needs the current window chrome state or subscribes to later state changes in desktop mode
- **THEN** the facade provides a normalized state snapshot or event stream that identifies whether the main window is maximized, minimized, visible, or focused
- **AND** the shared frame does not need to import raw platform APIs directly

#### Scenario: Shell action degrades outside desktop mode
- **WHEN** the same shell action or window-state request is issued in web mode
- **THEN** the shared facade returns the documented unsupported or not-applicable result
- **AND** the frontend interaction continues without requiring desktop-only APIs
