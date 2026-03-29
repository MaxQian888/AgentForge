## ADDED Requirements

### Requirement: Desktop notification activation resumes the shell and preserves target context
When a delivered desktop notification is activated, the system SHALL hand that interaction to the shared desktop shell action contract instead of inventing a notification-only route path. Activation handling MUST preserve the originating business notification identifier plus any available `href` or entity context, MUST restore or focus the main window before frontend navigation when desktop mode is available, and MUST NOT mark the business notification as read solely because the native notification was activated.

#### Scenario: Activating a notification focuses the app and opens its target
- **WHEN** an operator clicks a delivered desktop notification that preserved a valid `href`
- **THEN** the desktop shell restores or focuses the main window
- **AND** the frontend receives a normalized shell action event containing the related notification identifier and target route
- **AND** the notification remains unread until the existing notification read flow marks it read

#### Scenario: Activating a notification without a direct route stays truthful
- **WHEN** an operator activates a delivered desktop notification that does not include a direct `href`
- **THEN** the desktop shell still emits a normalized activation event with the related notification identifier
- **AND** the frontend can route the user to the supported fallback surface such as the notification center
- **AND** the activation does not silently claim that a deep link existed
