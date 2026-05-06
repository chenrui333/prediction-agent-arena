CREATE TABLE IF NOT EXISTS round_teams (
	round_id INTEGER NOT NULL REFERENCES rounds(id) ON DELETE CASCADE,
	team_id INTEGER NOT NULL REFERENCES teams(id) ON DELETE CASCADE,
	status TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'paused', 'withdrawn')),
	created_at TEXT NOT NULL,
	updated_at TEXT NOT NULL,
	PRIMARY KEY (round_id, team_id)
);

INSERT OR IGNORE INTO round_teams(round_id, team_id, status, created_at, updated_at)
SELECT r.id, t.id, 'active', r.created_at, r.updated_at
FROM rounds r
JOIN teams t ON t.is_active = 1;

CREATE INDEX IF NOT EXISTS idx_round_teams_round_status ON round_teams(round_id, status);
CREATE INDEX IF NOT EXISTS idx_round_teams_team_status ON round_teams(team_id, status);
