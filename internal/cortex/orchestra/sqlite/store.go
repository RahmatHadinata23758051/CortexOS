package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

// Store owns the durable Orchestra SQLite connection. It is intentionally
// independent from Workspace's database so Orchestra can evolve its schema
// without leaking persistence details into the domain package.
type Store struct {
	db   *sql.DB
	path string
}

func Open(ctx context.Context, path string) (*Store, error) {
	if ctx == nil {
		return nil, fmt.Errorf("open orchestra database: context is required")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if path == "" || !filepath.IsAbs(path) {
		return nil, fmt.Errorf("open orchestra database: path must be absolute")
	}
	cleanPath, err := filepath.Abs(filepath.Clean(path))
	if err != nil {
		return nil, fmt.Errorf("open orchestra database: canonicalize path: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(cleanPath), 0o700); err != nil {
		return nil, fmt.Errorf("open orchestra database: create parent directory: %w", err)
	}
	db, err := sql.Open("sqlite", cleanPath)
	if err != nil {
		return nil, fmt.Errorf("open orchestra database: %w", err)
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	store := &Store{db: db, path: cleanPath}
	if _, err := db.ExecContext(ctx, `PRAGMA foreign_keys = ON`); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("open orchestra database: enable foreign keys: %w", err)
	}
	if err := ApplyMigrations(ctx, db); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("open orchestra database: %w", err)
	}
	return store, nil
}

func (s *Store) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}

// OpenRawForCrashSimulation opens a second raw connection to the same database
// file. Crash-recovery tests use it to begin a transaction, mutate rows, then
// abandon the transaction without commit — emulating SIGKILL mid-transaction.
// Production code must never use this; all durable writes go through Store.
func OpenRawForCrashSimulation(ctx context.Context, path string) (*sql.DB, error) {
	if ctx == nil {
		return nil, fmt.Errorf("open raw orchestra database: context is required")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if path == "" || !filepath.IsAbs(path) {
		return nil, fmt.Errorf("open raw orchestra database: path must be absolute")
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open raw orchestra database: %w", err)
	}
	if _, err := db.ExecContext(ctx, `PRAGMA foreign_keys = ON`); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("open raw orchestra database: enable foreign keys: %w", err)
	}
	return db, nil
}

// CheckIntegrity runs PRAGMA integrity_check and fails on any corruption report.
// Reopened stores call this after a simulated crash to prove the file survived
// an abandoned transaction without torn pages.
func (s *Store) CheckIntegrity(ctx context.Context) error {
	if s == nil || s.db == nil {
		return fmt.Errorf("check orchestra database integrity: store unavailable")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	rows, err := s.db.QueryContext(ctx, `PRAGMA integrity_check`)
	if err != nil {
		return fmt.Errorf("check orchestra database integrity: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var msg string
		if err := rows.Scan(&msg); err != nil {
			return fmt.Errorf("decode integrity result: %w", err)
		}
		if msg != "ok" {
			return fmt.Errorf("orchestra database integrity check failed: %s", msg)
		}
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate integrity result: %w", err)
	}
	return nil
}
