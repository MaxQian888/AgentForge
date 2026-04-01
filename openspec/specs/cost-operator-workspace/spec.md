# cost-operator-workspace Specification

## Purpose
Define the standalone cost workspace behavior so it renders only authoritative cost query data with explicit context, loading, empty, and failure states.
## Requirements
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

### Requirement: Cost workspace explains external runtime cost coverage and attribution
The standalone cost workspace at `app/(dashboard)/cost` SHALL render explicit coverage and attribution context for external runtime spend using the authoritative project cost summary. The workspace MUST distinguish authoritative billed totals, officially-estimated totals, and unpriced runtime activity instead of presenting one undifferentiated spend figure with no provenance.

#### Scenario: Mixed authoritative and estimated external runtime spend is labeled explicitly
- **WHEN** the selected project's cost summary contains both authoritative external runtime totals and officially-estimated totals
- **THEN** the workspace renders a coverage summary that identifies both categories
- **THEN** runtime or model breakdown rows display badges or copy that make the attribution mode visible to the operator

#### Scenario: Unpriced runtime activity remains visible
- **WHEN** the selected project's cost summary reports one or more unpriced external runtime runs
- **THEN** the workspace shows an explicit warning or empty-state style explanation that some runtime activity is outside truthful USD coverage
- **THEN** the workspace SHALL NOT silently omit those runs or imply that the displayed total spend fully covers all recorded external runtime activity

### Requirement: Cost workspace renders external runtime breakdown from the authoritative summary
The standalone cost workspace SHALL render a runtime/provider/model breakdown section derived directly from the authoritative project cost summary so operators can compare Claude Code, Codex, and other external runtime families without reconstructing that grouping client-side.

#### Scenario: Runtime breakdown section renders project external runtime totals
- **WHEN** the selected project's summary includes runtime/provider/model breakdown entries
- **THEN** the workspace renders those entries in a dedicated breakdown section
- **THEN** each row reflects the same runtime/provider/model grouping and priced or unpriced counts returned by the API

#### Scenario: Runtime breakdown empty state stays explicit
- **WHEN** the selected project's summary contains no external runtime breakdown entries
- **THEN** the workspace renders an explicit empty state for that section
- **THEN** the rest of the workspace continues to use the authoritative summary without synthesizing a fake breakdown from unrelated stores

