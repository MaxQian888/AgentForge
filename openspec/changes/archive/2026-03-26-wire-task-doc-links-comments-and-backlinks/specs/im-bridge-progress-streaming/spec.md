## MODIFIED Requirements

### Requirement: IM actions for message-to-doc and message-to-task conversion
The IM bridge SHALL support actions to convert an IM message into a wiki page or a task.

#### Scenario: Convert message to doc page
- **WHEN** user triggers the "Save as Doc" action on an IM message
- **THEN** the IM bridge creates a wiki page in the project's doc space with the message content as the body, and replies with a link to the created page

#### Scenario: Convert message to task
- **WHEN** user triggers the "Create Task" action on an IM message
- **THEN** the IM bridge creates a task in the project backlog with the message content as the description, sets origin=im, and replies with a link to the created task
