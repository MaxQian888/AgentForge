## ADDED Requirements

### Requirement: Shared platform capability facade delivers structured business notifications
The platform SHALL expose desktop notification delivery through the shared platform capability facade using a structured payload rather than only free-form title/body strings. The structured payload MUST support stable business metadata needed for coordination, including notification identifier, notification type, title, body, optional `href`, timestamp, and delivery-policy hints. Consumers in the dashboard shell or future desktop-aware surfaces MUST NOT import raw Tauri notification APIs directly for supported business-notification flows.

#### Scenario: Desktop shell delivers a structured business notification through the shared facade
- **WHEN** a desktop-aware consumer sends a structured business notification through the shared platform capability facade in Tauri mode
- **THEN** the facade routes the request through the supported desktop notification path
- **AND** it returns a normalized success or failure result without exposing raw Tauri plugin APIs to the caller

#### Scenario: Structured business notification request stays safe outside desktop mode
- **WHEN** the same structured business notification request is issued outside Tauri mode
- **THEN** the shared facade returns the documented non-desktop result or fallback behavior for that delivery policy
- **AND** the caller does not fail only because raw Tauri APIs are unavailable
