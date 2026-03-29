ALTER TABLE members
    ADD COLUMN IF NOT EXISTS status VARCHAR(20);

UPDATE members
SET status = CASE
    WHEN is_active THEN 'active'
    ELSE 'inactive'
END
WHERE status IS NULL OR status = '';

ALTER TABLE members
    ALTER COLUMN status SET DEFAULT 'active';

ALTER TABLE members
    ALTER COLUMN status SET NOT NULL;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conname = 'members_status_check'
    ) THEN
        ALTER TABLE members
            ADD CONSTRAINT members_status_check
            CHECK (status IN ('active', 'inactive', 'suspended'));
    END IF;
END $$;

ALTER TABLE members
    ADD COLUMN IF NOT EXISTS im_platform VARCHAR(50);

ALTER TABLE members
    ADD COLUMN IF NOT EXISTS im_user_id VARCHAR(255);

CREATE UNIQUE INDEX IF NOT EXISTS idx_members_im
    ON members(im_platform, im_user_id)
    WHERE im_platform IS NOT NULL AND im_user_id IS NOT NULL AND im_user_id <> '';
