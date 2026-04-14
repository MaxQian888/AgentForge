CREATE TABLE team_artifacts (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    team_id    UUID NOT NULL REFERENCES agent_teams(id) ON DELETE CASCADE,
    run_id     UUID NOT NULL REFERENCES agent_runs(id) ON DELETE CASCADE,
    role       VARCHAR(30) NOT NULL DEFAULT '',
    key        VARCHAR(120) NOT NULL DEFAULT '',
    value      JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_team_artifacts_team ON team_artifacts(team_id, created_at);
CREATE INDEX idx_team_artifacts_team_role ON team_artifacts(team_id, role);
