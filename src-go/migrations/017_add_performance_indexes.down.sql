DROP INDEX IF EXISTS idx_agent_runs_active;
DROP INDEX IF EXISTS idx_tasks_kanban;
DROP INDEX IF EXISTS idx_tasks_active;
DROP INDEX IF EXISTS idx_members_skills;
DROP INDEX IF EXISTS idx_tasks_labels;
DROP TRIGGER IF EXISTS trig_tasks_search ON tasks;
DROP FUNCTION IF EXISTS tasks_search_trigger();
DROP INDEX IF EXISTS idx_tasks_search;
ALTER TABLE tasks DROP COLUMN IF EXISTS search_vector;
