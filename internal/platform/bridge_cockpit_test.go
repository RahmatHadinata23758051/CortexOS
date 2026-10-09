package platform

import (
	"context"
	"testing"

	"github.com/RahmatHadinata23758051/CortexOS/internal/cortex/application"
	"github.com/RahmatHadinata23758051/CortexOS/internal/cortex/workspace"
)

func TestCockpitBridgeExposesRuntimeAndWorkspace(t *testing.T) {
	t.Parallel()

	memory := workspace.NewMemoryWorkspace()
	service, err := workspace.NewService(workspace.Dependencies{
		State: memory, Projects: memory, Worktrees: memory, Retrieval: memory,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := memory.RegisterProject(context.Background(), workspace.Project{
		ID:             "project-1",
		Name:           "Project",
		RepositoryRoot: "C:/Projects/project-1",
		VaultRoot:      "C:/Projects/project-1/vault",
	}); err != nil {
		t.Fatal(err)
	}

	appService := application.NewServiceWithWorkspace(service)
	cockpitBridge := NewCockpitBridge(appService)

	runtime, err := cockpitBridge.GetCockpitRuntime()
	if err != nil {
		t.Fatal(err)
	}
	if runtime.SchemaVersion != application.CockpitBridgeSchemaVersion || runtime.Status != "ready" {
		t.Fatalf("runtime = %#v", runtime)
	}

	ws, err := cockpitBridge.GetCockpitWorkspace(application.CockpitWorkspaceRequest{
		SchemaVersion: application.CockpitBridgeSchemaVersion,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(ws.Projects) != 1 || ws.Projects[0].Name != "Project" {
		t.Fatalf("ws = %#v", ws)
	}

	// Test unified Bridge as well
	bridge := NewBridge(appService)
	bridgeRuntime, err := bridge.GetCockpitRuntime()
	if err != nil {
		t.Fatal(err)
	}
	if bridgeRuntime.Status != "ready" {
		t.Fatalf("bridgeRuntime = %#v", bridgeRuntime)
	}

	bridgeWS, err := bridge.GetCockpitWorkspace(application.CockpitWorkspaceRequest{
		SchemaVersion: application.CockpitBridgeSchemaVersion,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(bridgeWS.Projects) != 1 {
		t.Fatalf("bridgeWS = %#v", bridgeWS)
	}
}

func TestCockpitBridgeWithContextHonorsCancellation(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	appService := application.NewService()
	cockpitBridge := NewCockpitBridge(appService).WithContext(ctx)

	_, err := cockpitBridge.GetCockpitWorkspace(application.CockpitWorkspaceRequest{
		SchemaVersion: application.CockpitBridgeSchemaVersion,
	})
	if err == nil {
		t.Fatal("expected error on canceled context")
	}

	bridge := NewBridge(appService).WithContext(ctx)
	_, err = bridge.GetCockpitWorkspace(application.CockpitWorkspaceRequest{
		SchemaVersion: application.CockpitBridgeSchemaVersion,
	})
	if err == nil {
		t.Fatal("expected error on canceled context")
	}
}
