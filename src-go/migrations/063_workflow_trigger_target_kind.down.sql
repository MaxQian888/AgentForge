DROP INDEX IF EXISTS workflow_triggers_config_hash_uniq;
CREATE UNIQUE INDEX workflow_triggers_config_hash_uniq
    ON workflow_triggers (workflow_id, source, md5(config::text));

ALTER TABLE workflow_triggers DROP CONSTRAINT IF EXISTS workflow_triggers_target_identifier_check;
ALTER TABLE workflow_triggers ALTER COLUMN workflow_id SET NOT NULL;
ALTER TABLE workflow_triggers DROP COLUMN IF EXISTS plugin_id;
ALTER TABLE workflow_triggers DROP COLUMN IF EXISTS disabled_reason;
ALTER TABLE workflow_triggers DROP COLUMN IF EXISTS target_kind;
