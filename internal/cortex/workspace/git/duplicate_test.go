package git

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/RahmatHadinata23758051/CortexOS/internal/cortex/workspace"
)

func TestCreateWorktreeRejectsDuplicateTargetAndBranch(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not installed")
	}

	ctx := context.Background()
	root := t.TempDir()
	repositoryRoot := filepath.Join(root, "repository")
	if err := os.MkdirAll(repositoryRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	runGit(t, repositoryRoot, "init", "-b", "main")
	runGit(t, repositoryRoot, "config", "user.email", "test@example.invalid")
	runGit(t, repositoryRoot, "config", "user.name", "CortexOS Test")
	if err := os.WriteFile(filepath.Join(repositoryRoot, "README.md"), []byte("fixture\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, repositoryRoot, "add", "README.md")
	runGit(t, repositoryRoot, "commit", "-m", "initial")

	memory := workspace.NewMemoryWorkspace()
	if err := memory.Open(ctx); err != nil {
		t.Fatal(err)
	}
	project, err := memory.RegisterProject(ctx, workspace.Project{
		ID:             "project-duplicate",
		Name:           "Duplicate Fixture",
		RepositoryRoot: repositoryRoot,
		VaultRoot:      filepath.Join(root, "vault"),
		DefaultBranch:  "main",
	})
	if err != nil {
		t.Fatal(err)
	}
	adapter, err := NewWithRegistry(memory, nil, filepath.Join(root, "worktrees"))
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, "worktrees", string(project.ID), "one")
	if _, err := adapter.CreateWorktree(ctx, workspace.Worktree{ID: "one", ProjectID: project.ID, Path: path, Branch: "feature/duplicate"}); err != nil {
		t.Fatal(err)
	}
	if _, err := adapter.CreateWorktree(ctx, workspace.Worktree{ID: "two", ProjectID: project.ID, Path: path, Branch: "feature/other"}); workspace.ErrorCodeOf(err) != workspace.ErrConflict {
		t.Fatalf("duplicate path code = %q, want %q", workspace.ErrorCodeOf(err), workspace.ErrConflict)
	}
	if _, err := adapter.CreateWorktree(ctx, workspace.Worktree{ID: "three", ProjectID: project.ID, Path: filepath.Join(root, "worktrees", string(project.ID), "two"), Branch: "feature/duplicate"}); workspace.ErrorCodeOf(err) != workspace.ErrConflict {
		t.Fatalf("duplicate branch code = %q, want %q", workspace.ErrorCodeOf(err), workspace.ErrConflict)
	}
}
