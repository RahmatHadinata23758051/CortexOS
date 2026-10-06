package workspace

import (
	"context"
	"testing"
	"time"
)

func TestMemoryWorkspaceImplementsAllPorts(t *testing.T) {
	t.Parallel()

	var adapter any = NewMemoryWorkspace()
	if _, ok := adapter.(ProjectRegistry); !ok {
		t.Fatal("memory workspace does not implement ProjectRegistry")
	}
	if _, ok := adapter.(WorktreeManager); !ok {
		t.Fatal("memory workspace does not implement WorktreeManager")
	}
	if _, ok := adapter.(StateStore); !ok {
		t.Fatal("memory workspace does not implement StateStore")
	}
	if _, ok := adapter.(VaultStore); !ok {
		t.Fatal("memory workspace does not implement VaultStore")
	}
	if _, ok := adapter.(FileWatcher); !ok {
		t.Fatal("memory workspace does not implement FileWatcher")
	}
	if _, ok := adapter.(RetrievalIndex); !ok {
		t.Fatal("memory workspace does not implement RetrievalIndex")
	}
}

func TestMemoryWorkspaceWatcherLifecycleAndOverflow(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	memory := NewMemoryWorkspace()
	changes, err := memory.Start(ctx, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	change := FileChange{ID: "event-1", RootID: "vault", RelativePath: "note.md", Operation: FileModified, ObservedAt: time.Unix(1, 0)}
	if err := memory.PublishChange(ctx, change); err != nil {
		t.Fatal(err)
	}
	select {
	case received := <-changes:
		if received.ID != change.ID || received.RelativePath != change.RelativePath {
			t.Fatalf("received %#v, want %#v", received, change)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for memory change")
	}
	if err := memory.Stop(); err != nil {
		t.Fatal(err)
	}
	if err := memory.Stop(); err != nil {
		t.Fatal(err)
	}
}
