ALTER TABLE reviews
  ADD COLUMN IF NOT EXISTS parent_review_id uuid REFERENCES reviews(id) ON DELETE SET NULL;
CREATE INDEX IF NOT EXISTS idx_reviews_parent_review_id ON reviews(parent_review_id) WHERE parent_review_id IS NOT NULL;
