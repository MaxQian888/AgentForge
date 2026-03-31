## ADDED Requirements

### Requirement: Dashboard displays interactive metric widgets

The system SHALL display metric widgets with sparkline charts showing 7-day trends for key metrics (tasks, agents, costs, reviews).

#### Scenario: User views dashboard with trend data
- **WHEN** user navigates to the dashboard page
- **THEN** system displays metric cards with sparkline charts showing last 7 days of data
- **AND** each sparkline visually indicates upward or downward trend

#### Scenario: Metric widget shows loading state
- **WHEN** dashboard data is being fetched
- **THEN** system displays skeleton loaders for each metric widget
- **AND** skeletons match the dimensions of the final widgets

### Requirement: Dashboard widgets support drill-down interaction

The system SHALL allow users to click on metric widgets to navigate to detailed views for that metric.

#### Scenario: User clicks on task progress widget
- **WHEN** user clicks on the task progress metric widget
- **THEN** system navigates to the task board page with filters pre-applied for in-progress tasks

#### Scenario: User clicks on cost widget
- **WHEN** user clicks on the weekly cost metric widget
- **THEN** system navigates to the cost dashboard page

### Requirement: Activity feed supports filtering

The system SHALL allow users to filter the activity feed by event type (task, review, agent, system) and time range.

#### Scenario: User filters activity by event type
- **WHEN** user selects "Agents" from the activity filter dropdown
- **THEN** system displays only agent-related activity events
- **AND** filter selection persists until changed or page refresh

#### Scenario: User filters activity by time range
- **WHEN** user selects "Last 24 hours" from the time range filter
- **THEN** system displays only events from the last 24 hours
- **AND** event count updates to reflect filtered total

### Requirement: Real-time status indicators show system health

The system SHALL display real-time status indicators for agents, IM bridges, and scheduler health with color-coded states.

#### Scenario: All systems operational
- **WHEN** all monitored systems are healthy
- **THEN** system displays green status indicators for each category
- **AND** tooltip shows "All systems operational"

#### Scenario: Agent degradation detected
- **WHEN** one or more agents are in error state
- **THEN** system displays yellow warning indicator for agents
- **AND** tooltip shows count of affected agents

#### Scenario: IM bridge connection lost
- **WHEN** IM bridge connection is lost
- **THEN** system displays red error indicator for IM bridge
- **AND** tooltip shows "Connection lost - retrying"

### Requirement: Dashboard supports project context filtering

The system SHALL allow users to filter dashboard data by selected project context.

#### Scenario: User selects specific project
- **WHEN** user selects a project from the project selector
- **THEN** dashboard metrics update to show data for the selected project only
- **AND** activity feed filters to project-specific events

#### Scenario: No project selected
- **WHEN** no project is selected in the project selector
- **THEN** dashboard displays aggregate data across all accessible projects
- **AND** activity feed shows events from all projects

### Requirement: Quick actions provide keyboard shortcuts

The system SHALL display keyboard shortcut hints for quick action buttons and support keyboard activation.

#### Scenario: User views quick actions
- **WHEN** dashboard quick actions are displayed
- **THEN** each action button shows its keyboard shortcut hint (e.g., "N" for new task)

#### Scenario: User presses keyboard shortcut
- **WHEN** user presses "N" key while dashboard is focused
- **THEN** system opens the new task creation dialog
