package git

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"github.com/RahmatHadinata23758051/CortexOS/internal/cortex/workspace"
)

func TestParseWorktreeListKeepsTypedFacts(t *testing.T) {
	t.Parallel()

	entries := parseWorktreeList("worktree C:\\worktrees\\one\nHEAD abc123\nbranch refs/heads/feature/one\n\nworktree C:\\worktrees\\two\nHEAD def456\ndetached\n")
	if len(entries) != 2 {
		t.Fatalf("entries = %#v", entries)
	}
	if entries[0].Path != `C:\worktrees\one` || entries[0].Revision != "abc123" || entries[0].Branch != "feature/one" {
		t.Fatalf("first entry = %#v", entries[0])
	}
	if entries[1].Branch != "" || entries[1].Revision != "def456" {
		t.Fatalf("detached entry = %#v", entries[1])
	}
}

func TestAdapterRejectsCanceledOperationsBeforeRunner(t *testing.T) {
	t.Parallel()

	adapter, err := NewWithRunner(&scriptedRunner{})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := adapter.InspectRepository(ctx, filepath.Join(t.TempDir(), "repo")); workspace.ErrorCodeOf(err) != workspace.ErrCanceled {
		t.Fatalf("error code = %q, want %q", workspace.ErrorCodeOf(err), workspace.ErrCanceled)
	}
}

func TestClassifyGitErrorDoesNotExposeProcessOutput(t *testing.T) {
	t.Parallel()

	err := classifyGitError([]byte("fatal: token=secret-value not a git repository\n"), errors.New("exit status 128"))
	if workspace.ErrorCodeOf(err) != workspace.ErrNotGitRepository {
		t.Fatalf("error code = %q", workspace.ErrorCodeOf(err))
	}
	if err.Error() != "workspace.not_git_repository: path is not a Git repository" {
		t.Fatalf("safe error = %q", err.Error())
	}
}
