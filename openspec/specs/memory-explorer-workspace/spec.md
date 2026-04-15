# memory-explorer-workspace Specification

## Purpose
Define the frontend memory explorer workspace contract so operators can inspect project memory, review full memory detail, export scoped history, and run safe cleanup flows from `/memory`.
## Requirements
### Requirement: Memory workspace presents project-scoped stats and filtered explorer results
The system SHALL provide a project-scoped memory explorer workspace on `/memory` that reuses the currently selected dashboard project, loads summary stats plus filtered memory results from the existing memory explorer API, and exposes operator filters for query text, scope, category, role, tag, date window, and result window without duplicating backend truth.

#### Scenario: Selected project loads memory stats and results together
- **WHEN** an operator opens `/memory` with a selected project
- **THEN** the workspace displays summary stats and a memory result list sourced from the authenticated `/api/v1/projects/:pid/memory` and `/memory/stats` endpoints
- **THEN** the visible filters use one shared query state so the result list and stats stay aligned to the same project scope

#### Scenario: Operator refines explorer filters
- **WHEN** an operator changes the query, scope, category, role, tag, date window, or result window filters
- **THEN** the workspace refreshes the explorer results using those filters instead of silently dropping them
- **THEN** the workspace makes it clear which filters are currently active before any destructive or export action is triggered

#### Scenario: No project is selected
- **WHEN** the operator opens `/memory` without a selected dashboard project
- **THEN** the page shows an explicit project-selection empty state instead of a broken explorer shell
- **THEN** memory explorer API requests are not fired until a project scope exists

### Requirement: Memory workspace supports full-entry inspection through a responsive detail surface
The system SHALL let operators inspect one memory entry at a time through a detail surface that loads the full memory record from the authenticated detail endpoint, shows complete content plus explorer metadata, tags, curation state, and single-entry export actions, and remains accessible across wide and narrow viewports.

#### Scenario: Operator opens a memory entry on a wide viewport
- **WHEN** an operator selects a memory result while the workspace has room for a split layout
- **THEN** the workspace keeps the result list visible and opens a detail surface showing the entry content, scope, category, timestamps, access information, structured metadata, normalized tags, and curation affordances returned by the API
- **THEN** the operator can inspect the selected entry without losing the current filter or list context

#### Scenario: Operator opens a memory entry on a narrow viewport
- **WHEN** an operator selects a memory result on a narrow viewport
- **THEN** the workspace presents the detail content through an accessible sheet, dialog, or equivalent focused surface
- **THEN** dismissing that surface returns the operator to the same filtered result list state

#### Scenario: Detail loading or failure is isolated
- **WHEN** loading the selected memory detail is slow or fails
- **THEN** the workspace shows a detail-scoped loading or error state without discarding the current result list
- **THEN** the operator can retry detail loading without resetting the broader explorer filters

### Requirement: Memory workspace provides safe export and cleanup operations
The system SHALL expose operator management actions for single delete, bulk delete, episodic cleanup, filtered JSON export, and single-entry export using the existing memory explorer API or loaded detail payload, and it SHALL require explicit confirmation for destructive actions.

#### Scenario: Operator exports the current filtered memory scope
- **WHEN** an operator triggers export from the workspace while filters are active
- **THEN** the workspace requests the authenticated `/memory/export` endpoint using the current explorer filter scope
- **THEN** the downloaded payload reflects the current filtered scope instead of an unfiltered project dump

#### Scenario: Operator exports the selected memory from detail
- **WHEN** an operator triggers single-entry export from the detail surface
- **THEN** the workspace downloads a file containing that memory's full detail payload, including metadata and tags
- **THEN** the export preserves the currently viewed entry even if broader list filters are active

#### Scenario: Operator bulk deletes selected entries
- **WHEN** an operator selects multiple memory entries and confirms bulk deletion
- **THEN** the workspace submits only the explicitly selected memory IDs to the authenticated `/memory/bulk-delete` endpoint
- **THEN** the confirmation and success feedback report how many entries were targeted and removed

#### Scenario: Operator clears old episodic memories
- **WHEN** an operator confirms cleanup for episodic memories older than an explicit cutoff or retention window
- **THEN** the workspace calls the authenticated `/memory/cleanup` endpoint with that cleanup scope instead of deleting arbitrary memory categories
- **THEN** the operator receives a deleted-count result and can distinguish cleanup from manual selection-based deletion

#### Scenario: Operator deletes a single memory entry from list or detail
- **WHEN** an operator triggers deletion for one memory entry
- **THEN** the workspace asks for confirmation before issuing the delete request
- **THEN** the same deletion semantics apply whether the action starts from the result list or the detail surface

### Requirement: Memory workspace keeps feedback and data surfaces synchronized after operator actions
The system SHALL keep list results, stats, selection state, detail state, and operator feedback synchronized after explorer actions so the workspace remains truthful without requiring a full page reload.

#### Scenario: Successful action refreshes explorer truth
- **WHEN** an export-neutral mutating action such as note creation, note update, tag curation, single delete, bulk delete, or episodic cleanup succeeds
- **THEN** the workspace refreshes the result list and summary stats for the current filter scope
- **THEN** any deleted or now-invalid selected memory entry is cleared or replaced with an explicit empty detail state

#### Scenario: Action failure preserves explorer context
- **WHEN** a create, update, delete, cleanup, or export action fails
- **THEN** the workspace preserves the current filters, list results, and current selection instead of resetting the explorer state
- **THEN** the error feedback is visible in the relevant action surface so the operator can retry deliberately

#### Scenario: Empty filtered result remains actionable
- **WHEN** the current filters produce zero matching memory entries
- **THEN** the workspace shows an explicit empty-result state that still preserves the active filter context
- **THEN** the operator can clear filters, create a note, export scope, or adjust cleanup intent from that state without guessing why the list is empty

### Requirement: Memory workspace supports operator note authoring and controlled curation
The system SHALL let operators record project notes and maintain tags without leaving `/memory`. The workspace MUST allow note creation with title, content, and optional tags, permit content edits only for editable operator-authored notes, and present clear read-only guidance for system-generated memories.

#### Scenario: Operator creates a note from the workspace
- **WHEN** an operator submits the note composer with a title, content, and optional tags while a project is selected
- **THEN** the workspace stores the note through the canonical project memory write API
- **THEN** the new note appears in the current result set and detail surface without requiring a full page reload

#### Scenario: Operator edits an existing operator note
- **WHEN** an operator opens an editable note and saves updated content or tags
- **THEN** the workspace calls the supported memory update path
- **THEN** the refreshed detail and list surfaces show the saved curation state

#### Scenario: Workspace blocks unsupported content edits
- **WHEN** an operator opens a system-generated memory that is not editable
- **THEN** the workspace hides or disables content-edit actions for that entry
- **THEN** any attempted unsupported edit surfaces an explicit read-only explanation instead of failing silently

