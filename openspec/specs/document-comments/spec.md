# document-comments Specification

## Purpose
Define page-level and inline wiki comment workflows, including mentions, resolve lifecycle, permalinks, and realtime updates.

## Requirements
### Requirement: Page-level comments
The system SHALL support page-level comment threads on wiki pages.

#### Scenario: Add page-level comment
- **WHEN** user submits a comment on a wiki page without selecting a block
- **THEN** the system creates a page-level comment visible in the comments panel

#### Scenario: Reply to comment
- **WHEN** user replies to an existing comment
- **THEN** the system creates a threaded reply nested under the parent comment

### Requirement: Inline block-level comments
The system SHALL support inline comments anchored to a specific block in the document.

#### Scenario: Add inline comment on a block
- **WHEN** user selects a block and creates a comment
- **THEN** the system creates a comment anchored to that block's ID, and the block is visually highlighted in the editor

#### Scenario: Orphaned inline comment
- **WHEN** the block a comment is anchored to is deleted
- **THEN** the system moves the comment to a "Detached Comments" section rather than deleting it

### Requirement: @-mention in comments
The system SHALL support @-mentioning project members in comments.

#### Scenario: @-mention triggers notification
- **WHEN** user types `@username` in a comment and submits
- **THEN** the mentioned user receives a notification with a link to the comment

#### Scenario: @-mention autocomplete
- **WHEN** user types `@` followed by characters in the comment input
- **THEN** the system shows an autocomplete dropdown of matching project members

### Requirement: Comment resolve lifecycle
The system SHALL support resolving and reopening comments.

#### Scenario: Resolve a comment thread
- **WHEN** user clicks "Resolve" on a comment
- **THEN** the comment thread is marked as resolved and visually collapsed in the comments panel

#### Scenario: Reopen a resolved comment
- **WHEN** user clicks "Reopen" on a resolved comment
- **THEN** the comment thread is marked as active again

### Requirement: Comment permalink
The system SHALL provide a copyable permalink for each comment.

#### Scenario: Copy comment link
- **WHEN** user clicks "Copy link" on a comment
- **THEN** the system copies a URL that navigates directly to the page with the comment highlighted

### Requirement: Comment API
The system SHALL expose REST endpoints for comment operations under `/api/v1/projects/:pid/wiki/pages/:id/comments`.

#### Scenario: List comments for a page
- **WHEN** client sends `GET /api/v1/projects/:pid/wiki/pages/:id/comments`
- **THEN** the system returns all comments (page-level and inline) with their threads, ordered by creation time

#### Scenario: Create comment via API
- **WHEN** client sends `POST /api/v1/projects/:pid/wiki/pages/:id/comments` with body, optional anchor_block_id, and optional parent_comment_id
- **THEN** the system creates the comment and returns it with 201 status

### Requirement: Real-time comment updates
The system SHALL broadcast WebSocket events when comments are created, resolved, or deleted.

#### Scenario: Comment created event
- **WHEN** a comment is created on a wiki page
- **THEN** the system broadcasts a `wiki.comment.created` event to all project members viewing that page
