# task-comment-threads Specification

## Purpose
Define threaded task comment conversations, mentions, resolution state, and cross-links to related document discussions.

## Requirements
### Requirement: Task comment CRUD
The system SHALL support comment threads on task cards with create, read, update, and delete operations.

#### Scenario: Add comment to task
- **WHEN** user submits a comment on a task
- **THEN** the system creates a task comment and displays it in the task detail comments section

#### Scenario: Reply to task comment
- **WHEN** user replies to an existing task comment
- **THEN** the system creates a threaded reply nested under the parent comment

#### Scenario: Delete task comment
- **WHEN** user deletes their own comment
- **THEN** the system soft-deletes the comment and displays "[deleted]" in the thread

### Requirement: Task comment @-mentions
The system SHALL support @-mentioning project members in task comments.

#### Scenario: @-mention triggers notification
- **WHEN** user types `@username` in a task comment and submits
- **THEN** the mentioned user receives a notification with a link to the task comment

#### Scenario: @-mention autocomplete
- **WHEN** user types `@` in the task comment input
- **THEN** the system shows an autocomplete dropdown of project members

### Requirement: Task comment resolve lifecycle
The system SHALL support resolving and reopening task comments.

#### Scenario: Resolve a task comment
- **WHEN** user resolves a task comment thread
- **THEN** the thread is marked as resolved and visually collapsed

#### Scenario: Reopen a task comment
- **WHEN** user reopens a resolved task comment thread
- **THEN** the thread is marked as active and expanded

### Requirement: Task comment API
The system SHALL expose REST endpoints for task comment operations.

#### Scenario: List task comments via API
- **WHEN** client sends `GET /api/v1/projects/:pid/tasks/:tid/comments`
- **THEN** the system returns all comments with threads, ordered by creation time

#### Scenario: Create task comment via API
- **WHEN** client sends `POST /api/v1/projects/:pid/tasks/:tid/comments` with body and optional parent_comment_id
- **THEN** the system creates the comment and returns 201

### Requirement: Cross-link between task and doc comments
The system SHALL allow linking a task comment to a related document comment.

#### Scenario: Link task comment to doc comment
- **WHEN** user references a document comment in a task comment using a comment permalink
- **THEN** the system creates a cross-link and displays a clickable reference in both comments
