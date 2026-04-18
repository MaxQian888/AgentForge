# Project Audit Log

Project-scoped audit events recording every gated write attempt against the
RBAC matrix. Independent of `logs` (operational/execution) and
`plugin_event_audit` (plugin lifecycle).

See also:
- [`docs/api/rbac.md`](./rbac.md) — the matrix that drives audit ActionIDs.
- `openspec/specs/project-audit-log/spec.md` — normative spec.

## Storage

Migration `058_create_project_audit_events.up.sql` adds the
`project_audit_events` table:

| Column                       | Notes                                                          |
|------------------------------|----------------------------------------------------------------|
| `id`                         | UUID                                                           |
| `project_id`                 | FK to projects, ON DELETE CASCADE                              |
| `occurred_at`                | TIMESTAMPTZ                                                    |
| `actor_user_id`              | nullable when `system_initiated=true`                          |
| `actor_project_role_at_time` | snapshot at event time (owner/admin/editor/viewer)             |
| `action_id`                  | exact value from the RBAC ActionID enum                        |
| `resource_type`              | `project|member|task|team_run|workflow|wiki|settings|automation|dashboard|auth` |
| `resource_id`                | nullable                                                       |
| `payload_snapshot_json`      | sanitized + size-bounded JSON                                  |
| `system_initiated`           | true for scheduler/automation auto-trigger paths               |
| `configured_by_user_id`      | the human who last authorized the automation (when system_initiated=true) |
| `request_id` / `ip` / `user_agent` | request correlation context                              |

Indexes cover the three primary query shapes:
- `(project_id, occurred_at DESC)` — list view
- `(project_id, action_id)` — filter by action
- `(project_id, actor_user_id)` — filter by actor

Retention is unbounded during internal testing; no TTL or partitioning yet.

## Query API

Both endpoints require `audit.read` (admin+).

### List

```
GET /api/v1/projects/{pid}/audit-events?<filters>
Authorization: Bearer <accessToken>
```

| Param          | Notes                                                     |
|----------------|-----------------------------------------------------------|
| `actionId`     | exact match against the canonical ActionID enum           |
| `actorUserId`  | UUID; 400 if malformed                                    |
| `resourceType` | one of `project|member|task|...`                          |
| `resourceId`   | exact match                                               |
| `from` / `to`  | RFC3339 timestamps                                        |
| `cursor`       | opaque, returned by a previous page                       |
| `limit`        | 1..200, default 50                                        |

```json
200 OK
{
  "events": [
    {
      "id": "uuid",
      "projectId": "uuid",
      "occurredAt": "2026-04-17T03:14:15Z",
      "actorUserId": "uuid",
      "actorProjectRoleAtTime": "admin",
      "actionId": "task.dispatch",
      "resourceType": "auth",
      "resourceId": "task-uuid",
      "payloadSnapshotJson": "{\"outcome\":\"allowed\"}",
      "systemInitiated": false,
      "requestId": "...",
      "ip": "1.2.3.4",
      "userAgent": "..."
    }
  ],
  "nextCursor": "base64-cursor-or-empty"
}

403 not_a_project_member        — caller is not a member of the project
403 insufficient_project_role   — caller below admin
```

### Detail

```
GET /api/v1/projects/{pid}/audit-events/{eventId}
```

Returns the same DTO shape as the list element. `404` if the event does
not exist or is not in the named project.

## Frontend

- Store: [`lib/stores/audit-store.ts`](../../lib/stores/audit-store.ts)
- Panel: [`components/project/audit-log-panel.tsx`](../../components/project/audit-log-panel.tsx)
- Mounted under Settings → Audit Log; gated by
  `useProjectRole().can('audit.read')`.
- i18n keys live in `messages/en/audit.json` and `messages/zh-CN/audit.json`.

## Emission model

Audit events are emitted at multiple layers; today only the RBAC layer is
fully wired (the handler/service-layer enrichment is an active follow-up).

### RBAC layer (today)

`internal/middleware/rbac.go` — every `Require(action)` pass produces an
event, both on allow and on deny:

- **Allow** — `payload = {"outcome":"allowed"}`, `actor_project_role_at_time`
  populated.
- **Deny** — `resource_type=auth`, `payload = {"outcome":"denied"}`. The sink
  applies a 60s dedup window on `(actor_user_id, action_id, resource_id)`
  to suppress abuse-driven floods.

### Handler/service layer (in progress)

Handlers can emit a follow-up event after the business write succeeds with
a richer payload — `before/after` for updates, `created` for inserts,
`deleted_snapshot` for deletes. This lands together with the agent action
service signature refactor (`Caller{UserID, SystemInitiated, ConfiguredByUserID}`).

### System-initiated paths (in progress)

Scheduler/automation-triggered paths set `system_initiated=true` and
populate `configured_by_user_id`. Activation rechecks that the configured
user's current `projectRole` is still `≥ admin`; otherwise the run is
rejected and an `rbac_snapshot_invalid` warning event is emitted.

## Sink behavior

The audit sink (`internal/service/audit_sink.go`) decouples persistence
from request paths. The originating business operation never blocks on
audit availability.

| Knob              | Default                             |
|-------------------|-------------------------------------|
| `QueueCapacity`   | 1000 events                         |
| `MaxAttempts`     | 8 retries                           |
| `InitialBackoff`  | 200 ms                              |
| `MaxBackoff`      | 30 s                                |
| `DegradationWindow` | 5 min sustained failure           |
| `SpillFilePath`   | `logs/audit_backlog.jsonl`          |
| `DedupWindow`     | 60 s for `rbac_denied` events       |

### Degradation signals

- Per-attempt failures: `audit_sink: insert failed; will retry` (warn)
- Past `DegradationWindow`: events spill to the JSONL file with an
  envelope `{spilledAt, reason, event: AuditEventDTO}`. The log line
  `audit_sink: event spilled to disk; operator replay required` includes
  `audit_sink_degraded=true` for alerting.
- Queue full: events spill immediately and log
  `audit_sink: queue full, spilling event to disk`.

### Replay procedure (manual)

Until automated replay lands, operators can replay spill files with a
short ad-hoc script that reads JSONL and inserts directly into
`project_audit_events`. The spill envelope preserves every column needed
to reconstruct the row. Truncate the file after a successful replay to
prevent double-insertion.

## Payload sanitization

`internal/service/audit_sanitizer.go`:

- **Redaction denylist** (case-insensitive substring match on JSON keys):
  `secret`, `token`, `api_key`, `apikey`, `password`, `access_token`,
  `refresh_token`, `private_key`, `privatekey`, `client_secret`,
  `clientsecret`, `authorization`. Any matched value is replaced with
  `"[REDACTED]"`.
- **Recursive walk** — works through nested objects and arrays so a key
  named `nested.oauth_token` or `items[0].refreshToken` redacts.
- **Size cap** — 64 KiB after redaction. Past the cap the payload becomes
  `{"_truncated": true, "_originalSizeBytes": N, "_summary": {"topLevelKeys": [...]}}`.
- **Non-JSON input** — wrapped as `{"raw": "..."}` so the persisted
  payload is always valid JSON.

When emitting from a new code path, prefer maps with already-redacted
sensitive fields; the sanitizer is a defense-in-depth net, not the
primary deduplication layer for secrets.

## ActionID validation

Every `RecordEvent` consults the canonical RBAC matrix via the
`ActionIDValidator` callback wired at server startup
([`internal/server/routes.go`](../../src-go/internal/server/routes.go)).
Events with unknown ActionIDs are rejected at the service boundary
(`ErrUnknownAuditActionID`) so the audit table never accumulates rows
under undeclared actions.

## Tests

- `internal/service/audit_sanitizer_test.go` — denylist, truncation,
  invalid input, case insensitivity.
- `internal/service/audit_service_test.go` — unknown ActionID/resource
  rejection, sanitization on enqueue, ID assignment.
- `internal/service/audit_sink_test.go` — rbac_denied dedup window,
  non-auth events not deduped, queue-full disk spill.
