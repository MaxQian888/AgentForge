CREATE TABLE custom_field_values (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    task_id UUID NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
    field_def_id UUID NOT NULL REFERENCES custom_field_defs(id) ON DELETE CASCADE,
    value JSONB NOT NULL DEFAULT 'null',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_custom_field_values_task_field UNIQUE (task_id, field_def_id)
);

CREATE INDEX idx_custom_field_values_task ON custom_field_values(task_id);
CREATE INDEX idx_custom_field_values_field ON custom_field_values(field_def_id);
CREATE INDEX idx_custom_field_values_value_gin ON custom_field_values USING GIN(value);

CREATE TRIGGER custom_field_values_updated_at_trigger
    BEFORE UPDATE ON custom_field_values
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
