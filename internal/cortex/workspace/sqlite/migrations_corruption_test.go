package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	_ "modernc.org/sqlite"
)

func TestApplyMigrationsRejectsNonContiguousMetadata(t *testing.T) {
	t.Parallel()

	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec(`CREATE TABLE workspace_schema (version INTEGER PRIMARY KEY NOT NULL, name TEXT NOT NULL, applied_at TEXT NOT NULL);
		INSERT INTO workspace_schema(version, name, applied_at) VALUES (2, 'workspace-2', CURRENT_TIMESTAMP);`); err != nil {
		t.Fatal(err)
	}
	if err := ApplyMigrations(context.Background(), db); err == nil {
		t.Fatal("expected non-contiguous metadata error")
	} else if !errors.Is(err, ErrUnknownSchema) {
		t.Fatalf("error = %v, want unknown schema classification", err)
	}
}
