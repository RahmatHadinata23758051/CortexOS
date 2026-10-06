package retrieval

import (
	"context"
	"testing"
	"time"

	"github.com/RahmatHadinata23758051/CortexOS/internal/cortex/workspace"
)

func TestIndexRebuildsFromVaultSource(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	memory := workspace.NewMemoryWorkspace()
	if err := memory.Open(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := memory.RegisterProject(ctx, workspace.Project{ID: "project", Name: "Project"}); err != nil {
		t.Fatal(err)
	}
	created := time.Date(2026, 10, 6, 12, 0, 0, 0, time.UTC)
	if _, err := memory.CreateNote(ctx, workspace.VaultNote{
		ID: "note", ProjectID: "project", RelativePath: "note.md", Title: "Note", Body: "authoritative token\n",
		FormatVersion: "cortexos.vault.v1", Source: "manual", Author: "tester", CreatedAt: created, UpdatedAt: created, ContentHash: "source",
	}); err != nil {
		t.Fatal(err)
	}
	index, err := NewWithSource(t.TempDir(), memory)
	if err != nil {
		t.Fatal(err)
	}
	if err := index.Rebuild(ctx, "project"); err != nil {
		t.Fatal(err)
	}
	results, err := index.Query(ctx, "project", "authoritative", 10)
	if err != nil || len(results) != 1 || results[0].NoteID != "note" {
		t.Fatalf("results = %#v, err=%v", results, err)
	}
}

func TestIndexRebuildWithoutSourceFailsExplicitly(t *testing.T) {
	t.Parallel()

	index, err := New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err := index.Rebuild(context.Background(), "project"); workspace.ErrorCodeOf(err) != workspace.ErrInvalidRequest {
		t.Fatalf("rebuild error code = %q", workspace.ErrorCodeOf(err))
	}
}
