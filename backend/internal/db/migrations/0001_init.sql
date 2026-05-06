CREATE TABLE IF NOT EXISTS teams (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	slug TEXT NOT NULL UNIQUE,
	name TEXT NOT NULL,
	api_token_hash TEXT NOT NULL UNIQUE,
	is_active INTEGER NOT NULL DEFAULT 1,
	created_at TEXT NOT NULL,
	updated_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS rounds (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	slug TEXT NOT NULL UNIQUE,
	name TEXT NOT NULL,
	mode TEXT NOT NULL CHECK (mode IN ('practice', 'live_paper', 'replay')),
	status TEXT NOT NULL CHECK (status IN ('draft', 'active', 'paused', 'completed')),
	initial_balance_cents INTEGER NOT NULL,
	starts_at TEXT,
	ends_at TEXT,
	created_at TEXT NOT NULL,
	updated_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS markets (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	venue TEXT NOT NULL,
	external_id TEXT NOT NULL,
	slug TEXT NOT NULL UNIQUE,
	title TEXT NOT NULL,
	category TEXT NOT NULL,
	status TEXT NOT NULL,
	yes_price_bps INTEGER NOT NULL,
	no_price_bps INTEGER NOT NULL,
	metadata_json TEXT NOT NULL DEFAULT '{}',
	created_at TEXT NOT NULL,
	updated_at TEXT NOT NULL,
	UNIQUE (venue, external_id)
);

CREATE TABLE IF NOT EXISTS round_markets (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	round_id INTEGER NOT NULL REFERENCES rounds(id) ON DELETE CASCADE,
	market_id INTEGER NOT NULL REFERENCES markets(id) ON DELETE CASCADE,
	UNIQUE (round_id, market_id)
);

CREATE TABLE IF NOT EXISTS portfolio_snapshots (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	round_id INTEGER NOT NULL REFERENCES rounds(id) ON DELETE CASCADE,
	team_id INTEGER NOT NULL REFERENCES teams(id) ON DELETE CASCADE,
	cash_cents INTEGER NOT NULL,
	equity_cents INTEGER NOT NULL,
	realized_pnl_cents INTEGER NOT NULL,
	unrealized_pnl_cents INTEGER NOT NULL,
	gross_exposure_cents INTEGER NOT NULL,
	max_drawdown_bps INTEGER NOT NULL,
	created_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS decisions (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	round_id INTEGER NOT NULL REFERENCES rounds(id) ON DELETE CASCADE,
	team_id INTEGER NOT NULL REFERENCES teams(id) ON DELETE CASCADE,
	market_id INTEGER NOT NULL REFERENCES markets(id) ON DELETE CASCADE,
	observed_price_bps INTEGER NOT NULL,
	estimated_probability_bps INTEGER,
	edge_bps INTEGER NOT NULL,
	action TEXT NOT NULL,
	outcome TEXT NOT NULL,
	amount_cents INTEGER NOT NULL,
	confidence TEXT NOT NULL,
	reason TEXT NOT NULL,
	raw_payload_json TEXT NOT NULL,
	created_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS orders (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	round_id INTEGER NOT NULL REFERENCES rounds(id) ON DELETE CASCADE,
	team_id INTEGER NOT NULL REFERENCES teams(id) ON DELETE CASCADE,
	market_id INTEGER NOT NULL REFERENCES markets(id) ON DELETE CASCADE,
	venue_order_id TEXT NOT NULL DEFAULT '',
	action TEXT NOT NULL CHECK (action IN ('buy', 'sell')),
	outcome TEXT NOT NULL CHECK (outcome IN ('yes', 'no')),
	amount_cents INTEGER NOT NULL,
	limit_price_bps INTEGER NOT NULL,
	status TEXT NOT NULL CHECK (status IN ('submitted', 'rejected', 'open', 'filled', 'canceled', 'failed')),
	rejection_reason TEXT NOT NULL DEFAULT '',
	created_at TEXT NOT NULL,
	updated_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS fills (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	round_id INTEGER NOT NULL REFERENCES rounds(id) ON DELETE CASCADE,
	team_id INTEGER NOT NULL REFERENCES teams(id) ON DELETE CASCADE,
	order_id INTEGER NOT NULL REFERENCES orders(id) ON DELETE CASCADE,
	market_id INTEGER NOT NULL REFERENCES markets(id) ON DELETE CASCADE,
	action TEXT NOT NULL,
	outcome TEXT NOT NULL,
	amount_cents INTEGER NOT NULL,
	fill_price_bps INTEGER NOT NULL,
	fee_cents INTEGER NOT NULL,
	slippage_bps INTEGER NOT NULL,
	created_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS positions (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	round_id INTEGER NOT NULL REFERENCES rounds(id) ON DELETE CASCADE,
	team_id INTEGER NOT NULL REFERENCES teams(id) ON DELETE CASCADE,
	market_id INTEGER NOT NULL REFERENCES markets(id) ON DELETE CASCADE,
	outcome TEXT NOT NULL CHECK (outcome IN ('yes', 'no')),
	quantity_cents INTEGER NOT NULL,
	updated_at TEXT NOT NULL,
	UNIQUE (round_id, team_id, market_id, outcome)
);

CREATE TABLE IF NOT EXISTS risk_events (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	round_id INTEGER NOT NULL REFERENCES rounds(id) ON DELETE CASCADE,
	team_id INTEGER NOT NULL REFERENCES teams(id) ON DELETE CASCADE,
	order_id INTEGER REFERENCES orders(id) ON DELETE SET NULL,
	type TEXT NOT NULL,
	message TEXT NOT NULL,
	created_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS score_snapshots (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	round_id INTEGER NOT NULL REFERENCES rounds(id) ON DELETE CASCADE,
	team_id INTEGER NOT NULL REFERENCES teams(id) ON DELETE CASCADE,
	composite_score INTEGER NOT NULL,
	return_score INTEGER NOT NULL,
	risk_score INTEGER NOT NULL,
	calibration_score INTEGER NOT NULL,
	execution_score INTEGER NOT NULL,
	cost_score INTEGER NOT NULL,
	equity_cents INTEGER NOT NULL,
	return_bps INTEGER NOT NULL,
	max_drawdown_bps INTEGER NOT NULL,
	brier_score_bps INTEGER NOT NULL,
	trade_count INTEGER NOT NULL,
	created_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS agent_heartbeats (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	round_id INTEGER NOT NULL REFERENCES rounds(id) ON DELETE CASCADE,
	team_id INTEGER NOT NULL REFERENCES teams(id) ON DELETE CASCADE,
	status TEXT NOT NULL,
	metadata_json TEXT NOT NULL DEFAULT '{}',
	created_at TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_teams_active ON teams(is_active);
CREATE INDEX IF NOT EXISTS idx_rounds_status ON rounds(status);
CREATE INDEX IF NOT EXISTS idx_markets_status ON markets(status);
CREATE INDEX IF NOT EXISTS idx_round_markets_round ON round_markets(round_id);
CREATE INDEX IF NOT EXISTS idx_portfolio_round_team_created ON portfolio_snapshots(round_id, team_id, created_at);
CREATE INDEX IF NOT EXISTS idx_decisions_round_team_market_created ON decisions(round_id, team_id, market_id, created_at);
CREATE INDEX IF NOT EXISTS idx_orders_round_team_status_created ON orders(round_id, team_id, status, created_at);
CREATE INDEX IF NOT EXISTS idx_orders_market ON orders(market_id);
CREATE INDEX IF NOT EXISTS idx_fills_round_team_created ON fills(round_id, team_id, created_at);
CREATE INDEX IF NOT EXISTS idx_positions_round_team_market ON positions(round_id, team_id, market_id);
CREATE INDEX IF NOT EXISTS idx_risk_events_round_team_created ON risk_events(round_id, team_id, created_at);
CREATE INDEX IF NOT EXISTS idx_scores_round_team_created ON score_snapshots(round_id, team_id, created_at);
CREATE INDEX IF NOT EXISTS idx_heartbeats_round_team_created ON agent_heartbeats(round_id, team_id, created_at);
