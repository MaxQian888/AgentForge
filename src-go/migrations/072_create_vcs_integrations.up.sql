-- Per-(project, repo) VCS integration. Authoritative store for webhook
-- registration metadata and the secret-store refs used at outbound time.
-- Plaintext PAT / webhook_secret are NEVER stored here — only the
-- secrets.name they map to in the 1B secrets store.
CREATE TABLE IF NOT EXISTS vcs_integrations (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id          UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    provider            VARCHAR(16) NOT NULL,
    host                VARCHAR(256) NOT NULL,
    owner               VARCHAR(128) NOT NULL,
    repo                VARCHAR(128) NOT NULL,
    default_branch      VARCHAR(128) NOT NULL DEFAULT 'main',
    webhook_id          VARCHAR(64),
    webhook_secret_ref  VARCHAR(128) NOT NULL,
    token_secret_ref    VARCHAR(128) NOT NULL,
    status              VARCHAR(16) NOT NULL DEFAULT 'active',
    acting_employee_id  UUID REFERENCES employees(id) ON DELETE SET NULL,
    last_synced_at      TIMESTAMPTZ,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (project_id, provider, host, owner, repo)
);
CREATE INDEX IF NOT EXISTS vcs_integrations_project_idx ON vcs_integrations(project_id);
CREATE INDEX IF NOT EXISTS vcs_integrations_status_idx ON vcs_integrations(status);

CREATE TRIGGER set_vcs_integrations_updated_at
    BEFORE UPDATE ON vcs_integrations
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

-- Extend audit resource_type CHECK so vcs_integration.* events can persist.
-- (1B added 'secret', 3A added 'qianchuan_binding'; we add 'vcs_integration'
-- on top of that list.)
ALTER TABLE project_audit_events
    DROP CONSTRAINT IF EXISTS project_audit_events_resource_type_check;
ALTER TABLE project_audit_events
    ADD CONSTRAINT project_audit_events_resource_type_check
    CHECK (resource_type IN (
        'project','member','task','team_run','workflow',
        'wiki','settings','automation','dashboard','auth',
        'invitation','secret','qianchuan_binding','vcs_integration'
    ));
