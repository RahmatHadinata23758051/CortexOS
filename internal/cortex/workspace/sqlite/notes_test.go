package sqlite

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/RahmatHadinata23758051/CortexOS/internal/cortex/workspace"
)

func TestNoteMetadataStoreRoundTripAndDeterministicList(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	store, err := Open(context.Background(), filepath.Join(root, "state", "workspace.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	ctx := context.Background()
	project, err := store.RegisterProject(ctx, workspace.Project{
		ID: "project-1", Name: "Project", RepositoryRoot: filepath.Join(root, "repo"), VaultRoot: filepath.Join(root, "vault"),
	})
	if err != nil {
		t.Fatal(err)
	}
	created := time.Date(2026, 10, 6, 12, 0, 0, 0, time.UTC)
	for _, note := range []workspace.VaultNote{
		{ID: "note-b", ProjectID: project.ID, RelativePath: "z.md", Title: "Z", FormatVersion: "cortexos.vault.v1", Source: "manual", Author: "tester", CreatedAt: created, UpdatedAt: created, ContentHash: "sha256:b", Status: workspace.NoteStatusActive},
		{ID: "note-a", ProjectID: project.ID, RelativePath: "a.md", Title: "A", FormatVersion: "cortexos.vault.v1", Source: "manual", Author: "tester", CreatedAt: created, UpdatedAt: created, ContentHash: "sha256:a", Status: workspace.NoteStatusActive},
	} {
		if err := store.UpsertNoteMetadata(ctx, note); err != nil {
			t.Fatal(err)
		}
	}
	listed, err := store.ListNoteMetadata(ctx, project.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(listed) != 2 || listed[0].ID != "note-a" || listed[1].ID != "note-b" {
		t.Fatalf("listed = %#v", listed)
	}
	loaded, err := store.GetNoteMetadata(ctx, "note-a")
	if err != nil {
		t.Fatal(err)
	}
	if loaded.ContentHash != "sha256:a" || loaded.Body != "" {
		t.Fatalf("loaded = %#v", loaded)
	}
	if err := store.DeleteNoteMetadata(ctx, "note-a"); err != nil {
		t.Fatal(err)
	}
	if _, err := store.GetNoteMetadata(ctx, "note-a"); workspace.ErrorCodeOf(err) != workspace.ErrNotFound {
		t.Fatalf("deleted metadata code = %q", workspace.ErrorCodeOf(err))
	}
}
