## MODIFIED Requirements

### Requirement: Layer 2 deep review runs parallel review dimensions through the bridge
The system SHALL execute Layer 2 deep review through the TypeScript bridge using one execution plan that includes the built-in review dimensions and any enabled `ReviewPlugin` instances that match the current review trigger. The aggregated result MUST preserve per-dimension or per-plugin provenance while still returning one unified review outcome.

#### Scenario: Built-in and custom review plugins complete successfully
- **WHEN** the bridge receives a valid deep-review execution request and one or more enabled review plugins match that request
- **THEN** it runs the built-in review dimensions together with the matching review plugins and returns one aggregated result containing findings, summary, recommendation, and provenance metadata

#### Scenario: One review plugin fails while others complete
- **WHEN** one enabled review plugin errors or times out while other built-in or custom review dimensions complete
- **THEN** the aggregated result marks that plugin failure explicitly and preserves the successful findings from the remaining dimensions

#### Scenario: Duplicate findings are consolidated across built-in and plugin sources
- **WHEN** multiple built-in dimensions or review plugins report materially identical findings for the same code region
- **THEN** the aggregation step emits a deduplicated finding set while preserving the original plugin or dimension metadata needed for auditability

### Requirement: Layer 2 review results are persisted and observable
The system SHALL persist each Layer 2 review run with structured findings and make the result available through backend APIs and real-time events. Persisted findings and execution metadata MUST preserve which built-in dimension or `ReviewPlugin` produced each contribution.

#### Scenario: Review result stores plugin provenance
- **WHEN** the backend receives an aggregated Layer 2 result that includes built-in and custom review plugin contributions
- **THEN** it stores the review with structured findings, plugin or dimension provenance, summary, recommendation, and cost information

#### Scenario: Review completion is broadcast with plugin-aware metadata
- **WHEN** a Layer 2 review transitions to a terminal state
- **THEN** the backend emits the corresponding review event and makes the persisted plugin-aware review metadata queryable through review APIs

