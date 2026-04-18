-- 061_create_project_invitations.up.sql
--
-- Introduces `project_invitations`: pending contracts that materialize into
-- a human `members` row only after the invited user accepts. Stores a
-- SHA-256 hash of the one-time accept token; plaintext is never persisted.
--
-- See:
--   - openspec/specs/member-invitation-flow/spec.md
--   - openspec/changes/2026-04-17-add-member-invitation-flow/design.md
--
-- Status enum values:
--   pending   — created, awaiting accept/decline/revoke/expire
--   accepted  — accept path completed, member materialized
--   declined  — invited identity refused
--   expired   — scheduler or accept-time check found expires_at < now()
--   revoked   — admin cancelled the pending invitation
--
-- Identity shape (JSONB) is validated in the application layer:
--   {"kind":"email","value":"x@y.com"}
--   {"kind":"im","platform":"feishu","userId":"…","displayName":"…"}
--
-- This migration also extends `project_audit_events.resource_type` to
-- accept 'invitation' so the RBAC audit emitter can record `invitation.*`
-- actions without a special case in the service layer.

CREATE TABLE IF NOT EXISTS project_invitations (
    id                            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id                    UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    inviter_user_id               UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    invited_identity              JSONB NOT NULL,
    invited_user_id               UUID REFERENCES users(id) ON DELETE SET NULL,
    project_role                  VARCHAR(16) NOT NULL,
    status                        VARCHAR(16) NOT NULL,
    token_hash                    VARCHAR(128),
    message                       TEXT NOT NULL DEFAULT '',
    expires_at                    TIMESTAMPTZ NOT NULL,
    created_at                    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at                    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    accepted_at                   TIMESTAMPTZ,
    decline_reason                TEXT NOT NULL DEFAULT '',
    revoke_reason                 TEXT NOT NULL DEFAULT '',
    last_delivery_status          VARCHAR(32) NOT NULL DEFAULT '',
    last_delivery_attempted_at    TIMESTAMPTZ,
    CONSTRAINT project_invitations_status_check
        CHECK (status IN ('pending', 'accepted', 'declined', 'expired', 'revoked')),
    CONSTRAINT project_invitations_project_role_check
        CHECK (project_role IN ('owner', 'admin', 'editor', 'viewer'))
);

CREATE INDEX IF NOT EXISTS idx_project_invitations_project_status_expires
    ON project_invitations(project_id, status, expires_at);

CREATE UNIQUE INDEX IF NOT EXISTS uq_project_invitations_token_hash_pending
    ON project_invitations(token_hash)
    WHERE status = 'pending' AND token_hash IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_project_invitations_invited_user_status
    ON project_invitations(invited_user_id, status);

-- Extend the project_audit_events resource_type CHECK to include
-- 'invitation'. CHECK constraints cannot be altered in place; drop-and-
-- recreate under the same name keeps downstream readers unaware.
ALTER TABLE project_audit_events
    DROP CONSTRAINT IF EXISTS project_audit_events_resource_type_check;

ALTER TABLE project_audit_events
    ADD CONSTRAINT project_audit_events_resource_type_check
    CHECK (resource_type IN (
        'project', 'member', 'task', 'team_run', 'workflow',
        'wiki', 'settings', 'automation', 'dashboard', 'auth',
        'invitation'
    ));
