CREATE TABLE false_positives (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    pattern TEXT NOT NULL,
    category VARCHAR(64) NOT NULL,
    file_pattern TEXT NOT NULL DEFAULT '',
    reason TEXT NOT NULL DEFAULT '',
    reporter_id UUID,
    occurrences INT NOT NULL DEFAULT 1,
    is_strong BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_false_positives_project ON false_positives(project_id);
CREATE INDEX idx_false_positives_category ON false_positives(category);
