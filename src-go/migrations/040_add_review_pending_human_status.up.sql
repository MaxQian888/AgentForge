ALTER TABLE reviews
DROP CONSTRAINT IF EXISTS reviews_status_check;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conname = 'reviews_status_check'
    ) THEN
        ALTER TABLE reviews
        ADD CONSTRAINT reviews_status_check
        CHECK (status IN ('pending', 'in_progress', 'completed', 'failed', 'pending_human'));
    END IF;
END $$;
