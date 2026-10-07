package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

func newTestDB(t *testing.T) *sql.DB {
	t.Helper()
	path := fmt.Sprintf("%s/harness_test_%d.db", os.TempDir(), time.Now().UnixNano())
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close(); _ = os.Remove(path) })
	return db
}

func TestApplyMigrationsIdempotent(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()

	if err := ApplyMigrations(ctx, db); err != nil {
		t.Fatalf("first apply: %v", err)
	}
	if err := ApplyMigrations(ctx, db); err != nil {
		t.Fatalf("second apply (idempotent): %v", err)
	}

	rows, err := db.QueryContext(ctx, `SELECT version, name FROM harness_schema ORDER BY version`)
	if err != nil {
		t.Fatalf("query schema: %v", err)
	}
	defer rows.Close()

	applied := 0
	for rows.Next() {
		var v int
		var n string
		if err := rows.Scan(&v, &n); err != nil {
			t.Fatalf("scan: %v", err)
		}
		applied++
		if v != applied || n != fmt.Sprintf("harness-%d", v) {
			t.Errorf("unexpected migration row: version=%d name=%q, want %d harness-%d", v, n, applied, applied)
		}
	}
	if applied != len(migrations) {
		t.Errorf("expected %d migrations, got %d", len(migrations), applied)
	}
}

func TestForeignKeysEnabled(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	if _, err := db.ExecContext(ctx, `PRAGMA foreign_keys = ON`); err != nil {
		t.Fatalf("enable foreign keys: %v", err)
	}
	if err := ApplyMigrations(ctx, db); err != nil {
		t.Fatalf("apply: %v", err)
	}

	// Insert a worker
	_, err := db.ExecContext(ctx, `INSERT INTO harness_workers (id, instance_id, kind, class, version, schema_version, owner_pid, owner_process_id, registered_at, last_heartbeat_at, status) VALUES ('w1', 'inst1', 'pi', 'pi', '1.0', 'v1', 123, 'proc1', '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z', 'healthy')`)
	if err != nil {
		t.Fatalf("insert worker: %v", err)
	}

	// FK violation on reservation: owner_worker_id must exist
	_, err = db.ExecContext(ctx, `INSERT INTO harness_reservations (id, task_id, engine_class, priority, memory_mb, cpu_priority, trace_id, owner_worker_id, owner_process_id, created_at, active) VALUES ('r1', 'task1', 'pi', 50, 200, 1, 'trace', 'nonexistent', 'proc1', '2026-01-01T00:00:00Z', 1)`)
	if err == nil {
		t.Fatal("expected FK violation on nonexistent worker")
	}

	// FK violation on heartbeat
	_, err = db.ExecContext(ctx, `INSERT INTO harness_heartbeats (id, worker_id, process_id, healthy, memory_mb, cpu_priority, at) VALUES ('h1', 'nonexistent', 'proc1', 1, 100, 1, '2026-01-01T00:00:00Z')`)
	if err == nil {
		t.Fatal("expected FK violation on heartbeat for nonexistent worker")
	}

	// Valid FK should succeed
	_, err = db.ExecContext(ctx, `INSERT INTO harness_reservations (id, task_id, engine_class, priority, memory_mb, cpu_priority, trace_id, owner_worker_id, owner_process_id, created_at, active) VALUES ('r1', 'task1', 'pi', 50, 200, 1, 'trace', 'w1', 'proc1', '2026-01-01T00:00:00Z', 1)`)
	if err != nil {
		t.Fatalf("valid FK failed: %v", err)
	}
}

func TestApplyMigrationsRejectsNonContiguousMetadata(t *testing.T) {
	db := newTestDB(t)
	if _, err := db.Exec(`CREATE TABLE harness_schema (version INTEGER PRIMARY KEY NOT NULL, name TEXT NOT NULL, applied_at TEXT NOT NULL);
		INSERT INTO harness_schema(version, name, applied_at) VALUES (2, 'harness-2', CURRENT_TIMESTAMP);`); err != nil {
		t.Fatal(err)
	}
	if err := ApplyMigrations(context.Background(), db); err == nil {
		t.Fatal("expected non-contiguous metadata error")
	} else if !errors.Is(err, ErrUnknownSchema) {
		t.Fatalf("error = %v, want unknown schema classification", err)
	}
}

func TestConcurrentMigrationMutex(t *testing.T) {
	path := fmt.Sprintf("%s/harness_concurrent_%d.db", os.TempDir(), time.Now().UnixNano())
	defer os.Remove(path)

	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	done := make(chan error, 2)
	ctx := context.Background()
	for i := 0; i < 2; i++ {
		go func() {
			done <- ApplyMigrations(ctx, db)
		}()
	}

	for i := 0; i < 2; i++ {
		if err := <-done; err != nil {
			t.Errorf("concurrent migration %d: %v", i, err)
		}
	}

	rows, err := db.QueryContext(ctx, `SELECT COUNT(*) FROM harness_schema`)
	if err != nil {
		t.Fatalf("count: %v", err)
	}
	var count int
	rows.Next()
	rows.Scan(&count)
	if count != len(migrations) {
		t.Errorf("schema entries = %d, want %d", count, len(migrations))
	}
}
