DROP TRIGGER IF EXISTS set_vcs_integrations_updated_at ON vcs_integrations;
DROP INDEX IF EXISTS vcs_integrations_status_idx;
DROP INDEX IF EXISTS vcs_integrations_project_idx;
DROP TABLE IF EXISTS vcs_integrations;

ALTER TABLE project_audit_events
    DROP CONSTRAINT IF EXISTS project_audit_events_resource_type_check;
ALTER TABLE project_audit_events
    ADD CONSTRAINT project_audit_events_resource_type_check
    CHECK (resource_type IN (
        'project','member','task','team_run','workflow',
        'wiki','settings','automation','dashboard','auth',
        'invitation','secret','qianchuan_binding'
    ));
