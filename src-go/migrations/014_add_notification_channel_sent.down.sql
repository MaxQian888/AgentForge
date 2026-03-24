DROP INDEX IF EXISTS idx_notifications_unsent;
DROP INDEX IF EXISTS idx_notifications_channel;
ALTER TABLE notifications DROP COLUMN IF EXISTS sent;
ALTER TABLE notifications DROP COLUMN IF EXISTS channel;
