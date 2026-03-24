CREATE TABLE agent_memory (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    scope VARCHAR(20) DEFAULT 'project' CHECK (scope IN ('global','project','role')),
    role_id TEXT DEFAULT '',
    category VARCHAR(30) DEFAULT 'episodic' CHECK (category IN ('episodic','semantic','procedural')),
    key TEXT NOT NULL,
    content TEXT NOT NULL DEFAULT '',
    metadata JSONB DEFAULT '{}',
    relevance_score NUMERIC(5,4) DEFAULT 1.0,
    access_count INTEGER DEFAULT 0,
    last_accessed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX idx_agent_memory_project ON agent_memory(project_id);
CREATE INDEX idx_agent_memory_scope_role ON agent_memory(scope, role_id);
CREATE INDEX idx_agent_memory_key ON agent_memory(key);
