package refresh

import (
	"context"
	"errors"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/RahmatHadinata23758051/CortexOS/internal/cortex/workspace"
)

func TestCoordinatorPublishesRefreshRequestsAndFailures(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	memory := workspace.NewMemoryWorkspace()
	if err := memory.Open(ctx); err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	var mu sync.Mutex
	var handled []Request
	coordinator, err := New(memory, func(_ context.Context, request Request) error {
		mu.Lock()
		handled = append(handled, request)
		mu.Unlock()
		if request.RelativePath == "broken.md" {
			return errors.New("injected refresh failure")
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	requests := coordinator.Requests()
	failures := coordinator.Failures()
	if err := coordinator.Start(ctx, "project-1", root); err != nil {
		t.Fatal(err)
	}
	if err := memory.PublishChange(ctx, workspace.FileChange{RootID: root, RelativePath: "note.md", Operation: workspace.FileModified, ContentHash: "sha256:one", ObservedAt: time.Now().UTC()}); err != nil {
		t.Fatal(err)
	}
	if err := memory.PublishChange(ctx, workspace.FileChange{RootID: root, RelativePath: "broken.md", Operation: workspace.FileModified, ContentHash: "sha256:two", ObservedAt: time.Now().UTC()}); err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	deadline := time.After(2 * time.Second)
	for len(seen) < 2 {
		select {
		case request := <-requests:
			seen[request.RelativePath] = true
			if request.ProjectID != "project-1" || request.RootID != root || request.ContentHash == "" {
				t.Fatalf("request = %#v", request)
			}
		case failure := <-failures:
			if failure.Request.RelativePath != "broken.md" || failure.Error == nil {
				t.Fatalf("failure = %#v", failure)
			}
		case <-deadline:
			t.Fatalf("timed out; seen=%v", seen)
		}
	}
	if err := coordinator.Stop(); err != nil {
		t.Fatal(err)
	}
	mu.Lock()
	defer mu.Unlock()
	if len(handled) != 2 {
		t.Fatalf("handled = %#v", handled)
	}
}

func TestCoordinatorCoalescesSamePathAndStopsIdempotently(t *testing.T) {
	t.Parallel()

	memory := workspace.NewMemoryWorkspace()
	if err := memory.Open(context.Background()); err != nil {
		t.Fatal(err)
	}
	coordinator, err := New(memory, func(context.Context, Request) error { return nil })
	if err != nil {
		t.Fatal(err)
	}
	requests := coordinator.Requests()
	root := filepath.Clean(t.TempDir())
	if err := coordinator.Start(context.Background(), "project", root); err != nil {
		t.Fatal(err)
	}
	for _, hash := range []string{"one", "two", "three"} {
		if err := memory.PublishChange(context.Background(), workspace.FileChange{RootID: root, RelativePath: "note.md", Operation: workspace.FileModified, ContentHash: hash, ObservedAt: time.Now().UTC()}); err != nil {
			t.Fatal(err)
		}
	}
	select {
	case request := <-requests:
		if request.RelativePath != "note.md" {
			t.Fatalf("request = %#v", request)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for coalesced request")
	}
	select {
	case extra := <-requests:
		t.Fatalf("unexpected duplicate request = %#v", extra)
	case <-time.After(150 * time.Millisecond):
	}
	if err := coordinator.Stop(); err != nil {
		t.Fatal(err)
	}
	if err := coordinator.Stop(); err != nil {
		t.Fatal(err)
	}
}

func TestCoordinatorRequiresControlledDependencies(t *testing.T) {
	t.Parallel()

	if _, err := New(nil, func(context.Context, Request) error { return nil }); workspace.ErrorCodeOf(err) != workspace.ErrInvalidRequest {
		t.Fatalf("nil watcher error code = %q", workspace.ErrorCodeOf(err))
	}
}
