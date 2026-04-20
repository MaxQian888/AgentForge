-- Declarative strategy library. project_id NULL = system seed strategy.
-- yaml_source = the raw YAML the author saw.
-- parsed_spec = compiled form (pre-resolved expressions, normalized actions)
--               cached for fast strategy-runtime use (see internal/qianchuan/strategy).
-- status: draft -> published (immutable) -> archived. New version row on edit-after-publish.
CREATE TABLE IF NOT EXISTS qianchuan_strategies (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id    UUID REFERENCES projects(id) ON DELETE CASCADE,
    name          VARCHAR(128) NOT NULL,
    description   TEXT,
    yaml_source   TEXT NOT NULL,
    parsed_spec   JSONB NOT NULL,
    version       INT NOT NULL DEFAULT 1,
    status        VARCHAR(16) NOT NULL DEFAULT 'draft'
                  CHECK (status IN ('draft','published','archived')),
    created_by    UUID NOT NULL,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (project_id, name, version)
);
CREATE INDEX IF NOT EXISTS qianchuan_strategies_project_idx
    ON qianchuan_strategies(project_id);
CREATE INDEX IF NOT EXISTS qianchuan_strategies_status_idx
    ON qianchuan_strategies(status);

CREATE TRIGGER set_qianchuan_strategies_updated_at
    BEFORE UPDATE ON qianchuan_strategies
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();
