package application

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/RahmatHadinata23758051/CortexOS/internal/cortex/workspace"
)

func TestCockpitBridgeContracts(t *testing.T) {
	t.Parallel()

	memory := workspace.NewMemoryWorkspace()
	service, err := workspace.NewService(workspace.Dependencies{
		State: memory, Projects: memory, Worktrees: memory, Retrieval: memory,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := memory.RegisterProject(context.Background(), workspace.Project{ID: "project-1", Name: "Project", RepositoryRoot: "C:/Projects/project-1", VaultRoot: "C:/Projects/project-1/vault"}); err != nil {
		t.Fatal(err)
	}

	s := NewServiceWithWorkspace(service)
	s = NewServiceWithFull(service, nil, nil, nil, nil)

	runtime, err := s.GetCockpitRuntime(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if runtime.SchemaVersion != CockpitBridgeSchemaVersion || runtime.Status != "ready" {
		t.Fatalf("runtime = %#v", runtime)
	}

	snapshot, err := s.GetCockpitWorkspace(context.Background(), CockpitWorkspaceRequest{SchemaVersion: CockpitBridgeSchemaVersion})
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.SchemaVersion != CockpitBridgeSchemaVersion || len(snapshot.Projects) != 1 {
		t.Fatalf("snapshot = %#v", snapshot)
	}
	if snapshot.Projects[0].Name != "Project" {
		t.Fatalf("project = %#v", snapshot.Projects[0])
	}
	if snapshot.Projects[0].SchemaVersion != CockpitBridgeSchemaVersion {
		t.Fatalf("project schema version mismatch: %s", snapshot.Projects[0].SchemaVersion)
	}
	if snapshot.Worktrees == nil {
		t.Fatal("worktrees should not be nil")
	}

	project, err := s.RegisterCockpitProject(context.Background(), CockpitProjectRequest{
		SchemaVersion:  CockpitBridgeSchemaVersion,
		ID:             "project-2",
		Name:           "Project Two",
		RepositoryRoot: "C:/Projects/project-2",
		VaultRoot:      "C:/Projects/project-2/vault",
		DefaultBranch:  "main",
	})
	if err != nil {
		t.Fatal(err)
	}
	if project.ID != "project-2" || project.SchemaVersion != CockpitBridgeSchemaVersion {
		t.Fatalf("project = %#v", project)
	}

	results, err := s.QueryCockpitWorkspace(context.Background(), CockpitQueryRequest{
		SchemaVersion: CockpitBridgeSchemaVersion,
		ProjectID:     "project-1",
		Query:         "test",
		Limit:         10,
	})
	if err != nil {
		t.Fatal(err)
	}
	if results == nil {
		t.Fatal("results should not be nil")
	}
}

func TestCockpitBridgeErrorCodes(t *testing.T) {
	t.Parallel()

	s := NewService()

	_, err := s.GetCockpitWorkspace(context.Background(), CockpitWorkspaceRequest{SchemaVersion: CockpitBridgeSchemaVersion})
	if err == nil {
		t.Fatal("expected error when workspace service unavailable")
	}
	var unavailableErr *CockpitBridgeError
	if !errors.As(err, &unavailableErr) {
		t.Fatalf("expected CockpitBridgeError, got %T", err)
	}
	if unavailableErr.Code != "cockpit.internal" {
		t.Fatalf("unexpected error code: %s", unavailableErr.Code)
	}

	_, err = s.GetCockpitWorkspace(context.Background(), CockpitWorkspaceRequest{SchemaVersion: "invalid"})
	if err == nil {
		t.Fatal("expected error for invalid schema version")
	}
	var bridgeErr *CockpitBridgeError
	if !errors.As(err, &bridgeErr) {
		t.Fatalf("expected CockpitBridgeError, got %T", err)
	}
	if bridgeErr.Code != "cockpit.unsupported_version" {
		t.Fatalf("expected unsupported_version, got %s", bridgeErr.Code)
	}
}

func TestCockpitWorkspaceFiltersByProject(t *testing.T) {
	t.Parallel()

	memory := workspace.NewMemoryWorkspace()
	service, err := workspace.NewService(workspace.Dependencies{
		State: memory, Projects: memory, Worktrees: memory, Retrieval: memory,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := memory.RegisterProject(context.Background(), workspace.Project{ID: "project-1", Name: "Project One", RepositoryRoot: "C:/Projects/project-1", VaultRoot: "C:/Projects/project-1/vault"}); err != nil {
		t.Fatal(err)
	}
	if _, err := memory.RegisterProject(context.Background(), workspace.Project{ID: "project-2", Name: "Project Two", RepositoryRoot: "C:/Projects/project-2", VaultRoot: "C:/Projects/project-2/vault"}); err != nil {
		t.Fatal(err)
	}

	s := NewServiceWithWorkspace(service)
	s = NewServiceWithFull(service, nil, nil, nil, nil)

	snapshot, err := s.GetCockpitWorkspace(context.Background(), CockpitWorkspaceRequest{SchemaVersion: CockpitBridgeSchemaVersion, ProjectID: "project-1"})
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshot.Projects) != 1 || snapshot.Projects[0].ID != "project-1" {
		t.Fatalf("expected 1 project, got %d: %#v", len(snapshot.Projects), snapshot.Projects)
	}
}

func TestCockpitRuntimeSnapshotIsDeterministic(t *testing.T) {
	t.Parallel()

	s := NewServiceWithCockpit(nil)
	r1, err := s.GetCockpitRuntime(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	time.Sleep(1 * time.Millisecond)
	r2, err := s.GetCockpitRuntime(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if r1.SchemaVersion != r2.SchemaVersion || r1.Status != r2.Status || r1.Environment != r2.Environment || r1.Provider != r2.Provider {
		t.Fatalf("runtime snapshots differ: %#v vs %#v", r1, r2)
	}
}
