DROP TRIGGER IF EXISTS set_qianchuan_bindings_updated_at ON qianchuan_bindings;
DROP INDEX IF EXISTS qianchuan_bindings_status_idx;
DROP INDEX IF EXISTS qianchuan_bindings_project_idx;
DROP TABLE IF EXISTS qianchuan_bindings;

ALTER TABLE project_audit_events
    DROP CONSTRAINT IF EXISTS project_audit_events_resource_type_check;
ALTER TABLE project_audit_events
    ADD CONSTRAINT project_audit_events_resource_type_check
    CHECK (resource_type IN (
        'project','member','task','team_run','workflow',
        'wiki','settings','automation','dashboard','auth',
        'invitation','secret'
    ));
