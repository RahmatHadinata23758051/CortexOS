package git

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/RahmatHadinata23758051/CortexOS/internal/cortex/workspace"
)

func TestApprovedTargetRejectsOutsideProjectRoot(t *testing.T) {
	t.Parallel()

	adapter, err := NewWithRegistry(workspace.NewMemoryWorkspace(), nil, filepath.Join(t.TempDir(), "worktrees"))
	if err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(t.TempDir(), "outside")
	if _, err := adapter.approvedTarget("project-1", outside); workspace.ErrorCodeOf(err) != workspace.ErrPathDenied {
		t.Fatalf("error code = %q, want %q", workspace.ErrorCodeOf(err), workspace.ErrPathDenied)
	}
}

func TestApprovedTargetRejectsExistingSymlinkEscape(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink creation requires elevated privileges on some Windows configurations")
	}
	t.Parallel()

	root := t.TempDir()
	adapter, err := NewWithRegistry(workspace.NewMemoryWorkspace(), nil, root)
	if err != nil {
		t.Fatal(err)
	}
	projectRoot := filepath.Join(root, "project-1")
	if err := os.MkdirAll(projectRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	outside := t.TempDir()
	link := filepath.Join(projectRoot, "escape")
	if err := os.Symlink(outside, link); err != nil {
		t.Skipf("symlink unsupported: %v", err)
	}
	if _, err := adapter.approvedTarget("project-1", filepath.Join(link, "worktree")); workspace.ErrorCodeOf(err) != workspace.ErrPathDenied {
		t.Fatalf("error code = %q, want %q", workspace.ErrorCodeOf(err), workspace.ErrPathDenied)
	}
}
