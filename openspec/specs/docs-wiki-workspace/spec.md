# docs-wiki-workspace Specification

## Purpose
Define the project-scoped docs and wiki workspace so each project has a navigable page tree with CRUD, favorites, pins, recents, and realtime updates.

## Requirements
### Requirement: Project wiki space lifecycle
The system SHALL create a wiki space automatically when a project is created. Each project SHALL have exactly one wiki space. The wiki space SHALL be deleted when its project is deleted.

#### Scenario: Wiki space auto-creation
- **WHEN** a new project is created
- **THEN** the system creates a wiki space scoped to that project with an empty page tree

#### Scenario: Wiki space deletion cascades
- **WHEN** a project is deleted
- **THEN** the system soft-deletes the wiki space and all its pages, versions, and comments

### Requirement: Page tree CRUD
The system SHALL support creating, reading, updating, and soft-deleting pages within a wiki space. Pages SHALL be organized in a hierarchical tree with parent-child relationships.

#### Scenario: Create a root page
- **WHEN** user creates a page with no parent specified
- **THEN** the system creates a root-level page in the wiki space with the given title and empty content

#### Scenario: Create a child page
- **WHEN** user creates a page with a valid parent page ID
- **THEN** the system creates the page as a child of the specified parent, appended at the end of the parent's children

#### Scenario: Update page title and content
- **WHEN** user updates a page's title or content
- **THEN** the system persists the changes and updates the `updated_at` timestamp

#### Scenario: Soft-delete a page with children
- **WHEN** user deletes a page that has child pages
- **THEN** the system soft-deletes the page and all its descendants recursively

### Requirement: Page tree ordering and move
The system SHALL support reordering pages within their parent and moving pages to a different parent.

#### Scenario: Reorder page within parent
- **WHEN** user drags a page to a new position among its siblings
- **THEN** the system updates the sort order so the page appears at the new position

#### Scenario: Move page to different parent
- **WHEN** user moves a page to a different parent
- **THEN** the system updates the page's parent and materialized path, and recursively updates all descendant paths

#### Scenario: Prevent circular move
- **WHEN** user attempts to move a page into one of its own descendants
- **THEN** the system rejects the operation with a validation error

### Requirement: Favorites, pins, and recent access
The system SHALL track user-level favorites, project-level pinned pages, and per-user recent-access history.

#### Scenario: Favorite a page
- **WHEN** user marks a page as favorite
- **THEN** the page appears in the user's favorites list for that project

#### Scenario: Pin a page
- **WHEN** a project admin pins a page
- **THEN** the page appears in the pinned section at the top of the page tree for all project members

#### Scenario: Recent access tracking
- **WHEN** user opens a page
- **THEN** the system records the access and the page appears in the user's "Recent" list, ordered by last access time

### Requirement: Page tree API
The system SHALL expose REST endpoints for page tree operations under `/api/v1/projects/:pid/wiki/pages`.

#### Scenario: List page tree
- **WHEN** client sends `GET /api/v1/projects/:pid/wiki/pages`
- **THEN** the system returns the full page tree with id, title, parent_id, sort_order, and children (nested), excluding soft-deleted pages

#### Scenario: Create page via API
- **WHEN** client sends `POST /api/v1/projects/:pid/wiki/pages` with title, optional parent_id, and optional content
- **THEN** the system creates the page and returns the created page object with 201 status

### Requirement: Real-time page tree updates
The system SHALL broadcast WebSocket events when pages are created, updated, moved, or deleted.

#### Scenario: Page created event
- **WHEN** a page is created in a wiki space
- **THEN** the system broadcasts a `wiki.page.created` event to all project members with the page's id, title, and parent_id

#### Scenario: Page moved event
- **WHEN** a page is moved or reordered
- **THEN** the system broadcasts a `wiki.page.moved` event with the page's id, old parent, new parent, and new sort_order
