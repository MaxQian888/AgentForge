BEGIN;

DROP TABLE IF EXISTS events_dead_letter;
DROP TABLE IF EXISTS events;

CREATE TABLE IF NOT EXISTS agent_events (
    id          UUID PRIMARY KEY,
    run_id      UUID NOT NULL,
    task_id     UUID NOT NULL,
    project_id  UUID NOT NULL,
    event_type  TEXT NOT NULL,
    payload     TEXT NOT NULL,
    occurred_at TIMESTAMPTZ NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

COMMIT;
