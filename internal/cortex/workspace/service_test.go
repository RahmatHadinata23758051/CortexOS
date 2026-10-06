package workspace

import (
	"context"
	"testing"
)

func TestNewServiceRequiresCoreDependencies(t *testing.T) {
	t.Parallel()

	if _, err := NewService(Dependencies{}); ErrorCodeOf(err) != ErrInvalidRequest {
		t.Fatalf("missing dependency code = %q, want %q", ErrorCodeOf(err), ErrInvalidRequest)
	}
	memory := NewMemoryWorkspace()
	if _, err := NewService(Dependencies{State: memory, Projects: memory}); ErrorCodeOf(err) != ErrInvalidRequest {
		t.Fatalf("missing worktree manager code = %q, want %q", ErrorCodeOf(err), ErrInvalidRequest)
	}
}

func TestServiceComposesMemoryWorkspaceSnapshot(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	memory := NewMemoryWorkspace()
	service, err := NewService(Dependencies{
		State:     memory,
		Projects:  memory,
		Worktrees: memory,
		Vault:     memory,
		Watcher:   memory,
		Retrieval: memory,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := service.Open(ctx); err != nil {
		t.Fatal(err)
	}
	defer service.Close()
	project, err := service.RegisterProject(ctx, Project{ID: "project-1", Name: "Workspace"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := memory.CreateNote(ctx, VaultNote{ID: "note-1", ProjectID: project.ID, RelativePath: "note.md"}); err != nil {
		t.Fatal(err)
	}
	snapshot, err := service.Snapshot(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.SchemaVersion != ContractVersion || len(snapshot.Projects) != 1 || snapshot.VaultNoteCount != 1 {
		t.Fatalf("snapshot = %#v", snapshot)
	}
	if snapshot.WatcherState != WatcherStateReady || snapshot.RetrievalState != "ready" {
		t.Fatalf("adapter states = %#v", snapshot)
	}
}

func TestServicePropagatesCancellation(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	memory := NewMemoryWorkspace()
	service, err := NewService(Dependencies{State: memory, Projects: memory, Worktrees: memory})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.Snapshot(ctx); ErrorCodeOf(err) != ErrCanceled {
		t.Fatalf("snapshot error code = %q, want %q", ErrorCodeOf(err), ErrCanceled)
	}
}
