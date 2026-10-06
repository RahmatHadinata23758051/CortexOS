package platform

import (
	"context"
	"testing"

	"github.com/RahmatHadinata23758051/CortexOS/internal/cortex/application"
	"github.com/RahmatHadinata23758051/CortexOS/internal/cortex/workspace"
)

func TestBridgeExposesWorkspaceSnapshotAndQueryThroughTypedDTOs(t *testing.T) {
	t.Parallel()

	memory := workspace.NewMemoryWorkspace()
	service, err := workspace.NewService(workspace.Dependencies{
		State: memory, Projects: memory, Worktrees: memory, Retrieval: memory,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := memory.RegisterProject(context.Background(), workspace.Project{ID: "project-1", Name: "Project"}); err != nil {
		t.Fatal(err)
	}
	bridge := NewBridge(application.NewServiceWithWorkspace(service))
	snapshot, err := bridge.GetWorkspaceSnapshot(application.WorkspaceSnapshotRequest{
		SchemaVersion: application.WorkspaceSchemaVersion,
	})
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.SchemaVersion != application.WorkspaceSchemaVersion || len(snapshot.Projects) != 1 {
		t.Fatalf("snapshot = %#v", snapshot)
	}
	if snapshot.Projects[0].Name != "Project" {
		t.Fatalf("project = %#v", snapshot.Projects[0])
	}
	results, err := bridge.QueryWorkspace(application.WorkspaceNoteQueryRequest{
		SchemaVersion: application.WorkspaceSchemaVersion, ProjectID: "project-1", Query: "project", Limit: 10,
	})
	if err != nil || len(results) != 0 {
		t.Fatalf("results = %#v, err = %v", results, err)
	}
}

func TestBridgeWorkspaceErrorsDoNotExposeAbsolutePaths(t *testing.T) {
	t.Parallel()

	bridge := NewBridge(application.NewService())
	_, err := bridge.GetWorkspaceSnapshot(application.WorkspaceSnapshotRequest{
		SchemaVersion: application.WorkspaceSchemaVersion,
	})
	if err == nil || err.Error() != "workspace.storage_unavailable: workspace service is unavailable" {
		t.Fatalf("error = %v", err)
	}
}
