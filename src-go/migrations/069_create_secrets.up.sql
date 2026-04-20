-- Project-scoped encrypted secrets store. Plaintext NEVER persisted.
-- ciphertext + nonce produced by AES-256-GCM (see internal/secrets/cipher.go).
-- key_version reserved for future rotation; only version 1 is supported today.
CREATE TABLE IF NOT EXISTS secrets (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id    UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    name          VARCHAR(128) NOT NULL,
    ciphertext    BYTEA NOT NULL,
    nonce         BYTEA NOT NULL,
    key_version   INT NOT NULL DEFAULT 1,
    description   TEXT,
    last_used_at  TIMESTAMPTZ,
    created_by    UUID NOT NULL,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (project_id, name)
);
CREATE INDEX IF NOT EXISTS secrets_project_idx ON secrets(project_id);

CREATE TRIGGER set_secrets_updated_at
    BEFORE UPDATE ON secrets
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

-- Extend audit resource_type CHECK so secret.* events can persist.
ALTER TABLE project_audit_events
    DROP CONSTRAINT IF EXISTS project_audit_events_resource_type_check;
ALTER TABLE project_audit_events
    ADD CONSTRAINT project_audit_events_resource_type_check
    CHECK (resource_type IN (
        'project','member','task','team_run','workflow',
        'wiki','settings','automation','dashboard','auth',
        'invitation','secret'
    ));
