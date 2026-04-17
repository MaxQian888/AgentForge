## MODIFIED Requirements

### Requirement: Page tree CRUD

The system SHALL support creating, reading, updating, and soft-deleting wiki pages within a wiki space through the unified knowledge-asset API. Pages SHALL be organized in a hierarchical tree with parent-child relationships. Internally, each page SHALL be a `KnowledgeAsset` with `kind=wiki_page`.

#### Scenario: Create a root page

- **WHEN** user creates a page with no parent specified
- **THEN** the system creates a root-level `kind=wiki_page` asset in the wiki space with the given title and empty content

#### Scenario: Create a child page

- **WHEN** user creates a page with a valid parent page ID
- **THEN** the system creates the page as a child of the specified parent, appended at the end of the parent's children

#### Scenario: Update page title and content

- **WHEN** user updates a page's title or content
- **THEN** the system persists the changes and updates the `updated_at` timestamp

#### Scenario: Soft-delete a page with children

- **WHEN** user deletes a page that has child pages
- **THEN** the system soft-deletes the page and all its descendants recursively

### Requirement: Page tree API

The system SHALL expose REST endpoints for page tree operations under `/api/v1/projects/:pid/knowledge/assets` with `kind=wiki_page` scoping.

#### Scenario: List page tree

- **WHEN** client sends `GET /api/v1/projects/:pid/knowledge/assets/tree?kind=wiki_page`
- **THEN** the system returns the full page tree with `id`, `title`, `parent_id`, `sort_order`, and children (nested), excluding soft-deleted pages

#### Scenario: Create page via API

- **WHEN** client sends `POST /api/v1/projects/:pid/knowledge/assets` with `{kind:"wiki_page", title, parent_id?, content_json?}`
- **THEN** the system creates the page and returns the created asset with 201 status

### Requirement: Real-time page tree updates

The system SHALL broadcast WebSocket events when pages are created, updated, moved, or deleted. Event names SHALL use the `knowledge.asset.*` namespace with `kind=wiki_page` in the payload.

#### Scenario: Page created event

- **WHEN** a page is created in a wiki space
- **THEN** the system broadcasts a `knowledge.asset.created` event to all project members with the page's `id`, `title`, `parent_id`, and `kind=wiki_page`

#### Scenario: Page moved event

- **WHEN** a page is moved or reordered
- **THEN** the system broadcasts a `knowledge.asset.moved` event with the page's `id`, old `parent_id`, new `parent_id`, new `sort_order`, and `kind=wiki_page`
