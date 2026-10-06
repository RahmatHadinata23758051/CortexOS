package workspace_test

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/RahmatHadinata23758051/CortexOS/internal/cortex/workspace"
	workspacegit "github.com/RahmatHadinata23758051/CortexOS/internal/cortex/workspace/git"
	"github.com/RahmatHadinata23758051/CortexOS/internal/cortex/workspace/refresh"
	"github.com/RahmatHadinata23758051/CortexOS/internal/cortex/workspace/retrieval"
	workspacesqlite "github.com/RahmatHadinata23758051/CortexOS/internal/cortex/workspace/sqlite"
	"github.com/RahmatHadinata23758051/CortexOS/internal/cortex/workspace/vault"
	"github.com/RahmatHadinata23758051/CortexOS/internal/cortex/workspace/watch"
)

func TestPhase2RegisterNoteWatchQueryAndReopen(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not installed")
	}

	ctx := context.Background()
	root := t.TempDir()
	repositoryRoot := filepath.Join(root, "repository")
	initPhase2Repository(t, repositoryRoot)
	dataRoot := filepath.Join(root, "data")

	state, err := workspacesqlite.Open(ctx, filepath.Join(dataRoot, "state", "workspace.db"))
	if err != nil {
		t.Fatal(err)
	}
	project, err := state.RegisterProject(ctx, workspace.Project{
		ID: "phase2-project", Name: "Phase 2 Fixture", RepositoryRoot: repositoryRoot,
		VaultRoot: filepath.Join(dataRoot, "vault"), DefaultBranch: "main",
	})
	if err != nil {
		state.Close()
		t.Fatal(err)
	}
	vaultStore, err := vault.NewWithMetadata(project.VaultRoot, state)
	if err != nil {
		state.Close()
		t.Fatal(err)
	}
	index, err := retrieval.NewWithSource(filepath.Join(dataRoot, "retrieval"), vaultStore)
	if err != nil {
		state.Close()
		t.Fatal(err)
	}
	gitAdapter, err := workspacegit.NewWithRegistry(state, nil, filepath.Join(dataRoot, "worktrees"))
	if err != nil {
		state.Close()
		t.Fatal(err)
	}
	service, err := workspace.NewService(workspace.Dependencies{
		State: state, Projects: state, Worktrees: gitAdapter, Vault: vaultStore,
		Watcher: watch.New(), Retrieval: index,
	})
	if err != nil {
		state.Close()
		t.Fatal(err)
	}
	if err := service.Open(ctx); err != nil {
		state.Close()
		t.Fatal(err)
	}
	worktree, err := service.CreateWorktree(ctx, workspace.Worktree{
		ID: "phase2-worktree", ProjectID: project.ID,
		Path: filepath.Join(dataRoot, "worktrees", string(project.ID), "phase2"), Branch: "feature/phase2",
	})
	if err != nil {
		service.Close()
		t.Fatal(err)
	}
	if worktree.Status != workspace.WorktreeStatusActive || worktree.Revision == "" {
		service.Close()
		t.Fatalf("worktree = %#v", worktree)
	}
	created := time.Date(2026, 10, 6, 12, 0, 0, 0, time.UTC)
	note, err := service.CreateNote(ctx, workspace.VaultNote{
		ID: "phase2-note", ProjectID: project.ID, RelativePath: "phase2.md", Title: "Phase 2",
		Body: "durable fixture token\n", FormatVersion: vault.FormatVersion, Source: "fixture", Author: "test",
		CreatedAt: created, UpdatedAt: created,
	})
	if err != nil {
		service.Close()
		t.Fatal(err)
	}
	if err := service.RebuildRetrieval(ctx, project.ID); err != nil {
		service.Close()
		t.Fatal(err)
	}
	initial, err := service.QueryRetrieval(ctx, project.ID, "durable fixture", 10)
	if err != nil || len(initial) != 1 || initial[0].SourceHash != note.ContentHash {
		service.Close()
		t.Fatalf("initial query = %#v, err = %v", initial, err)
	}

	watcher := watch.New()
	coordinator, err := refresh.New(watcher, refresh.NewRebuildHandler(index))
	if err != nil {
		service.Close()
		t.Fatal(err)
	}
	requests := coordinator.Requests()
	if err := coordinator.Start(ctx, project.ID, project.VaultRoot); err != nil {
		service.Close()
		t.Fatal(err)
	}
	updated, err := service.UpdateNote(ctx, workspace.VaultNote{
		ID: note.ID, ProjectID: note.ProjectID, RelativePath: note.RelativePath, Title: note.Title,
		Body: "reopened fixture token\n", FormatVersion: vault.FormatVersion, Source: note.Source, Author: note.Author,
		CreatedAt: note.CreatedAt, UpdatedAt: created.Add(time.Minute),
	}, note.ContentHash)
	if err != nil {
		coordinator.Stop()
		service.Close()
		t.Fatal(err)
	}
	// A second ordinary write models an external editor save after the
	// service's atomic mutation and gives the fsnotify adapter a deterministic
	// content-bearing event to translate into the refresh request.
	data, err := vault.Serialize(updated)
	if err != nil {
		coordinator.Stop()
		service.Close()
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(project.VaultRoot, updated.RelativePath), data, 0o600); err != nil {
		coordinator.Stop()
		service.Close()
		t.Fatal(err)
	}
	requestDeadline := time.After(3 * time.Second)
	requestObserved := false
	for !requestObserved {
		select {
		case request := <-requests:
			if request.RelativePath != filepath.Clean(note.RelativePath) || request.ContentHash == "" {
				continue
			}
			requestObserved = true
		case <-requestDeadline:
			coordinator.Stop()
			service.Close()
			t.Fatal("timed out waiting for refresh request")
		}
	}

	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		results, queryErr := service.QueryRetrieval(ctx, project.ID, "reopened fixture", 10)
		if queryErr == nil && len(results) == 1 && results[0].SourceHash == updated.ContentHash {
			break
		}
		time.Sleep(25 * time.Millisecond)
	}
	results, err := service.QueryRetrieval(ctx, project.ID, "reopened fixture", 10)
	if !requestObserved || err != nil || len(results) != 1 || results[0].SourceHash != updated.ContentHash {
		coordinator.Stop()
		service.Close()
		t.Fatalf("updated query = %#v, err = %v", results, err)
	}
	if err := coordinator.Stop(); err != nil {
		service.Close()
		t.Fatal(err)
	}
	if err := service.Close(); err != nil {
		t.Fatal(err)
	}
	if err := reopenPhase2State(ctx, dataRoot, project.ID, project.VaultRoot); err != nil {
		t.Fatal(err)
	}
}

func reopenPhase2State(ctx context.Context, dataRoot string, projectID workspace.ProjectID, vaultRoot string) error {
	state, err := workspacesqlite.Open(ctx, filepath.Join(dataRoot, "state", "workspace.db"))
	if err != nil {
		return err
	}
	defer state.Close()
	project, err := state.GetProject(ctx, projectID)
	if err != nil {
		return err
	}
	if project.VaultRoot != vaultRoot {
		return workspace.NewError(workspace.ErrConflict, "reopened Vault root changed")
	}
	gitAdapter, err := workspacegit.NewWithRegistry(state, nil, filepath.Join(dataRoot, "worktrees"))
	if err != nil {
		return err
	}
	worktrees, err := gitAdapter.ListWorktrees(ctx, projectID)
	if err != nil {
		return err
	}
	if len(worktrees) != 1 || worktrees[0].ProjectID != projectID {
		return workspace.NewError(workspace.ErrConflict, "reopened worktree state is incomplete")
	}
	notes, err := state.ListNoteMetadata(ctx, projectID)
	if err != nil {
		return err
	}
	if len(notes) != 1 || notes[0].ContentHash == "" {
		return workspace.NewError(workspace.ErrConflict, "reopened note metadata is incomplete")
	}
	return nil
}

func initPhase2Repository(t *testing.T, root string) {
	t.Helper()
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	runPhase2Git(t, root, "init", "-b", "main")
	runPhase2Git(t, root, "config", "user.email", "test@example.invalid")
	runPhase2Git(t, root, "config", "user.name", "CortexOS Test")
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("fixture\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runPhase2Git(t, root, "add", "README.md")
	runPhase2Git(t, root, "commit", "-m", "initial")
}

func runPhase2Git(t *testing.T, root string, args ...string) {
	t.Helper()
	command := exec.Command("git", append([]string{"-C", root}, args...)...)
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("git %v failed: %v\n%s", args, err, output)
	}
}
