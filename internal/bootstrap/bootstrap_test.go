package bootstrap_test

import (
	"context"
	"testing"

	"github.com/RahmatHadinata23758051/CortexOS/internal/bootstrap"
	"github.com/RahmatHadinata23758051/CortexOS/internal/cortex/workspace"
)

func TestOpenOrchestra_Lifecycle(t *testing.T) {
	dir := t.TempDir()
	ctx := context.Background()

	// 1. Open workspace service
	wsService, closeWS, err := bootstrap.OpenWorkspace(ctx, dir)
	if err != nil {
		t.Fatalf("open workspace: %v", err)
	}
	defer func() { _ = closeWS() }()

	canonicalRoot, err := workspace.CanonicalRoot(dir)
	if err != nil {
		t.Fatalf("canonical root: %v", err)
	}

	// 2. Open orchestra port
	orchPort, closeOrch, err := bootstrap.OpenOrchestra(ctx, canonicalRoot, wsService)
	if err != nil {
		t.Fatalf("open orchestra: %v", err)
	}

	if orchPort == nil {
		t.Fatal("expected non-nil orchestra port")
	}

	// 3. Verify read operations work against clean database
	tasks, err := orchPort.ListTasks(ctx)
	if err != nil {
		t.Fatalf("list tasks from clean store: %v", err)
	}
	if len(tasks) != 0 {
		t.Fatalf("expected 0 initial tasks, got %d", len(tasks))
	}

	// 4. Close orchestra port cleanly
	if err := closeOrch(); err != nil {
		t.Fatalf("close orchestra: %v", err)
	}
}
