CREATE TABLE IF NOT EXISTS market_outcomes (
	market_id INTEGER PRIMARY KEY REFERENCES markets(id) ON DELETE CASCADE,
	outcome TEXT NOT NULL CHECK (outcome IN ('yes', 'no', 'unknown')),
	resolved_at TEXT,
	resolved_by TEXT NOT NULL DEFAULT '',
	created_at TEXT NOT NULL,
	updated_at TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_market_outcomes_outcome_resolved ON market_outcomes(outcome, resolved_at);
