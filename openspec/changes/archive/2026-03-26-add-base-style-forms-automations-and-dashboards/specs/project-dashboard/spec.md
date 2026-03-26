## ADDED Requirements

### Requirement: Dashboard configuration
The system SHALL allow project members to create and configure dashboards with a grid-based widget layout.

#### Scenario: Create a dashboard
- **WHEN** user creates a new dashboard with name "Sprint Overview"
- **THEN** the system creates an empty dashboard with a grid layout

#### Scenario: Add widget to dashboard
- **WHEN** user adds a "burndown" widget to the dashboard
- **THEN** the widget appears on the grid with default configuration and can be positioned/resized

#### Scenario: Configure widget
- **WHEN** user configures a throughput widget to show the last 30 days grouped by week
- **THEN** the widget updates to display the configured data range and grouping

### Requirement: Widget library
The system SHALL provide the following widget types: throughput_chart, burndown, blocker_count, budget_consumption, agent_cost, review_backlog, task_aging, sla_compliance.

#### Scenario: Throughput chart widget
- **WHEN** a throughput_chart widget is rendered
- **THEN** it displays a bar/line chart showing tasks completed per time period

#### Scenario: Blocker count widget
- **WHEN** a blocker_count widget is rendered
- **THEN** it displays the current number of blocked tasks with a trend indicator

#### Scenario: Budget consumption widget
- **WHEN** a budget_consumption widget is rendered
- **THEN** it displays total budget spent vs. allocated with a progress bar and trend line

#### Scenario: Task aging widget
- **WHEN** a task_aging widget is rendered
- **THEN** it displays a histogram of task ages (days since creation) grouped by status

### Requirement: Server-side widget data aggregation
The system SHALL compute widget data on the server and return aggregated results.

#### Scenario: Fetch widget data via API
- **WHEN** client sends `GET /api/v1/projects/:pid/dashboard/widgets/:type?config=...`
- **THEN** the server computes the aggregation and returns the data points

#### Scenario: Widget data caching
- **WHEN** widget data is requested within 60 seconds of a previous identical request
- **THEN** the server returns the cached result from Redis

### Requirement: Dashboard sharing
The system SHALL support sharing dashboards with project members.

#### Scenario: Share dashboard
- **WHEN** user shares a dashboard
- **THEN** all project members can view the dashboard (read-only unless they are the owner)

### Requirement: Dashboard API
The system SHALL expose REST endpoints for dashboard CRUD.

#### Scenario: List dashboards via API
- **WHEN** client sends `GET /api/v1/projects/:pid/dashboards`
- **THEN** the system returns all dashboards accessible to the current user

#### Scenario: Save dashboard layout via API
- **WHEN** client sends `PUT /api/v1/projects/:pid/dashboards/:did` with updated layout JSON
- **THEN** the system persists the layout and returns 200
