ALTER TABLE notifications ADD COLUMN channel VARCHAR(32) NOT NULL DEFAULT 'in_app';
ALTER TABLE notifications ADD COLUMN sent BOOLEAN NOT NULL DEFAULT false;
CREATE INDEX idx_notifications_channel ON notifications(channel);
CREATE INDEX idx_notifications_unsent ON notifications(created_at) WHERE sent = false;
