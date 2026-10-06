package watch

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/RahmatHadinata23758051/CortexOS/internal/cortex/workspace"
	"github.com/fsnotify/fsnotify"
)

func TestWatcherRejectsEventOutsideRoot(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	outside := t.TempDir()
	change, accepted := New().toChange(root, fsnotify.Event{Name: filepath.Join(outside, "outside.md"), Op: fsnotify.Write})
	if accepted || change.RelativePath != "" {
		t.Fatalf("outside event accepted: %#v", change)
	}
}

func TestWatcherRejectsSymlinkEventEscapeWhenSupported(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink creation may require elevation on Windows")
	}
	t.Parallel()

	root := t.TempDir()
	outside := t.TempDir()
	link := filepath.Join(root, "escape")
	if err := os.Symlink(outside, link); err != nil {
		t.Skipf("symlink unsupported: %v", err)
	}
	change, accepted := New().toChange(root, fsnotify.Event{Name: filepath.Join(link, "outside.md"), Op: fsnotify.Write})
	if accepted || workspace.ErrorCodeOf(verifyEventContainment(root, filepath.Join(link, "outside.md"))) != workspace.ErrPathDenied || change.RelativePath != "" {
		t.Fatalf("symlink event accepted: %#v", change)
	}
}
