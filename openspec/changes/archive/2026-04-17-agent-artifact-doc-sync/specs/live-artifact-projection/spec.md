## ADDED Requirements

### Requirement: LiveArtifactProjector interface and registry

The system SHALL define a `LiveArtifactProjector` Go interface with one registered implementation per supported `live_kind`. The interface SHALL expose `Kind()`, `RequiredRole()`, `Project(ctx, principal, projectID, targetRef, viewOpts) (ProjectionResult, error)`, and `Subscribe(targetRef) []EventTopic`. A central `Registry` SHALL bind `live_kind` values to projector instances at startup.

#### Scenario: Registry rejects unknown live kinds

- **WHEN** the projection endpoint is invoked with a block whose `live_kind` is not registered
- **THEN** the system returns an error for that block with `Status=degraded` and `Diagnostics` indicating the unknown kind, without failing the batch

#### Scenario: Four kinds registered at startup

- **WHEN** the server boots
- **THEN** projectors for `agent_run`, `cost_summary`, `review`, and `task_group` are registered

### Requirement: Projection REST endpoint returns BlockNote fragments

The system SHALL expose `POST /api/v1/projects/:pid/knowledge/assets/:id/live-artifacts/project` accepting a batch of `{block_id, live_kind, target_ref, view_opts}` entries and returning a map from `block_id` to `{status, projection?, projected_at, ttl_hint?, diagnostics?}`. The `projection` field SHALL be a BlockNote JSON fragment the client renders read-only.

#### Scenario: Batch projection succeeds for all blocks

- **WHEN** the client calls the endpoint with three resolvable blocks
- **THEN** the response contains three entries all with `status=ok` and a `projection` BlockNote fragment each, plus a `projected_at` ISO timestamp

#### Scenario: Mixed outcomes across the batch

- **WHEN** the client calls the endpoint with a batch containing a found entity, a deleted entity, and an entity the principal cannot read
- **THEN** the response returns `status=ok` for the first, `status=not_found` for the second, and `status=forbidden` for the third — the batch does not fail overall

#### Scenario: p95 latency budget for batches of 10

- **WHEN** the endpoint is called with a batch of up to 10 blocks
- **THEN** the p95 server-side response time is at or below 500 milliseconds

### Requirement: Agent-run projector

The `agent_run` projector SHALL render a BlockNote fragment containing the run's title, status, runtime/provider/model, duration, accumulated cost, step summary, and the last N log lines (default 10, configurable via `view_opts.show_log_lines`).

#### Scenario: Render running agent

- **WHEN** the projector is invoked against an in-progress `AgentRun` with `view_opts.show_log_lines=10`
- **THEN** the returned BlockNote fragment includes the run's title, a "running" status pill, the runtime/model, elapsed duration, current cost, and the last 10 log lines

#### Scenario: Render completed agent

- **WHEN** the projector is invoked against a completed `AgentRun`
- **THEN** the fragment shows the final status, total duration, final cost, outcome summary, and the last 10 log lines

#### Scenario: Required role enforced

- **WHEN** the projector is invoked by a principal who lacks `viewer` role on the project
- **THEN** the result is `Status=forbidden`

### Requirement: Cost-summary projector

The `cost_summary` projector SHALL render a BlockNote fragment containing a total, a top-N breakdown by the requested grouping dimension, and a compact delta indicator. Scope SHALL be project-only. Filter SHALL include `range_start`, `range_end`, and optional `runtime`, `provider`, `member_id`.

#### Scenario: Render cost summary with runtime grouping

- **WHEN** the projector is invoked with `view_opts.group_by=runtime` over a one-week range
- **THEN** the fragment shows total spend, top 5 runtime rows with their amounts, and a percentage delta vs the prior equal-length window

#### Scenario: Respect cost read gate

- **WHEN** the projector is invoked by a principal without cost-read permission
- **THEN** the result is `Status=forbidden` even if the principal has `viewer` on the project

### Requirement: Review projector

The `review` projector SHALL render a BlockNote fragment containing the review's title, state, findings count, reviewer, linked task title, and created/updated timestamps.

#### Scenario: Render in-progress review

- **WHEN** the projector is invoked against an in-progress `Review`
- **THEN** the fragment shows the review title, the current state pill, findings count, reviewer display name, and the linked task's title with a navigation link

#### Scenario: Render finalized review

- **WHEN** the projector is invoked against a finalized `Review`
- **THEN** the fragment adds the outcome and final findings summary to the rendered content

### Requirement: Task-group projector

The `task_group` projector SHALL render a BlockNote fragment containing a compact table of tasks matching the supplied filter, page-limited to 50 rows, with a "N more" footer when truncated. Filter SHALL accept a saved-view id or an inline `{status?, assignee?, tag?, sprint_id?, milestone_id?}` object.

#### Scenario: Render saved-view-based task group

- **WHEN** the projector is invoked with `target_ref.filter.saved_view_id` set
- **THEN** the fragment shows the saved view's name as a caption and the first 50 matching tasks with columns `title, status, assignee, due_date`

#### Scenario: Render inline-filter task group

- **WHEN** the projector is invoked with an inline filter `{status: "in_progress", tag: "release"}`
- **THEN** the fragment shows matching tasks filtered by those criteria

#### Scenario: Truncation footer when over 50 rows

- **WHEN** the filter matches more than 50 tasks
- **THEN** the fragment shows the first 50 and includes a "N more" footer pointing to `/tasks` with the filter applied

### Requirement: Projection schema evolution

Each projector SHALL version its `view_opts` schema via a `view_opts_schema_version` integer. When a projector bumps the schema version, older persisted blocks SHALL normalize to current defaults for any missing fields, and when normalization cannot proceed safely, the projector SHALL return `Status=degraded` with a diagnostic.

#### Scenario: Block with older view_opts schema renders with defaults

- **WHEN** a persisted block has `view_opts_schema_version=1` and the projector has moved to schema 2
- **THEN** the projector applies defaults for the new fields and returns `Status=ok`

#### Scenario: Block with unparseable view_opts degrades

- **WHEN** the projector cannot parse the stored `view_opts` against any known schema version
- **THEN** the projector returns `Status=degraded` with a diagnostic "unrecognized view_opts schema"
