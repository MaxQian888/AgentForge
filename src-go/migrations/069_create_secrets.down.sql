DROP TRIGGER IF EXISTS set_secrets_updated_at ON secrets;
DROP INDEX IF EXISTS secrets_project_idx;
DROP TABLE IF EXISTS secrets;

ALTER TABLE project_audit_events
    DROP CONSTRAINT IF EXISTS project_audit_events_resource_type_check;
ALTER TABLE project_audit_events
    ADD CONSTRAINT project_audit_events_resource_type_check
    CHECK (resource_type IN (
        'project','member','task','team_run','workflow',
        'wiki','settings','automation','dashboard','auth','invitation'
    ));
