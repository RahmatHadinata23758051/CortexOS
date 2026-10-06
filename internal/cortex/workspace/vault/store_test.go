package vault

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/RahmatHadinata23758051/CortexOS/internal/cortex/workspace"
)

func TestStoreCreateListUpdateDeleteWithConflictProtection(t *testing.T) {
	t.Parallel()

	store, err := New(filepath.Join(t.TempDir(), "vault"))
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	createdAt := time.Date(2026, 10, 6, 12, 0, 0, 0, time.UTC)
	note, err := store.CreateNote(ctx, workspace.VaultNote{
		ID:            "note-1",
		ProjectID:     "project-1",
		RelativePath:  "decisions/storage.md",
		Title:         "Storage",
		Body:          "# Storage\n",
		FormatVersion: FormatVersion,
		Source:        "manual",
		Author:        "tester",
		CreatedAt:     createdAt,
		UpdatedAt:     createdAt,
	})
	if err != nil {
		t.Fatal(err)
	}
	if note.ContentHash != HashBody(note.Body) {
		t.Fatalf("hash = %q", note.ContentHash)
	}
	listed, err := store.ListNotes(ctx, note.ProjectID)
	if err != nil {
		t.Fatal(err)
	}
	if len(listed) != 1 || listed[0].ID != note.ID {
		t.Fatalf("listed = %#v", listed)
	}

	if err := os.WriteFile(filepath.Join(store.Root(), note.RelativePath), []byte("external edit\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := store.UpdateNote(ctx, workspace.VaultNote{
		ID:            note.ID,
		ProjectID:     note.ProjectID,
		RelativePath:  note.RelativePath,
		Title:         "Updated",
		Body:          "updated\n",
		FormatVersion: FormatVersion,
		Source:        note.Source,
		Author:        note.Author,
		CreatedAt:     note.CreatedAt,
		UpdatedAt:     note.UpdatedAt.Add(time.Minute),
	}, note.ContentHash); workspace.ErrorCodeOf(err) != workspace.ErrInvalidRequest {
		t.Fatalf("malformed external edit code = %q, want %q", workspace.ErrorCodeOf(err), workspace.ErrInvalidRequest)
	}

	if err := os.WriteFile(filepath.Join(store.Root(), note.RelativePath), serializeFixture(t, workspace.VaultNote{
		ID:            note.ID,
		ProjectID:     note.ProjectID,
		RelativePath:  note.RelativePath,
		Title:         note.Title,
		Body:          "external\n",
		FormatVersion: FormatVersion,
		Source:        note.Source,
		Author:        note.Author,
		CreatedAt:     note.CreatedAt,
		UpdatedAt:     note.UpdatedAt.Add(time.Minute),
	}), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := store.UpdateNote(ctx, note, note.ContentHash); workspace.ErrorCodeOf(err) != workspace.ErrConflict {
		t.Fatalf("external conflict code = %q, want %q", workspace.ErrorCodeOf(err), workspace.ErrConflict)
	}
	if err := store.DeleteNote(ctx, note.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := store.GetNote(ctx, note.ID); workspace.ErrorCodeOf(err) != workspace.ErrNotFound {
		t.Fatalf("deleted note code = %q, want %q", workspace.ErrorCodeOf(err), workspace.ErrNotFound)
	}
}

func serializeFixture(t *testing.T, note workspace.VaultNote) []byte {
	t.Helper()
	data, err := Serialize(note)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func TestStoreRejectsTraversalAndNonMarkdownPaths(t *testing.T) {
	t.Parallel()

	store, err := New(filepath.Join(t.TempDir(), "vault"))
	if err != nil {
		t.Fatal(err)
	}
	base := workspace.VaultNote{ID: "note", ProjectID: "project", Title: "Note", Body: "body\n", FormatVersion: FormatVersion, Source: "manual", Author: "tester"}
	for _, path := range []string{"../escape.md", "note.txt", "C:/escape.md"} {
		base.RelativePath = path
		if _, err := store.CreateNote(context.Background(), base); workspace.ErrorCodeOf(err) != workspace.ErrPathDenied && workspace.ErrorCodeOf(err) != workspace.ErrInvalidRequest {
			t.Errorf("path %q error code = %q", path, workspace.ErrorCodeOf(err))
		}
	}
}

func TestStoreRejectsSymlinkEscapeWhenSupported(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	store, err := New(filepath.Join(root, "vault"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(store.Root(), 0o700); err != nil {
		t.Fatal(err)
	}
	outside := t.TempDir()
	link := filepath.Join(store.Root(), "escape")
	if err := os.Symlink(outside, link); err != nil {
		t.Skipf("symlink unsupported: %v", err)
	}
	_, err = store.CreateNote(context.Background(), workspace.VaultNote{
		ID: "note-link", ProjectID: "project", RelativePath: "escape/note.md", Title: "Link", Body: "body\n",
		FormatVersion: FormatVersion, Source: "manual", Author: "tester",
	})
	if workspace.ErrorCodeOf(err) != workspace.ErrPathDenied {
		t.Fatalf("symlink escape error code = %q, want %q", workspace.ErrorCodeOf(err), workspace.ErrPathDenied)
	}
}
