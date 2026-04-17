# doc-driven-task-decomposition Specification

## Purpose
Define how wiki page selections can be decomposed into linked tasks with API support and block-level traceability back to the source document.

## Requirements
### Requirement: Decompose tasks from document selection

The system SHALL allow users to select content in a `kind=wiki_page` knowledge asset and create linked sub-tasks from the selection. Decomposition SHALL only be supported on `wiki_page` kinds; attempts against other kinds SHALL be rejected.

#### Scenario: Create tasks from selected section

- **WHEN** user selects one or more blocks in a `wiki_page` asset and clicks "Create Tasks"
- **THEN** the system creates tasks for each selected block/item, each linked back to the source asset with `link_type=requirement` and the `anchor_block_id` of the source block

#### Scenario: Created tasks appear in project backlog

- **WHEN** tasks are created from a document selection
- **THEN** the tasks appear in the project's backlog with status "todo" and a link to the source asset visible in the task detail

#### Scenario: Reject decomposition on non-wiki-page kinds

- **WHEN** a caller invokes the decompose endpoint against a `kind=template` or `kind=ingested_file` asset
- **THEN** the system rejects the request with a validation error

### Requirement: Decomposition API

The system SHALL expose an endpoint for document-driven task decomposition, addressed by knowledge-asset id.

#### Scenario: Decompose via API

- **WHEN** client sends `POST /api/v1/projects/:pid/knowledge/assets/:id/decompose-tasks` with an array of block IDs and optional `parent_task_id` against a `wiki_page` asset
- **THEN** the system creates tasks from the specified blocks, links them to the asset with `link_type=requirement`, and returns the created tasks with 201

### Requirement: Paragraph-level traceability

Each task created from a document SHALL store the source asset ID, asset `kind` at the moment of decomposition, and anchor block ID for traceability.

#### Scenario: Task traces to source paragraph

- **WHEN** user views a task created from a document
- **THEN** the task detail shows a "Source" link that navigates to the source asset scrolled to the specific block

#### Scenario: Document shows decomposed tasks inline

- **WHEN** user views a `wiki_page` asset that has had tasks decomposed from it
- **THEN** blocks that generated tasks display a small task-count badge indicating how many tasks were created from that block
