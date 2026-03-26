## MODIFIED Requirements

### Requirement: Automation-triggered desktop notifications
The desktop notification delivery system SHALL deliver notifications triggered by automation rule actions.

#### Scenario: Automation sends desktop notification
- **WHEN** an automation rule executes a send_notification action
- **THEN** the system delivers a desktop notification to the specified user with the configured title, body, and deep link
