## ADDED Requirements

### Requirement: Dashboard review backlog and task review entry points share one workspace contract
The system SHALL provide a shared review workspace contract for dashboard review surfaces so `/reviews` and task-level review sections render review summaries, status badges, recommendation badges, finding counts, provenance snippets, and empty/loading states through reusable review UI building blocks backed by the same `ReviewDTO` shape.

#### Scenario: Backlog and task views show consistent review summary metadata
- **WHEN** an operator views the same review from `/reviews` and from a task-level review section
- **THEN** both surfaces present the same status, risk level, recommendation, summary, cost, and timestamp semantics
- **THEN** both surfaces derive those values from the same shared review presentation contract rather than page-local duplicate mappings

#### Scenario: Review workspace handles loading and empty states consistently
- **WHEN** a review surface is loading data or a task/project has no reviews
- **THEN** the shared review workspace presents a consistent loading or empty state pattern appropriate to the host page

### Requirement: Review detail and decision actions are reusable across backlog and task contexts
The system SHALL expose one reusable review detail surface that supports task-bound and standalone deep reviews, including findings, execution metadata, decision history, and pending-human actions. Host pages MAY choose different outer layout shells, but the detail content and action semantics MUST remain consistent.

#### Scenario: Operator opens a task-bound review from the backlog
- **WHEN** a review associated with a task is opened from `/reviews`
- **THEN** the shared detail surface shows findings, execution metadata, decision history, and available human actions using the same interaction model as the task-level detail view

#### Scenario: Operator opens a standalone deep review with no task binding
- **WHEN** a detached review created from only a PR URL is opened in the dashboard
- **THEN** the shared detail surface renders without requiring task metadata
- **THEN** the review still exposes status tracking, findings, execution metadata, and decision history

#### Scenario: Pending-human review actions are available through reusable controls
- **WHEN** a review is in `pending_human` state
- **THEN** any dashboard surface using the shared review workspace can render approve and request-changes controls with the same validation and comment behavior

### Requirement: Manual deep-review trigger is a workspace-level flow
The system SHALL provide a reusable manual deep-review trigger flow that can initiate task-bound reviews and standalone deep reviews from dashboard surfaces using one validated request pattern.

#### Scenario: Operator triggers deep review from a task context
- **WHEN** an operator opens the manual trigger flow from a task review section and submits a valid PR URL
- **THEN** the workspace sends a task-bound deep review request that includes the task identifier and PR reference

#### Scenario: Operator triggers standalone deep review from the backlog
- **WHEN** an operator opens the manual trigger flow from the review backlog and submits a valid PR URL without selecting a task
- **THEN** the workspace sends a standalone deep review request that contains the PR reference and optional project context only

#### Scenario: Invalid PR input is rejected consistently
- **WHEN** an operator submits an empty or malformed PR URL through any shared trigger entry point
- **THEN** the workspace blocks submission and shows the same validation semantics across backlog and task contexts
