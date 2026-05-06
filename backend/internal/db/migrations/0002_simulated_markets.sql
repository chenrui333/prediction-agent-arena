CREATE TABLE IF NOT EXISTS simulated_market_states (
	market_id INTEGER PRIMARY KEY REFERENCES markets(id) ON DELETE CASCADE,
	true_probability_bps INTEGER,
	current_tick INTEGER NOT NULL DEFAULT 0,
	price_path_json TEXT NOT NULL,
	final_outcome TEXT NOT NULL DEFAULT 'unknown' CHECK (final_outcome IN ('yes', 'no', 'unknown')),
	resolved_at TEXT,
	resolved_by TEXT NOT NULL DEFAULT '',
	updated_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS market_price_ticks (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	market_id INTEGER NOT NULL REFERENCES markets(id) ON DELETE CASCADE,
	tick INTEGER NOT NULL,
	yes_price_bps INTEGER NOT NULL,
	no_price_bps INTEGER NOT NULL,
	source TEXT NOT NULL,
	created_at TEXT NOT NULL,
	UNIQUE (market_id, tick)
);

CREATE INDEX IF NOT EXISTS idx_simulated_market_states_resolved ON simulated_market_states(resolved_at);
CREATE INDEX IF NOT EXISTS idx_market_price_ticks_market_created ON market_price_ticks(market_id, created_at);
