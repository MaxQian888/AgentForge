## ADDED Requirements

### Requirement: Frontend uses a unified platform capability contract
The frontend SHALL consume desktop capabilities through a shared platform-capability facade that chooses the Tauri command path in desktop mode and a documented fallback or explicit unsupported result in web mode. Direct page-level Tauri command imports MUST NOT be required for supported capability flows.

#### Scenario: A supported capability runs in desktop mode
- **WHEN** a supported platform capability is invoked in Tauri mode
- **THEN** the shared facade routes the request through the matching Tauri command and returns a normalized success or error payload

#### Scenario: The same capability runs in web mode
- **WHEN** the same platform capability is invoked outside Tauri
- **THEN** the shared facade executes the documented web fallback if one exists or returns a stable unsupported or not-applicable result

### Requirement: Native file selection and notification are available in desktop mode
The desktop shell SHALL provide native file selection and system notification commands that the shared facade can call. File selection MUST return normalized selected-path results, and notification MUST accept title and body payloads without requiring business pages to know OS-specific APIs.

#### Scenario: Desktop file selection succeeds
- **WHEN** a desktop page requests file selection
- **THEN** Tauri opens the native picker and returns the selected file paths to the frontend

#### Scenario: Notification falls back in web mode
- **WHEN** a page requests a notification in web mode
- **THEN** the shared facade uses the web notification fallback instead of failing because Tauri is unavailable

### Requirement: Tray, global shortcut, and update checks degrade predictably
The platform SHALL define tray, global shortcut, and update-check capabilities as desktop-enhanced features with explicit non-desktop semantics. Web mode MUST NOT silently pretend these capabilities succeeded.

#### Scenario: Tray state updates on desktop
- **WHEN** the desktop runtime is ready and the frontend requests a tray status update
- **THEN** Tauri updates the tray state and returns a success acknowledgement

#### Scenario: Global shortcut is unavailable on web
- **WHEN** a web session requests global shortcut registration
- **THEN** the shared facade returns an explicit unsupported result

#### Scenario: Update check is not applicable on web
- **WHEN** a web session requests an update check
- **THEN** the shared facade returns not-applicable without triggering a desktop-only command
