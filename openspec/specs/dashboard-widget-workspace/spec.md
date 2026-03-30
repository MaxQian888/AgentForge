# dashboard-widget-workspace Specification

## Purpose
Define the project-scoped dashboard workspace for dashboard CRUD, widget composition, layout persistence, and server-backed widget data.
## Requirements
### Requirement: Dashboard workspace supports CRUD operations on dashboards
The system SHALL allow project members to create, select, rename, delete, and configure dashboards within a project-scoped workspace. The workspace MUST expose the active dashboard explicitly and persist the selected dashboard identity.

#### Scenario: Create dashboard from empty state
- **WHEN** a user opens the dashboard workspace for a project with no dashboards
- **THEN** the workspace shows an explicit empty state with a "Create Dashboard" action
- **AND** creating a dashboard immediately makes it the active dashboard

#### Scenario: User switches between dashboards
- **WHEN** a project has multiple dashboards and the user selects a different one
- **THEN** the workspace renders that dashboard's widget grid
- **AND** the selected dashboard persists across page refreshes

#### Scenario: User deletes the active dashboard
- **WHEN** a user deletes the currently active dashboard
- **THEN** the workspace falls back to another dashboard or shows the empty state
- **AND** the deleted dashboard's widgets are no longer accessible

### Requirement: Widget catalog supports add, configure, refresh, and remove flows
The system SHALL provide a widget catalog with supported widget types: `throughput_chart`, `burndown`, `blocker_count`, `budget_consumption`, `agent_cost`, `review_backlog`, `task_aging`, `sla_compliance`. Each widget MUST support add, configure, refresh, and remove operations.

#### Scenario: User adds a widget from the catalog
- **WHEN** a user opens the widget catalog and selects a widget type to add
- **THEN** the widget appears in the dashboard grid with default configuration
- **AND** the workspace remains in the same dashboard context without page reload

#### Scenario: User configures a widget
- **WHEN** a user opens the configuration dialog for a throughput_chart widget and changes the time period
- **THEN** the widget refreshes with the new configuration
- **AND** the configuration change persists through the dashboard save contract

#### Scenario: User removes a widget
- **WHEN** a user removes a widget from the dashboard
- **THEN** the widget disappears from the grid after deletion succeeds
- **AND** no stale layout slots remain

#### Scenario: User refreshes a widget
- **WHEN** a user clicks the refresh action on a widget
- **THEN** the widget fetches fresh data from the server
- **AND** a loading indicator shows during the fetch

### Requirement: Widget grid supports drag-and-resize layout
The system SHALL support repositioning and resizing widgets within the dashboard grid. Layout changes MUST persist through the dashboard configuration contract.

#### Scenario: User repositions a widget
- **WHEN** a user drags a widget to a new position in the grid
- **THEN** the grid updates immediately
- **AND** the new layout is persisted with visible save feedback

#### Scenario: User resizes a widget
- **WHEN** a user resizes a widget by dragging its edge
- **THEN** the widget and surrounding widgets reflow accordingly
- **AND** the layout change persists

### Requirement: Widgets display explicit empty and error states
Each widget MUST explain empty or failed data states instead of rendering placeholder numbers.

#### Scenario: Widget data is empty
- **WHEN** a widget query returns no data points
- **THEN** the widget shows an explanatory empty state (e.g., "No completed tasks in this period")
- **AND** the rest of the dashboard remains usable

#### Scenario: Widget data fetch fails
- **WHEN** a widget data request fails
- **THEN** only that widget shows a retryable error state
- **AND** other widgets continue to display their data

### Requirement: Server-side widget data aggregation
The system SHALL compute widget data on the server via `GET /api/v1/projects/:pid/dashboard/widgets/:type` and return aggregated results.

#### Scenario: Throughput chart data
- **WHEN** a `throughput_chart` widget requests data
- **THEN** the server returns tasks-completed-per-period data points

#### Scenario: Blocker count data
- **WHEN** a `blocker_count` widget requests data
- **THEN** the server returns the current blocked task count with a trend indicator

#### Scenario: Budget consumption data
- **WHEN** a `budget_consumption` widget requests data
- **THEN** the server returns total budget spent vs. allocated with percentage

### Requirement: Dashboard workspace state feedback
The dashboard workspace SHALL distinguish page-level and widget-level loading, empty, saving, and failure states.

#### Scenario: Dashboard list is loading
- **WHEN** the workspace is fetching dashboards for the project
- **THEN** the page shows a loading skeleton for the dashboard selector
- **AND** the widget grid is not rendered until dashboards are loaded

#### Scenario: Layout save fails
- **WHEN** a dashboard layout save fails
- **THEN** the workspace keeps the user's draft layout and marks the save as failed
- **AND** the user can retry the save

#### Scenario: No project selected
- **WHEN** a user opens the dashboard without a project selection
- **THEN** the workspace explains that a project must be selected
- **AND** no widget controls are rendered
