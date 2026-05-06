CREATE INDEX IF NOT EXISTS idx_heartbeats_agent ON agent_heartbeats(agent_id);
CREATE INDEX IF NOT EXISTS idx_decisions_agent ON decisions(agent_id);
CREATE INDEX IF NOT EXISTS idx_orders_agent ON orders(agent_id);
CREATE INDEX IF NOT EXISTS idx_risk_events_agent ON risk_events(agent_id);
CREATE INDEX IF NOT EXISTS idx_api_requests_rate_limited_created ON api_requests(rate_limited, created_at);

CREATE TABLE IF NOT EXISTS round_agents (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	round_id INTEGER NOT NULL REFERENCES rounds(id) ON DELETE CASCADE,
	team_id INTEGER NOT NULL REFERENCES teams(id) ON DELETE CASCADE,
	agent_id INTEGER NOT NULL REFERENCES agents(id) ON DELETE CASCADE,
	commit_sha TEXT NOT NULL DEFAULT '',
	docker_image TEXT NOT NULL DEFAULT '',
	metadata_json TEXT NOT NULL DEFAULT '{}',
	locked_by TEXT NOT NULL DEFAULT 'admin',
	created_at TEXT NOT NULL,
	updated_at TEXT NOT NULL,
	UNIQUE (round_id, team_id),
	UNIQUE (round_id, agent_id)
);

CREATE INDEX IF NOT EXISTS idx_round_agents_round ON round_agents(round_id);
CREATE INDEX IF NOT EXISTS idx_round_agents_agent ON round_agents(agent_id);
CREATE INDEX IF NOT EXISTS idx_round_agents_team ON round_agents(team_id);
