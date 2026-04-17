## 1. Schema and model

- [ ] 1.1 Add migration to create table `project_audit_events` with columns per design (`id`, `project_id`, `occurred_at`, `actor_user_id`, `actor_project_role_at_time`, `action_id`, `resource_type`, `resource_id`, `payload_snapshot_json`, `system_initiated`, `configured_by_user_id`, `request_id`, `ip`, `user_agent`, `created_at`).
- [ ] 1.2 Add composite indexes: `(project_id, occurred_at DESC)`, `(project_id, action_id)`, `(project_id, actor_user_id)`.
- [ ] 1.3 Add `internal/model/audit_event.go` with the canonical struct and JSON tag mapping; include `ResourceType` enum constants covering `project|member|task|team_run|workflow|wiki|settings|automation|dashboard|auth`.
- [ ] 1.4 Add validation helper ensuring `action_id` values come exclusively from the RBAC `ActionID` enum introduced by `add-project-rbac`.

## 2. Repository and service

- [ ] 2.1 Implement `internal/repository/audit_event_repo.go` with `Insert(ctx, event) error` and `List(ctx, projectID, filters, cursor, limit)` returning events + next cursor.
- [ ] 2.2 Implement payload sanitization in `internal/service/audit_sanitizer.go`: recursive JSONB walk redacting any field whose name matches the denylist (`secret`, `token`, `api_key`, `password`, `access_token`, `refresh_token`, case-insensitive) and truncating total payload to 64 KB with `_truncated=true` marker.
- [ ] 2.3 Implement `internal/service/audit_service.go` with `RecordEvent(ctx, event)` that sanitizes payload, validates action_id, and enqueues the write via the internal sink. Provide `Query(ctx, projectID, filters, cursor, limit)` wrapping the repo.

## 3. Eventbus integration and sink

- [ ] 3.1 Define `AuditableEvent` payload type in `internal/eventbus` (or a dedicated audit package) with all fields needed to construct a `project_audit_events` row.
- [ ] 3.2 Implement `AuditSink` consumer: subscribes to `AuditableEvent`, calls `audit_service.RecordEvent`; on DB failure pushes to bounded in-memory retry queue (1000 slots, exponential backoff).
- [ ] 3.3 When retry queue exceeds 5 minutes of failed writes, spill events to `logs/audit_backlog.jsonl` (append-only) and emit a `audit_sink_degraded` metric + warning log.
- [ ] 3.4 Register sink lifecycle in `internal/server/routes.go` (start on server boot, graceful drain on shutdown).

## 4. Emission points

- [ ] 4.1 RBAC middleware allow path: after a gated write is permitted, emit a pre-action event capturing `action_id`, `actor_user_id`, `actor_project_role_at_time`, resource references parsed from route params, without payload.
- [ ] 4.2 RBAC middleware deny path: emit a `rbac_denied` event with `action_id` of the denied attempt and `resource_type=auth`; apply 60s dedup window on `(actor_user_id, action_id, resource_id)`.
- [ ] 4.3 Handler/service success emission: at the end of each write-capable handler's success path, emit a follow-up event carrying the full payload snapshot (`before`/`after` for updates, `created` for creates, `deleted_snapshot` for deletes) so the audit table contains the authoritative business-level record, not only the RBAC-level attempt record.
- [ ] 4.4 Automation/scheduler system-initiated paths: set `system_initiated=true` and populate `configured_by_user_id`; record the configuring user's current role snapshot at trigger time.

## 5. Query API

- [ ] 5.1 `GET /projects/:pid/audit-events` with query params `actionId`, `actorUserId`, `resourceType`, `resourceId`, `from`, `to`, `cursor`, `limit` (default 50, max 200). Require `projectRole ≥ admin` via RBAC matrix.
- [ ] 5.2 `GET /projects/:pid/audit-events/:eventId` returning the full event with sanitized payload snapshot. Same permission.
- [ ] 5.3 Register both routes with the `audit.read` ActionID (add to `add-project-rbac`'s matrix if not already present).

## 6. Frontend

- [ ] 6.1 Add `lib/stores/audit-store.ts` with `fetchEvents(projectId, filters, cursor)` and `fetchEventDetail(projectId, eventId)`; cache per project with invalidation on filter change.
- [ ] 6.2 Add `components/project/audit-log-panel.tsx` presenting the list view with filters (actor combobox, action dropdown sourced from shared ActionID enum, resource type, time range), infinite scroll / cursor pagination, and a detail drawer.
- [ ] 6.3 Mount the panel under `app/(dashboard)/settings/` as a new Audit Log section guarded by `use-project-role` `can('audit.read')`.
- [ ] 6.4 Localize strings (`messages/en/audit.json`, `messages/zh-CN/audit.json`) for panel labels, filter labels, and detail drawer fields.

## 7. Tests

- [ ] 7.1 Backend unit tests for `audit_service` sanitizer (denylist fields redacted, payload truncated past 64 KB).
- [ ] 7.2 Backend integration tests for emission points: member role update, project settings update, task dispatch, team run start, workflow execute each produce expected event rows.
- [ ] 7.3 Backend integration test for `rbac_denied` emission with dedup window behavior.
- [ ] 7.4 Backend integration test for system-initiated automation emitting correct `system_initiated=true` + `configured_by_user_id` fields.
- [ ] 7.5 Backend integration test for query API returning expected filters and 403 for editor/viewer callers.
- [ ] 7.6 Frontend tests for `audit-store` filter application and cursor pagination, and for panel rendering under admin vs editor (gated out) roles.
- [ ] 7.7 Run `pnpm exec tsc --noEmit`, `pnpm test`, `cd src-go && go test ./...`.

## 8. Docs

- [ ] 8.1 Update API reference with audit endpoints, query params, error codes.
- [ ] 8.2 Add an ops runbook section describing sink degradation signals, the `logs/audit_backlog.jsonl` spill file, and the manual replay procedure (procedure itself is scoped for follow-up).
- [ ] 8.3 Document the redaction denylist and payload size limits so feature authors know how to shape emission payloads.
