## ADDED Requirements

### Requirement: Automatic backlink extraction from documents
The system SHALL extract `[[entity-id]]` references from wiki page content on every save and create entity_link records with link_type "mention."

#### Scenario: Backlink created on page save
- **WHEN** user saves a wiki page containing `[[task-abc123]]` in the content
- **THEN** the system creates an entity_link from the page to task abc123 with link_type=mention

#### Scenario: Backlink removed when reference deleted
- **WHEN** user edits a page to remove a `[[task-abc123]]` reference and saves
- **THEN** the system soft-deletes the corresponding mention-type entity_link

#### Scenario: Multiple references in one page
- **WHEN** a page contains `[[task-a]]`, `[[task-b]]`, and `[[page-c]]`
- **THEN** the system creates three entity_link records (one per reference), all with link_type=mention

### Requirement: Automatic backlink extraction from task descriptions
The system SHALL extract `[[entity-id]]` references from task descriptions on every save and create entity_link records with link_type "mention."

#### Scenario: Backlink from task description
- **WHEN** user saves a task description containing `[[page-xyz]]`
- **THEN** the system creates an entity_link from the task to page xyz with link_type=mention

### Requirement: Backlinks panel
The system SHALL display a "Backlinks" panel on every wiki page and task detail, showing all inbound mention-type links.

#### Scenario: View backlinks on a page
- **WHEN** user views a wiki page that is referenced by other pages or tasks
- **THEN** a "Backlinks" panel shows each referencing entity with title, type, and a link to navigate there

#### Scenario: View backlinks on a task
- **WHEN** user views a task that is referenced by documents
- **THEN** a "Backlinks" panel shows each referencing page with title and a navigation link

### Requirement: Backlink extraction performance
The system SHALL complete backlink extraction within the same database transaction as the content save, with target execution time under 50ms for documents under 200KB.

#### Scenario: Extraction within transaction
- **WHEN** a page save triggers backlink extraction
- **THEN** the extraction and link upserts complete in the same transaction as the content update
