package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sync"
)

const schemaVersion = "cortexos.staff.sqlite.v1"

var ErrUnknownSchema = errors.New("unknown staff sqlite schema")

// migrationMutex protects migration execution within this process.
// Cross-connection and cross-process safety are ensured by SQLite LevelSerializable
// transactions, busy timeout, and the staff_schema version table.
var migrationMutex sync.Mutex

var migrations = []string{
	`CREATE TABLE staff_definitions (
		id TEXT PRIMARY KEY NOT NULL,
		name TEXT NOT NULL,
		role TEXT NOT NULL,
		workspace_id TEXT NOT NULL,
		project_id TEXT NOT NULL DEFAULT '',
		worktree_id TEXT NOT NULL DEFAULT '',
		lifecycle TEXT NOT NULL,
		availability TEXT NOT NULL,
		created_at TEXT NOT NULL,
		updated_at TEXT NOT NULL,
		schema_version TEXT NOT NULL
	);`,
	`CREATE INDEX staff_definitions_workspace_idx ON staff_definitions(workspace_id, id);`,
	`CREATE INDEX staff_definitions_project_idx ON staff_definitions(project_id, id);`,
	`CREATE INDEX staff_definitions_role_idx ON staff_definitions(role, id);`,
	`CREATE INDEX staff_definitions_lifecycle_idx ON staff_definitions(lifecycle, id);`,
	`CREATE INDEX staff_definitions_availability_idx ON staff_definitions(availability, id);`,
	`CREATE TABLE staff_capabilities (
		staff_id TEXT NOT NULL,
		position INTEGER NOT NULL,
		capability TEXT NOT NULL,
		PRIMARY KEY (staff_id, position),
		UNIQUE (staff_id, capability),
		FOREIGN KEY (staff_id) REFERENCES staff_definitions(id) ON DELETE CASCADE
	);`,
	`CREATE TABLE staff_permissions (
		staff_id TEXT NOT NULL,
		position INTEGER NOT NULL,
		permission_id TEXT NOT NULL,
		action TEXT NOT NULL,
		resource TEXT NOT NULL,
		effect TEXT NOT NULL,
		priority INTEGER NOT NULL,
		PRIMARY KEY (staff_id, position),
		UNIQUE (staff_id, permission_id),
		FOREIGN KEY (staff_id) REFERENCES staff_definitions(id) ON DELETE CASCADE
	);`,
	`CREATE TABLE staff_skills (
		staff_id TEXT NOT NULL,
		position INTEGER NOT NULL,
		skill_id TEXT NOT NULL,
		skill_version TEXT NOT NULL,
		PRIMARY KEY (staff_id, position),
		UNIQUE (staff_id, skill_id),
		FOREIGN KEY (staff_id) REFERENCES staff_definitions(id) ON DELETE CASCADE
	);`,
	`CREATE TABLE staff_memory (
		staff_id TEXT NOT NULL,
		position INTEGER NOT NULL,
		memory_id TEXT NOT NULL,
		memory_kind TEXT NOT NULL,
		memory_version TEXT NOT NULL,
		PRIMARY KEY (staff_id, position),
		UNIQUE (staff_id, memory_id),
		FOREIGN KEY (staff_id) REFERENCES staff_definitions(id) ON DELETE CASCADE
	);`,
}

// SchemaVersion returns the durable Staff SQLite contract version.
func SchemaVersion() string {
	return schemaVersion
}

// ApplyMigrations applies all pending Staff SQLite migrations idempotently.
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
	if _, err := db.ExecContext(ctx, `PRAGMA foreign_keys = ON;`); err != nil {
		return fmt.Errorf("enable staff foreign keys: %w", err)
	}
	if _, err := db.ExecContext(ctx, `PRAGMA busy_timeout = 5000;`); err != nil {
		return fmt.Errorf("configure staff busy timeout: %w", err)
	}
	if _, err := db.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS staff_schema (
		version INTEGER PRIMARY KEY NOT NULL,
		name TEXT NOT NULL,
		applied_at TEXT NOT NULL
	);`); err != nil {
		return fmt.Errorf("create staff schema metadata: %w", err)
	}

	rows, err := db.QueryContext(ctx, `SELECT version, name FROM staff_schema ORDER BY version ASC`)
	if err != nil {
		return fmt.Errorf("read staff schema metadata: %w", err)
	}
	defer rows.Close()

	applied := 0
	for rows.Next() {
		var version int
		var name string
		if err := rows.Scan(&version, &name); err != nil {
			return fmt.Errorf("decode staff schema metadata: %w", err)
		}
		expectedVersion := applied + 1
		expectedName := fmt.Sprintf("staff-%d", expectedVersion)
		if version != expectedVersion || name != expectedName || version > len(migrations) {
			return fmt.Errorf("%w: found migration version %d (%q), expected %d (%q)", ErrUnknownSchema, version, name, expectedVersion, expectedName)
		}
		applied++
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate staff schema metadata: %w", err)
	}
	if err := rows.Close(); err != nil {
		return fmt.Errorf("close staff schema metadata: %w", err)
	}

	for version := applied + 1; version <= len(migrations); version++ {
		if err := applyOneMigration(ctx, db, version, migrations[version-1]); err != nil {
			return err
		}
	}
	return nil
}

func applyOneMigration(ctx context.Context, db *sql.DB, version int, statement string) error {
	tx, err := db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		return fmt.Errorf("begin staff migration %d: %w", version, err)
	}
	defer func() { _ = tx.Rollback() }()

	var existingCount int
	err = tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM staff_schema WHERE version = ?`, version).Scan(&existingCount)
	if err == nil && existingCount > 0 {
		return nil
	}

	if _, err := tx.ExecContext(ctx, statement); err != nil {
		return fmt.Errorf("apply staff migration %d: %w", version, err)
	}
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO staff_schema(version, name, applied_at) VALUES (?, ?, CURRENT_TIMESTAMP)`,
		version, fmt.Sprintf("staff-%d", version)); err != nil {
		return fmt.Errorf("record staff migration %d: %w", version, err)
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit staff migration %d: %w", version, err)
	}
	return nil
}
