CREATE TABLE IF NOT EXISTS documents (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id UUID NOT NULL,
    name TEXT NOT NULL,
    file_type VARCHAR(10) NOT NULL,
    file_size BIGINT DEFAULT 0,
    storage_key TEXT NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'pending',
    chunk_count INT DEFAULT 0,
    error TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_documents_project_id ON documents(project_id);
