package workspace_test

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/RahmatHadinata23758051/CortexOS/internal/cortex/workspace"
	workspacegit "github.com/RahmatHadinata23758051/CortexOS/internal/cortex/workspace/git"
	workspacesqlite "github.com/RahmatHadinata23758051/CortexOS/internal/cortex/workspace/sqlite"
)

func TestServiceComposesSQLiteProjectsAndGitWorktrees(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not installed")
	}

	ctx := context.Background()
	root := t.TempDir()
	repositoryRoot := filepath.Join(root, "repository")
	initRepository(t, repositoryRoot)

	store, err := workspacesqlite.Open(ctx, filepath.Join(root, "state", "workspace.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	gitAdapter, err := workspacegit.NewWithRegistry(store, nil, filepath.Join(root, "worktrees"))
	if err != nil {
		t.Fatal(err)
	}
	service, err := workspace.NewService(workspace.Dependencies{
		State:     store,
		Projects:  store,
		Worktrees: gitAdapter,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := service.Open(ctx); err != nil {
		t.Fatal(err)
	}
	defer service.Close()

	project, err := service.RegisterProject(ctx, workspace.Project{
		ID:             "project-production-adapters",
		Name:           "Production Adapter Fixture",
		RepositoryRoot: repositoryRoot,
		VaultRoot:      filepath.Join(root, "vault"),
		DefaultBranch:  "main",
	})
	if err != nil {
		t.Fatal(err)
	}
	worktree, err := service.CreateWorktree(ctx, workspace.Worktree{
		ID:        "worktree-production-adapters",
		ProjectID: project.ID,
		Path:      filepath.Join(root, "worktrees", string(project.ID), "feature"),
		Branch:    "feature/service",
	})
	if err != nil {
		t.Fatal(err)
	}
	if worktree.Dirty || worktree.Branch != "feature/service" {
		t.Fatalf("worktree = %#v", worktree)
	}

	snapshot, err := service.Snapshot(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.SchemaVersion != workspace.ContractVersion || len(snapshot.Projects) != 1 || len(snapshot.Worktrees) != 1 {
		t.Fatalf("snapshot = %#v", snapshot)
	}
	if snapshot.Worktrees[0].ID != worktree.ID {
		t.Fatalf("snapshot worktree = %#v", snapshot.Worktrees[0])
	}

	if err := service.RemoveWorktree(ctx, worktree.ID); err != nil {
		t.Fatal(err)
	}
}

func initRepository(t *testing.T, root string) {
	t.Helper()
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	runGit(t, root, "init", "-b", "main")
	runGit(t, root, "config", "user.email", "test@example.invalid")
	runGit(t, root, "config", "user.name", "CortexOS Test")
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("fixture\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, root, "add", "README.md")
	runGit(t, root, "commit", "-m", "initial")
}

func runGit(t *testing.T, root string, args ...string) {
	t.Helper()
	command := exec.Command("git", append([]string{"-C", root}, args...)...)
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("git %v failed: %v\n%s", args, err, output)
	}
}
