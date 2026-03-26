## Why

AgentForge's task workspace uses a fixed schema (status, assignee, priority, due date) with no way to add custom fields, create intake forms, define automation rules, or build project dashboards. Teams that need custom workflows — risk tracking, business-line tagging, milestone grouping, approval chains — must work around the rigid schema. Adding a Base-style custom field system, saved views, form-based intake, an automation rule engine, and project dashboards turns the task workspace into an adaptable project management platform without duplicating the existing board/list/timeline/calendar views.

## What Changes

- **Custom field system**: Define typed fields (text, number, select, multi-select, date, user, URL, checkbox) per project. Fields appear as columns in list/table views and as properties on task cards.
- **Saved views**: Personal and shared views with persisted filter, sort, group, and column configuration. Default view per project.
- **Form intake**: Project-level forms (bug report, feature request, onboarding request) that create tasks in a target backlog with pre-mapped field values.
- **Automation rule engine**: Event → condition → action rules. Events: status change, assignee change, due date approaching, field value change, review outcome, budget threshold. Actions: update field, assign, notify, move to column, create sub-task, send IM, invoke plugin.
- **Automation activity log**: Searchable log of rule triggers, successes, failures, and retries.
- **Project dashboard**: Configurable widgets — throughput chart, blocker count, burndown, budget consumption, agent cost breakdown, review backlog, task aging histogram, SLA compliance.
- **Milestone & roadmap lane**: Milestones as first-class entities above sprints; roadmap view with milestone lanes and release markers.

## Capabilities

### New Capabilities
- `custom-field-system`: Project-level custom field definitions (typed, ordered, required/optional), field value storage on tasks, field rendering in list/table/card views, and field-based filtering/sorting/grouping.
- `saved-views`: Personal and shared view configurations (filter, sort, group, columns, layout) with per-project default view and view-level sharing to specific members or roles.
- `form-intake`: Project-level form builder with field mapping to task properties and custom fields, public/private form links, and auto-backlog routing.
- `automation-rule-engine`: Event-condition-action rule definitions per project, rule evaluation on task/field/review/budget events, built-in action library (update, assign, notify, move, create, invoke), and rule execution audit trail.
- `project-dashboard`: Configurable dashboard with widget library (throughput, burndown, blockers, budget, agent cost, review backlog, aging, SLA), layout persistence, and sharing.
- `milestone-roadmap`: Milestones as first-class entities with release markers, roadmap lane view, and milestone-to-sprint/task aggregation.

### Modified Capabilities
- `task-multi-view-board`: Board/list/table/timeline views consume custom fields for columns, filters, sorts, and grouping.
- `task-progress-tracking`: Progress metrics feed into dashboard widgets and automation conditions.
- `dispatch-budget-governance`: Budget thresholds become automation event sources; budget data feeds dashboard widgets.
- `desktop-notification-delivery`: Automation actions can trigger desktop notifications.
- `im-bridge-progress-streaming`: Automation actions can send messages to IM channels.

## Impact

- **Backend (src-go)**: New models (`custom_field_def`, `custom_field_value`, `saved_view`, `form_definition`, `form_submission`, `automation_rule`, `automation_log`, `dashboard_config`, `dashboard_widget`, `milestone`); new service/handler/repository layers; new migrations.
- **Frontend**: New `components/fields/`, `components/views/`, `components/forms/`, `components/automations/`, `components/dashboard/` component families; new stores for each domain; sidebar additions for dashboards.
- **API**: ~15 new REST endpoints across custom fields, views, forms, automations, dashboards, and milestones.
- **Dependencies**: Charting library (e.g. `recharts` or `@nivo/core`) for dashboard widgets.
