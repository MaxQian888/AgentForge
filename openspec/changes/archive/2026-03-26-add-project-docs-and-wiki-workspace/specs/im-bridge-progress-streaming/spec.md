## MODIFIED Requirements

### Requirement: Document event streaming to IM
The IM bridge progress streaming system SHALL forward document-related events to configured IM channels.

#### Scenario: Page created event streamed to IM
- **WHEN** a wiki page is created in a project with an IM channel configured for doc events
- **THEN** the IM bridge sends a message to the channel with the page title, creator, and a link to the page

#### Scenario: Comment mention forwarded to IM
- **WHEN** a user is @-mentioned in a wiki comment and has IM notifications enabled
- **THEN** the IM bridge sends a direct message to the user with the comment context and a link to the comment
