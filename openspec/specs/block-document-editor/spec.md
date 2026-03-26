# block-document-editor Specification

## Purpose
Define the project wiki block editor surface so document pages support rich structured authoring, embedded entities, lazy loading, and optimistic autosave.

## Requirements
### Requirement: Block editor rendering
The system SHALL render a block-level document editor on each wiki page using BlockNote. The editor SHALL support the following block types: heading (H1-H3), paragraph, bullet list, numbered list, to-do list, callout, table, code block (with syntax highlighting), image, and horizontal rule.

#### Scenario: Render page content in editor
- **WHEN** user opens a wiki page
- **THEN** the system renders the page's JSON block content in an editable BlockNote editor with all supported block types

#### Scenario: Create new block via slash menu
- **WHEN** user types `/` in the editor
- **THEN** the system shows a slash command menu listing all available block types, and inserting a selection creates the corresponding block

### Requirement: Formula block
The system SHALL support a formula block that renders LaTeX math using KaTeX.

#### Scenario: Insert formula block
- **WHEN** user inserts a formula block and types LaTeX syntax
- **THEN** the system renders the formula visually using KaTeX in the document

### Requirement: Diagram block
The system SHALL support a diagram block that renders Mermaid diagram syntax.

#### Scenario: Insert diagram block
- **WHEN** user inserts a diagram block and writes Mermaid syntax
- **THEN** the system renders the diagram visually (flowchart, sequence, gantt, etc.) in the document

### Requirement: Embedded entity card block
The system SHALL support an embedded card block that displays a live preview of a task, agent, or review entity.

#### Scenario: Embed a task card
- **WHEN** user inserts an entity card block and selects a task
- **THEN** the editor renders an inline card showing the task's title, status, assignee, and due date, updated in real-time

#### Scenario: Click embedded card to navigate
- **WHEN** user clicks an embedded entity card
- **THEN** the system navigates to the entity's detail page

### Requirement: Auto-save
The system SHALL auto-save document content after the user stops typing for 2 seconds.

#### Scenario: Auto-save on edit pause
- **WHEN** user edits document content and pauses for 2 seconds
- **THEN** the system saves the current content to the backend via PUT request

#### Scenario: Optimistic locking on save
- **WHEN** user saves content but the page's `updated_at` on the server is newer than the local version
- **THEN** the system shows a conflict notification and offers to reload or force-save

### Requirement: Editor lazy loading
The system SHALL lazy-load the block editor module to avoid increasing the initial bundle size.

#### Scenario: Editor loaded on demand
- **WHEN** user navigates to a wiki page
- **THEN** the editor JavaScript is loaded dynamically (not included in the main bundle), with a loading skeleton shown during load
