-- workflow_triggers: add target_kind discriminator so each row declares
-- which execution engine handles it.  Default 'dag' preserves behaviour for
-- all pre-existing rows; CHECK constraint restricts to the currently
-- registered engine set (dag, plugin).
ALTER TABLE workflow_triggers
    ADD COLUMN target_kind TEXT NOT NULL DEFAULT 'dag'
        CHECK (target_kind IN ('dag', 'plugin'));

-- disabled_reason: when the registrar fails to resolve a trigger's target
-- (e.g. workflow id missing in the declared engine) the row is persisted
-- with enabled=false and a machine-readable reason.  Null for currently-
-- enabled rows.
ALTER TABLE workflow_triggers
    ADD COLUMN disabled_reason TEXT;

-- plugin_id: non-null when target_kind='plugin', identifying the workflow
-- plugin record in the plugin runtime.  Must be null when target_kind='dag'
-- (DAG rows keep using workflow_id for their workflow_definitions FK).
ALTER TABLE workflow_triggers
    ADD COLUMN plugin_id TEXT;

-- DAG rows continue to require workflow_id; plugin rows must NOT rely on
-- workflow_id (there is no workflow_definitions row for a plugin).  We relax
-- the workflow_id FK to ON DELETE CASCADE-compatible NULLability so a plugin
-- trigger can store NULL here.
ALTER TABLE workflow_triggers ALTER COLUMN workflow_id DROP NOT NULL;

-- Enforce the (target_kind → identifier) invariant as a CHECK.
ALTER TABLE workflow_triggers
    ADD CONSTRAINT workflow_triggers_target_identifier_check CHECK (
        (target_kind = 'dag'    AND workflow_id IS NOT NULL AND plugin_id IS NULL)
     OR (target_kind = 'plugin' AND plugin_id  IS NOT NULL AND workflow_id IS NULL)
    );

-- Replace the existing unique index so the target_kind + plugin_id are part
-- of the canonical key.  For DAG rows, plugin_id is NULL (distinct); for
-- plugin rows, workflow_id is NULL (distinct).  md5() on NULL returns NULL
-- which is distinct-by-default in Postgres unique indexes, so we coalesce.
DROP INDEX IF EXISTS workflow_triggers_config_hash_uniq;
CREATE UNIQUE INDEX workflow_triggers_config_hash_uniq
    ON workflow_triggers (
        target_kind,
        COALESCE(workflow_id::text, plugin_id),
        source,
        md5(config::text)
    );
