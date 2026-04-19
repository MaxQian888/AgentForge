-- Employees: persistent capability carriers scoped per project.
CREATE TABLE employees (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id    UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    name          TEXT NOT NULL,
    display_name  TEXT,
    role_id       TEXT NOT NULL,
    runtime_prefs JSONB NOT NULL DEFAULT '{}'::jsonb,
    config        JSONB NOT NULL DEFAULT '{}'::jsonb,
    state         TEXT NOT NULL DEFAULT 'active'
                     CHECK (state IN ('active','paused','archived')),
    created_by    UUID REFERENCES members(id) ON DELETE SET NULL,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (project_id, name)
);
CREATE INDEX employees_project_state_idx ON employees(project_id, state);

CREATE TABLE employee_skills (
    employee_id UUID NOT NULL REFERENCES employees(id) ON DELETE CASCADE,
    skill_path  TEXT NOT NULL,
    auto_load   BOOLEAN NOT NULL DEFAULT true,
    overrides   JSONB NOT NULL DEFAULT '{}'::jsonb,
    added_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (employee_id, skill_path)
);

-- Workflow triggers: materialized registration rows that map external events to workflow starts.
CREATE TABLE workflow_triggers (
    id                       UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workflow_id              UUID NOT NULL REFERENCES workflow_definitions(id) ON DELETE CASCADE,
    project_id               UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    source                   TEXT NOT NULL CHECK (source IN ('im','schedule')),
    config                   JSONB NOT NULL,
    input_mapping            JSONB NOT NULL DEFAULT '{}'::jsonb,
    idempotency_key_template TEXT,
    dedupe_window_seconds    INTEGER NOT NULL DEFAULT 0 CHECK (dedupe_window_seconds >= 0),
    enabled                  BOOLEAN NOT NULL DEFAULT true,
    created_by               UUID REFERENCES members(id) ON DELETE SET NULL,
    created_at               TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at               TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX workflow_triggers_config_hash_uniq
    ON workflow_triggers (workflow_id, source, md5(config::text));
CREATE INDEX workflow_triggers_source_enabled_idx
    ON workflow_triggers (source) WHERE enabled = true;

-- agent_runs: optional employee binding.
ALTER TABLE agent_runs
    ADD COLUMN employee_id UUID REFERENCES employees(id) ON DELETE SET NULL;
CREATE INDEX agent_runs_employee_idx ON agent_runs(employee_id) WHERE employee_id IS NOT NULL;

-- agent_memory: support an employee scope with dedicated employee_id column.
-- NOTE: ON DELETE CASCADE is intentional — a memory row with scope='employee'
-- has no meaningful owner once its employee is hard-deleted. State=archived
-- is the normal retirement path and preserves memory; only hard-delete
-- (privacy/GDPR-style erase) triggers cascade.
ALTER TABLE agent_memory
    DROP CONSTRAINT IF EXISTS agent_memory_scope_check;
ALTER TABLE agent_memory
    ADD CONSTRAINT agent_memory_scope_check
    CHECK (scope IN ('global','project','role','employee'));
ALTER TABLE agent_memory
    ADD COLUMN employee_id UUID REFERENCES employees(id) ON DELETE CASCADE;
CREATE INDEX agent_memory_employee_idx
    ON agent_memory(employee_id) WHERE employee_id IS NOT NULL;

-- workflow_executions: which trigger fired this run (NULL = manual/legacy).
ALTER TABLE workflow_executions
    ADD COLUMN triggered_by UUID REFERENCES workflow_triggers(id) ON DELETE SET NULL;
CREATE INDEX workflow_executions_triggered_by_idx
    ON workflow_executions(triggered_by) WHERE triggered_by IS NOT NULL;

-- reviews: link back to workflow execution (NULL for legacy rows).
ALTER TABLE reviews
    ADD COLUMN execution_id UUID REFERENCES workflow_executions(id) ON DELETE SET NULL;
CREATE INDEX reviews_execution_id_idx
    ON reviews(execution_id) WHERE execution_id IS NOT NULL;
