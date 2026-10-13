package bridge

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/RahmatHadinata23758051/CortexOS/internal/cortex/workspace"
	workspacegit "github.com/RahmatHadinata23758051/CortexOS/internal/cortex/workspace/git"
	"github.com/RahmatHadinata23758051/CortexOS/internal/cortex/workspace/retrieval"
	workspacesqlite "github.com/RahmatHadinata23758051/CortexOS/internal/cortex/workspace/sqlite"
	"github.com/RahmatHadinata23758051/CortexOS/internal/cortex/workspace/vault"
	"github.com/RahmatHadinata23758051/CortexOS/internal/cortex/workspace/watch"
)

func newTestWorkspaceService(t *testing.T) (*workspace.Service, string) {
	t.Helper()
	dir := t.TempDir()
	ctx := context.Background()

	state, err := workspacesqlite.Open(ctx, filepath.Join(dir, "state.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = state.Close() })

	vDir := filepath.Join(dir, "vault")
	if err := os.MkdirAll(vDir, 0o700); err != nil {
		t.Fatal(err)
	}
	vaultStore, err := vault.NewWithMetadata(vDir, state)
	if err != nil {
		t.Fatal(err)
	}

	retDir := filepath.Join(dir, "retrieval")
	if err := os.MkdirAll(retDir, 0o700); err != nil {
		t.Fatal(err)
	}
	retIndex, err := retrieval.NewWithSource(retDir, vaultStore)
	if err != nil {
		t.Fatal(err)
	}

	gitAdapter, err := workspacegit.NewWithRegistry(state, nil, filepath.Join(dir, "worktrees"))
	if err != nil {
		t.Fatal(err)
	}

	svc, err := workspace.NewService(workspace.Dependencies{
		State: state, Projects: state, Worktrees: gitAdapter, Vault: vaultStore,
		Watcher: watch.New(), Retrieval: retIndex,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.Open(ctx); err != nil {
		t.Fatal(err)
	}

	// Register test project
	_, err = svc.RegisterProject(ctx, workspace.Project{
		ID: "p-test", Name: "Test Project", RepositoryRoot: dir,
		VaultRoot: vDir, DefaultBranch: "main",
	})
	if err != nil {
		t.Fatal(err)
	}
	return svc, "p-test"
}

func TestWorkspaceKnowledgeAdapter_HappyPath(t *testing.T) {
	ws, projectID := newTestWorkspaceService(t)
	adapter, err := NewWorkspaceKnowledgeAdapter(ws)
	if err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	now := time.Now().UTC()
	_, err = ws.CreateNote(ctx, workspace.VaultNote{
		ID: "note-1", ProjectID: workspace.ProjectID(projectID),
		RelativePath: "arch.md", Title: "System Architecture",
		Body: "Clean decoupled architecture pattern.", Status: "active",
		FormatVersion: vault.FormatVersion, Source: "test", Author: "tester",
		CreatedAt: now, UpdatedAt: now,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := ws.RebuildRetrieval(ctx, workspace.ProjectID(projectID)); err != nil {
		t.Fatal(err)
	}

	sources, err := adapter.ListKnowledgeSources(ctx, projectID)
	if err != nil {
		t.Fatalf("ListKnowledgeSources failed: %v", err)
	}
	if len(sources) != 1 || sources[0].ID != "note-1" {
		t.Fatalf("unexpected sources: %#v", sources)
	}

	matches, err := adapter.QueryKnowledge(ctx, projectID, "architecture", 5)
	if err != nil {
		t.Fatalf("QueryKnowledge failed: %v", err)
	}
	if len(matches) != 1 || matches[0].Score <= 0 {
		t.Fatalf("unexpected query matches: %#v", matches)
	}
}

func TestWorkspaceKnowledgeAdapter_RejectsInvalid(t *testing.T) {
	if _, err := NewWorkspaceKnowledgeAdapter(nil); err == nil {
		t.Fatal("expected error for nil workspace")
	}
	ws, _ := newTestWorkspaceService(t)
	adapter, _ := NewWorkspaceKnowledgeAdapter(ws)

	if _, err := adapter.ListKnowledgeSources(context.Background(), ""); err == nil {
		t.Fatal("expected error for empty projectID")
	}
	if _, err := adapter.QueryKnowledge(context.Background(), "", "query", 5); err == nil {
		t.Fatal("expected error for empty projectID")
	}
	if _, err := adapter.QueryKnowledge(context.Background(), "p-1", "", 5); err == nil {
		t.Fatal("expected error for empty query")
	}
}
