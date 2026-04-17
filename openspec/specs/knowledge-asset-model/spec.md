# knowledge-asset-model Specification

## Purpose
Define the unified `KnowledgeAsset` domain model that collapses wiki pages, ingested files, and templates into a single project-scoped entity discriminated by `kind`, along with its invariants, RBAC gating, REST surface, and WebSocket lifecycle events.

## Requirements

### Requirement: Knowledge asset is the single first-class document entity

The system SHALL model every project-scoped document with a single `KnowledgeAsset` entity discriminated by a `kind` field. Supported kinds SHALL be `wiki_page`, `ingested_file`, and `template`. Each asset SHALL carry `id`, `project_id`, `kind`, `title`, `owner_id`, `created_by`, `updated_by`, `created_at`, `updated_at`, `deleted_at`, and a monotonically-increasing `version` used for optimistic locking.

#### Scenario: Create a wiki-page asset

- **WHEN** a caller creates an asset with `kind=wiki_page`, a title, an optional `parent_id`, and an optional `content_json`
- **THEN** the system persists a new `KnowledgeAsset` row with `kind=wiki_page`, `wiki_space_id` set to the project's wiki space, `content_json` set (defaulting to an empty BlockNote document), `file_ref` null, and returns the created asset

#### Scenario: Create an ingested-file asset

- **WHEN** a caller creates an asset with `kind=ingested_file`, a title, and a `file_ref`
- **THEN** the system persists a new `KnowledgeAsset` row with `kind=ingested_file`, `file_ref` set, `wiki_space_id` null, `parent_id` null, `content_json` null, and `ingest_status=pending`

#### Scenario: Create a template asset

- **WHEN** a caller creates an asset with `kind=template`, a title, `template_category`, and `content_json`
- **THEN** the system persists a new `KnowledgeAsset` row with `kind=template`, `wiki_space_id` set, `template_category` set, `parent_id` null, and `content_json` set

### Requirement: Kind-specific invariants are enforced by the repository

The repository layer SHALL reject writes that violate the kind-specific invariants, regardless of whether the caller is an HTTP handler, a workflow invocation, or an internal service.

#### Scenario: Reject wiki_page without content_json

- **WHEN** a caller attempts to create or update a `kind=wiki_page` asset with `content_json` null
- **THEN** the system rejects the write with a validation error

#### Scenario: Reject ingested_file with wiki_space_id or parent_id

- **WHEN** a caller attempts to create or update a `kind=ingested_file` asset with non-null `wiki_space_id` or non-null `parent_id`
- **THEN** the system rejects the write with a validation error

#### Scenario: Reject template without template_category

- **WHEN** a caller attempts to create or update a `kind=template` asset with null `template_category`
- **THEN** the system rejects the write with a validation error

### Requirement: Asset CRUD operations are gated by project RBAC

All asset CRUD operations SHALL be gated by the project RBAC role resolved from the caller's `PrincipalContext`. Human callers supply the role via the HTTP session; agent-initiated callers supply it via the agent's `roleId` mapped to a project role. Required roles:

- Read / list / search: `viewer` or higher
- Create / update / soft-delete / comment / version-snapshot: `editor` or higher
- Restore soft-deleted asset / move tree / manage system templates: `admin` or higher

#### Scenario: Viewer can list assets but cannot create

- **WHEN** a principal with role `viewer` calls the list endpoint
- **THEN** the system returns the asset list
- **WHEN** the same principal attempts to create an asset
- **THEN** the system rejects the request with a forbidden error

#### Scenario: Editor can update, cannot restore

- **WHEN** a principal with role `editor` updates an asset's `content_json`
- **THEN** the system persists the change
- **WHEN** the same principal attempts to restore a previously soft-deleted asset
- **THEN** the system rejects the request with a forbidden error

#### Scenario: Agent-initiated action uses the same gate

- **WHEN** a workflow step invokes the asset service to update an asset, carrying a `PrincipalContext` with role `editor`
- **THEN** the system performs the update under the same RBAC rules a human `editor` would use

### Requirement: Asset soft-delete with tree cascade for wiki pages

The system SHALL soft-delete assets (`deleted_at = now`) rather than hard-delete. Soft-deleting a `kind=wiki_page` with descendants SHALL cascade the soft-delete to all descendants.

#### Scenario: Soft-delete wiki page with children

- **WHEN** a caller soft-deletes a `kind=wiki_page` asset that has child pages
- **THEN** the system marks the asset and every descendant with `deleted_at = now`

#### Scenario: Soft-delete ingested file does not cascade

- **WHEN** a caller soft-deletes an `ingested_file` asset
- **THEN** the system marks only that asset with `deleted_at = now`

### Requirement: Asset REST API

The system SHALL expose a unified REST surface under `/api/v1/projects/:pid/knowledge/assets`.

#### Scenario: List assets filtered by kind

- **WHEN** a client sends `GET /api/v1/projects/:pid/knowledge/assets?kind=ingested_file`
- **THEN** the system returns non-deleted assets of that kind with pagination metadata

#### Scenario: Get asset by id

- **WHEN** a client sends `GET /api/v1/projects/:pid/knowledge/assets/:id`
- **THEN** the system returns the asset including kind-specific fields

#### Scenario: Update asset with optimistic locking

- **WHEN** a client sends `PUT /api/v1/projects/:pid/knowledge/assets/:id` with a `version` that matches the server's current version
- **THEN** the system persists the update and returns the new version
- **WHEN** the supplied `version` is stale
- **THEN** the system rejects the request with a conflict error

### Requirement: Asset WebSocket events

The system SHALL broadcast asset lifecycle events to subscribed project members. Every event payload SHALL include `project_id`, `asset_id`, `kind`, and `version`.

#### Scenario: Asset created event

- **WHEN** an asset is created
- **THEN** the system broadcasts `knowledge.asset.created` with the asset fields

#### Scenario: Asset content changed event

- **WHEN** a `wiki_page` or `template` asset's `content_json` is saved
- **THEN** the system broadcasts `knowledge.asset.content_changed` with `asset_id`, `kind`, `version`, and `content_text_length`

#### Scenario: Asset moved event

- **WHEN** a `wiki_page` asset is moved or reordered
- **THEN** the system broadcasts `knowledge.asset.moved` with `asset_id`, old `parent_id`, new `parent_id`, and new `sort_order`

#### Scenario: Asset deleted event

- **WHEN** an asset is soft-deleted
- **THEN** the system broadcasts `knowledge.asset.deleted` with `asset_id` and `kind`

### Requirement: Asset content_json preserves live-artifact block references across lifecycle operations

The system SHALL round-trip `live_artifact` block props through every asset lifecycle operation — save, load, version snapshot, version restore, and copy-paste into another asset — without modifying, stripping, or flattening the block's reference props.

#### Scenario: Save then load preserves references

- **WHEN** an asset containing one or more live-artifact blocks is saved and then reloaded
- **THEN** each live-artifact block's `live_kind`, `target_ref`, `view_opts`, and `view_opts_schema_version` are byte-identical to what was saved

#### Scenario: Version snapshot captures references not projections

- **WHEN** a named version is created on an asset containing live-artifact blocks
- **THEN** the version's stored `content_json` contains the block references but no embedded projection payload

#### Scenario: Version restore keeps references live

- **WHEN** an asset is restored from a version that contains live-artifact blocks
- **THEN** the restored blocks remain live — they project against current entity state, not state as of the snapshot

#### Scenario: Copy-paste replicates the reference

- **WHEN** a live-artifact block is copied from one wiki-page asset and pasted into another
- **THEN** the destination asset's `content_json` contains a live-artifact block with identical reference props (the BlockNote id may be regenerated)
