package orchestra_test

import (
	"context"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/RahmatHadinata23758051/CortexOS/internal/cortex/orchestra"
	orchestrasqlite "github.com/RahmatHadinata23758051/CortexOS/internal/cortex/orchestra/sqlite"
)

func TestProductionAdapter_MergeTask_ConcurrentOnlyOneSucceeds(t *testing.T) {
	adapter, orchStore, harnessStore := setupTestAdapter(t)
	ctx := context.Background()

	task, _ := seedTaskAwaitingInspection(t, orchStore, harnessStore, "concurrent-task", "wt-1")

	const callers = 4
	var wg sync.WaitGroup
	results := make([]error, callers)
	wg.Add(callers)
	for i := 0; i < callers; i++ {
		go func(idx int) {
			defer wg.Done()
			_, err := adapter.MergeTask(ctx, task.ID, "concurrent request")
			results[idx] = err
		}(i)
	}
	wg.Wait()

	successes := 0
	for _, err := range results {
		if err == nil {
			successes++
		}
	}
	if successes != 1 {
		t.Fatalf("expected exactly one successful merge, got %d (errors: %v)", successes, results)
	}

	persisted, err := orchStore.GetTask(ctx, task.ID)
	if err != nil {
		t.Fatalf("read merged task: %v", err)
	}
	if persisted.Status != orchestra.TaskSuccess {
		t.Fatalf("merged status = %s, want success", persisted.Status)
	}

	events, err := orchStore.ListEvents(ctx, task.ID)
	if err != nil {
		t.Fatalf("list events: %v", err)
	}
	acceptedCount := 0
	for _, ev := range events {
		if ev.Type == orchestra.EventInspectionAccepted {
			acceptedCount++
		}
	}
	if acceptedCount != 1 {
		t.Fatalf("accepted events = %d, want exactly one", acceptedCount)
	}
}

func TestCommitMerge_PersistsAcrossReopen(t *testing.T) {
	path := filepath.Join(t.TempDir(), "orchestra.db")
	ctx := context.Background()

	store, err := orchestrasqlite.Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}

	seed := func(target *orchestrasqlite.Store) {
		parent := orchestra.Task{
			ID: "reopen-parent", ProjectID: "p-reopen", WorktreeID: "wt-reopen",
			Title: "reopen merge test", AcceptanceCriteria: []string{"survives restart"},
			Status: orchestra.TaskAwaitingInspection, AttemptCount: 1, MaxAttempts: 3,
			CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC(), SchemaVersion: orchestra.ContractVersion,
		}
		if err := target.SaveTask(ctx, parent); err != nil {
			t.Fatalf("save parent: %v", err)
		}
		child := orchestra.Task{
			ID: "reopen-child", ProjectID: "p-reopen", WorktreeID: "wt-reopen",
			Title: "dependent reopen", AcceptanceCriteria: []string{"child ready"},
			Dependencies: []orchestra.TaskID{"reopen-parent"}, Status: orchestra.TaskDraft,
			AttemptCount: 0, MaxAttempts: 3, CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC(),
			SchemaVersion: orchestra.ContractVersion,
		}
		if err := target.SaveTask(ctx, child); err != nil {
			t.Fatalf("save child: %v", err)
		}
		if err := target.SaveExecution(ctx, orchestra.Execution{
			ID: "reopen-parent-1", TaskID: "reopen-parent", Attempt: 1,
			Status: orchestra.TaskAwaitingInspection, StartedAt: time.Now().UTC(),
		}); err != nil {
			t.Fatalf("save execution: %v", err)
		}
	}
	seed(store)

	parent, _ := store.GetTask(ctx, "reopen-parent")
	child, _ := store.GetTask(ctx, "reopen-child")
	res, _ := orchestra.ApplyTransition(parent, orchestra.TransitionInspectAccepted)
	childRes, _ := orchestra.ApplyTransition(child, orchestra.TransitionValidate)
	now := time.Now().UTC()
	res.Task.UpdatedAt = now
	childRes.Task.UpdatedAt = now

	err = store.CommitMerge(ctx, orchestra.MergeCommit{
		MergedTask: res.Task,
		AcceptedEvent: orchestra.TaskEvent{
			TaskID: "reopen-parent", ExecutionID: "reopen-parent-1", Type: orchestra.EventInspectionAccepted,
			From: orchestra.TaskAwaitingInspection, To: orchestra.TaskSuccess,
			Message: "reopen merge", OccurredAt: now,
		},
		UnblockedTasks: []orchestra.Task{childRes.Task},
		ValidatedEvents: []orchestra.TaskEvent{
			{TaskID: "reopen-child", Type: orchestra.EventTaskValidated, From: orchestra.TaskDraft, To: orchestra.TaskReady, Message: "unblocked", OccurredAt: now},
		},
	})
	if err != nil {
		_ = store.Close()
		t.Fatalf("commit merge: %v", err)
	}
	if err := store.Close(); err != nil {
		t.Fatalf("close store: %v", err)
	}

	reopened, err := orchestrasqlite.Open(ctx, path)
	if err != nil {
		t.Fatalf("reopen orchestra db: %v", err)
	}
	defer reopened.Close()

	merged, err := reopened.GetTask(ctx, "reopen-parent")
	if err != nil {
		t.Fatalf("get parent after reopen: %v", err)
	}
	if merged.Status != orchestra.TaskSuccess {
		t.Fatalf("parent after reopen = %s, want success", merged.Status)
	}
	dep, err := reopened.GetTask(ctx, "reopen-child")
	if err != nil {
		t.Fatalf("get child after reopen: %v", err)
	}
	if dep.Status != orchestra.TaskReady {
		t.Fatalf("child after reopen = %s, want ready", dep.Status)
	}
	events, err := reopened.ListEvents(ctx, "reopen-parent")
	if err != nil {
		t.Fatalf("list events after reopen: %v", err)
	}
	found := false
	for _, ev := range events {
		if ev.Type == orchestra.EventInspectionAccepted && ev.To == orchestra.TaskSuccess {
			found = true
		}
	}
	if !found {
		t.Fatal("expected accepted event after reopen")
	}
	childEvents, err := reopened.ListEvents(ctx, "reopen-child")
	if err != nil {
		t.Fatalf("list child events after reopen: %v", err)
	}
	if len(childEvents) == 0 {
		t.Fatal("expected validated event after reopen")
	}
}

// nonAtomicFailingStore embeds TaskStore but does NOT implement MergeCommitter,
// and fails when saving a targeted task.
type nonAtomicFailingStore struct {
	orchestra.TaskStore
	failTaskID orchestra.TaskID
}

func (f *nonAtomicFailingStore) SaveTask(ctx context.Context, task orchestra.Task) error {
	if task.ID == f.failTaskID {
		return context.DeadlineExceeded
	}
	return f.TaskStore.SaveTask(ctx, task)
}

func TestProductionAdapter_MergeTask_NonAtomicFallbackReportsFailure(t *testing.T) {
	_, orchStore, harnessStore := setupTestAdapter(t)
	ctx := context.Background()

	task, _ := seedTaskAwaitingInspection(t, orchStore, harnessStore, "fallback-parent", "wt-1")
	child := orchestra.Task{
		ID: "fallback-child", ProjectID: task.ProjectID, WorktreeID: "wt-1",
		Title: "fallback dependent", AcceptanceCriteria: []string{"parent done"},
		Dependencies: []orchestra.TaskID{task.ID}, Status: orchestra.TaskDraft,
		AttemptCount: 0, MaxAttempts: 3, CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC(),
		SchemaVersion: orchestra.ContractVersion,
	}
	if err := orchStore.SaveTask(ctx, child); err != nil {
		t.Fatalf("save child: %v", err)
	}

	fallback := &nonAtomicFailingStore{TaskStore: orchStore, failTaskID: "fallback-child"}
	adapter, err := orchestra.NewProductionAdapter(orchestra.ProductionAdapterConfig{
		TaskStore:         fallback,
		HarnessStore:      harnessStore,
		WorktreeInspector: &mockWorktreeInspector{worktreeID: "wt-1", revision: "rev-1", valid: true},
		Inspector:         orchestra.MergeAuthority{Capability: orchestra.MergeCapability},
	})
	if err != nil {
		t.Fatalf("create adapter: %v", err)
	}

	if _, err := adapter.MergeTask(ctx, task.ID, "fallback failure"); err == nil {
		t.Fatal("expected fallback failure when child save fails, got nil")
	}
}
