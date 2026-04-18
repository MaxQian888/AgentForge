-- 058_create_project_audit_events.down.sql
DROP INDEX IF EXISTS idx_project_audit_events_project_actor;
DROP INDEX IF EXISTS idx_project_audit_events_project_action;
DROP INDEX IF EXISTS idx_project_audit_events_project_occurred;
DROP TABLE IF EXISTS project_audit_events;
