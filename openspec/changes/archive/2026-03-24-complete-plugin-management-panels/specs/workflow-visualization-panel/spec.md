## ADDED Requirements

### Requirement: Project workflow configuration includes a readable visual representation
The system SHALL provide a workflow visualization panel for the selected project in addition to the editable transition and trigger controls. The visualization SHALL render the configured task statuses and allowed transitions as a readable flow representation and SHALL show the configured trigger rules in a form that can be correlated with the graph.

#### Scenario: Render a configured workflow graph
- **WHEN** the workflow page loads a project that has stored transitions and triggers
- **THEN** the page SHALL render a visual representation of the status graph using the returned transition configuration
- **THEN** the page SHALL also show the configured trigger rules alongside the graph so operators can inspect both structure and automation behavior together

#### Scenario: Render an empty workflow state
- **WHEN** the selected project has no saved workflow configuration
- **THEN** the page SHALL show an explicit empty visualization state rather than a broken or blank graph
- **THEN** the editing controls SHALL remain available so the operator can start defining transitions and triggers

### Requirement: The workflow panel shows recent automation activity and realtime health
The system SHALL surface recent workflow automation activity on the workflow page and SHALL consume the existing workflow trigger realtime channel when available. The page SHALL maintain a bounded recent-activity list for workflow trigger events and SHALL show a degraded realtime state when the workflow activity feed cannot be trusted to be live.

#### Scenario: Workflow trigger event appears in recent activity
- **WHEN** the client receives a `workflow.trigger_fired` event for the selected project
- **THEN** the workflow page SHALL append a recent-activity entry describing the trigger transition and action
- **THEN** the newest activity SHALL appear without requiring a manual page refresh

#### Scenario: Realtime connection is degraded
- **WHEN** the workflow page is open and the shared realtime connection is unavailable
- **THEN** the page SHALL indicate that workflow activity is degraded or not live
- **THEN** the operator SHALL still be able to review and edit the persisted workflow configuration

### Requirement: Visualization and editing stay synchronized with save state
The system SHALL keep the workflow visualization synchronized with the editable configuration state and SHALL surface dirty, saving, and error states clearly. Operators SHALL be able to understand whether they are viewing persisted workflow behavior or unsaved edits.

#### Scenario: Unsaved workflow changes update the draft visualization
- **WHEN** the operator changes transitions or trigger rules before saving
- **THEN** the visualization SHALL reflect the draft workflow state in the current page session
- **THEN** the page SHALL indicate that the current workflow contains unsaved changes

#### Scenario: Save failure preserves the draft
- **WHEN** a workflow save request fails
- **THEN** the page SHALL preserve the current draft transitions and trigger rules
- **THEN** the error state SHALL be visible without discarding the operator's unsaved edits
