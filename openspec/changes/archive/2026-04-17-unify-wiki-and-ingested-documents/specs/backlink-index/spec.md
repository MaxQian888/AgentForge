## MODIFIED Requirements

### Requirement: Automatic backlink extraction from documents

The system SHALL extract `[[entity-id]]` references from the derived `content_text` of any `KnowledgeAsset` whose content changes, and create `entity_link` records with `link_type=mention`. For `kind=wiki_page` and `kind=template`, extraction runs on save. For `kind=ingested_file`, extraction runs when ingest completes (`ingest_status=ready`) and on reupload.

#### Scenario: Backlink created on wiki page save

- **WHEN** a user saves a `wiki_page` asset whose `content_text` contains `[[task-abc123]]`
- **THEN** the system creates an `entity_link` from the asset to task abc123 with `link_type=mention`

#### Scenario: Backlink created on ingest completion

- **WHEN** an `ingested_file` asset transitions to `ingest_status=ready` and its parsed `content_text` contains `[[page-xyz]]`
- **THEN** the system creates an `entity_link` from the ingested-file asset to page xyz with `link_type=mention`

#### Scenario: Backlink removed when reference deleted

- **WHEN** a user edits a `wiki_page` to remove a `[[task-abc123]]` reference and saves
- **THEN** the system soft-deletes the corresponding mention-type `entity_link`

#### Scenario: Multiple references in one asset

- **WHEN** an asset's `content_text` contains `[[task-a]]`, `[[task-b]]`, and `[[page-c]]`
- **THEN** the system creates three `entity_link` records (one per reference), all with `link_type=mention`

### Requirement: Automatic backlink extraction from task descriptions

The system SHALL extract `[[entity-id]]` references from task descriptions on every save and create `entity_link` records with `link_type=mention`. Target IDs SHALL resolve to any `KnowledgeAsset` kind.

#### Scenario: Backlink from task description to wiki page

- **WHEN** user saves a task description containing `[[page-xyz]]` where `page-xyz` is a `wiki_page` asset id
- **THEN** the system creates an `entity_link` from the task to the wiki-page asset with `link_type=mention`

#### Scenario: Backlink from task description to ingested file

- **WHEN** user saves a task description containing `[[file-123]]` where `file-123` is an `ingested_file` asset id
- **THEN** the system creates an `entity_link` from the task to the ingested-file asset with `link_type=mention`

### Requirement: Backlinks panel

The system SHALL display a "Backlinks" panel on every knowledge asset detail view and task detail view, showing all inbound mention-type links. Each listed backlink SHALL include the referencing entity's title, type, and `kind` (for asset backlinks), plus a link to navigate there.

#### Scenario: View backlinks on a wiki page

- **WHEN** user views a `wiki_page` asset that is referenced by other assets or tasks
- **THEN** the "Backlinks" panel shows each referencing entity with title, type, `kind` (when the referrer is a knowledge asset), and a navigation link

#### Scenario: View backlinks on an ingested file

- **WHEN** user views an `ingested_file` asset that is referenced by wiki pages or tasks
- **THEN** the "Backlinks" panel shows each referencing entity with title, type, `kind`, and a navigation link

#### Scenario: View backlinks on a task

- **WHEN** user views a task that is referenced by knowledge assets
- **THEN** the "Backlinks" panel shows each referencing asset with title, `kind`, and a navigation link

### Requirement: Backlink extraction performance

The system SHALL complete backlink extraction within the same database transaction as the content save for editable kinds. Target execution time SHALL be under 50ms for content under 200KB. For `ingested_file` kinds, backlink extraction MAY run asynchronously as part of the ingest pipeline rather than blocking the upload request.

#### Scenario: Extraction within transaction for wiki save

- **WHEN** a `wiki_page` save triggers backlink extraction
- **THEN** the extraction and link upserts complete in the same transaction as the content update

#### Scenario: Extraction asynchronous for ingested file

- **WHEN** an `ingested_file` asset reaches `ingest_status=ready`
- **THEN** backlink extraction runs as part of the ingest pipeline and completes before the `knowledge.ingest.status_changed` event is emitted
