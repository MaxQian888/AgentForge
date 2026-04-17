## ADDED Requirements

### Requirement: Project-scoped unified search across kinds

The system SHALL expose a single search endpoint that returns matching knowledge assets across all kinds within a project. Callers SHALL be able to filter by `kind`, by soft-delete state, by updated-range, and by free-text query. The endpoint SHALL be gated by the project RBAC `viewer` role or higher.

#### Scenario: Query returns results from multiple kinds

- **WHEN** a client sends `GET /api/v1/projects/:pid/knowledge/search?q=authentication`
- **THEN** the system returns matching assets of every kind the caller is permitted to read, in a single ranked list

#### Scenario: Filter by kind

- **WHEN** a client sends `GET /api/v1/projects/:pid/knowledge/search?q=authentication&kind=wiki_page`
- **THEN** the system returns only matching `wiki_page` assets

#### Scenario: Filter by updated range

- **WHEN** a client sends `GET /api/v1/projects/:pid/knowledge/search?q=authentication&updated_after=2026-01-01`
- **THEN** the system returns matching assets whose `updated_at` is on or after the supplied date

#### Scenario: Non-viewer receives forbidden

- **WHEN** a principal without project membership calls the search endpoint
- **THEN** the system rejects the request with a forbidden error

### Requirement: Search result shape is provider-agnostic

Every search result item SHALL carry `id`, `kind`, `title`, `updated_at`, a `snippet` string, a `score` number, and a `match_locations` opaque string. `score` and `snippet` are provider-specific and not guaranteed to be stable across provider swaps.

#### Scenario: Result item fields

- **WHEN** the search endpoint returns results
- **THEN** each item contains at minimum `id`, `kind`, `title`, `updated_at`, `snippet`, `score`, `match_locations`

#### Scenario: Pagination via cursor

- **WHEN** more results are available than the page size
- **THEN** the response includes a `nextCursor` string that the client can pass back via `?cursor=` to retrieve the next page

### Requirement: Lexical search backing index is maintained on save

The system SHALL maintain a lexical search index (Postgres tsvector or equivalent) keyed on each asset's `title` and `content_text`. The index SHALL update whenever `content_text` or `title` changes.

#### Scenario: Index updates on wiki save

- **WHEN** a `wiki_page` asset is saved and its derived `content_text` changes
- **THEN** the system updates the search index entry for the asset within the same transaction as the content update

#### Scenario: Index updates on ingest completion

- **WHEN** an `ingested_file` asset transitions to `ingest_status=ready`
- **THEN** the system updates the search index entry using the parsed `content_text`

#### Scenario: Soft-deleted assets excluded from results

- **WHEN** an asset is soft-deleted
- **THEN** subsequent search calls do not return that asset

### Requirement: Provider seam allows swapping the backend

The search endpoint SHALL be implemented against a `SearchProvider` Go interface with a single `Search(ctx, ProjectID, Query) (Results, error)` method and an implementation-defined upsert/delete hook. The default implementation SHALL use Postgres FTS. A future implementation (Bleve, embedding-based) SHALL be swappable without changing the REST contract.

#### Scenario: Default provider is Postgres FTS

- **WHEN** the system boots without a search provider override
- **THEN** the `SearchProvider` interface is bound to the Postgres-FTS implementation

#### Scenario: Provider swap preserves REST shape

- **WHEN** a future change replaces the provider with an embedding-based implementation
- **THEN** the REST endpoint response shape — field names, types, pagination, filter semantics — remains unchanged
