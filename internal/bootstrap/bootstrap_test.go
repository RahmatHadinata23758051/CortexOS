package bootstrap

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/RahmatHadinata23758051/CortexOS/internal/cortex/workspace"
)

func TestOpenWorkspaceComposesDurableAdapters(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	service, closeWorkspace, err := OpenWorkspace(context.Background(), filepath.Join(root, "data"))
	if err != nil {
		t.Fatal(err)
	}
	if service == nil || closeWorkspace == nil {
		t.Fatal("OpenWorkspace returned an incomplete composition")
	}
	if err := service.Open(context.Background()); err != nil {
		t.Fatal(err)
	}
	project, err := service.RegisterProject(context.Background(), workspace.Project{
		ID: "project-1", Name: "Fixture", RepositoryRoot: filepath.Join(root, "repo"), VaultRoot: filepath.Join(root, "vault"),
	})
	if err != nil {
		t.Fatal(err)
	}
	if project.Status != workspace.ProjectStatusActive {
		t.Fatalf("project = %#v", project)
	}
	if err := closeWorkspace(); err != nil {
		t.Fatal(err)
	}
}

func TestOpenWorkspaceRejectsRelativeDataRoot(t *testing.T) {
	t.Parallel()

	if _, _, err := OpenWorkspace(context.Background(), "relative-data"); workspace.ErrorCodeOf(err) != workspace.ErrPathDenied {
		t.Fatalf("error code = %q, err = %v", workspace.ErrorCodeOf(err), err)
	}
}
