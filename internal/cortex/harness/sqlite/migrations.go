package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sync"
)

const schemaVersion = "cortexos.harness.sqlite.v1"

var ErrUnknownSchema = errors.New("unknown harness sqlite schema")

var migrationMutex sync.Mutex

var migrations = []string{
	// Policy metadata table
	`CREATE TABLE harness_policies (
		id TEXT PRIMARY KEY NOT NULL,
		name TEXT NOT NULL,
		contract_version TEXT NOT NULL,
		rules_json TEXT NOT NULL,
		created_at TEXT NOT NULL,
		updated_at TEXT NOT NULL,
		active INTEGER NOT NULL DEFAULT 0
	);`,

	// Tool definitions and capabilities
	`CREATE TABLE harness_tools (
		id TEXT PRIMARY KEY NOT NULL,
		name TEXT NOT NULL UNIQUE,
		kind TEXT NOT NULL,
		description TEXT NOT NULL,
		capabilities_json TEXT NOT NULL,
		input_schema_json TEXT NOT NULL,
		output_schema_json TEXT NOT NULL,
		timeout_ms INTEGER NOT NULL,
		requires_ask INTEGER NOT NULL DEFAULT 0,
		version TEXT NOT NULL,
		registered_at TEXT NOT NULL,
		updated_at TEXT NOT NULL,
		active INTEGER NOT NULL DEFAULT 1
	);`,

	// Capability bindings with policy defaults
	`CREATE TABLE harness_capabilities (
		capability TEXT PRIMARY KEY NOT NULL,
		description TEXT NOT NULL,
		requires_ask INTEGER NOT NULL DEFAULT 0,
		default_effect TEXT NOT NULL,
		registered_at TEXT NOT NULL,
		updated_at TEXT NOT NULL
	);`,

	// Process/worker registrations with ownership
	`CREATE TABLE harness_workers (
		id TEXT PRIMARY KEY NOT NULL,
		instance_id TEXT NOT NULL UNIQUE,
		kind TEXT NOT NULL,
		class TEXT NOT NULL,
		version TEXT NOT NULL,
		schema_version TEXT NOT NULL,
		owner_pid INTEGER NOT NULL,
		owner_process_id TEXT NOT NULL,
		registered_at TEXT NOT NULL,
		last_heartbeat_at TEXT NOT NULL,
		status TEXT NOT NULL DEFAULT 'healthy',
		metadata_json TEXT NOT NULL DEFAULT '{}'
	);`,

	`CREATE INDEX harness_workers_heartbeat_idx ON harness_workers(last_heartbeat_at);`,

	// Active process reservations (governor leases)
	`CREATE TABLE harness_reservations (
		id TEXT PRIMARY KEY NOT NULL,
		task_id TEXT NOT NULL,
		engine_class TEXT NOT NULL,
		priority INTEGER NOT NULL,
		memory_mb INTEGER NOT NULL,
		cpu_priority INTEGER NOT NULL,
		trace_id TEXT NOT NULL,
		owner_worker_id TEXT NOT NULL,
		owner_process_id TEXT NOT NULL,
		created_at TEXT NOT NULL,
		released_at TEXT,
		active INTEGER NOT NULL DEFAULT 1,
		FOREIGN KEY(owner_worker_id) REFERENCES harness_workers(id)
	);`,

	`CREATE INDEX harness_reservations_task_idx ON harness_reservations(task_id);`,
	`CREATE INDEX harness_reservations_worker_idx ON harness_reservations(owner_worker_id, active);`,

	// Resource heartbeats for active workers
	`CREATE TABLE harness_heartbeats (
		id TEXT PRIMARY KEY NOT NULL,
		worker_id TEXT NOT NULL,
		process_id TEXT NOT NULL,
		healthy INTEGER NOT NULL,
		memory_mb INTEGER NOT NULL,
		cpu_priority INTEGER NOT NULL,
		at TEXT NOT NULL,
		details_json TEXT NOT NULL DEFAULT '{}',
		FOREIGN KEY(worker_id) REFERENCES harness_workers(id)
	);`,

	`CREATE INDEX harness_heartbeats_worker_idx ON harness_heartbeats(worker_id, at DESC);`,

	// Redacted execution evidence references (not raw output)
	`CREATE TABLE harness_evidence (
		id TEXT PRIMARY KEY NOT NULL,
		contract_version TEXT NOT NULL,
		execution_id TEXT NOT NULL,
		task_id TEXT NOT NULL,
		worktree_id TEXT NOT NULL,
		kind TEXT NOT NULL,
		digest TEXT NOT NULL,
		redacted_payload_json TEXT NOT NULL,
		audit_json TEXT NOT NULL DEFAULT '{}',
		collected_at TEXT NOT NULL,
		schema_version TEXT NOT NULL
	);`,

	`CREATE INDEX harness_evidence_execution_idx ON harness_evidence(execution_id);`,
	`CREATE INDEX harness_evidence_task_idx ON harness_evidence(task_id);`,
	`CREATE INDEX harness_evidence_digest_idx ON harness_evidence(digest);`,

	// Approvals for policy "ask" decisions
	`CREATE TABLE harness_approvals (
		id TEXT PRIMARY KEY NOT NULL,
		execution_id TEXT NOT NULL UNIQUE,
		tool_name TEXT,
		action TEXT,
		approver TEXT NOT NULL,
		reason TEXT,
		granted_at TEXT NOT NULL
	);`,

	// Audit log entries for broker operations
	`CREATE TABLE harness_audit_log (
		id TEXT PRIMARY KEY NOT NULL,
		operation TEXT NOT NULL,
		actor TEXT NOT NULL,
		details_json TEXT NOT NULL DEFAULT '{}',
		occurred_at TEXT NOT NULL
	);`,

	`CREATE INDEX harness_audit_log_actor_idx ON harness_audit_log(actor, occurred_at DESC);`,
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
	if _, err := db.ExecContext(ctx, `PRAGMA foreign_keys = ON`); err != nil {
		return fmt.Errorf("enable harness foreign keys: %w", err)
	}

	if _, err := db.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS harness_schema (
		version INTEGER PRIMARY KEY NOT NULL,
		name TEXT NOT NULL,
		applied_at TEXT NOT NULL
	)`); err != nil {
		return fmt.Errorf("create harness schema metadata: %w", err)
	}

	rows, err := db.QueryContext(ctx, `SELECT version, name FROM harness_schema ORDER BY version ASC`)
	if err != nil {
		return fmt.Errorf("read harness schema metadata: %w", err)
	}
	defer rows.Close()

	applied := 0
	for rows.Next() {
		var version int
		var name string
		if err := rows.Scan(&version, &name); err != nil {
			return fmt.Errorf("decode harness schema metadata: %w", err)
		}
		expectedVersion := applied + 1
		expectedName := fmt.Sprintf("harness-%d", expectedVersion)
		if version != expectedVersion || name != expectedName || version > len(migrations) {
			return fmt.Errorf("%w: found migration version %d (%q), expected %d (%q)", ErrUnknownSchema, version, name, expectedVersion, expectedName)
		}
		applied++
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate harness schema metadata: %w", err)
	}
	if err := rows.Close(); err != nil {
		return fmt.Errorf("close harness schema metadata: %w", err)
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
		return fmt.Errorf("begin harness migration %d: %w", version, err)
	}
	defer func() { _ = tx.Rollback() }()

	var existingCount int
	err = tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM harness_schema WHERE version = ?`, version).Scan(&existingCount)
	if err == nil && existingCount > 0 {
		return nil
	}

	if _, err := tx.ExecContext(ctx, statement); err != nil {
		return fmt.Errorf("apply harness migration %d: %w", version, err)
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO harness_schema(version, name, applied_at) VALUES (?, ?, CURRENT_TIMESTAMP)`, version, fmt.Sprintf("harness-%d", version)); err != nil {
		return fmt.Errorf("record harness migration %d: %w", version, err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit harness migration %d: %w", version, err)
	}
	return nil
}
