package watch

import (
	"context"
	"testing"

	"github.com/RahmatHadinata23758051/CortexOS/internal/cortex/workspace"
)

func TestWatcherCanRestartAfterStop(t *testing.T) {
	t.Parallel()

	watcher := New()
	first, err := watcher.Start(context.Background(), t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err := watcher.Stop(); err != nil {
		t.Fatal(err)
	}
	select {
	case _, ok := <-first:
		if ok {
			t.Fatal("first watcher channel still emitted after stop")
		}
	default:
	}
	if _, err := watcher.Start(context.Background(), t.TempDir()); err != nil {
		t.Fatalf("restart failed: %v", err)
	}
	if err := watcher.Stop(); err != nil {
		t.Fatal(err)
	}
}

func TestWatcherRejectsSecondStart(t *testing.T) {
	t.Parallel()

	watcher := New()
	if _, err := watcher.Start(context.Background(), t.TempDir()); err != nil {
		t.Fatal(err)
	}
	defer watcher.Stop()
	if _, err := watcher.Start(context.Background(), t.TempDir()); workspace.ErrorCodeOf(err) != workspace.ErrConflict {
		t.Fatalf("second start error code = %q", workspace.ErrorCodeOf(err))
	}
}
