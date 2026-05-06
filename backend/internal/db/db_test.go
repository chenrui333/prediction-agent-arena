package db

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"testing"
)

func TestMigrateIdempotentAndPragmas(t *testing.T) {
	ctx := context.Background()
	conn, err := Open(ctx, t.TempDir()+"/arena.db")
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if err := Migrate(ctx, conn); err != nil {
		t.Fatal(err)
	}
	if err := Migrate(ctx, conn); err != nil {
		t.Fatalf("second migration failed: %v", err)
	}
	tests := []struct {
		pragma string
		want   int
	}{
		{pragma: "PRAGMA foreign_keys", want: 1},
		{pragma: "PRAGMA busy_timeout", want: 5000},
	}
	for _, tt := range tests {
		t.Run(tt.pragma, func(t *testing.T) {
			var got int
			if err := conn.QueryRowContext(ctx, tt.pragma).Scan(&got); err != nil {
				t.Fatal(err)
			}
			if got != tt.want {
				t.Fatalf("%s = %d, want %d", tt.pragma, got, tt.want)
			}
		})
	}
	var mode string
	if err := conn.QueryRowContext(ctx, "PRAGMA journal_mode").Scan(&mode); err != nil {
		t.Fatal(err)
	}
	if mode != "wal" {
		t.Fatalf("journal_mode = %q, want wal", mode)
	}
}

func TestMigrationsIncludeSecurityAndAccountingSchema(t *testing.T) {
	ctx := context.Background()
	conn, err := Open(ctx, t.TempDir()+"/arena.db")
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if err := Migrate(ctx, conn); err != nil {
		t.Fatal(err)
	}
	for _, table := range []string{"agents", "api_requests", "round_agents", "settlements", "admin_actions", "worker_heartbeats"} {
		t.Run(table, func(t *testing.T) {
			if !tableExists(t, conn, table) {
				t.Fatalf("missing table %s", table)
			}
		})
	}
	columns := map[string][]string{
		"positions":        {"avg_entry_price_bps", "realized_pnl_cents"},
		"agent_heartbeats": {"agent_id"},
		"decisions":        {"agent_id"},
		"orders":           {"agent_id"},
		"risk_events":      {"agent_id"},
		"api_requests":     {"team_id", "agent_id", "rate_limited", "ip_hash", "user_agent_hash"},
	}
	for table, names := range columns {
		t.Run(table+"_columns", func(t *testing.T) {
			for _, name := range names {
				if !columnExists(t, conn, table, name) {
					t.Fatalf("missing %s.%s", table, name)
				}
			}
		})
	}
	indexes := []string{
		"idx_agents_team_status",
		"idx_agents_token_hash",
		"idx_decisions_agent",
		"idx_orders_agent",
		"idx_risk_events_agent",
		"idx_heartbeats_agent",
		"idx_api_requests_created",
		"idx_api_requests_team_agent_created",
		"idx_api_requests_rate_limited_created",
		"idx_round_agents_round",
		"idx_round_agents_agent",
		"idx_round_agents_team",
	}
	for _, index := range indexes {
		t.Run(index, func(t *testing.T) {
			if !indexExists(t, conn, index) {
				t.Fatalf("missing index %s", index)
			}
		})
	}
}

func TestUpgradeFromPreAgentSchemaPreservesRows(t *testing.T) {
	ctx := context.Background()
	conn, err := Open(ctx, t.TempDir()+"/arena.db")
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if err := createSchemaMigrations(ctx, conn); err != nil {
		t.Fatal(err)
	}
	for _, version := range []string{
		"0001_init",
		"0002_simulated_markets",
		"0003_market_outcomes",
		"0004_accounting_operator_hardening",
	} {
		if err := applyNamedMigration(ctx, conn, version); err != nil {
			t.Fatal(err)
		}
	}
	now := "2026-05-06T00:00:00Z"
	if _, err := conn.ExecContext(ctx, "INSERT INTO teams(id, slug, name, api_token_hash, is_active, created_at, updated_at) VALUES (1, 'team-01', 'Team 01', 'hash-team', 1, ?, ?)", now, now); err != nil {
		t.Fatal(err)
	}
	if _, err := conn.ExecContext(ctx, "INSERT INTO rounds(id, slug, name, mode, status, initial_balance_cents, created_at, updated_at) VALUES (1, 'practice-1', 'Practice', 'practice', 'active', 1000000, ?, ?)", now, now); err != nil {
		t.Fatal(err)
	}
	if _, err := conn.ExecContext(ctx, "INSERT INTO markets(id, venue, external_id, slug, title, category, status, yes_price_bps, no_price_bps, metadata_json, created_at, updated_at) VALUES (1, 'fake', 'm1', 'm1', 'Market 1', 'bootcamp', 'open', 5000, 5000, '{}', ?, ?)", now, now); err != nil {
		t.Fatal(err)
	}
	if _, err := conn.ExecContext(ctx, "INSERT INTO orders(id, round_id, team_id, market_id, action, outcome, amount_cents, limit_price_bps, status, created_at, updated_at) VALUES (1, 1, 1, 1, 'buy', 'yes', 1000, 5000, 'open', ?, ?)", now, now); err != nil {
		t.Fatal(err)
	}
	for _, version := range []string{"0005_agents_audit_rate_limits", "0006_round_agent_locks"} {
		if err := applyNamedMigration(ctx, conn, version); err != nil {
			t.Fatal(err)
		}
	}
	var orderCount int
	if err := conn.QueryRowContext(ctx, "SELECT COUNT(*) FROM orders WHERE id = 1 AND agent_id IS NULL").Scan(&orderCount); err != nil {
		t.Fatal(err)
	}
	if orderCount != 1 {
		t.Fatalf("old order rows not preserved, count=%d", orderCount)
	}
	if _, err := conn.ExecContext(ctx, "INSERT INTO agents(id, team_id, slug, name, api_token_hash, status, kind, created_at, updated_at) VALUES (1, 1, 'default', 'Default', 'hash-agent', 'active', 'student', ?, ?)", now, now); err != nil {
		t.Fatal(err)
	}
	if _, err := conn.ExecContext(ctx, "UPDATE orders SET agent_id = 1 WHERE id = 1"); err != nil {
		t.Fatal(err)
	}
	if _, err := conn.ExecContext(ctx, "INSERT INTO api_requests(team_id, agent_id, method, path, status, rate_limited, ip_hash, user_agent_hash, created_at) VALUES (1, 1, 'POST', '/api/v1/orders', 201, 0, 'iphash', 'uahash', ?)", now); err != nil {
		t.Fatal(err)
	}
	if _, err := conn.ExecContext(ctx, "INSERT INTO round_agents(round_id, team_id, agent_id, commit_sha, docker_image, metadata_json, locked_by, created_at, updated_at) VALUES (1, 1, 1, 'abc123', 'agent:latest', '{}', 'test', ?, ?)", now, now); err != nil {
		t.Fatal(err)
	}
}

func tableExists(t *testing.T, conn *sql.DB, table string) bool {
	t.Helper()
	var name string
	err := conn.QueryRowContext(context.Background(), "SELECT name FROM sqlite_master WHERE type = 'table' AND name = ?", table).Scan(&name)
	return err == nil
}

func columnExists(t *testing.T, conn *sql.DB, table, column string) bool {
	t.Helper()
	rows, err := conn.QueryContext(context.Background(), "PRAGMA table_info("+table+")")
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	for rows.Next() {
		var cid int
		var name, typ string
		var notNull int
		var defaultValue sql.NullString
		var pk int
		if err := rows.Scan(&cid, &name, &typ, &notNull, &defaultValue, &pk); err != nil {
			t.Fatal(err)
		}
		if name == column {
			return true
		}
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return false
}

func indexExists(t *testing.T, conn *sql.DB, index string) bool {
	t.Helper()
	var name string
	err := conn.QueryRowContext(context.Background(), "SELECT name FROM sqlite_master WHERE type = 'index' AND name = ?", index).Scan(&name)
	return err == nil
}

func createSchemaMigrations(ctx context.Context, conn *sql.DB) error {
	_, err := conn.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version TEXT PRIMARY KEY,
			applied_at TEXT NOT NULL
		)
	`)
	return err
}

func applyNamedMigration(ctx context.Context, conn *sql.DB, version string) error {
	content, err := migrationFS.ReadFile("migrations/" + version + ".sql")
	if err != nil {
		return err
	}
	if err := applyMigration(ctx, conn, version, string(content)); err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint failed: schema_migrations.version") {
			return fmt.Errorf("migration %s was already applied: %w", version, err)
		}
		return err
	}
	return nil
}
