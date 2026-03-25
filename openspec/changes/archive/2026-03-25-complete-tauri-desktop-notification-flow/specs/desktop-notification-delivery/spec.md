## ADDED Requirements

### Requirement: Desktop mode routes eligible business notifications through the existing notification truth sources
The system SHALL derive desktop notification delivery candidates from the existing authenticated notification flow rather than a second desktop-only backend channel. In desktop mode, delivery candidates MUST come from persisted notification hydration and realtime `notification` events after they are normalized by the shared frontend notification store. Native delivery MUST preserve the business notification identifier, title, message, type, timestamp, and any available `href` context so the in-app notification center remains consistent with the desktop-enhanced view.

#### Scenario: Realtime notification becomes a desktop delivery candidate
- **WHEN** an authenticated desktop session receives a new `notification` websocket event
- **THEN** the notification is added to the shared notification store
- **AND** the desktop notification bridge evaluates that same normalized notification for native delivery without requiring a separate backend fetch path

#### Scenario: Existing unread notifications can hydrate into desktop delivery
- **WHEN** an authenticated desktop session hydrates unread notifications from the standard notification API
- **THEN** the desktop notification bridge may surface eligible unread notifications through native delivery using the same normalized notification records already shown in-app

### Requirement: Desktop notification delivery avoids duplicate popups and preserves unread truth
The system SHALL deduplicate native desktop delivery by business notification identifier across initial hydration, websocket replay, and repeated store updates. Showing, suppressing, or failing a native desktop notification MUST NOT by itself mark that business notification as read, remove it from the in-app notification center, or mutate the authoritative notification record outside the existing notification read APIs.

#### Scenario: Hydration and websocket replay do not double-deliver the same notification
- **WHEN** the same unread business notification appears through both initial fetch hydration and a later websocket replay
- **THEN** the system emits at most one native desktop notification for that notification identifier within the configured bridge session
- **AND** the in-app notification list still contains the normalized notification record exactly once

#### Scenario: Native delivery does not mark a notification as read
- **WHEN** a business notification is shown or suppressed by the desktop notification bridge
- **THEN** the notification remains unread until the existing in-app notification handling path marks it read

### Requirement: Desktop notification delivery cooperates with focused sessions and tray summary
The system SHALL apply a documented foreground policy for native delivery in desktop mode. When the desktop shell is already foregrounded and a notification is eligible for suppression, the system MUST keep the notification in the in-app store, MUST update the desktop unread/tray summary, and MUST record the suppression outcome instead of silently dropping the event.

#### Scenario: Foreground session suppresses a redundant native popup
- **WHEN** the desktop shell is focused and a new notification arrives with a delivery policy that allows foreground suppression
- **THEN** the system suppresses the native popup
- **AND** the unread state and in-app notification center still update normally

#### Scenario: Tray summary reflects unread desktop notifications
- **WHEN** the unread notification state changes during a desktop session
- **THEN** the system updates the desktop tray summary using the supported shared desktop path so the shell still signals pending attention even when some native popups were suppressed

### Requirement: Desktop delivery failures do not break the main notification path
If native desktop delivery is unavailable or fails, the system SHALL preserve the existing notification behavior through the persisted notification API, websocket stream, and in-app notification UI. Desktop delivery failure MUST be reported as an additive outcome rather than causing the business notification to disappear or the authenticated shell to stop processing notifications.

#### Scenario: Desktop notification delivery fails safely
- **WHEN** the desktop notification bridge cannot invoke the native notification path for an eligible business notification
- **THEN** the notification remains available through the existing in-app notification center
- **AND** the system records a desktop delivery failure outcome instead of dropping the business notification
