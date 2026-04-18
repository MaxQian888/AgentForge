-- 061_create_project_invitations.down.sql

DROP INDEX IF EXISTS idx_project_invitations_invited_user_status;
DROP INDEX IF EXISTS uq_project_invitations_token_hash_pending;
DROP INDEX IF EXISTS idx_project_invitations_project_status_expires;
DROP TABLE IF EXISTS project_invitations;

-- Restore the original project_audit_events.resource_type CHECK from
-- migration 058 (without 'invitation').
ALTER TABLE project_audit_events
    DROP CONSTRAINT IF EXISTS project_audit_events_resource_type_check;

ALTER TABLE project_audit_events
    ADD CONSTRAINT project_audit_events_resource_type_check
    CHECK (resource_type IN (
        'project', 'member', 'task', 'team_run', 'workflow',
        'wiki', 'settings', 'automation', 'dashboard', 'auth'
    ));
