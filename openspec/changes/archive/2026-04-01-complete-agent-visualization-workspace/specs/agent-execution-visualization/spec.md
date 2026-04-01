## ADDED Requirements

### Requirement: Agent workspace provides a readable execution flow visualization
The system SHALL provide a dedicated flow-style visualization view inside `/agents` that renders the current execution scope as relationships between tasks, queued or blocked dispatch entries, active agents, and runtime targets.

#### Scenario: Render the current agent execution scope
- **WHEN** the agents workspace has visible agents or queued admissions for the current scope
- **THEN** the visualization renders nodes and edges that correlate task, dispatch/queue, agent, and runtime relationships from the currently loaded workspace data
- **THEN** the operator can inspect those relationships without leaving the existing `/agents` workspace shell

#### Scenario: Runtime targets are shared across multiple agents
- **WHEN** multiple visible agents use the same runtime, provider, and model tuple
- **THEN** the visualization groups them against one shared runtime target instead of duplicating identical runtime nodes for every agent
- **THEN** the operator can still distinguish the individual agents connected to that shared runtime target

### Requirement: Visualization stays synchronized with workspace scope and selection
The system SHALL keep the flow visualization synchronized with the current `/agents` scope, including URL-driven agent selection and member-scoped filtering.

#### Scenario: Operator selects an agent from the visualization
- **WHEN** the operator clicks an agent node in the visualization
- **THEN** the workspace updates selection through the same URL-driven agent detail flow used by the sidebar and existing deep links
- **THEN** the operator is shown the existing agent detail surface rather than a divergent graph-only detail mode

#### Scenario: Workspace is scoped to one member
- **WHEN** the operator opens `/agents?member=<member-id>` or otherwise scopes the workspace to one member
- **THEN** the visualization renders only the tasks, queue entries, and agents that remain visible in that scoped workspace
- **THEN** the visualization does not show unrelated agents outside the current scope

### Requirement: Visualization surfaces explicit loading, empty, and degraded states
The visualization SHALL preserve explicit operator-facing loading, empty, and degraded states instead of collapsing into a blank canvas when the workspace lacks trustworthy graph data.

#### Scenario: Visualization is loading before any graph data exists
- **WHEN** the agents workspace is still fetching data and the current scope has no loaded agents or queue entries yet
- **THEN** the visualization shows an explicit loading state inside the graph region
- **THEN** the surrounding workspace framing remains visible so the operator understands which surface is loading

#### Scenario: Current scope has no graphable data
- **WHEN** the current workspace scope contains no visible agents and no queue entries
- **THEN** the visualization shows an explicit empty or no-match state instead of an empty canvas
- **THEN** the empty state makes it clear whether the operator has no agent data yet or the current scope filtered everything out

#### Scenario: Bridge diagnostics are degraded
- **WHEN** bridge health is degraded while graphable data still exists
- **THEN** the visualization shows a visible degraded indicator explaining that runtime or pool data is not fully healthy
- **THEN** the operator can still inspect the last available graph relationships instead of losing the visualization entirely

### Requirement: Visualization nodes expose operator-readable execution summaries
The system SHALL expose concise operational summaries directly on graph nodes so operators can triage execution state from the visualization before drilling into detail.

#### Scenario: Operator inspects an active agent node
- **WHEN** the visualization renders an agent node for a visible agent
- **THEN** that node shows the agent's status emphasis and runtime identity
- **THEN** budget pressure is expressed through a warning or destructive cue when the agent is near or over its budget threshold

#### Scenario: Operator inspects a queued or blocked dispatch node
- **WHEN** the visualization renders a queued or blocked dispatch entry from the pool queue
- **THEN** the node shows the dispatch status and any blocking reason or outcome hint that is available
- **THEN** the operator can identify queue pressure or guardrail blockage without opening the table view first
