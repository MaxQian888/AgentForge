## MODIFIED Requirements

### Requirement: Automation-triggered IM messages
The IM bridge progress streaming system SHALL deliver messages triggered by automation rule actions.

#### Scenario: Automation sends IM message
- **WHEN** an automation rule executes a send_im_message action with a channel and template
- **THEN** the IM bridge renders the template with event context and sends the message to the specified channel
