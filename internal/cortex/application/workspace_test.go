package application

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/RahmatHadinata23758051/CortexOS/internal/cortex/workspace"
)

func newWorkspaceApplication(t *testing.T) (*Service, *workspace.MemoryWorkspace) {
	t.Helper()
	memory := workspace.NewMemoryWorkspace()
	service, err := workspace.NewService(workspace.Dependencies{
		State: memory, Projects: memory, Worktrees: memory, Vault: memory, Retrieval: memory,
	})
	if err != nil {
		t.Fatal(err)
	}
	return NewServiceWithWorkspace(service), memory
}

func TestWorkspaceSnapshotRedactsMachinePaths(t *testing.T) {
	t.Parallel()

	service, memory := newWorkspaceApplication(t)
	ctx := context.Background()
	project, err := memory.RegisterProject(ctx, workspace.Project{
		ID: "project-1", Name: "Workspace", RepositoryRoot: `C:\private\repo`, VaultRoot: `C:\private\vault`,
	})
	if err != nil {
		t.Fatal(err)
	}
	if project.ID == "" {
		t.Fatal("project was not registered")
	}
	got, err := service.GetWorkspaceSnapshot(ctx, WorkspaceSnapshotRequest{SchemaVersion: WorkspaceSchemaVersion})
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Projects) != 1 || got.Projects[0].Name != "Workspace" {
		t.Fatalf("snapshot = %#v", got)
	}
	encoded := got.Projects[0].ID + got.Projects[0].Name + got.Projects[0].Status
	if strings.Contains(encoded, `C:\private`) {
		t.Fatal("snapshot leaked a machine path")
	}
}

func TestWorkspaceQueryMapsAttributionAndExcerpt(t *testing.T) {
	t.Parallel()

	service, memory := newWorkspaceApplication(t)
	ctx := context.Background()
	if _, err := memory.RegisterProject(ctx, workspace.Project{ID: "project-1", Name: "Workspace"}); err != nil {
		t.Fatal(err)
	}
	created := time.Date(2026, 10, 6, 12, 0, 0, 0, time.UTC)
	if err := memory.Upsert(ctx, workspace.RetrievalDocument{
		ID: "document-1", NoteID: "note-1", ProjectID: "project-1", RelativePath: "notes/one.md",
		SourceHash: "sha256:one", IndexVersion: "cortexos.retrieval.v1", Content: "searchable body", Attribution: "manual/tester", IndexedAt: created,
	}); err != nil {
		t.Fatal(err)
	}
	got, err := service.QueryWorkspace(ctx, WorkspaceNoteQueryRequest{
		SchemaVersion: WorkspaceSchemaVersion, ProjectID: "project-1", Query: "searchable", Limit: 10,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Attribution != "manual/tester" || got[0].Excerpt != "searchable body" {
		t.Fatalf("results = %#v", got)
	}
}

func TestWorkspaceBridgeErrorsAreStableAndRedacted(t *testing.T) {
	t.Parallel()

	service, _ := newWorkspaceApplication(t)
	_, err := service.GetWorkspaceSnapshot(context.Background(), WorkspaceSnapshotRequest{SchemaVersion: "unknown"})
	bridgeErr, ok := err.(*WorkspaceBridgeError)
	if !ok || bridgeErr.Code != string(workspace.ErrUnsupportedVersion) {
		t.Fatalf("error = %#v", err)
	}
	if strings.Contains(err.Error(), "unknown") {
		t.Fatal("error leaked request detail")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = service.GetWorkspaceSnapshot(ctx, WorkspaceSnapshotRequest{SchemaVersion: WorkspaceSchemaVersion})
	bridgeErr, ok = err.(*WorkspaceBridgeError)
	if !ok || bridgeErr.Code != string(workspace.ErrCanceled) {
		t.Fatalf("canceled error = %#v", err)
	}
}
