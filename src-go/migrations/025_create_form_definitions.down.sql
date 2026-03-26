DROP TRIGGER IF EXISTS form_definitions_updated_at_trigger ON form_definitions;
DROP INDEX IF EXISTS idx_form_definitions_active_slug;
DROP TABLE IF EXISTS form_definitions;
