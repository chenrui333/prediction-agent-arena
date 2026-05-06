ALTER TABLE positions ADD COLUMN avg_entry_price_bps INTEGER NOT NULL DEFAULT 0;
ALTER TABLE positions ADD COLUMN realized_pnl_cents INTEGER NOT NULL DEFAULT 0;

CREATE TABLE IF NOT EXISTS settlements (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	round_id INTEGER NOT NULL REFERENCES rounds(id) ON DELETE CASCADE,
	team_id INTEGER NOT NULL REFERENCES teams(id) ON DELETE CASCADE,
	market_id INTEGER NOT NULL REFERENCES markets(id) ON DELETE CASCADE,
	outcome TEXT NOT NULL CHECK (outcome IN ('yes', 'no')),
	resolved_outcome TEXT NOT NULL CHECK (resolved_outcome IN ('yes', 'no')),
	quantity_cents INTEGER NOT NULL,
	settlement_price_bps INTEGER NOT NULL,
	cash_delta_cents INTEGER NOT NULL,
	realized_pnl_cents INTEGER NOT NULL,
	created_at TEXT NOT NULL,
	UNIQUE (round_id, team_id, market_id, outcome)
);

CREATE TABLE IF NOT EXISTS admin_actions (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	round_id INTEGER REFERENCES rounds(id) ON DELETE SET NULL,
	team_id INTEGER REFERENCES teams(id) ON DELETE SET NULL,
	action TEXT NOT NULL,
	actor TEXT NOT NULL DEFAULT 'admin',
	metadata_json TEXT NOT NULL DEFAULT '{}',
	created_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS worker_heartbeats (
	service TEXT PRIMARY KEY,
	last_seen_at TEXT NOT NULL,
	metadata_json TEXT NOT NULL DEFAULT '{}',
	updated_at TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_settlements_round_team_market ON settlements(round_id, team_id, market_id);
CREATE INDEX IF NOT EXISTS idx_admin_actions_created ON admin_actions(created_at);
CREATE INDEX IF NOT EXISTS idx_admin_actions_round_team ON admin_actions(round_id, team_id);
