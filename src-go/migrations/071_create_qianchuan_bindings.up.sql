-- Project-scoped Qianchuan (巨量千川) advertiser bindings.
-- Tokens are NOT stored here; the *_secret_ref columns reference
-- secrets.name rows owned by Plan 1B's secrets store.
--
-- Spec: docs/superpowers/specs/2026-04-20-ecommerce-streaming-employee-design.md §6.1
--   (Spec drift: 3A omits policy / strategy_id / trigger_id / tick_interval_sec;
--    those columns are added by Plan 3C / 3D ALTER TABLEs.)
CREATE TABLE IF NOT EXISTS qianchuan_bindings (
    id                       UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id               UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    advertiser_id            VARCHAR(64) NOT NULL,
    aweme_id                 VARCHAR(64) NOT NULL DEFAULT '',
    display_name             VARCHAR(128),
    status                   VARCHAR(16) NOT NULL DEFAULT 'active'
                             CHECK (status IN ('active','auth_expired','paused')),
    acting_employee_id       UUID REFERENCES employees(id) ON DELETE SET NULL,
    access_token_secret_ref  VARCHAR(128) NOT NULL,
    refresh_token_secret_ref VARCHAR(128) NOT NULL,
    token_expires_at         TIMESTAMPTZ,
    last_synced_at           TIMESTAMPTZ,
    created_by               UUID NOT NULL,
    created_at               TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at               TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (project_id, advertiser_id, aweme_id)
);
CREATE INDEX IF NOT EXISTS qianchuan_bindings_project_idx
    ON qianchuan_bindings(project_id);
CREATE INDEX IF NOT EXISTS qianchuan_bindings_status_idx
    ON qianchuan_bindings(status) WHERE status IN ('active','auth_expired');

CREATE TRIGGER set_qianchuan_bindings_updated_at
    BEFORE UPDATE ON qianchuan_bindings
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

-- Extend audit resource_type CHECK so qianchuan_binding.* events can persist.
ALTER TABLE project_audit_events
    DROP CONSTRAINT IF EXISTS project_audit_events_resource_type_check;
ALTER TABLE project_audit_events
    ADD CONSTRAINT project_audit_events_resource_type_check
    CHECK (resource_type IN (
        'project','member','task','team_run','workflow',
        'wiki','settings','automation','dashboard','auth',
        'invitation','secret','qianchuan_binding'
    ));
