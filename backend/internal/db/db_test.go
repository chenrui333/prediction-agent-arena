package db

import (
	"context"
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
