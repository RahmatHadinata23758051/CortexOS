package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

const schemaVersion = "workspace.sqlite.v1"

var ErrUnknownSchema = errors.New("unknown workspace sqlite schema")

var migrations = []string{
	`CREATE TABLE projects (
		id TEXT PRIMARY KEY NOT NULL,
		name TEXT NOT NULL,
		repository_root TEXT NOT NULL,
		vault_root TEXT NOT NULL,
		default_branch TEXT NOT NULL DEFAULT '',
		status TEXT NOT NULL,
		created_at TEXT NOT NULL,
		updated_at TEXT NOT NULL,
		schema_version TEXT NOT NULL
	);`,
	`CREATE INDEX projects_status_idx ON projects(status);`,
	`CREATE TABLE vault_notes (
		id TEXT PRIMARY KEY NOT NULL,
		project_id TEXT NOT NULL,
		worktree_id TEXT NOT NULL DEFAULT '',
		relative_path TEXT NOT NULL,
		title TEXT NOT NULL,
		format_version TEXT NOT NULL,
		source TEXT NOT NULL,
		author TEXT NOT NULL,
		created_at TEXT NOT NULL,
		updated_at TEXT NOT NULL,
		content_hash TEXT NOT NULL,
		status TEXT NOT NULL,
		UNIQUE(project_id, relative_path),
		FOREIGN KEY(project_id) REFERENCES projects(id)
	);`,
	`CREATE INDEX vault_notes_project_idx ON vault_notes(project_id, relative_path);`,
}

func SchemaVersion() string { return schemaVersion }

// ApplyMigrations creates the schema metadata and applies each migration in an
// individual transaction. The caller owns driver registration and connection
// lifecycle; this package only depends on database/sql.
func ApplyMigrations(ctx context.Context, db *sql.DB) error {
	if ctx == nil {
		return fmt.Errorf("%w: context is required", ErrUnknownSchema)
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if db == nil {
		return fmt.Errorf("%w: database is nil", ErrUnknownSchema)
	}
	if _, err := db.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS workspace_schema (
		version INTEGER PRIMARY KEY NOT NULL,
		name TEXT NOT NULL,
		applied_at TEXT NOT NULL
	)`); err != nil {
		return fmt.Errorf("create workspace schema metadata: %w", err)
	}

	rows, err := db.QueryContext(ctx, `SELECT version, name FROM workspace_schema ORDER BY version ASC`)
	if err != nil {
		return fmt.Errorf("read workspace schema metadata: %w", err)
	}
	defer rows.Close()

	applied := 0
	for rows.Next() {
		var version int
		var name string
		if err := rows.Scan(&version, &name); err != nil {
			return fmt.Errorf("decode workspace schema metadata: %w", err)
		}
		expectedVersion := applied + 1
		expectedName := fmt.Sprintf("workspace-%d", expectedVersion)
		if version != expectedVersion || name != expectedName || version > len(migrations) {
			return fmt.Errorf("%w: found migration version %d (%q), expected %d (%q)", ErrUnknownSchema, version, name, expectedVersion, expectedName)
		}
		applied++
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate workspace schema metadata: %w", err)
	}
	for index := applied; index < len(migrations); index++ {
		if err := applyOne(ctx, db, index+1, migrations[index]); err != nil {
			return err
		}
	}
	return nil
}

func applyOne(ctx context.Context, db *sql.DB, version int, statement string) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin workspace migration %d: %w", version, err)
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.ExecContext(ctx, statement); err != nil {
		return fmt.Errorf("apply workspace migration %d: %w", version, err)
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO workspace_schema(version, name, applied_at) VALUES (?, ?, CURRENT_TIMESTAMP)`, version, fmt.Sprintf("workspace-%d", version)); err != nil {
		return fmt.Errorf("record workspace migration %d: %w", version, err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit workspace migration %d: %w", version, err)
	}
	return nil
}
