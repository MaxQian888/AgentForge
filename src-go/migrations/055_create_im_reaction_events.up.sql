CREATE TABLE IF NOT EXISTS im_reaction_events (
    id          UUID PRIMARY KEY,
    platform    TEXT NOT NULL,
    chat_id     TEXT NOT NULL DEFAULT '',
    message_id  TEXT NOT NULL,
    user_id     TEXT NOT NULL DEFAULT '',
    emoji       TEXT NOT NULL DEFAULT '',
    event_type  TEXT NOT NULL CHECK (event_type IN ('created', 'deleted')),
    raw_payload JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS im_reaction_events_message_created_idx
    ON im_reaction_events (message_id, created_at DESC);

CREATE INDEX IF NOT EXISTS im_reaction_events_platform_created_idx
    ON im_reaction_events (platform, created_at DESC);
