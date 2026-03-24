ALTER TABLE tasks
    ADD COLUMN planned_start_at TIMESTAMPTZ,
    ADD COLUMN planned_end_at TIMESTAMPTZ;

CREATE INDEX idx_tasks_planned_start_at ON tasks(planned_start_at);
