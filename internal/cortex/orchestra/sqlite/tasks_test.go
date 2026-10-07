package sqlite

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/RahmatHadinata23758051/CortexOS/internal/cortex/orchestra"
)

func testTask() orchestra.Task {
	return orchestra.Task{
		ID:                 "task-1",
		ProjectID:          "project-1",
		WorktreeID:         "worktree-1",
		Title:              "persist task",
		AcceptanceCriteria: []string{"evidence is recorded"},
		Status:             orchestra.TaskReady,
		MaxAttempts:        2,
		SchemaVersion:      orchestra.ContractVersion,
	}
}

func TestStorePersistsTasksExecutionsAndEventsAcrossReopen(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state", "orchestra.db")
	ctx := context.Background()
	store, err := Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}

	task := testTask()
	if err := store.SaveTask(ctx, task); err != nil {
		t.Fatal(err)
	}
	execution := orchestra.Execution{
		ID:        "task-1-1",
		TaskID:    task.ID,
		Attempt:   1,
		Status:    orchestra.TaskRunning,
		StartedAt: time.Now().UTC(),
	}
	if err := store.SaveExecution(ctx, execution); err != nil {
		t.Fatal(err)
	}
	if err := store.AppendEvent(ctx, orchestra.TaskEvent{TaskID: task.ID, ExecutionID: execution.ID, Type: orchestra.EventTaskDispatched, From: orchestra.TaskReady, To: orchestra.TaskRunning}); err != nil {
		t.Fatal(err)
	}
	if err := store.AppendEvent(ctx, orchestra.TaskEvent{TaskID: task.ID, ExecutionID: execution.ID, Type: orchestra.EventEvidenceCollected, From: orchestra.TaskRunning, To: orchestra.TaskAwaitingInspection, EvidenceIDs: []string{"evidence-1"}}); err != nil {
		t.Fatal(err)
	}
	if err := store.RegisterActive(ctx, task.ID, execution.ID, "worker-1", time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}

	reopened, err := Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	loaded, err := reopened.GetTask(ctx, task.ID)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.ID != task.ID || loaded.Title != task.Title || loaded.MaxAttempts != task.MaxAttempts {
		t.Fatalf("loaded task = %#v", loaded)
	}
	loadedExecution, err := reopened.GetExecution(ctx, execution.ID)
	if err != nil {
		t.Fatal(err)
	}
	if loadedExecution.TaskID != task.ID || loadedExecution.Attempt != 1 {
		t.Fatalf("loaded execution = %#v", loadedExecution)
	}
	events, err := reopened.ListEvents(ctx, task.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 2 || events[0].Sequence != 1 || events[1].Sequence != 2 || events[1].EvidenceIDs[0] != "evidence-1" {
		t.Fatalf("loaded events = %#v", events)
	}
}

func TestAppendEventConcurrentSequencesAreUnique(t *testing.T) {
	store, err := Open(context.Background(), filepath.Join(t.TempDir(), "orchestra.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	ctx := context.Background()
	if err := store.SaveTask(ctx, testTask()); err != nil {
		t.Fatal(err)
	}
	const count = 32
	for i := 0; i < count; i++ {
		if err := store.AppendEvent(ctx, orchestra.TaskEvent{TaskID: "task-1", Type: orchestra.EventTaskValidated, Message: "concurrent"}); err != nil {
			t.Fatal(err)
		}
	}
	events, err := store.ListEvents(ctx, "task-1")
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != count {
		t.Fatalf("event count = %d, want %d", len(events), count)
	}
	for i, event := range events {
		if event.Sequence != int64(i+1) {
			t.Fatalf("event %d sequence = %d", i, event.Sequence)
		}
	}
}

func TestStoreRejectsMissingTasksAndHonorsCancellation(t *testing.T) {
	store, err := Open(context.Background(), filepath.Join(t.TempDir(), "orchestra.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if _, err := store.GetTask(context.Background(), "missing"); !errors.Is(err, orchestra.ErrNotFound) {
		t.Fatalf("missing task error = %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := store.SaveTask(ctx, testTask()); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled save error = %v", err)
	}
}

func TestRecoverInterruptedTasksIsDeterministicAndBounded(t *testing.T) {
	path := filepath.Join(t.TempDir(), "orchestra.db")
	ctx := context.Background()
	store, err := Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	first := testTask()
	first.ID = "task-a"
	first.Status = orchestra.TaskRunning
	first.AttemptCount = 1
	if err := store.SaveTask(ctx, first); err != nil {
		t.Fatal(err)
	}
	second := testTask()
	second.ID = "task-b"
	second.Status = orchestra.TaskRunning
	second.AttemptCount = 2
	if err := store.SaveTask(ctx, second); err != nil {
		t.Fatal(err)
	}
	if err := store.SaveExecution(ctx, orchestra.Execution{ID: "task-a-1", TaskID: first.ID, Attempt: 1, Status: orchestra.TaskRunning}); err != nil {
		t.Fatal(err)
	}
	if err := store.SaveExecution(ctx, orchestra.Execution{ID: "task-b-2", TaskID: second.ID, Attempt: 2, Status: orchestra.TaskRunning}); err != nil {
		t.Fatal(err)
	}
	if err := store.RegisterActive(ctx, first.ID, "task-a-1", "worker-a", time.Now()); err != nil {
		t.Fatal(err)
	}
	if err := store.RegisterActive(ctx, second.ID, "task-b-2", "worker-b", time.Now()); err != nil {
		t.Fatal(err)
	}

	recovered, err := store.RecoverInterrupted(ctx, "restart")
	if err != nil {
		t.Fatal(err)
	}
	if len(recovered) != 2 || recovered[0] != "task-a" || recovered[1] != "task-b" {
		t.Fatalf("recovered = %#v", recovered)
	}
	loaded, err := store.GetTask(ctx, "task-a")
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Status != orchestra.TaskReady {
		t.Fatalf("task-a status = %q", loaded.Status)
	}
	loaded, err = store.GetTask(ctx, "task-b")
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Status != orchestra.TaskFailed {
		t.Fatalf("task-b status = %q", loaded.Status)
	}
}
