CREATE TABLE IF NOT EXISTS plugins (
    plugin_id TEXT PRIMARY KEY,
    kind TEXT NOT NULL,
    name TEXT NOT NULL,
    version TEXT NOT NULL,
    description TEXT,
    tags TEXT[] NOT NULL DEFAULT '{}',
    manifest JSONB NOT NULL,
    source_type TEXT NOT NULL,
    source_path TEXT,
    runtime TEXT NOT NULL,
    lifecycle_state TEXT NOT NULL DEFAULT 'installed',
    runtime_host TEXT NOT NULL,
    last_health_at TIMESTAMPTZ,
    last_error TEXT,
    restart_count INTEGER NOT NULL DEFAULT 0,
    resolved_source_path TEXT,
    runtime_metadata JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS plugin_instances (
    plugin_id TEXT PRIMARY KEY REFERENCES plugins(plugin_id) ON DELETE CASCADE,
    project_id TEXT,
    runtime_host TEXT NOT NULL,
    lifecycle_state TEXT NOT NULL,
    resolved_source_path TEXT,
    runtime_metadata JSONB,
    restart_count INTEGER NOT NULL DEFAULT 0,
    last_health_at TIMESTAMPTZ,
    last_error TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS plugin_events (
    id TEXT PRIMARY KEY,
    plugin_id TEXT NOT NULL REFERENCES plugins(plugin_id) ON DELETE CASCADE,
    event_type TEXT NOT NULL,
    event_source TEXT NOT NULL,
    lifecycle_state TEXT,
    summary TEXT,
    payload JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_plugins_kind ON plugins(kind);
CREATE INDEX IF NOT EXISTS idx_plugins_lifecycle_state ON plugins(lifecycle_state);
CREATE INDEX IF NOT EXISTS idx_plugins_runtime_host ON plugins(runtime_host);
CREATE INDEX IF NOT EXISTS idx_plugin_instances_project_id ON plugin_instances(project_id);
CREATE INDEX IF NOT EXISTS idx_plugin_events_plugin_created_at ON plugin_events(plugin_id, created_at DESC);

