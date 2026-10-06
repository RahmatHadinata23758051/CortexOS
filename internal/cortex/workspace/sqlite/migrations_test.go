package sqlite

import (
	"context"
	"testing"
)

func TestSchemaVersionIsStable(t *testing.T) {
	t.Parallel()
	if SchemaVersion() != "workspace.sqlite.v1" {
		t.Fatalf("schema version = %q", SchemaVersion())
	}
}

func TestApplyMigrationsRejectsNilDatabase(t *testing.T) {
	t.Parallel()
	if err := ApplyMigrations(context.Background(), nil); err == nil {
		t.Fatal("expected nil database error")
	}
}
