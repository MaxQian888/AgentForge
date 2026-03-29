# desktop-native-capabilities Specification

## Purpose
Define the unified desktop-native capability contract for AgentForge, including Tauri-backed file selection, notifications, tray behavior, global shortcuts, update checks, and their required web fallbacks.
## Requirements
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
The platform SHALL define tray, global shortcut, and updater lifecycle capabilities as desktop-enhanced features with explicit non-desktop semantics. Web mode MUST NOT silently pretend these capabilities succeeded. Desktop update handling MUST surface normalized update metadata, progress, install outcome, and post-install restart requirements through the shared facade instead of reducing the flow to a single success or failure flag.

#### Scenario: Tray state updates on desktop
- **WHEN** the desktop runtime is ready and the frontend requests a tray status update
- **THEN** Tauri updates the tray state and returns a success acknowledgement

#### Scenario: Global shortcut is unavailable on web
- **WHEN** a web session requests global shortcut registration
- **THEN** the shared facade returns an explicit unsupported result

#### Scenario: Update lifecycle is not applicable on web
- **WHEN** a web session requests an update check or update installation
- **THEN** the shared facade returns not-applicable without triggering desktop-only updater APIs

#### Scenario: Desktop update metadata is returned before install starts
- **WHEN** a desktop session checks configured updater endpoints and a newer signed release is available
- **THEN** the shared facade returns the available version, release notes or body, and release date without starting download or installation implicitly

### Requirement: Desktop update installation exposes progress and restart handoff
The desktop platform facade SHALL support an explicit install flow for a discovered update. The install flow MUST expose normalized download or install progress, MUST report failure without terminating the current session, and MUST provide a restart handoff once installation succeeds. Frontend pages MUST NOT need to import raw Tauri updater or process APIs directly to complete this flow.

#### Scenario: Download and install completes successfully
- **WHEN** an operator confirms installation for an available desktop update
- **THEN** the shared facade reports started, progress, and finished states for the install session
- **AND** the install result transitions to a stable state that indicates the app is ready to relaunch into the new version

#### Scenario: Download or install fails
- **WHEN** the updater download or installation step fails
- **THEN** the shared facade returns a stable failed result with an error summary
- **AND** the current app session remains usable without falsely reporting the update as installed

#### Scenario: Restart is triggered after a successful install
- **WHEN** an installed update is waiting to be activated and the operator chooses restart now
- **THEN** the desktop shell relaunches the application through the supported process capability
- **AND** the page does not need to call raw platform APIs outside the shared facade

### Requirement: Shared platform capability facade delivers structured business notifications
The platform SHALL expose desktop notification delivery through the shared platform capability facade using a structured business payload rather than only free-form title and body strings. The structured payload MUST support stable business metadata needed for coordination, including notification identifier, notification type, title, body, optional `href`, timestamp, and delivery-policy hints. Consumers in the authenticated shell or future desktop-aware surfaces MUST NOT import raw Tauri notification APIs directly for supported business-notification flows.

#### Scenario: Desktop shell delivers a structured business notification through the shared facade
- **WHEN** a desktop-aware consumer sends a structured business notification through the shared platform capability facade in Tauri mode
- **THEN** the facade routes the request through the supported desktop notification path
- **AND** it returns a normalized success or failure result that preserves the business notification context without exposing raw Tauri plugin APIs to the caller

#### Scenario: Structured business notification request stays safe outside desktop mode
- **WHEN** the same structured business notification request is issued outside Tauri mode
- **THEN** the shared facade returns the documented non-desktop result or fallback behavior for that delivery policy
- **AND** the caller does not fail only because raw Tauri APIs are unavailable

### Requirement: Shared platform capability facade can synchronize desktop notification tray summary
The shared platform capability facade SHALL support notification-driven tray summary updates for the authenticated shell. The tray summary update path MUST accept the current unread count plus optional summary text, MUST route through the supported desktop tray path in Tauri mode, and MUST degrade through the documented non-desktop behavior when a native tray is unavailable.

#### Scenario: Desktop unread notification summary updates the tray
- **WHEN** the authenticated shell recalculates the current unread notification summary in desktop mode
- **THEN** the shared facade updates the tray title, tooltip, or visibility through the supported desktop tray path
- **AND** it returns a normalized desktop result to the caller

#### Scenario: Notification tray summary degrades outside desktop mode
- **WHEN** the same tray summary sync is requested outside Tauri mode
- **THEN** the shared facade applies the documented web fallback or non-desktop result
- **AND** it does not claim that a native tray update occurred

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
