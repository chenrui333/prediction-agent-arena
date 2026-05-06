package db

import (
	"context"
	"database/sql"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

//go:embed migrations/*.sql
var migrationFS embed.FS

func Open(ctx context.Context, path string) (*sql.DB, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	dsn := fmt.Sprintf("file:%s?cache=shared", path)
	conn, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	conn.SetMaxOpenConns(1)
	conn.SetMaxIdleConns(1)
	conn.SetConnMaxLifetime(time.Hour)

	pragmas := []string{
		"PRAGMA journal_mode=WAL",
		"PRAGMA foreign_keys=ON",
		"PRAGMA busy_timeout=5000",
		"PRAGMA synchronous=NORMAL",
	}
	for _, pragma := range pragmas {
		if _, err := conn.ExecContext(ctx, pragma); err != nil {
			_ = conn.Close()
			return nil, fmt.Errorf("%s: %w", pragma, err)
		}
	}
	if err := conn.PingContext(ctx); err != nil {
		_ = conn.Close()
		return nil, err
	}
	return conn, nil
}

func Migrate(ctx context.Context, conn *sql.DB) error {
	if _, err := conn.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version TEXT PRIMARY KEY,
			applied_at TEXT NOT NULL
		)
	`); err != nil {
		return err
	}

	entries, err := fs.ReadDir(migrationFS, "migrations")
	if err != nil {
		return err
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".sql") {
			continue
		}
		version := strings.TrimSuffix(entry.Name(), ".sql")
		applied, err := migrationApplied(ctx, conn, version)
		if err != nil {
			return err
		}
		if applied {
			continue
		}
		content, err := migrationFS.ReadFile("migrations/" + entry.Name())
		if err != nil {
			return err
		}
		if err := applyMigration(ctx, conn, version, string(content)); err != nil {
			return err
		}
	}
	return nil
}

func migrationApplied(ctx context.Context, conn *sql.DB, version string) (bool, error) {
	var existing string
	err := conn.QueryRowContext(ctx, "SELECT version FROM schema_migrations WHERE version = ?", version).Scan(&existing)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	return err == nil, err
}

func applyMigration(ctx context.Context, conn *sql.DB, version, content string) error {
	tx, err := conn.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()
	if _, err = tx.ExecContext(ctx, content); err != nil {
		return fmt.Errorf("migration %s: %w", version, err)
	}
	if _, err = tx.ExecContext(ctx, "INSERT INTO schema_migrations(version, applied_at) VALUES (?, ?)", version, time.Now().UTC().Format(time.RFC3339Nano)); err != nil {
		return err
	}
	return tx.Commit()
}
