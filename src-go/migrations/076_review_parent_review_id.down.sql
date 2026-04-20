DROP INDEX IF EXISTS idx_reviews_parent_review_id;
ALTER TABLE reviews DROP COLUMN IF EXISTS parent_review_id;
