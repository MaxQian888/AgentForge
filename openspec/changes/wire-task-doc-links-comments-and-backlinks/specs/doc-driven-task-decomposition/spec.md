## ADDED Requirements

### Requirement: Decompose tasks from document selection
The system SHALL allow users to select content in a wiki page and create linked sub-tasks from the selection.

#### Scenario: Create tasks from selected section
- **WHEN** user selects one or more blocks in a document and clicks "Create Tasks"
- **THEN** the system creates tasks for each selected block/item, each linked back to the source page with link_type=requirement and the anchor_block_id of the source block

#### Scenario: Created tasks appear in project backlog
- **WHEN** tasks are created from a document selection
- **THEN** the tasks appear in the project's backlog with status "todo" and a link to the source document visible in the task detail

### Requirement: Decomposition API
The system SHALL expose an endpoint for document-driven task decomposition.

#### Scenario: Decompose via API
- **WHEN** client sends `POST /api/v1/projects/:pid/wiki/pages/:id/decompose-tasks` with an array of block_ids and optional parent_task_id
- **THEN** the system creates tasks from the specified blocks, links them to the page, and returns the created tasks with 201

### Requirement: Paragraph-level traceability
Each task created from a document SHALL store the source page ID and anchor block ID for traceability.

#### Scenario: Task traces to source paragraph
- **WHEN** user views a task created from a document
- **THEN** the task detail shows a "Source" link that navigates to the document page scrolled to the specific block

#### Scenario: Document shows decomposed tasks inline
- **WHEN** user views a document page that has had tasks decomposed from it
- **THEN** blocks that generated tasks display a small task-count badge indicating how many tasks were created from that block
