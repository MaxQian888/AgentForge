ALTER TABLE reviews
ADD COLUMN IF NOT EXISTS execution_metadata JSONB DEFAULT '{}';
