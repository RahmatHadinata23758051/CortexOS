package refresh

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/RahmatHadinata23758051/CortexOS/internal/cortex/workspace"
	"github.com/RahmatHadinata23758051/CortexOS/internal/cortex/workspace/retrieval"
	"github.com/RahmatHadinata23758051/CortexOS/internal/cortex/workspace/vault"
	"github.com/RahmatHadinata23758051/CortexOS/internal/cortex/workspace/watch"
)

func TestWatcherCoordinatorRefreshesRetrievalFromVaultEdit(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	root := t.TempDir()
	vaultStore, err := vault.New(filepath.Join(root, "vault"))
	if err != nil {
		t.Fatal(err)
	}
	created := time.Date(2026, 10, 6, 12, 0, 0, 0, time.UTC)
	note, err := vaultStore.CreateNote(ctx, workspace.VaultNote{
		ID: "note-1", ProjectID: "project-1", RelativePath: "note.md", Title: "Note", Body: "before token\n",
		FormatVersion: vault.FormatVersion, Source: "manual", Author: "tester", CreatedAt: created, UpdatedAt: created,
	})
	if err != nil {
		t.Fatal(err)
	}
	index, err := retrieval.NewWithSource(filepath.Join(root, "index"), vaultStore)
	if err != nil {
		t.Fatal(err)
	}
	if err := index.Rebuild(ctx, note.ProjectID); err != nil {
		t.Fatal(err)
	}
	watcher := watch.New()
	coordinator, err := New(watcher, NewRebuildHandler(index))
	if err != nil {
		t.Fatal(err)
	}
	requests := coordinator.Requests()
	if err := coordinator.Start(ctx, note.ProjectID, vaultStore.Root()); err != nil {
		t.Fatal(err)
	}
	defer coordinator.Stop()

	updated, err := vaultStore.UpdateNote(ctx, workspace.VaultNote{
		ID: note.ID, ProjectID: note.ProjectID, RelativePath: note.RelativePath, Title: note.Title,
		Body: "after token\n", FormatVersion: vault.FormatVersion, Source: note.Source, Author: note.Author,
		CreatedAt: note.CreatedAt, UpdatedAt: created.Add(time.Minute),
	}, note.ContentHash)
	if err != nil {
		t.Fatal(err)
	}
	deadline := time.After(3 * time.Second)
	for {
		select {
		case request := <-requests:
			if request.RelativePath != note.RelativePath {
				continue
			}
			if request.ProjectID != note.ProjectID || request.ContentHash == "" {
				t.Fatalf("request = %#v", request)
			}
			goto refreshed
		case <-deadline:
			t.Fatal("timed out waiting for Vault edit refresh request")
		}
	}

refreshed:
	refreshDeadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(refreshDeadline) {
		results, queryErr := index.Query(ctx, note.ProjectID, "after", 10)
		if queryErr == nil && len(results) == 1 && results[0].SourceHash == updated.ContentHash {
			return
		}
		time.Sleep(25 * time.Millisecond)
	}
	t.Fatal("retrieval index did not reflect Vault edit")
}
