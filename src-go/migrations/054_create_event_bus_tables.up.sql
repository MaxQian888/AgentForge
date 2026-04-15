BEGIN;

CREATE TABLE IF NOT EXISTS events (
    id           TEXT PRIMARY KEY,
    type         TEXT NOT NULL,
    source       TEXT NOT NULL,
    target       TEXT NOT NULL,
    visibility   TEXT NOT NULL DEFAULT 'channel',
    payload      JSONB,
    metadata     JSONB NOT NULL DEFAULT '{}'::jsonb,
    project_id   UUID,
    occurred_at  BIGINT NOT NULL,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS events_type_idx        ON events (type, occurred_at DESC);
CREATE INDEX IF NOT EXISTS events_target_idx      ON events (target, occurred_at DESC);
CREATE INDEX IF NOT EXISTS events_project_idx     ON events (project_id, occurred_at DESC);

CREATE TABLE IF NOT EXISTS events_dead_letter (
    id            BIGSERIAL PRIMARY KEY,
    event_id      TEXT NOT NULL,
    envelope      JSONB NOT NULL,
    last_error    TEXT NOT NULL,
    retry_count   INTEGER NOT NULL DEFAULT 0,
    first_seen_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_seen_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS events_dlq_event_idx ON events_dead_letter (event_id);

DROP TABLE IF EXISTS agent_events;

COMMIT;
