## MODIFIED Requirements

### Requirement: Dashboard presents actionable operational insights
The system SHALL provide an authenticated dashboard summary experience that surfaces actionable project insights instead of only static headline counts. The summary MUST include task progress, active agents, pending reviews, weekly cost, and team/member capacity indicators for the selected dashboard scope.

The dashboard widget area SHALL lay out the four primary widgets (Activity Feed, Agent Fleet, Team Health, Budget) in a symmetric two-column grid where the left column contains Activity Feed and Team Health, and the right column contains Agent Fleet and Budget. The project scope filter and quick action shortcuts SHALL each render as independent full-width rows outside the two-column widget grid, with no empty grid columns.

#### Scenario: Summary data loads successfully
- **WHEN** an authenticated user opens the dashboard and summary data is available for the selected scope
- **THEN** the system displays populated insight sections for task progress, active agents, pending reviews, weekly cost, and team/member capacity
- **AND** each section uses labels and values that can be understood without opening another page

#### Scenario: Selected scope has no work yet
- **WHEN** the selected dashboard scope has no tasks, agents, reviews, or members
- **THEN** the system displays an explicit empty state instead of blank cards
- **AND** the empty state includes a next-step action such as creating a project, task, or team member

#### Scenario: Widget area has no empty columns
- **WHEN** the dashboard renders at desktop width (lg breakpoint and above)
- **THEN** the four widget cards occupy both columns of the widget grid with no empty grid cells
- **AND** neither column contains a large blank area taller than its content

#### Scenario: Project filter renders full width
- **WHEN** one or more projects exist and the project scope filter is visible
- **THEN** the project filter spans the full available width as its own layout row
- **AND** it does not share a grid row with widget cards
