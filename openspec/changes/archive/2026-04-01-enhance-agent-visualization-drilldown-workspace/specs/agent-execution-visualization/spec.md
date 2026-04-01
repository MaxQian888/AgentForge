## ADDED Requirements

### Requirement: Visualization exposes contextual drilldown for non-agent nodes
The system SHALL allow operators to focus task, dispatch, and runtime nodes inside the `/agents` visualization and inspect node-specific context without leaving the visualization workspace.

#### Scenario: Operator focuses a task node from the visualization
- **WHEN** the operator selects a task node in the visualization
- **THEN** the workspace shows a task-focused drilldown surface alongside the graph
- **THEN** the drilldown identifies the task, summarizes the related agent and queue counts, and exposes any available dispatch history for that task

#### Scenario: Operator focuses a queued or blocked dispatch node
- **WHEN** the operator selects a dispatch node that represents a queued or blocked admission
- **THEN** the workspace shows the admission outcome, blocking or queue reason, and stored runtime or budget context for that dispatch entry
- **THEN** the operator can inspect the dispatch attempt context without switching to the Dispatch tab first

#### Scenario: Operator focuses a runtime node
- **WHEN** the operator selects a runtime node in the visualization
- **THEN** the workspace shows the runtime's availability and diagnostics context derived from the current runtime catalog
- **THEN** the drilldown also summarizes which visible agents or dispatch entries are connected to that runtime node

### Requirement: Visualization drilldown surfaces explicit loading and empty states
The visualization SHALL preserve explicit operator-facing states while drilldown data is loading or unavailable, instead of collapsing the focused detail area into silence.

#### Scenario: Dispatch history is still loading for a focused task or dispatch node
- **WHEN** the operator focuses a task or dispatch node whose dispatch history has not been loaded yet
- **THEN** the drilldown surface shows an explicit loading state while the history request is in flight
- **THEN** the graph itself remains visible and interactive during that loading period

#### Scenario: Focused node has no additional contextual records
- **WHEN** the operator focuses a task, dispatch, or runtime node that has no extra history or diagnostics beyond the current graph snapshot
- **THEN** the drilldown surface shows an explicit empty state describing that no additional context is available
- **THEN** the operator can still clear focus or choose another node without losing the graph view
