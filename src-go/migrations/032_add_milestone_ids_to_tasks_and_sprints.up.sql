ALTER TABLE tasks
    ADD COLUMN milestone_id UUID REFERENCES milestones(id) ON DELETE SET NULL;

ALTER TABLE sprints
    ADD COLUMN milestone_id UUID REFERENCES milestones(id) ON DELETE SET NULL;

CREATE INDEX idx_tasks_milestone ON tasks(milestone_id);
CREATE INDEX idx_sprints_milestone ON sprints(milestone_id);
