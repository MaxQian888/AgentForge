# task-doc-bidirectional-links Specification

## Purpose
Define typed bidirectional links between tasks and wiki pages so related documents and tasks remain visible, queryable, and removable from either side.

## Requirements
### Requirement: Entity link creation

The system SHALL allow users to create typed links between tasks and knowledge assets. Supported link types SHALL be: `requirement`, `design`, `test`, `retro`, `reference`, and `materialized_from`. Link endpoints SHALL identify knowledge assets by `target_type=knowledge_asset` paired with the asset's `id` and `kind`.

#### Scenario: Link a task to a wiki page

- **WHEN** user links a task to a `wiki_page` asset with link type `requirement`
- **THEN** the system creates an `entity_link` record with `source_type=task`, `target_type=knowledge_asset`, `target_kind=wiki_page`, and `link_type=requirement`

#### Scenario: Link a task to an ingested file

- **WHEN** user links a task to an `ingested_file` asset with link type `reference`
- **THEN** the system creates an `entity_link` record with `source_type=task`, `target_type=knowledge_asset`, `target_kind=ingested_file`, and `link_type=reference`

#### Scenario: Link a wiki page to a task

- **WHEN** user links a `wiki_page` asset to a task with link type `design`
- **THEN** the system creates an `entity_link` record with `source_type=knowledge_asset`, `source_kind=wiki_page`, `target_type=task`, and `link_type=design`

### Requirement: Bidirectional link display

The system SHALL display links from both directions. A task's detail panel SHALL show a "Related Docs" section. A knowledge-asset view SHALL show a "Related Tasks" section.

#### Scenario: Task shows related assets

- **WHEN** user views a task that has linked knowledge assets
- **THEN** the task detail panel displays a "Related Docs" section listing each linked asset with its title, `kind`, link type, and last-updated timestamp

#### Scenario: Asset shows related tasks with live status

- **WHEN** user views a knowledge asset that has linked tasks
- **THEN** the asset view displays a "Related Tasks" panel showing each task's title, status, assignee, and due date, updated in real-time

### Requirement: Link removal

The system SHALL allow users to remove links between entities.

#### Scenario: Remove a task-asset link

- **WHEN** user removes a link between a task and a knowledge asset
- **THEN** the system soft-deletes the `entity_link` record and the link disappears from both the task and asset views

### Requirement: Link API

The system SHALL expose REST endpoints for entity link operations.

#### Scenario: Create link via API

- **WHEN** client sends `POST /api/v1/projects/:pid/links` with `source_type`, `source_id`, `source_kind?` (required when `source_type=knowledge_asset`), `target_type`, `target_id`, `target_kind?` (required when `target_type=knowledge_asset`), and `link_type`
- **THEN** the system creates the link and returns 201

#### Scenario: List links for an entity via API

- **WHEN** client sends `GET /api/v1/projects/:pid/links?source_type=task&source_id=:tid`
- **THEN** the system returns all links where the specified entity is either source or target, including `target_kind` / `source_kind` fields for knowledge-asset endpoints

#### Scenario: Delete link via API

- **WHEN** client sends `DELETE /api/v1/projects/:pid/links/:linkId`
- **THEN** the system soft-deletes the link and returns 204

### Requirement: Real-time link updates

The system SHALL broadcast WebSocket events when links are created or deleted.

#### Scenario: Link created event

- **WHEN** a new entity link is created
- **THEN** the system broadcasts a `link.created` event to project members with source, target, and kind details
