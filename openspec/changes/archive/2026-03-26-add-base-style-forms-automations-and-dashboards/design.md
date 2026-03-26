## Context

AgentForge's task workspace uses a fixed schema (title, status, assignee, priority, due_date, sprint, description, dependencies). There are no custom fields, no saved views, no form-based intake, no automation rules, and no project dashboards. Teams that need workflow-specific metadata, intake pipelines, or operational visibility must work around these limitations. This change adds the Base-style data layer.

## Goals / Non-Goals

**Goals:**
- Project-level custom field definitions with typed values stored on tasks.
- Saved personal and shared views with persisted filter/sort/group/column config.
- Form builder for intake (bug reports, feature requests, approvals) that auto-creates tasks.
- Event-condition-action automation rule engine with an execution audit log.
- Configurable project dashboards with widget library.
- Milestones as first-class entities with roadmap view.

**Non-Goals:**
- Cross-project custom fields / global field catalog — fields are project-scoped.
- Visual automation builder (drag-and-drop flow) — use a form-based rule editor.
- Real-time dashboard streaming — dashboards refresh on load and on manual refresh; WebSocket push is future.
- External data source connectors for dashboards (BI tool integration) — out of scope.
- Approval workflows with multi-step chains — use simple single-action rules for now.

## Decisions

### D1: Custom field storage → EAV with JSONB value column

`custom_field_def` table: `id`, `project_id`, `name`, `field_type` (text, number, select, multi_select, date, user, url, checkbox), `options JSONB` (for select/multi-select choices), `sort_order`, `required`.

`custom_field_value` table: `id`, `task_id`, `field_def_id`, `value JSONB`.

**Alternatives considered**:
- **Wide table with ALTER TABLE**: Each field adds a column. Fast reads but schema migrations on every field add/remove.
- **Separate typed tables**: One table per field type. Type-safe but complex joins.

**Rationale**: EAV with JSONB is the standard pattern for user-defined fields. JSONB stores any type uniformly. Querying is slightly slower than wide-table but avoids DDL churn. Add a GIN index on `value` for filtering.

### D2: Saved views → `saved_view` table with JSONB config

`saved_view`: `id`, `project_id`, `name`, `owner_id` (NULL = shared), `is_default`, `config JSONB`.

Config shape:
```json
{
  "layout": "board|list|table|timeline|calendar",
  "filters": [{"field": "...", "op": "eq|in|gt|lt|between", "value": "..."}],
  "sorts": [{"field": "...", "dir": "asc|desc"}],
  "groups": [{"field": "..."}],
  "columns": ["title", "status", "cf:risk", "cf:module"]
}
```

**Rationale**: JSONB config is flexible and avoids relational modeling of filter/sort/group combinations. Custom fields are referenced with `cf:` prefix.

### D3: Form intake → `form_definition` + `form_submission`

`form_definition`: `id`, `project_id`, `name`, `fields JSONB` (array of field mappings), `target_status`, `target_assignee`, `is_public`, `slug`.

Each form field maps to either a built-in task property or a custom field. On submission, a task is created with the mapped values.

**Rationale**: Forms are essentially field mapping configs. Public forms get a shareable URL (`/forms/:slug`); private forms require authentication.

### D4: Automation rule engine → Event-condition-action with synchronous evaluation

`automation_rule`: `id`, `project_id`, `name`, `enabled`, `event_type`, `conditions JSONB`, `actions JSONB`, `created_by`.

`automation_log`: `id`, `rule_id`, `task_id`, `event_type`, `triggered_at`, `status` (success, failed, skipped), `detail JSONB`.

Events: `task.status_changed`, `task.assignee_changed`, `task.due_date_approaching`, `task.field_changed`, `review.completed`, `budget.threshold_reached`.

Conditions: field comparisons, status checks, role checks.

Actions: `update_field`, `assign_user`, `send_notification`, `move_to_column`, `create_subtask`, `send_im_message`, `invoke_plugin`.

**Rationale**: Synchronous evaluation within the request that triggers the event keeps the model simple. If execution time grows, add an async queue later. The audit log tracks every evaluation.

### D5: Dashboard → Widget-based with server-side aggregation

`dashboard_config`: `id`, `project_id`, `name`, `layout JSONB` (grid positions), `created_by`.

`dashboard_widget`: `id`, `dashboard_id`, `widget_type`, `config JSONB`, `position JSONB` (x, y, w, h).

Widget types: `throughput_chart`, `burndown`, `blocker_count`, `budget_consumption`, `agent_cost`, `review_backlog`, `task_aging`, `sla_compliance`, `custom_query`.

Each widget type has a corresponding server-side aggregation endpoint: `GET /api/v1/projects/:pid/dashboard/widgets/:type?config=...`

**Rationale**: Server-side aggregation avoids shipping raw task data to the client. The frontend uses `recharts` for chart rendering. Each widget is independently configurable and refreshable.

### D6: Milestones → `milestone` table linked to sprints and tasks

`milestone`: `id`, `project_id`, `name`, `target_date`, `status` (planned, in_progress, completed, missed), `description`.

Tasks and sprints can reference a `milestone_id`. The roadmap view shows milestones as lanes with their associated sprints and tasks.

**Rationale**: Milestones are a lightweight entity above sprints. They provide the release/roadmap layer that sprints alone don't cover.

### D7: Charting library → Recharts

**Alternatives considered**:
- **@nivo/core**: Beautiful defaults but heavier bundle.
- **Chart.js + react-chartjs-2**: Lightweight but canvas-only; harder to style with Tailwind.
- **D3 direct**: Maximum flexibility but high development effort for standard charts.

**Rationale**: Recharts is React-native, composable, SVG-based (stylable), well-maintained, and has the smallest learning curve for the team. Tree-shakeable — only import used chart types.

## Risks / Trade-offs

- **[EAV query performance]** → Filtering/sorting by custom fields requires JSONB queries. Mitigate: GIN index on `custom_field_value.value`; for high-cardinality projects, consider materialized views.
- **[Automation cascades]** → An automation action can trigger another automation's event, causing infinite loops. Mitigate: add `triggered_by_automation = true` flag to events and skip automation evaluation on automated events (or cap recursion depth at 3).
- **[Dashboard aggregation cost]** → Complex queries on large projects. Mitigate: cache widget results with 60s TTL in Redis; add refresh indicator.
- **[Form abuse]** → Public forms can be spammed. Mitigate: rate limiting per IP; optional CAPTCHA for public forms.
- **[Migration complexity]** → 6+ new tables in one change. Mitigate: split migrations into numbered files; test rollback for each.
