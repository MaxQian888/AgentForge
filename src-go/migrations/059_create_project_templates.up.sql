-- 059_create_project_templates.up.sql
--
-- project_templates stores reusable "project configuration snapshots" — see:
--   - openspec/changes/2026-04-17-add-project-templates/proposal.md
--   - openspec/changes/2026-04-17-add-project-templates/design.md
--
-- snapshot_json carries the versioned structured configuration payload
-- (settings, customFields, savedViews, dashboards, automations,
-- workflowDefinitions, taskStatuses, memberRolePlaceholders). Business data
-- (members, tasks, wiki pages, runs, logs, memory, invitations) is explicitly
-- excluded and rejected by the sanitizer.
--
-- `source` distinguishes origin and drives visibility/editing rules:
--   * system      — built-in bundle, read-only, global
--   * user        — owner-private, mutable by owner
--   * marketplace — materialized from a marketplace install, owner = installer
--
-- `owner_user_id` is nullable because system templates have no owner. A partial
-- index on (source='user') accelerates the owner-scoped list query.

CREATE TABLE IF NOT EXISTS project_templates (
    id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    source           VARCHAR(16) NOT NULL,
    owner_user_id    UUID,
    name             VARCHAR(128) NOT NULL,
    description      TEXT NOT NULL DEFAULT '',
    snapshot_json    JSONB NOT NULL DEFAULT '{}'::jsonb,
    snapshot_version INTEGER NOT NULL DEFAULT 1,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT project_templates_source_check
        CHECK (source IN ('system', 'user', 'marketplace')),
    CONSTRAINT project_templates_owner_rule
        CHECK (
            (source = 'system' AND owner_user_id IS NULL) OR
            (source <> 'system' AND owner_user_id IS NOT NULL)
        )
);

CREATE INDEX IF NOT EXISTS idx_project_templates_source_owner
    ON project_templates(source, owner_user_id);

CREATE INDEX IF NOT EXISTS idx_project_templates_source_name
    ON project_templates(source, name);

CREATE INDEX IF NOT EXISTS idx_project_templates_user_owner
    ON project_templates(owner_user_id)
    WHERE source = 'user';

CREATE OR REPLACE FUNCTION trg_project_templates_touch_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at := NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS project_templates_touch_updated_at ON project_templates;
CREATE TRIGGER project_templates_touch_updated_at
    BEFORE UPDATE ON project_templates
    FOR EACH ROW
    EXECUTE FUNCTION trg_project_templates_touch_updated_at();
