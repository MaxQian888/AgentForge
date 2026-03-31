DROP TRIGGER IF EXISTS update_marketplace_reviews_updated_at ON marketplace_reviews;
DROP TRIGGER IF EXISTS update_marketplace_items_updated_at ON marketplace_items;

DROP FUNCTION IF EXISTS update_updated_at_column();

DROP INDEX IF EXISTS idx_marketplace_reviews_item;
DROP INDEX IF EXISTS idx_marketplace_item_versions;
DROP INDEX IF EXISTS idx_marketplace_items_author;
DROP INDEX IF EXISTS idx_marketplace_items_featured;
DROP INDEX IF EXISTS idx_marketplace_items_category;
DROP INDEX IF EXISTS idx_marketplace_items_type;

DROP TABLE IF EXISTS marketplace_reviews;
DROP TABLE IF EXISTS marketplace_item_versions;
DROP TABLE IF EXISTS marketplace_items;
