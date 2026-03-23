DROP TRIGGER IF EXISTS tasks_updated_at_trigger ON tasks;
DROP TRIGGER IF EXISTS tasks_search_vector_trigger ON tasks;
DROP FUNCTION IF EXISTS update_updated_at_column();
DROP FUNCTION IF EXISTS tasks_search_vector_update();
DROP TABLE IF EXISTS tasks CASCADE;
