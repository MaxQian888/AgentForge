## ADDED Requirements

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

## MODIFIED Requirements

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
