package application

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/RahmatHadinata23758051/CortexOS/internal/cortex/workspace"
)

func TestRegisterWorkspaceProjectDefaultsVaultUnderRepository(t *testing.T) {
	service, _ := newWorkspaceApplication(t)
	root := filepath.Join(t.TempDir(), "repo")

	project, err := service.RegisterWorkspaceProject(context.Background(), WorkspaceProjectRequest{
		SchemaVersion:  WorkspaceSchemaVersion,
		ID:             "project-1",
		Name:           "Project",
		RepositoryRoot: root,
	})
	if err != nil {
		t.Fatal(err)
	}
	if project.ID != "project-1" {
		t.Fatalf("project = %#v", project)
	}
	stored, err := service.workspace.GetProject(context.Background(), workspace.ProjectID("project-1"))
	if err != nil {
		t.Fatal(err)
	}
	wantVault := filepath.Join(root, ".cortexos", "vault")
	if stored.VaultRoot != wantVault {
		t.Fatalf("vault root = %q, want %q", stored.VaultRoot, wantVault)
	}
}
