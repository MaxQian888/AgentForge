-- Spec 3 §6.5 — short-lived CSRF nonces for Qianchuan OAuth bind flow.
-- Rows expire after 10 minutes; consumed_at marks single-use semantics.
CREATE TABLE IF NOT EXISTS qianchuan_oauth_states (
    state_token         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id          UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    redirect_uri        TEXT NOT NULL,
    initiated_by        UUID NOT NULL,
    display_name        VARCHAR(128),
    acting_employee_id  UUID,
    expires_at          TIMESTAMPTZ NOT NULL DEFAULT (now() + interval '10 minutes'),
    consumed_at         TIMESTAMPTZ,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_qoauth_active
    ON qianchuan_oauth_states (expires_at)
    WHERE consumed_at IS NULL;
