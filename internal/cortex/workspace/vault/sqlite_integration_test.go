package vault

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/RahmatHadinata23758051/CortexOS/internal/cortex/workspace"
	workspacesqlite "github.com/RahmatHadinata23758051/CortexOS/internal/cortex/workspace/sqlite"
)

func TestStoreSynchronizesSQLiteMetadataAfterValidatedFilesystemWrite(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	state, err := workspacesqlite.Open(context.Background(), filepath.Join(root, "state", "workspace.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer state.Close()
	ctx := context.Background()
	project, err := state.RegisterProject(ctx, workspace.Project{
		ID: "project-1", Name: "Vault", RepositoryRoot: filepath.Join(root, "repo"), VaultRoot: filepath.Join(root, "vault"),
	})
	if err != nil {
		t.Fatal(err)
	}
	store, err := NewWithMetadata(project.VaultRoot, state)
	if err != nil {
		t.Fatal(err)
	}
	created := time.Date(2026, 10, 6, 12, 0, 0, 0, time.UTC)
	note, err := store.CreateNote(ctx, workspace.VaultNote{
		ID: "note-1", ProjectID: project.ID, RelativePath: "decisions/one.md", Title: "One", Body: "body\n",
		FormatVersion: FormatVersion, Source: "manual", Author: "tester", CreatedAt: created, UpdatedAt: created,
	})
	if err != nil {
		t.Fatal(err)
	}
	metadata, err := state.GetNoteMetadata(ctx, note.ID)
	if err != nil {
		t.Fatal(err)
	}
	if metadata.ContentHash != note.ContentHash || metadata.Body != "" {
		t.Fatalf("metadata = %#v", metadata)
	}
	if err := store.DeleteNote(ctx, note.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := state.GetNoteMetadata(ctx, note.ID); workspace.ErrorCodeOf(err) != workspace.ErrNotFound {
		t.Fatalf("deleted metadata code = %q", workspace.ErrorCodeOf(err))
	}
}
