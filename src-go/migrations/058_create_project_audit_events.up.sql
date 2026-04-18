-- 058_create_project_audit_events.up.sql
--
-- Project-scoped audit event store. Independent of `logs` (operational) and
-- `plugin_event_audit` (plugin lifecycle). One row per gated write attempt or
-- successful business mutation. See:
--   - openspec/specs/project-audit-log/spec.md
--   - openspec/changes/2026-04-17-add-project-audit-log/design.md
--
-- Index choice rationale:
--   * (project_id, occurred_at DESC) — primary list query path
--   * (project_id, action_id)        — filter by ActionID
--   * (project_id, actor_user_id)    — filter by actor
--
-- payload_snapshot_json is bounded by the application sanitizer (64 KB hard
-- cap with `_truncated:true` marker). Sensitive fields are redacted in-place.

CREATE TABLE IF NOT EXISTS project_audit_events (
    id                            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id                    UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    occurred_at                   TIMESTAMPTZ NOT NULL,
    actor_user_id                 UUID,
    actor_project_role_at_time    VARCHAR(16),
    action_id                     VARCHAR(64) NOT NULL,
    resource_type                 VARCHAR(32) NOT NULL,
    resource_id                   VARCHAR(64),
    payload_snapshot_json         JSONB NOT NULL DEFAULT '{}'::jsonb,
    system_initiated              BOOLEAN NOT NULL DEFAULT FALSE,
    configured_by_user_id         UUID,
    request_id                    VARCHAR(64),
    ip                            VARCHAR(64),
    user_agent                    TEXT,
    created_at                    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT project_audit_events_role_check
        CHECK (
            actor_project_role_at_time IS NULL OR
            actor_project_role_at_time IN ('owner', 'admin', 'editor', 'viewer')
        ),
    CONSTRAINT project_audit_events_resource_type_check
        CHECK (resource_type IN (
            'project', 'member', 'task', 'team_run', 'workflow',
            'wiki', 'settings', 'automation', 'dashboard', 'auth'
        ))
);

CREATE INDEX IF NOT EXISTS idx_project_audit_events_project_occurred
    ON project_audit_events(project_id, occurred_at DESC);

CREATE INDEX IF NOT EXISTS idx_project_audit_events_project_action
    ON project_audit_events(project_id, action_id);

CREATE INDEX IF NOT EXISTS idx_project_audit_events_project_actor
    ON project_audit_events(project_id, actor_user_id);
