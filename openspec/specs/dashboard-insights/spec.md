# dashboard-insights Specification

## Purpose
Define the authenticated dashboard summary contract for AgentForge so users can see actionable task progress, agent activity, review pressure, cost signals, team capacity, recent activity, and drill-down paths without relying on static headline cards alone.
## Requirements
### Requirement: Dashboard presents actionable operational insights
The system SHALL provide an authenticated dashboard summary experience that surfaces actionable project insights instead of only static headline counts. The summary MUST include task progress, active agents, pending reviews, weekly cost, and team/member capacity indicators for the selected dashboard scope.

#### Scenario: Summary data loads successfully
- **WHEN** an authenticated user opens the dashboard and summary data is available for the selected scope
- **THEN** the system displays populated insight sections for task progress, active agents, pending reviews, weekly cost, and team/member capacity
- **AND** each section uses labels and values that can be understood without opening another page

#### Scenario: Selected scope has no work yet
- **WHEN** the selected dashboard scope has no tasks, agents, reviews, or members
- **THEN** the system displays an explicit empty state instead of blank cards
- **AND** the empty state includes a next-step action such as creating a project, task, or team member

### Requirement: Dashboard surfaces recent activity and risk signals
The system SHALL present a recent activity feed and risk-oriented signals so that a lead can quickly spot stalled execution, pending review pressure, or missing team coverage from the dashboard alone.

#### Scenario: Recent activity exists
- **WHEN** there are recent task, agent, review, or notification events for the selected dashboard scope
- **THEN** the dashboard displays them in time-descending order
- **AND** each activity item includes enough context to identify the related task, agent, review, or member

#### Scenario: Risks are detected
- **WHEN** the summary data indicates stalled tasks, overloaded review queues, budget pressure, or unassigned work
- **THEN** the dashboard highlights those items in a dedicated risk-oriented section
- **AND** the highlighted items provide a clear drill-down destination for follow-up

### Requirement: Dashboard supports consistent drill-down and resilient section states
The system SHALL let users move from summary insights into the underlying project, agent, review, or team surfaces, and it SHALL handle partial failures without collapsing the entire dashboard.

#### Scenario: User drills down from an insight card
- **WHEN** a user selects a dashboard card or activity item
- **THEN** the system navigates to the related detailed surface with relevant scope preserved
- **AND** the destination view is filtered or contextualized enough for the user to continue the same investigation

#### Scenario: One summary section fails to load
- **WHEN** one insight section cannot be loaded but other dashboard data remains available
- **THEN** the dashboard keeps the healthy sections visible
- **AND** the failed section renders an inline error state with a retry affordance instead of blanking the whole page
