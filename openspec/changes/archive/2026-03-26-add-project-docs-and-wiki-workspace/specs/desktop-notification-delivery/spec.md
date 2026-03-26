## MODIFIED Requirements

### Requirement: Notification event types
The notification delivery system SHALL support the following additional event types for document-related activities: `wiki.comment.mention`, `wiki.page.updated`, and `wiki.version.published`.

#### Scenario: Comment mention notification
- **WHEN** a user is @-mentioned in a wiki page comment
- **THEN** the system delivers a desktop notification with the page title, comment author, and a deep link to the comment

#### Scenario: Page update notification for subscribers
- **WHEN** a wiki page is updated and a user has subscribed to that page
- **THEN** the system delivers a notification with the page title, editor name, and a deep link to the page

#### Scenario: Version published notification
- **WHEN** a named version is published for a wiki page a user has subscribed to
- **THEN** the system delivers a notification with the page title, version name, and a deep link to the version
