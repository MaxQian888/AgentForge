-- Tasks table
CREATE TABLE tasks (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    parent_id UUID REFERENCES tasks(id) ON DELETE SET NULL,
    sprint_id UUID REFERENCES sprints(id) ON DELETE SET NULL,
    title TEXT NOT NULL,
    description TEXT DEFAULT '',
    status VARCHAR(30) DEFAULT 'inbox' CHECK (status IN (
        'inbox', 'triaged', 'assigned', 'in_progress', 'in_review',
        'changes_requested', 'done', 'cancelled', 'blocked', 'budget_exceeded'
    )),
    priority VARCHAR(10) DEFAULT 'medium' CHECK (priority IN ('urgent', 'high', 'medium', 'low')),
    assignee_id UUID REFERENCES members(id) ON DELETE SET NULL,
    assignee_type VARCHAR(10) CHECK (assignee_type IN ('human', 'agent')),
    reporter_id UUID REFERENCES members(id) ON DELETE SET NULL,
    labels TEXT[] DEFAULT '{}',
    budget_usd NUMERIC(10,2) DEFAULT 5.00,
    spent_usd NUMERIC(10,2) DEFAULT 0,
    agent_branch TEXT DEFAULT '',
    agent_worktree TEXT DEFAULT '',
    agent_session_id TEXT DEFAULT '',
    pr_url TEXT DEFAULT '',
    pr_number INTEGER,
    blocked_by UUID[] DEFAULT '{}',
    search_vector TSVECTOR,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW(),
    completed_at TIMESTAMPTZ
);

-- Indexes
CREATE INDEX idx_tasks_project_status ON tasks(project_id, status);
CREATE INDEX idx_tasks_assignee ON tasks(assignee_id);
CREATE INDEX idx_tasks_sprint ON tasks(sprint_id);
CREATE INDEX idx_tasks_parent ON tasks(parent_id);
CREATE INDEX idx_tasks_project_priority ON tasks(project_id, priority);
CREATE INDEX idx_tasks_labels ON tasks USING GIN(labels);
CREATE INDEX idx_tasks_search ON tasks USING GIN(search_vector);
CREATE INDEX idx_tasks_active ON tasks(project_id, status) WHERE status NOT IN ('done', 'cancelled');

-- Auto-update search_vector trigger
CREATE OR REPLACE FUNCTION tasks_search_vector_update() RETURNS trigger AS $$
BEGIN
    NEW.search_vector := to_tsvector('english', COALESCE(NEW.title, '') || ' ' || COALESCE(NEW.description, ''));
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER tasks_search_vector_trigger
    BEFORE INSERT OR UPDATE OF title, description ON tasks
    FOR EACH ROW EXECUTE FUNCTION tasks_search_vector_update();

-- Auto-update updated_at trigger
CREATE OR REPLACE FUNCTION update_updated_at_column() RETURNS trigger AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER tasks_updated_at_trigger
    BEFORE UPDATE ON tasks
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
