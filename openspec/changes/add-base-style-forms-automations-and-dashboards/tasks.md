## 1. Database Migrations & Models

- [x] 1.1 Create migration for `custom_field_defs` table (id, project_id, name, field_type, options JSONB, sort_order, required BOOL, created_at, updated_at, deleted_at)
- [x] 1.2 Create migration for `custom_field_values` table (id, task_id, field_def_id, value JSONB, created_at, updated_at) with GIN index on value
- [x] 1.3 Create migration for `saved_views` table (id, project_id, name, owner_id UUID NULL, is_default BOOL, shared_with JSONB, config JSONB, created_at, updated_at, deleted_at)
- [x] 1.4 Create migration for `form_definitions` table (id, project_id, name, slug UNIQUE, fields JSONB, target_status, target_assignee UUID NULL, is_public BOOL, created_at, updated_at, deleted_at)
- [x] 1.5 Create migration for `form_submissions` table (id, form_id, task_id, submitted_by TEXT NULL, submitted_at, ip_address TEXT)
- [x] 1.6 Create migration for `automation_rules` table (id, project_id, name, enabled BOOL, event_type, conditions JSONB, actions JSONB, created_by, created_at, updated_at, deleted_at)
- [x] 1.7 Create migration for `automation_logs` table (id, rule_id, task_id UUID NULL, event_type, triggered_at, status, detail JSONB)
- [x] 1.8 Create migration for `dashboard_configs` table (id, project_id, name, layout JSONB, created_by, created_at, updated_at, deleted_at)
- [x] 1.9 Create migration for `dashboard_widgets` table (id, dashboard_id, widget_type, config JSONB, position JSONB, created_at, updated_at)
- [x] 1.10 Create migration for `milestones` table (id, project_id, name, target_date, status, description TEXT, created_at, updated_at, deleted_at)
- [x] 1.11 Add milestone_id UUID NULL column to tasks table and sprints table
- [x] 1.12 Create Go model structs for all new entities in `src-go/internal/model/`

## 2. Backend Repository Layer

- [x] 2.1 Implement `custom_field_repo.go` with CRUD for field defs and field values, ListByProject, ListValuesByTask
- [x] 2.2 Implement `saved_view_repo.go` with CRUD, ListByProject (filtered by user access), SetDefault
- [x] 2.3 Implement `form_repo.go` with CRUD for definitions, CreateSubmission, GetBySlug
- [x] 2.4 Implement `automation_rule_repo.go` with CRUD, ListByProjectAndEvent
- [x] 2.5 Implement `automation_log_repo.go` with Create, ListByProject (with pagination and filters)
- [x] 2.6 Implement `dashboard_repo.go` with CRUD for configs and widgets
- [x] 2.7 Implement `milestone_repo.go` with CRUD, ListByProject, GetWithMetrics
- [x] 2.8 Write repository unit tests for all repos

## 3. Backend Service Layer — Custom Fields

- [x] 3.1 Implement `custom_field_service.go` with CreateField, UpdateField, DeleteField, ReorderFields, SetValue, ClearValue, GetValuesForTask, ValidateRequiredFields
- [x] 3.2 Integrate custom field filtering/sorting into task query builder (extend existing task list queries with JSONB field value joins)
- [x] 3.3 Write service unit tests

## 4. Backend Service Layer — Saved Views

- [x] 4.1 Implement `saved_view_service.go` with CreateView, UpdateView, DeleteView, ListAccessibleViews, SetDefaultView
- [x] 4.2 Write service unit tests

## 5. Backend Service Layer — Forms

- [x] 5.1 Implement `form_service.go` with CreateForm, UpdateForm, DeleteForm, SubmitForm (create task + map fields + record submission), GetFormBySlug
- [x] 5.2 Implement rate limiting for form submissions (per-IP, 10/min for public forms)
- [x] 5.3 Write service unit tests

## 6. Backend Service Layer — Automation Engine

- [ ] 6.1 Implement `automation_engine_service.go` with EvaluateRules: on event, find matching rules, evaluate conditions, execute actions sequentially, log results
- [ ] 6.2 Implement action executors: UpdateFieldAction, AssignUserAction, SendNotificationAction, MoveToColumnAction, CreateSubtaskAction, SendIMAction, InvokePluginAction
- [ ] 6.3 Implement cascade prevention: check triggered_by_automation flag and skip evaluation
- [ ] 6.4 Integrate automation evaluation into task_service (status change, assignee change, field change), review_service (review complete), and budget_governance_service (threshold reached)
- [ ] 6.5 Implement due-date-approaching scheduler: periodic check (every 15min) for tasks with due dates within configured threshold
- [ ] 6.6 Write service unit tests for engine, each action executor, and cascade prevention

## 7. Backend Service Layer — Dashboard

- [x] 7.1 Implement `dashboard_service.go` with CRUD for configs and widgets
- [x] 7.2 Implement widget data aggregation endpoints: ThroughputData, BurndownData, BlockerCount, BudgetConsumption, AgentCost, ReviewBacklog, TaskAging, SLACompliance
- [x] 7.3 Implement Redis caching for widget data (60s TTL)
- [x] 7.4 Write service unit tests

## 8. Backend Service Layer — Milestones

- [x] 8.1 Implement `milestone_service.go` with CRUD, AssignTaskToMilestone, AssignSprintToMilestone, GetCompletionMetrics
- [x] 8.2 Write service unit tests

## 9. Backend Handlers & Routes

- [x] 9.1 Implement `custom_field_handler.go` — field def CRUD + field value endpoints
- [x] 9.2 Implement `saved_view_handler.go` — view CRUD endpoints
- [x] 9.3 Implement `form_handler.go` — form CRUD + public submit endpoint at `/api/v1/forms/:slug/submit`
- [x] 9.4 Implement `automation_handler.go` — rule CRUD + log listing endpoints
- [x] 9.5 Implement `dashboard_handler.go` — dashboard CRUD + widget data endpoint
- [x] 9.6 Implement `milestone_handler.go` — milestone CRUD endpoints
- [x] 9.7 Register all new routes in `routes.go`
- [x] 9.8 Write handler integration tests

## 10. Frontend Dependencies & Store

- [ ] 10.1 Install `recharts` for chart rendering
- [ ] 10.2 Create `lib/stores/custom-field-store.ts` — field defs, field values per task
- [ ] 10.3 Create `lib/stores/saved-view-store.ts` — views, current view, view switching
- [ ] 10.4 Create `lib/stores/form-store.ts` — form defs, submissions
- [ ] 10.5 Create `lib/stores/automation-store.ts` — rules, logs
- [ ] 10.6 Create `lib/stores/dashboard-store.ts` — dashboard configs, widget data
- [ ] 10.7 Create `lib/stores/milestone-store.ts` — milestones, roadmap data
- [ ] 10.8 Write store unit tests

## 11. Frontend — Custom Fields UI

- [ ] 11.1 Create `components/fields/field-definition-editor.tsx` — admin UI for creating/editing field defs in project settings
- [ ] 11.2 Create `components/fields/field-value-cell.tsx` — inline editable cell for each field type (text, number, select, date, user, url, checkbox)
- [ ] 11.3 Create `components/fields/field-filter-control.tsx` — filter input per field type
- [ ] 11.4 Integrate custom field columns into task table/list views (`components/tasks/project-task-workspace.tsx`)
- [ ] 11.5 Integrate custom field properties into task detail panel

## 12. Frontend — Saved Views UI

- [ ] 12.1 Create `components/views/view-switcher.tsx` — dropdown in workspace header listing saved views
- [ ] 12.2 Create `components/views/save-view-dialog.tsx` — dialog for naming and configuring view visibility
- [ ] 12.3 Create `components/views/view-share-dialog.tsx` — dialog for sharing view with roles/members
- [ ] 12.4 Integrate view switcher into project-task-workspace header

## 13. Frontend — Forms UI

- [ ] 13.1 Create `components/forms/form-builder.tsx` — admin UI for creating/editing form definitions with field mapping
- [ ] 13.2 Create `components/forms/form-renderer.tsx` — renders a form for submission (used in both dashboard and public URL)
- [ ] 13.3 Create `app/(dashboard)/forms/[slug]/page.tsx` — public form submission page
- [ ] 13.4 Add forms management section to project settings

## 14. Frontend — Automation UI

- [ ] 14.1 Create `components/automations/rule-editor.tsx` — form for creating/editing rules: event picker, condition builder, action configurator
- [ ] 14.2 Create `components/automations/rule-list.tsx` — list of rules with enable/disable toggles
- [ ] 14.3 Create `components/automations/automation-log-viewer.tsx` — searchable/filterable activity log
- [ ] 14.4 Add automations tab to project settings

## 15. Frontend — Dashboard UI

- [ ] 15.1 Create `app/(dashboard)/project/dashboard/page.tsx` — dashboard route
- [ ] 15.2 Create `components/dashboard/dashboard-grid.tsx` — grid layout with drag-and-drop widget positioning
- [ ] 15.3 Create `components/dashboard/widget-wrapper.tsx` — common widget frame with title, config button, refresh
- [ ] 15.4 Create widget components: ThroughputChart, BurndownChart, BlockerCount, BudgetConsumption, AgentCost, ReviewBacklog, TaskAging, SLACompliance
- [ ] 15.5 Create `components/dashboard/add-widget-dialog.tsx` — widget type selector with preview
- [ ] 15.6 Add "Dashboard" entry to sidebar navigation

## 16. Frontend — Milestone & Roadmap UI

- [ ] 16.1 Create `components/milestones/milestone-editor.tsx` — create/edit milestone dialog
- [ ] 16.2 Create `components/milestones/roadmap-view.tsx` — timeline view with milestone lanes, sprint blocks, and release markers
- [ ] 16.3 Add milestone selector to task detail panel and sprint settings
- [ ] 16.4 Add "Roadmap" view option to project task workspace view switcher

## 17. Integration — Automation Event Sources

- [ ] 17.1 Wire task status/assignee/field changes to emit automation events
- [ ] 17.2 Wire review completion to emit review.completed automation event
- [ ] 17.3 Wire budget threshold crossing to emit budget.threshold_reached automation event
- [ ] 17.4 Wire automation send_notification action to desktop notification delivery
- [ ] 17.5 Wire automation send_im_message action to IM bridge
- [ ] 17.6 Write integration tests for end-to-end automation flows
