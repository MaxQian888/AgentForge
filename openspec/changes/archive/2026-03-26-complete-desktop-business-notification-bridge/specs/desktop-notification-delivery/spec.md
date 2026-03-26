## MODIFIED Requirements

### Requirement: Desktop mode routes eligible business notifications through the existing notification truth sources
The system SHALL derive desktop notification delivery candidates from the existing authenticated notification flow rather than a second desktop-only backend channel. In desktop mode, delivery candidates MUST come from persisted notification hydration and realtime `notification` events after they are normalized by the shared frontend notification store and observed by the authenticated shell coordination layer. Native delivery MUST preserve the business notification identifier, type, title, message, timestamp, any available `href` context, and any documented delivery-policy hints so the in-app notification center remains consistent with the desktop-enhanced view.

#### Scenario: Realtime notification becomes a desktop delivery candidate
- **WHEN** an authenticated desktop session receives a new `notification` websocket event
- **THEN** the notification is added to the shared notification store
- **AND** the desktop notification bridge evaluates that same normalized notification for native delivery without requiring a separate backend fetch path

#### Scenario: Existing unread notifications can hydrate into desktop delivery
- **WHEN** an authenticated desktop session hydrates unread notifications from the standard notification API
- **THEN** the desktop notification bridge may surface eligible unread notifications through native delivery using the same normalized notification records already shown in-app

### Requirement: Desktop notification delivery cooperates with focused sessions and tray summary
The system SHALL apply a documented foreground policy for native delivery in desktop mode. When the desktop shell is already foregrounded and a notification is eligible for suppression, the system MUST keep the notification in the in-app store, MUST update the desktop unread or tray summary through the shared platform capability path, and MUST record the suppression outcome instead of silently dropping the event.

#### Scenario: Foreground session suppresses a redundant native popup
- **WHEN** the desktop shell is focused and a new notification arrives with a delivery policy that allows foreground suppression
- **THEN** the system suppresses the native popup
- **AND** the unread state, in-app notification center, and tray summary still update normally

#### Scenario: Tray summary reflects unread desktop notifications
- **WHEN** the unread notification state changes during a desktop session
- **THEN** the system updates the desktop tray summary using the supported shared desktop path so the shell still signals pending attention even when some native popups were suppressed
