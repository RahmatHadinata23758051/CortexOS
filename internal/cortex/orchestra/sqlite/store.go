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
