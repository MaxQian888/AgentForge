-- Full-text search support for tasks
ALTER TABLE tasks ADD COLUMN IF NOT EXISTS search_vector tsvector;

CREATE INDEX IF NOT EXISTS idx_tasks_search ON tasks USING GIN(search_vector);

CREATE OR REPLACE FUNCTION tasks_search_trigger() RETURNS trigger AS $$
BEGIN
    NEW.search_vector :=
        setweight(to_tsvector('simple', COALESCE(NEW.title, '')), 'A') ||
        setweight(to_tsvector('simple', COALESCE(NEW.description, '')), 'B');
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trig_tasks_search ON tasks;
CREATE TRIGGER trig_tasks_search
    BEFORE INSERT OR UPDATE OF title, description ON tasks
    FOR EACH ROW EXECUTE FUNCTION tasks_search_trigger();

-- Backfill existing rows
UPDATE tasks SET search_vector =
    setweight(to_tsvector('simple', COALESCE(title, '')), 'A') ||
    setweight(to_tsvector('simple', COALESCE(description, '')), 'B');

-- GIN indexes for array/JSONB columns
CREATE INDEX IF NOT EXISTS idx_tasks_labels ON tasks USING GIN(labels);
CREATE INDEX IF NOT EXISTS idx_members_skills ON members USING GIN(skills);

-- Partial indexes for common queries
CREATE INDEX IF NOT EXISTS idx_tasks_active ON tasks(project_id, assignee_id, updated_at DESC) WHERE status NOT IN ('done', 'cancelled');
CREATE INDEX IF NOT EXISTS idx_tasks_kanban ON tasks(project_id, status, priority);
CREATE INDEX IF NOT EXISTS idx_agent_runs_active ON agent_runs(status, started_at DESC) WHERE status IN ('starting', 'running', 'paused');
