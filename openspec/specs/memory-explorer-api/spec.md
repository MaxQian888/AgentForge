# memory-explorer-api Specification

## Purpose
Define the authenticated Go backend contract for memory explorer listing, detail, stats, episodic export, and cleanup workflows.
## Requirements
### Requirement: Memory explorer API exposes filtered project memory listing and detail
The system SHALL provide authenticated memory explorer endpoints under `/api/v1/projects/:pid/memory` that support explorer-ready filtering for project memory records, and an authenticated detail endpoint under `/api/v1/projects/:pid/memory/:mid` that returns full entry content plus explorer metadata. The list contract MUST accept `query` as the canonical search parameter while remaining backward compatible with legacy `q`, and it MUST honor `scope`, `category`, `roleId`, `tag`, `startAt`, `endAt`, and `limit` filters when supplied. List and detail responses MUST expose normalized `tags` plus curation fields that indicate whether the record is an operator-authored note and whether content edits are allowed.

#### Scenario: Explorer list honors current frontend filters
- **WHEN** an authenticated caller requests project memories with `query`, `scope`, `category`, or `tag`
- **THEN** the API returns only entries that match those filters within the requested project
- **THEN** the response includes explorer-ready memory DTOs with normalized tag data instead of silently ignoring unsupported filter fields

#### Scenario: Legacy search parameter remains compatible
- **WHEN** an authenticated caller uses the legacy `q` parameter instead of `query`
- **THEN** the API applies the same search semantics as the canonical `query` parameter
- **THEN** existing consumers do not break during contract migration

#### Scenario: Memory detail exposes explorer metadata and curation state
- **WHEN** an authenticated caller requests a single memory entry by ID within the correct project scope
- **THEN** the API returns the full memory content, metadata, timestamps, access information, normalized tags, and curation flags for that entry
- **THEN** any related task or session hints derivable from stored metadata are surfaced as structured detail fields instead of requiring the frontend to reverse-engineer raw JSON strings

### Requirement: Memory explorer API supports bulk deletion and age-based cleanup
The system SHALL provide authenticated management endpoints that can delete selected memory entries or clear episodic memories older than an explicit cutoff while preserving project isolation and existing role-scoped access rules.

#### Scenario: Bulk delete removes selected memory entries
- **WHEN** an authenticated caller submits a bulk delete request containing explicit memory IDs for a project
- **THEN** the API deletes only the matching entries that are accessible within that project scope
- **THEN** the response reports how many entries were deleted

#### Scenario: Clear old episodic memories returns deleted count
- **WHEN** an authenticated caller requests cleanup of episodic memories older than a provided cutoff or retention window
- **THEN** the API deletes only episodic entries older than that threshold within the requested project scope
- **THEN** the response includes the deleted count so explorer clients can confirm the cleanup result

#### Scenario: Role-scoped cleanup does not cross access boundaries
- **WHEN** a cleanup or bulk delete request targets memories that include role-scoped entries for another role
- **THEN** the API does not delete inaccessible role-scoped records
- **THEN** the operation preserves the same role-scope access rules used by memory detail and history queries

### Requirement: Memory explorer API exposes summary stats and episodic export
The system SHALL provide authenticated endpoints for memory explorer summary stats and episodic-memory export so operators can inspect current usage and download filtered memory history without depending on mock data. Stats and export filters MUST remain aligned with the explorer list semantics, including tag-aware scoping when a tag filter is active.

#### Scenario: Stats summarize current explorer scope
- **WHEN** an authenticated caller requests memory explorer stats for a project with optional scope, category, role, tag, or date filters
- **THEN** the API returns counts and breakdowns for the matching memory set, including approximate storage usage derived from stored content and metadata
- **THEN** the stats reflect the same filter semantics as the corresponding explorer list query

#### Scenario: Episodic export respects current filter scope
- **WHEN** an authenticated caller exports episodic memories for a project with optional role, tag, or date filters
- **THEN** the API returns a JSON export payload containing only memories visible within that filtered scope
- **THEN** the export format remains stable enough for backup or later analysis by explorer consumers

### Requirement: Memory explorer API supports operator note authoring and controlled updates
The system SHALL provide authenticated create and update semantics on the existing `/api/v1/projects/:pid/memory` resource family so operators can record project notes, curate tags, and edit only the memories that are explicitly allowed to change content.

#### Scenario: Operator creates a project note with deterministic defaults
- **WHEN** an authenticated caller submits a memory create request for an operator-authored note with key, content, and optional tags
- **THEN** the API persists the note as the canonical operator-note shape within the requested project scope
- **THEN** omitted scope or category fields still resolve to the supported defaults instead of requiring raw repository values

#### Scenario: Legacy operator note category remains compatible
- **WHEN** an authenticated caller still submits `category = operator_note`
- **THEN** the API accepts the request as a compatibility alias
- **THEN** the persisted record and returned DTO expose the canonical stored category and operator-note curation fields

#### Scenario: Operator updates note content and tags
- **WHEN** an authenticated caller partially updates an editable operator note
- **THEN** the API applies the requested `key`, `content`, and `tags` changes in one request
- **THEN** the response returns the refreshed memory detail with updated curation fields

#### Scenario: System-generated memory rejects content edits
- **WHEN** an authenticated caller attempts to change the `key` or `content` of a non-editable system-generated memory
- **THEN** the API rejects the request with a clear client-visible error
- **THEN** tag-only curation remains available only within the supported access boundary

