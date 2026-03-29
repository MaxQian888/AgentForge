## ADDED Requirements

### Requirement: Shared platform capability facade exposes window control and shell action APIs
The shared platform capability facade SHALL expose desktop shell actions for main-window control and supported quick actions alongside the existing file, notification, tray, shortcut, and updater APIs. Supported desktop pages MUST consume these actions through the shared facade instead of importing raw Tauri window, menu, or tray APIs directly.

#### Scenario: Desktop shell action succeeds through the shared facade
- **WHEN** a desktop-aware frontend surface requests a supported shell action through the shared platform capability facade
- **THEN** the facade routes the request through the documented desktop path
- **AND** it returns a normalized success or failure payload without exposing raw Tauri window or menu APIs to the caller

#### Scenario: Shell action degrades outside desktop mode
- **WHEN** the same shell action is requested in web mode
- **THEN** the shared facade returns the documented unsupported or not-applicable result
- **AND** the frontend interaction continues without requiring desktop-only APIs
