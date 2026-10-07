package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sync"
)

const schemaVersion = "cortexos.orchestra.sqlite.v1"

var ErrUnknownSchema = errors.New("unknown orchestra sqlite schema")

var migrationMutex sync.Mutex

var migrations = []string{
	`CREATE TABLE orchestra_tasks (
		id TEXT PRIMARY KEY NOT NULL,
		project_id TEXT NOT NULL,
		worktree_id TEXT NOT NULL,
		title TEXT NOT NULL,
		acceptance_criteria TEXT NOT NULL,
		dependencies TEXT NOT NULL,
		status TEXT NOT NULL,
		attempt_count INTEGER NOT NULL DEFAULT 0,
		max_attempts INTEGER NOT NULL,
		created_at TEXT NOT NULL,
		updated_at TEXT NOT NULL,
		schema_version TEXT NOT NULL
	);`,
	`CREATE INDEX orchestra_tasks_status_idx ON orchestra_tasks(status, id);`,
	`CREATE TABLE orchestra_executions (
		id TEXT PRIMARY KEY NOT NULL,
		task_id TEXT NOT NULL,
		attempt INTEGER NOT NULL,
		status TEXT NOT NULL,
		started_at TEXT NOT NULL,
		finished_at TEXT,
		evidence_ids TEXT NOT NULL,
		FOREIGN KEY(task_id) REFERENCES orchestra_tasks(id)
	);`,
	`CREATE INDEX orchestra_executions_task_idx ON orchestra_executions(task_id, attempt);`,
	`CREATE TABLE orchestra_events (
		id TEXT PRIMARY KEY NOT NULL,
		task_id TEXT NOT NULL,
		execution_id TEXT,
		sequence INTEGER NOT NULL,
		type TEXT NOT NULL,
		from_status TEXT NOT NULL,
		to_status TEXT NOT NULL,
		message TEXT NOT NULL,
		evidence_ids TEXT NOT NULL,
		occurred_at TEXT NOT NULL,
		UNIQUE(task_id, sequence),
		FOREIGN KEY(task_id) REFERENCES orchestra_tasks(id),
		FOREIGN KEY(execution_id) REFERENCES orchestra_executions(id)
	);`,
	`CREATE INDEX orchestra_events_task_idx ON orchestra_events(task_id, sequence);`,
	`CREATE TABLE orchestra_active_tasks (
		task_id TEXT PRIMARY KEY NOT NULL,
		execution_id TEXT NOT NULL,
		worker_id TEXT NOT NULL,
		started_at TEXT NOT NULL,
		heartbeat_at TEXT NOT NULL,
		FOREIGN KEY(task_id) REFERENCES orchestra_tasks(id),
		FOREIGN KEY(execution_id) REFERENCES orchestra_executions(id)
	);`,
	`CREATE INDEX orchestra_active_tasks_heartbeat_idx ON orchestra_active_tasks(heartbeat_at);`,
}

func SchemaVersion() string { return schemaVersion }

func ApplyMigrations(ctx context.Context, db *sql.DB) error {
	migrationMutex.Lock()
	defer migrationMutex.Unlock()
	if ctx == nil {
		return fmt.Errorf("%w: context is required", ErrUnknownSchema)
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if db == nil {
		return fmt.Errorf("%w: database is nil", ErrUnknownSchema)
	}
	if _, err := db.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS orchestra_schema (
		version INTEGER PRIMARY KEY NOT NULL,
		name TEXT NOT NULL,
		applied_at TEXT NOT NULL
	)`); err != nil {
		return fmt.Errorf("create orchestra schema metadata: %w", err)
	}
	rows, err := db.QueryContext(ctx, `SELECT version, name FROM orchestra_schema ORDER BY version ASC`)
	if err != nil {
		return fmt.Errorf("read orchestra schema metadata: %w", err)
	}
	defer rows.Close()
	applied := 0
	for rows.Next() {
		var version int
		var name string
		if err := rows.Scan(&version, &name); err != nil {
			return fmt.Errorf("decode orchestra schema metadata: %w", err)
		}
		expectedVersion := applied + 1
		expectedName := fmt.Sprintf("orchestra-%d", expectedVersion)
		if version != expectedVersion || name != expectedName || version > len(migrations) {
			return fmt.Errorf("%w: found migration version %d (%q), expected %d (%q)", ErrUnknownSchema, version, name, expectedVersion, expectedName)
		}
		applied++
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate orchestra schema metadata: %w", err)
	}
	if err := rows.Close(); err != nil {
		return fmt.Errorf("close orchestra schema metadata: %w", err)
	}
	for index := applied; index < len(migrations); index++ {
		if err := applyOne(ctx, db, index+1, migrations[index]); err != nil {
			return err
		}
	}
	return nil
}

func applyOne(ctx context.Context, db *sql.DB, version int, statement string) error {
	tx, err := db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		return fmt.Errorf("begin orchestra migration %d: %w", version, err)
	}
	defer func() { _ = tx.Rollback() }()

	var existingCount int
	err = tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM orchestra_schema WHERE version = ?`, version).Scan(&existingCount)
	if err == nil && existingCount > 0 {
		return nil
	}

	if _, err := tx.ExecContext(ctx, statement); err != nil {
		return fmt.Errorf("apply orchestra migration %d: %w", version, err)
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO orchestra_schema(version, name, applied_at) VALUES (?, ?, CURRENT_TIMESTAMP)`, version, fmt.Sprintf("orchestra-%d", version)); err != nil {
		return fmt.Errorf("record orchestra migration %d: %w", version, err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit orchestra migration %d: %w", version, err)
	}
	return nil
}
