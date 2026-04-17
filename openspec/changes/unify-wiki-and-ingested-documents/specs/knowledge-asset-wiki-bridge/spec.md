## ADDED Requirements

### Requirement: Wiki page saves enqueue for knowledge indexing

The system SHALL emit an eventbus event `knowledge.asset.content_changed` whenever a `kind=wiki_page` or `kind=template` asset's content is saved. The event SHALL carry `asset_id`, `kind`, `project_id`, `version`, and `content_text_length`.

#### Scenario: Save wiki page triggers enqueue

- **WHEN** a `wiki_page` asset is saved and persisted
- **THEN** the system emits `knowledge.asset.content_changed` to the eventbus

#### Scenario: Save template triggers enqueue

- **WHEN** a `template` asset's content is saved
- **THEN** the system emits `knowledge.asset.content_changed` to the eventbus

#### Scenario: Reading does not enqueue

- **WHEN** a caller reads an asset without modifying it
- **THEN** no `knowledge.asset.content_changed` event is emitted

### Requirement: IndexPipeline interface receives enqueue events

The system SHALL define an `IndexPipeline` Go interface with a single `Enqueue(ctx, AssetEnqueueEvent) error` method. The default implementation SHALL be a `NoopIndexPipeline` that logs and records a metric but does not index. The interface SHALL be the integration point for a later change that introduces a real indexing pipeline.

#### Scenario: Default pipeline is Noop

- **WHEN** the system boots without an index-pipeline override
- **THEN** the `IndexPipeline` is bound to the `NoopIndexPipeline` implementation

#### Scenario: Event reaches the pipeline

- **WHEN** a `knowledge.asset.content_changed` event is emitted
- **THEN** the bound `IndexPipeline.Enqueue` is invoked with the event payload

### Requirement: Materialize ingested file as editable wiki page

The system SHALL support materializing an `ingested_file` asset into a sibling `wiki_page` asset whose content is prepopulated from the parsed chunks. The two assets SHALL remain decoupled after creation: subsequent changes to either SHALL NOT sync to the other.

#### Scenario: Materialize creates a linked wiki page

- **WHEN** a caller sends `POST /api/v1/projects/:pid/knowledge/assets/:id/materialize-as-wiki` against an `ingested_file` asset with `ingest_status=ready`, providing a target `parent_id` and optional title
- **THEN** the system creates a new `kind=wiki_page` asset whose `content_json` contains a header callout block referencing the source file and one paragraph block per parsed chunk
- **THEN** the system creates an `entity_link` record with `link_type=materialized_from` from the new wiki page to the source ingested file
- **THEN** the system returns the new wiki page asset

#### Scenario: Materialize rejected on non-ready ingest

- **WHEN** the action is invoked against an `ingested_file` with `ingest_status != ready`
- **THEN** the system rejects the request with a validation error

#### Scenario: Materialize rejected against wrong kind

- **WHEN** the action is invoked against a `wiki_page` or `template` asset
- **THEN** the system rejects the request with a validation error

#### Scenario: Later edits do not cross-propagate

- **WHEN** either the source file is re-uploaded or the materialized wiki page is edited
- **THEN** the other asset is unchanged and no sync event is emitted

### Requirement: Materialization surface exposes source linkage in UI

The wiki-page view SHALL surface the `materialized_from` link so users can navigate to the source file; the ingested-file view SHALL surface all wiki pages that were materialized from it.

#### Scenario: Wiki page shows source file pill

- **WHEN** a user views a wiki page that has a `materialized_from` link
- **THEN** the UI shows a pill linking to the source ingested-file asset with its title and file type

#### Scenario: Source file shows materializations panel

- **WHEN** a user views an ingested-file asset that has one or more materializations
- **THEN** the UI shows a "Materialized as" panel listing the linked wiki pages

### Requirement: Source-updated hint when re-ingested

When an `ingested_file` asset is re-uploaded, the system SHALL mark each related materialized wiki page with an advisory flag noting the source has changed so the user can decide whether to re-materialize.

#### Scenario: Re-upload flags materializations

- **WHEN** a re-upload transitions an ingested file to `ready` with new content
- **THEN** the system sets `source_updated_since_materialize=true` on every wiki page linked via `materialized_from` pointing at that file

#### Scenario: User acknowledges the update

- **WHEN** the user dismisses the hint on a materialized wiki page
- **THEN** the system clears the flag on that specific wiki page only
