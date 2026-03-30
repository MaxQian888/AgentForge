# agent-workspace-ui Specification

## Purpose
Define the operator-facing agent workspace UI requirements for AgentForge, covering shared workspace framing, explicit degraded/loading/empty states, consistent status and budget affordances, and console-readable output streaming on the `/agents` experience.

## Requirements
### Requirement: Agent workspace exposes one consistent operator workspace shell
The system SHALL present the operator-facing agent experience as one consistent workspace shell that supports monitor and dispatch views, URL-driven agent selection, and shared sidebar/detail framing instead of scattering those affordances across unrelated page-level layouts.

#### Scenario: Operator switches between monitor and dispatch views
- **WHEN** the operator is on `/agents` and switches between monitor and dispatch views
- **THEN** the workspace presents those views through one first-class navigation control within the same shell
- **THEN** the sidebar, toolbar, and page framing remain consistent while only the active view content changes

#### Scenario: Operator deep-links to a specific agent
- **WHEN** the operator opens `/agent?id=<agent-id>` or `/agents?agent=<agent-id>`
- **THEN** the experience resolves into the same agent workspace shell
- **THEN** the selected agent detail is shown without creating a separate, divergent page layout for agent detail

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
