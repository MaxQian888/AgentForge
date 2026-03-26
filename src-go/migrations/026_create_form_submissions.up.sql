CREATE TABLE form_submissions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    form_id UUID NOT NULL REFERENCES form_definitions(id) ON DELETE CASCADE,
    task_id UUID NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
    submitted_by TEXT,
    submitted_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    ip_address TEXT DEFAULT ''
);

CREATE INDEX idx_form_submissions_form ON form_submissions(form_id, submitted_at DESC);
CREATE INDEX idx_form_submissions_task ON form_submissions(task_id);
