## ADDED Requirements

### Requirement: Cost workspace renders from authoritative cost query results
The standalone cost workspace at `app/(dashboard)/cost` SHALL render its headline cards and detail sections from the dedicated cost query contracts for the currently selected project. The workspace MUST NOT synthesize total spend, active run count, or section data from unrelated stores when the authoritative cost queries are unavailable, mismatched, or empty.

#### Scenario: Selected project renders authoritative cost sections
- **WHEN** the operator opens the cost workspace with a selected project and the cost query endpoints succeed
- **THEN** the headline cards, cost trend, sprint breakdown, velocity section, performance section, and task breakdown render from the returned cost query data for that project
- **THEN** the workspace does not compute its primary totals from `agent-store` or another unrelated client-side fallback source

#### Scenario: No selected project shows explicit context requirement
- **WHEN** the operator opens the cost workspace without a current project selection
- **THEN** the workspace explains that a project must be selected before cost statistics can be shown
- **THEN** the workspace does not render misleading zeroed analytics as though they were authoritative project data

### Requirement: Cost workspace surfaces explicit loading, empty, and failure states
The standalone cost workspace SHALL distinguish loading, empty, and failure states for its authoritative cost queries so operators can tell whether data is still loading, genuinely absent, or failed to load. Missing data MUST be explained explicitly instead of disappearing behind hidden sections or silent fallback values.

#### Scenario: Summary query fails while workspace shell stays usable
- **WHEN** the project cost summary query fails
- **THEN** the workspace shows an explicit error state for the affected summary surface
- **THEN** the workspace shell remains usable so the operator can retry or change project context
- **THEN** stale or unrelated totals are not shown as if the request had succeeded

#### Scenario: Section has no data for the selected project
- **WHEN** a cost workspace section receives an empty but valid dataset for the selected project
- **THEN** that section shows an explicit empty state message
- **THEN** the workspace does not silently hide the section in a way that suggests the data was never requested

### Requirement: Performance section labels its grouping truthfully
The standalone cost workspace SHALL label performance rows according to the actual aggregation bucket returned by the cost query API. If the backend groups historical performance by execution role or an equivalent durable bucket, the workspace MUST render copy and labels that are truthful to that grouping instead of implying an exact per-agent history that the persisted data does not support.

#### Scenario: Role-based performance is rendered with truthful labels
- **WHEN** the performance API returns entries grouped by execution role or another durable bucket
- **THEN** the workspace renders each row using the returned display label for that bucket
- **THEN** the surrounding labels and copy do not imply that the table is a perfect history of individual ephemeral runtime instances
