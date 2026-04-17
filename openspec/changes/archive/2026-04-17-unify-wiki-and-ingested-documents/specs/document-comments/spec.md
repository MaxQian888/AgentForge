## MODIFIED Requirements

### Requirement: Page-level comments

The system SHALL support page-level comment threads on knowledge assets that allow commenting. Page-level comments SHALL be supported for `kind=wiki_page`, `kind=template`, and `kind=ingested_file`.

#### Scenario: Add page-level comment on wiki page

- **WHEN** user submits a comment on a `wiki_page` asset without selecting a block
- **THEN** the system creates a page-level comment visible in the comments panel

#### Scenario: Add page-level comment on ingested file

- **WHEN** user submits a comment on a `kind=ingested_file` asset
- **THEN** the system creates a page-level comment with `anchor_block_id=null`

#### Scenario: Reply to comment

- **WHEN** user replies to an existing comment
- **THEN** the system creates a threaded reply nested under the parent comment

### Requirement: Inline block-level comments

The system SHALL support inline comments anchored to a specific block in the asset content. Inline comments SHALL only be permitted on kinds whose content is block-structured — currently `kind=wiki_page` and `kind=template`.

#### Scenario: Add inline comment on a block

- **WHEN** user selects a block in a `wiki_page` or `template` asset and creates a comment
- **THEN** the system creates a comment anchored to that block's ID, and the block is visually highlighted in the editor

#### Scenario: Orphaned inline comment

- **WHEN** the block a comment is anchored to is deleted
- **THEN** the system moves the comment to a "Detached Comments" section rather than deleting it

#### Scenario: Reject inline comment on ingested file

- **WHEN** a caller attempts to create a comment with a non-null `anchor_block_id` on a `kind=ingested_file` asset
- **THEN** the system rejects the request with a validation error

### Requirement: Comment API

The system SHALL expose REST endpoints for comment operations under `/api/v1/projects/:pid/knowledge/assets/:id/comments`.

#### Scenario: List comments for an asset

- **WHEN** client sends `GET /api/v1/projects/:pid/knowledge/assets/:id/comments`
- **THEN** the system returns all comments (page-level and inline) with their threads, ordered by creation time

#### Scenario: Create comment via API

- **WHEN** client sends `POST /api/v1/projects/:pid/knowledge/assets/:id/comments` with body, optional `anchor_block_id`, and optional `parent_comment_id`
- **THEN** the system creates the comment and returns it with 201 status, subject to the kind-specific block-anchor rules

### Requirement: Real-time comment updates

The system SHALL broadcast WebSocket events when comments are created, resolved, or deleted. Event names SHALL use the `knowledge.comment.*` namespace.

#### Scenario: Comment created event

- **WHEN** a comment is created on an asset
- **THEN** the system broadcasts a `knowledge.comment.created` event to all project members viewing that asset, including `asset_id`, `kind`, and the comment fields
