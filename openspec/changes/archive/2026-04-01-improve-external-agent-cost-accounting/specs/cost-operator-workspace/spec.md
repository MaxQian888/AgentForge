## ADDED Requirements

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
