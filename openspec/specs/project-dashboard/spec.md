# project-dashboard Specification

## Purpose
Define configurable project dashboards, shared widget layouts, and server-side metric aggregation for operational visibility.

## Requirements
### Requirement: Dashboard configuration
The system SHALL allow project members to create, select, rename, delete, and configure dashboards within a project-scoped workspace that preserves the current project context. The workspace MUST expose the active dashboard explicitly instead of always defaulting to the first record, and dashboard layout changes MUST persist through the existing dashboard configuration contract.

#### Scenario: Create dashboard from an empty project workspace
- **WHEN** a user opens `/project/dashboard` for a project that has no dashboards yet
- **THEN** the workspace shows an explicit empty state with a create action
- **AND** creating a dashboard immediately makes the new dashboard active in the current workspace

#### Scenario: User switches to another dashboard
- **WHEN** a project has multiple dashboards and the user selects a different dashboard in the workspace
- **THEN** the workspace renders that dashboard without losing the current project scope
- **AND** refreshing or reopening the same workspace preserves the selected dashboard identity through route state or an equivalent persisted selection mechanism

#### Scenario: User updates dashboard metadata or removes the active dashboard
- **WHEN** a user renames the active dashboard or deletes it from the workspace
- **THEN** the system persists that mutation through the existing dashboard CRUD contract
- **AND** the workspace reflects the new dashboard name or falls back to another accessible dashboard or empty state without rendering a dead grid

#### Scenario: User repositions widgets in the active dashboard
- **WHEN** a user changes widget placement or size inside the active dashboard
- **THEN** the workspace updates the visible layout immediately
- **AND** the resulting layout is persisted through the existing dashboard configuration and widget save flows with visible save feedback

### Requirement: Widget library
The system SHALL provide the following widget types: throughput_chart, burndown, blocker_count, budget_consumption, agent_cost, review_backlog, task_aging, sla_compliance. Each widget type MUST be operable inside the project dashboard workspace through add, refresh, configure, and remove flows that use the existing widget contracts, and each widget card MUST explain empty or failed data states instead of silently rendering placeholder numbers.

#### Scenario: User adds a widget to the active dashboard
- **WHEN** a user opens the widget catalog and adds a supported widget type to the active dashboard
- **THEN** the widget appears in the current dashboard with its default configuration and placement metadata
- **AND** the workspace keeps the user in the same dashboard context instead of forcing a page reload

#### Scenario: User configures a widget
- **WHEN** a user updates the configuration of an existing throughput or burndown widget
- **THEN** the workspace persists the widget changes through the existing save widget contract
- **AND** the widget refreshes using the new configuration instead of requiring manual JSON editing

#### Scenario: Widget data is empty or temporarily unavailable
- **WHEN** a widget request returns no meaningful data or the data fetch fails
- **THEN** only that widget card shows an explicit empty or retryable error state
- **AND** the rest of the dashboard workspace remains usable

#### Scenario: User removes a widget from the active dashboard
- **WHEN** a user removes a widget from the active dashboard
- **THEN** the widget disappears from the grid after the delete succeeds
- **AND** the workspace does not leave behind stale controls or phantom layout slots

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

### Requirement: Dashboard workspace state feedback
The project dashboard workspace SHALL distinguish page-level and widget-level loading, empty, saving, and failure states so operators can understand whether the workspace, the selected dashboard, or a single widget needs attention. State feedback MUST not require users to infer failures from unchanged cards or missing controls.

#### Scenario: Dashboard list is still loading
- **WHEN** the workspace is fetching dashboards for the selected project
- **THEN** the page shows a loading state for the dashboard selector or shell
- **AND** it does not render the active dashboard grid until the workspace knows whether a dashboard exists

#### Scenario: Layout save fails after a local edit
- **WHEN** the workspace cannot persist a dashboard or widget layout mutation
- **THEN** the page keeps the user-visible draft and marks the save as failed
- **AND** the user can retry or otherwise resolve the failed save without reconstructing the dashboard manually

#### Scenario: Workspace has no selected project context
- **WHEN** a user opens `/project/dashboard` without a current project selection
- **THEN** the page explains that a project must be selected before a dashboard can be shown
- **AND** it does not render misleading widget controls for an unknown project

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
