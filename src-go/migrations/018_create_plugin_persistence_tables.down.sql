DROP INDEX IF EXISTS idx_plugin_events_plugin_created_at;
DROP INDEX IF EXISTS idx_plugin_instances_project_id;
DROP INDEX IF EXISTS idx_plugins_runtime_host;
DROP INDEX IF EXISTS idx_plugins_lifecycle_state;
DROP INDEX IF EXISTS idx_plugins_kind;

DROP TABLE IF EXISTS plugin_events;
DROP TABLE IF EXISTS plugin_instances;
DROP TABLE IF EXISTS plugins;
