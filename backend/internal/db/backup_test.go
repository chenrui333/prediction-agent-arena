package db

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite"
)

func TestBackupCreatesReadableSQLiteDatabase(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	source := filepath.Join(dir, "arena.db")
	conn, err := Open(ctx, source)
	if err != nil {
		t.Fatal(err)
	}
	if err := Migrate(ctx, conn); err != nil {
		t.Fatal(err)
	}
	if _, err := conn.ExecContext(ctx, "INSERT INTO teams(slug, name, api_token_hash, created_at, updated_at) VALUES ('team-a', 'Team A', 'hash-a', 'now', 'now')"); err != nil {
		t.Fatal(err)
	}
	if err := conn.Close(); err != nil {
		t.Fatal(err)
	}

	dest := filepath.Join(dir, "backup.db")
	if err := Backup(ctx, source, dest); err != nil {
		t.Fatal(err)
	}
	backup, err := sql.Open("sqlite", "file:"+dest)
	if err != nil {
		t.Fatal(err)
	}
	defer backup.Close()
	var count int
	if err := backup.QueryRowContext(ctx, "SELECT COUNT(*) FROM teams").Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("teams = %d, want 1", count)
	}
}
