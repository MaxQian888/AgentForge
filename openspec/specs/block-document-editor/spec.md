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

### Requirement: Live-artifact block type

The block editor SHALL support a `live_artifact` custom BlockNote block type whose visual and interactive behavior is owned by the `live-artifact-blocks` capability. The editor SHALL treat live-artifact blocks as opaque — it serializes and deserializes the block's `props` without inspecting them, renders a placeholder during loading, and defers content rendering to the live-artifact components.

#### Scenario: Editor serializes live blocks by props alone

- **WHEN** a document containing live-artifact blocks is saved
- **THEN** the BlockNote JSON persists only the block's `type`, `id`, and `props` — no content array or projected data

#### Scenario: Editor renders a loading placeholder before projection resolves

- **WHEN** a wiki page with live-artifact blocks is first rendered
- **THEN** each live-artifact block shows a skeleton placeholder until the projection endpoint resolves the block's content

#### Scenario: Editor defers actions to the live-artifact component

- **WHEN** the user interacts with a live-artifact block (freeze, open source, remove)
- **THEN** the editor passes the interaction through to the live-artifact component and does not treat those as regular block-editing operations

### Requirement: Entity-card block remains distinct from live-artifact blocks

The editor SHALL continue to offer the `entity-card` block (from the existing `Embedded entity card block` requirement) for inline single-entity references to tasks, agents, and reviews. The slash menu SHALL label entity-card and live-artifact entries distinctly so operators can choose between a compact inline card and a richer projection.

#### Scenario: Slash menu distinguishes the two

- **WHEN** the user opens the slash menu
- **THEN** the menu shows "Embed task card" (entity-card) and "Embed task group (live)" as distinct entries, and similarly for agent and review options vs their live-artifact equivalents where both exist
