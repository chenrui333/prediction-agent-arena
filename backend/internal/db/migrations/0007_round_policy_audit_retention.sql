ALTER TABLE rounds ADD COLUMN require_locked_agents INTEGER NOT NULL DEFAULT 0;

CREATE INDEX IF NOT EXISTS idx_rounds_require_locked_agents ON rounds(require_locked_agents);
CREATE INDEX IF NOT EXISTS idx_api_requests_created_id ON api_requests(created_at, id);
