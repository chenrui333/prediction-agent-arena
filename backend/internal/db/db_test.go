package db

import (
	"context"
	"database/sql"
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
	for _, table := range []string{"agents", "api_requests", "settlements", "admin_actions", "worker_heartbeats"} {
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
