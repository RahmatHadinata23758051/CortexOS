package sqlite

import (
	"context"
	"path/filepath"
	"testing"
)

func TestOpenCreatesAndReopensDatabase(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "workspace.db")
	first, err := Open(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	if first.Path() != path {
		t.Fatalf("path = %q, want %q", first.Path(), path)
	}
	if err := first.Close(); err != nil {
		t.Fatal(err)
	}

	second, err := Open(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	defer second.Close()
	var count int
	if err := second.DB().QueryRow(`SELECT COUNT(*) FROM workspace_schema`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 2 {
		t.Fatalf("migration count = %d, want 2", count)
	}
}

func TestOpenRejectsRelativeDatabasePath(t *testing.T) {
	t.Parallel()
	if _, err := Open(context.Background(), "workspace.db"); err == nil {
		t.Fatal("expected relative database path error")
	}
}
