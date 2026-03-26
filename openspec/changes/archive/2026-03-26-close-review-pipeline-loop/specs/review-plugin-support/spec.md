## ADDED Requirements

### Requirement: Review execution metadata is surfaced in the Web UI
The system SHALL expose `executionMetadata` from the review backend DTO in the frontend `ReviewRecord` TypeScript type so that plugin provenance, trigger event, changed file list, and per-plugin result breakdown are available for display. The review detail panel SHALL show a structured execution metadata section when `executionMetadata` is present.

#### Scenario: Review detail panel shows execution metadata for a completed review
- **WHEN** a user opens the detail panel for a review that includes `executionMetadata`
- **THEN** the panel displays the trigger event, changed file count, and a per-plugin result breakdown (plugin name, status, finding count)
- **THEN** the display does not show empty sections when `executionMetadata` is absent

#### Scenario: Frontend ReviewRecord type includes executionMetadata
- **WHEN** the backend returns a review response containing `executionMetadata`
- **THEN** the TypeScript review store deserializes and stores the metadata without type errors
- **THEN** components can access `review.executionMetadata` without casting to `any`

### Requirement: Findings table surfaces plugin provenance and sources
The system SHALL display the originating plugin or built-in dimension name alongside each finding in the review findings table. When a finding includes `sources` metadata (plugin identity, dimension name), the table SHALL show that context in a dedicated column or expandable row.

#### Scenario: Findings table shows plugin provenance column
- **WHEN** a review has findings that include plugin or dimension source metadata
- **THEN** the findings table renders a provenance column or badge showing the source plugin/dimension name for each finding

#### Scenario: Findings without sources are still displayed without errors
- **WHEN** a review has findings that do not include source metadata
- **THEN** the findings table renders those findings without a provenance column or with an empty provenance indicator
- **THEN** no runtime error or blank column heading appears
