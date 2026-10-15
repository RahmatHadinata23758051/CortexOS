package orchestra

import (
	"context"
	"testing"
	"time"

	"github.com/RahmatHadinata23758051/CortexOS/internal/cortex/harness"
)

type mockHarnessEvidenceReader struct{}

func (m *mockHarnessEvidenceReader) EvidenceForExecution(_ context.Context, _ string) ([]harness.EvidenceRecord, error) {
	return nil, nil
}

type testWorktreeInspector struct{}

func (t *testWorktreeInspector) InspectWorktreeState(_ context.Context, id string) (WorktreeState, error) {
	return WorktreeState{WorktreeID: id, Valid: true}, nil
}

func TestProductionAdapter_CreateTask_AutoDispatchSuccess(t *testing.T) {
	store := NewMemoryTaskStore()
	broker := newTestBroker(t)
	bridge := NewHarnessBridge(broker, store, "/tmp/worktree")
	dispatcher := NewDispatcher(store, bridge, "test-worker")

	adapter, err := NewProductionAdapter(ProductionAdapterConfig{
		TaskStore:         store,
		HarnessStore:      &mockHarnessEvidenceReader{},
		WorktreeInspector: &testWorktreeInspector{},
		Dispatcher:        dispatcher,
	})
	if err != nil {
		t.Fatalf("NewProductionAdapter: %v", err)
	}

	task := validTask()
	task.ID = "task-autodispatch-1"
	task.Status = TaskReady
	task.MaxAttempts = 3

	created, err := adapter.CreateTask(context.Background(), task)
	if err != nil {
		t.Fatalf("CreateTask failed: %v", err)
	}
	if created.ID != task.ID {
		t.Fatalf("expected task ID %s, got %s", task.ID, created.ID)
	}

	// Poll until task transitions from ready -> running -> awaitingInspection
	var finalTask Task
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		tItem, getErr := store.GetTask(context.Background(), task.ID)
		if getErr == nil && tItem.Status == TaskAwaitingInspection {
			finalTask = tItem
			break
		}
		time.Sleep(20 * time.Millisecond)
	}

	if finalTask.Status != TaskAwaitingInspection {
		t.Fatalf("expected task to reach TaskAwaitingInspection, got %s", finalTask.Status)
	}

	// Verify events show lifecycle progression
	events, err := store.ListEvents(context.Background(), task.ID)
	if err != nil {
		t.Fatalf("ListEvents failed: %v", err)
	}
	var hasCreated, hasDispatched, hasEvidence bool
	for _, e := range events {
		switch e.Type {
		case EventTaskCreated:
			hasCreated = true
		case EventTaskDispatched:
			hasDispatched = true
		case EventEvidenceCollected:
			hasEvidence = true
		}
	}
	if !hasCreated || !hasDispatched || !hasEvidence {
		t.Fatalf("missing expected lifecycle events (created=%v, dispatched=%v, evidence=%v)",
			hasCreated, hasDispatched, hasEvidence)
	}
}

func TestProductionAdapter_CreateTask_AutoDispatchFailure(t *testing.T) {
	store := NewMemoryTaskStore()

	engine := newFakeEngine(harness.ToolKindNative, harness.CapabilityCoding)
	engine.SetExecuteFunc(func(ctx harness.ExecutionContext, env harness.ExecutionEnvelope) (harness.ToolResult, error) {
		return harness.ToolResult{}, harness.ErrorFor(harness.ErrAdapterFailure)
	})
	broker := newTestBroker(t, engine)
	bridge := NewHarnessBridge(broker, store, "/tmp/worktree")
	dispatcher := NewDispatcher(store, bridge, "test-worker")

	adapter, err := NewProductionAdapter(ProductionAdapterConfig{
		TaskStore:         store,
		HarnessStore:      &mockHarnessEvidenceReader{},
		WorktreeInspector: &testWorktreeInspector{},
		Dispatcher:        dispatcher,
	})
	if err != nil {
		t.Fatalf("NewProductionAdapter: %v", err)
	}

	task := validTask()
	task.ID = "task-autodispatch-fail-1"
	task.Status = TaskReady
	task.MaxAttempts = 3

	_, err = adapter.CreateTask(context.Background(), task)
	if err != nil {
		t.Fatalf("CreateTask failed: %v", err)
	}

	// Poll until task reaches TaskFailed
	var finalTask Task
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		tItem, getErr := store.GetTask(context.Background(), task.ID)
		if getErr == nil && tItem.Status == TaskFailed {
			finalTask = tItem
			break
		}
		time.Sleep(20 * time.Millisecond)
	}

	if finalTask.Status != TaskFailed {
		t.Fatalf("expected task to reach TaskFailed, got %s", finalTask.Status)
	}

	// Verify failure event was appended
	events, err := store.ListEvents(context.Background(), task.ID)
	if err != nil {
		t.Fatalf("ListEvents failed: %v", err)
	}
	var hasFailed bool
	for _, e := range events {
		if e.Type == EventExecutionFailed {
			hasFailed = true
			break
		}
	}
	if !hasFailed {
		t.Fatal("expected EventExecutionFailed on failure")
	}
}
