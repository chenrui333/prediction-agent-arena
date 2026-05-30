ALTER TABLE orders ADD COLUMN client_order_id TEXT NOT NULL DEFAULT '';
ALTER TABLE orders ADD COLUMN request_hash TEXT NOT NULL DEFAULT '';
ALTER TABLE orders ADD COLUMN dispatched_at TEXT NOT NULL DEFAULT '';
ALTER TABLE orders ADD COLUMN decision_id INTEGER REFERENCES decisions(id) ON DELETE SET NULL;

CREATE UNIQUE INDEX IF NOT EXISTS idx_orders_idempotency
ON orders(round_id, team_id, COALESCE(agent_id, 0), client_order_id)
WHERE client_order_id != '';

CREATE INDEX IF NOT EXISTS idx_orders_decision ON orders(decision_id);
