CREATE TABLE IF NOT EXISTS agents (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	team_id INTEGER NOT NULL REFERENCES teams(id) ON DELETE CASCADE,
	slug TEXT NOT NULL,
	name TEXT NOT NULL,
	api_token_hash TEXT NOT NULL UNIQUE,
	status TEXT NOT NULL CHECK (status IN ('active', 'paused', 'revoked')),
	kind TEXT NOT NULL DEFAULT 'student',
	repo_url TEXT NOT NULL DEFAULT '',
	commit_sha TEXT NOT NULL DEFAULT '',
	docker_image TEXT NOT NULL DEFAULT '',
	metadata_json TEXT NOT NULL DEFAULT '{}',
	created_at TEXT NOT NULL,
	updated_at TEXT NOT NULL,
	UNIQUE (team_id, slug)
);

ALTER TABLE agent_heartbeats ADD COLUMN agent_id INTEGER REFERENCES agents(id) ON DELETE SET NULL;
ALTER TABLE decisions ADD COLUMN agent_id INTEGER REFERENCES agents(id) ON DELETE SET NULL;
ALTER TABLE orders ADD COLUMN agent_id INTEGER REFERENCES agents(id) ON DELETE SET NULL;
ALTER TABLE risk_events ADD COLUMN agent_id INTEGER REFERENCES agents(id) ON DELETE SET NULL;

CREATE TABLE IF NOT EXISTS api_requests (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	team_id INTEGER REFERENCES teams(id) ON DELETE SET NULL,
	agent_id INTEGER REFERENCES agents(id) ON DELETE SET NULL,
	method TEXT NOT NULL,
	path TEXT NOT NULL,
	status INTEGER NOT NULL,
	rate_limited INTEGER NOT NULL DEFAULT 0,
	ip_hash TEXT NOT NULL DEFAULT '',
	user_agent_hash TEXT NOT NULL DEFAULT '',
	created_at TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_agents_team_status ON agents(team_id, status);
CREATE INDEX IF NOT EXISTS idx_agents_token_hash ON agents(api_token_hash);
CREATE INDEX IF NOT EXISTS idx_heartbeats_agent_created ON agent_heartbeats(agent_id, created_at);
CREATE INDEX IF NOT EXISTS idx_decisions_agent_created ON decisions(agent_id, created_at);
CREATE INDEX IF NOT EXISTS idx_orders_agent_created ON orders(agent_id, created_at);
CREATE INDEX IF NOT EXISTS idx_risk_events_agent_created ON risk_events(agent_id, created_at);
CREATE INDEX IF NOT EXISTS idx_api_requests_created ON api_requests(created_at);
CREATE INDEX IF NOT EXISTS idx_api_requests_team_agent_created ON api_requests(team_id, agent_id, created_at);
