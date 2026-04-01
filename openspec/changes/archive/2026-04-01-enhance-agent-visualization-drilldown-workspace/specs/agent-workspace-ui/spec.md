## ADDED Requirements

### Requirement: Agent workspace URL preserves visualization view and focus state
The `/agents` workspace SHALL preserve the active workspace view and visualization focus in URL-driven state so operators can reload or share the same visualization context without rebuilding it manually.

#### Scenario: Operator deep-links into the visualization view
- **WHEN** the operator opens `/agents?view=visualization`
- **THEN** the shared agent workspace shell loads with the visualization view active instead of defaulting back to monitor
- **THEN** the existing sidebar, toolbar, and workspace framing remain consistent with the rest of `/agents`

#### Scenario: Operator deep-links to a focused visualization node
- **WHEN** the operator opens `/agents?view=visualization&vizNode=<kind>:<id>`
- **THEN** the workspace restores the visualization view and resolves the matching task, dispatch, or runtime focus if it exists in the current graph snapshot
- **THEN** the corresponding drilldown surface is shown without requiring the operator to reselect that node manually

#### Scenario: URL references an unavailable visualization focus
- **WHEN** the operator opens `/agents?view=visualization&vizNode=<kind>:<id>` and that node is not present in the current workspace scope
- **THEN** the workspace falls back to the visualization canvas without crashing or redirecting away from `/agents`
- **THEN** the operator can continue using the visualization and choose another visible node

### Requirement: Agent detail and visualization focus remain navigation-consistent
The workspace SHALL maintain one consistent precedence model between agent detail and visualization focus so browser navigation and in-app selection do not produce divergent states.

#### Scenario: Operator selects an agent while a visualization node is focused
- **WHEN** the operator already has a visualization task, dispatch, or runtime node focused and then selects an agent from the graph or sidebar
- **THEN** the workspace transitions to the existing URL-driven agent detail experience
- **THEN** the agent detail takes precedence over the visualization drilldown instead of rendering two conflicting detail surfaces

#### Scenario: Operator exits agent detail back to the visualization workspace
- **WHEN** the operator leaves the agent detail while `view=visualization` is still the active workspace mode
- **THEN** the workspace returns to the visualization view instead of resetting to monitor
- **THEN** any still-valid visualization focus can be restored through the same URL-driven workspace state
