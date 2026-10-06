package watch

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/RahmatHadinata23758051/CortexOS/internal/cortex/workspace"
)

func TestWatcherEmitsScopedDebouncedChangesAndStopsIdempotently(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	watcher := New()
	changes, err := watcher.Start(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	defer watcher.Stop()
	path := filepath.Join(root, "note.md")
	if err := os.WriteFile(path, []byte("one\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("two\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	select {
	case change := <-changes:
		if change.RelativePath != "note.md" || change.RootID != root || change.Operation == workspace.FileError {
			t.Fatalf("change = %#v", change)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for file change")
	}
	if err := watcher.Stop(); err != nil {
		t.Fatal(err)
	}
	if err := watcher.Stop(); err != nil {
		t.Fatal(err)
	}
}

func TestWatcherRejectsCanceledAndRelativeRoots(t *testing.T) {
	t.Parallel()

	watcher := New()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := watcher.Start(ctx, t.TempDir()); workspace.ErrorCodeOf(err) != workspace.ErrCanceled {
		t.Fatalf("canceled error code = %q", workspace.ErrorCodeOf(err))
	}
	if _, err := watcher.Start(context.Background(), "relative/root"); workspace.ErrorCodeOf(err) != workspace.ErrPathDenied {
		t.Fatalf("relative error code = %q", workspace.ErrorCodeOf(err))
	}
}

func TestWatcherCreateDirectoryAndDeleteEvents(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	watcher := New()
	changes, err := watcher.Start(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	defer watcher.Stop()
	directory := filepath.Join(root, "nested")
	if err := os.Mkdir(directory, 0o700); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(directory, "note.md")
	if err := os.WriteFile(file, []byte("body\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(file); err != nil {
		t.Fatal(err)
	}
	deadline := time.After(2 * time.Second)
	for {
		select {
		case change := <-changes:
			if change.RelativePath == "nested/note.md" && change.Operation == workspace.FileRemoved {
				return
			}
		case <-deadline:
			t.Fatal("timed out waiting for nested delete")
		}
	}
}
