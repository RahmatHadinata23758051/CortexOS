package git

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/RahmatHadinata23758051/CortexOS/internal/cortex/workspace"
)

func TestControlledWorktreeLifecycleAgainstDisposableRepository(t *testing.T) {
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
		ID:             "project-1",
		Name:           "Fixture",
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
	worktreePath := filepath.Join(root, "worktrees", string(project.ID), "feature-one")
	created, err := adapter.CreateWorktree(ctx, workspace.Worktree{
		ID:        "worktree-1",
		ProjectID: project.ID,
		Path:      worktreePath,
		Branch:    "feature/one",
	})
	if err != nil {
		t.Fatal(err)
	}
	if created.Branch != "feature/one" || created.Dirty {
		t.Fatalf("created worktree = %#v", created)
	}

	listed, err := adapter.ListWorktrees(ctx, project.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(listed) != 1 || listed[0].Path != worktreePath {
		t.Fatalf("listed worktrees = %#v", listed)
	}
	inspected, err := adapter.InspectWorktree(ctx, created.ID)
	if err != nil {
		t.Fatal(err)
	}
	if inspected.Revision == "" || inspected.Dirty {
		t.Fatalf("inspected worktree = %#v", inspected)
	}

	runGit(t, repositoryRoot, "worktree", "lock", "--reason", "active-reference", worktreePath)
	if err := adapter.RemoveWorktree(ctx, created.ID); workspace.ErrorCodeOf(err) != workspace.ErrConflict {
		t.Fatalf("locked removal code = %q, want %q", workspace.ErrorCodeOf(err), workspace.ErrConflict)
	}
	runGit(t, repositoryRoot, "worktree", "unlock", worktreePath)
	if err := adapter.RemoveWorktree(ctx, created.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(worktreePath); !os.IsNotExist(err) {
		t.Fatalf("worktree path still exists, stat error = %v", err)
	}
}

func TestControlledWorktreeRemovalRejectsDirtyChanges(t *testing.T) {
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
		ID:             "project-2",
		Name:           "Dirty Fixture",
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
	worktreePath := filepath.Join(root, "worktrees", string(project.ID), "dirty")
	created, err := adapter.CreateWorktree(ctx, workspace.Worktree{
		ID:        "worktree-2",
		ProjectID: project.ID,
		Path:      worktreePath,
		Branch:    "feature/dirty",
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(worktreePath, "dirty.txt"), []byte("uncommitted\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := adapter.RemoveWorktree(ctx, created.ID); workspace.ErrorCodeOf(err) != workspace.ErrConflict {
		t.Fatalf("dirty removal code = %q, want %q", workspace.ErrorCodeOf(err), workspace.ErrConflict)
	}
	if _, err := os.Stat(worktreePath); err != nil {
		t.Fatalf("dirty worktree was removed: %v", err)
	}
}

func runGit(t *testing.T, directory string, args ...string) string {
	t.Helper()
	command := exec.Command("git", append([]string{"-C", directory}, args...)...)
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v failed: %v\n%s", args, err, output)
	}
	return strings.TrimSpace(string(output))
}
