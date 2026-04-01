# agent-workspace-ui Specification

## Purpose
Define the operator-facing agent workspace UI requirements for AgentForge, covering shared workspace framing, explicit degraded/loading/empty states, consistent status and budget affordances, and console-readable output streaming on the `/agents` experience.
## Requirements
### Requirement: Agent workspace exposes one consistent operator workspace shell
The system SHALL present the operator-facing agent experience as one consistent workspace shell that supports monitor, visualization, and dispatch views, URL-driven agent selection, and shared sidebar/detail framing instead of scattering those affordances across unrelated page-level layouts.

#### Scenario: Operator switches between monitor, visualization, and dispatch views
- **WHEN** the operator is on `/agents` and switches between the monitor, visualization, and dispatch views
- **THEN** the workspace presents those views through one first-class navigation control within the same shell
- **THEN** the sidebar, toolbar, and page framing remain consistent while only the active view content changes

#### Scenario: Operator deep-links to a specific agent
- **WHEN** the operator opens `/agent?id=<agent-id>` or `/agents?agent=<agent-id>`
- **THEN** the experience resolves into the same agent workspace shell
- **THEN** the selected agent detail is shown without creating a separate, divergent page layout for agent detail

#### Scenario: Operator selects an agent from the visualization
- **WHEN** the operator chooses an agent from the visualization view
- **THEN** the workspace updates the same URL-driven selection state used by the rest of the agent workspace
- **THEN** the operator reaches the existing agent detail experience with the same controls and output surfaces

### Requirement: Agent workspace surfaces explicit degraded, loading, and empty states
The agent workspace SHALL render explicit operator-facing degraded, loading, and empty states for bridge health, roster loading, filtered results, and no-data conditions instead of relying on silent gaps or ad hoc text blocks.

#### Scenario: Bridge diagnostics are degraded
- **WHEN** bridge health is available and marked degraded on the agents workspace
- **THEN** the workspace shows a visible degraded state explaining that runtime or pool diagnostics are not fully healthy
- **THEN** runtime availability and related summaries remain understandable without hiding the degraded condition

#### Scenario: Workspace is loading with no roster data yet
- **WHEN** the agents workspace is fetching data and there are no agents loaded yet
- **THEN** the workspace shows an explicit loading state rather than a blank content region
- **THEN** the loading state preserves the surrounding workspace framing so operators know which surface is loading

#### Scenario: Search or roster yields no visible agents
- **WHEN** the operator has no agents in scope or a search/filter leaves no matching agents
- **THEN** the sidebar or overview shows an explicit empty state
- **THEN** the empty state distinguishes "no agents yet" from "no match for the current filter"

### Requirement: Agent summary surfaces share consistent status and budget affordances
The agent roster item, summary card, and detail statistics surfaces SHALL expose status, runtime identity, and budget consumption through consistent visual affordances so operators can compare agent state without relearning each panel.

#### Scenario: Operator compares multiple agent summaries
- **WHEN** the operator scans the sidebar, summary cards, or detail stats for active agents
- **THEN** status is shown through a consistent badge or equivalent state affordance
- **THEN** runtime/provider/model identity remains visible in a predictable summary position
- **THEN** budget consumption is represented with a consistent progress affordance across those surfaces

#### Scenario: Agent approaches or exceeds budget limits
- **WHEN** an agent's budget consumption crosses the workspace warning threshold
- **THEN** the progress affordance changes to a warning or destructive emphasis
- **THEN** the operator can identify elevated budget pressure without opening a different panel

### Requirement: Agent output stream preserves console readability within the workspace
The agent detail output stream SHALL preserve console-style readability, chronological output, and automatic tail-follow behavior while still presenting a distinct waiting or empty state within the workspace's shared UI framing.

#### Scenario: Agent has not produced output yet
- **WHEN** the operator opens an agent detail view before any runtime output has arrived
- **THEN** the output area shows a distinct waiting state instead of a visually empty console
- **THEN** the output area remains clearly associated with the rest of the agent detail workspace

#### Scenario: New output arrives while viewing the stream
- **WHEN** new runtime output lines are appended while the operator is viewing the stream
- **THEN** the stream keeps chronological ordering and monospace readability
- **THEN** the view follows newly appended output so the latest line remains visible

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

