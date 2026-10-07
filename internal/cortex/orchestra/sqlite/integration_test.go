package sqlite

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/RahmatHadinata23758051/CortexOS/internal/cortex/orchestra"
)

func TestSQLiteDispatcherFullLifecycle(t *testing.T) {
	path := filepath.Join(t.TempDir(), "orchestra.db")
	ctx := context.Background()
	store, err := Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	// Create and save initial task (start as Ready since we validate it first)
	task := orchestra.Task{
		ID:                 "integration-task",
		ProjectID:          "project-1",
		WorktreeID:         "worktree-1",
		Title:              "integration test task",
		AcceptanceCriteria: []string{"task completes"},
		Status:             orchestra.TaskReady,
		MaxAttempts:        2,
		SchemaVersion:      orchestra.ContractVersion,
	}
	if err := store.SaveTask(ctx, task); err != nil {
		t.Fatal(err)
	}

	// Dispatch task - persists execution and transitions to running
	_, err = store.GetExecution(ctx, "integration-task-1")
	if err != orchestra.ErrNotFound {
		t.Fatalf("execution should not exist before dispatch, got %v", err)
	}

	// Simulate dispatcher actions via store directly
	result, err := orchestra.ApplyTransition(task, orchestra.TransitionDispatch)
	if err != nil {
		t.Fatalf("dispatch transition failed: %v", err)
	}
	task = result.Task
	if err := store.SaveTask(ctx, task); err != nil {
		t.Fatal(err)
	}

	exec := orchestra.Execution{
		ID:        "integration-task-1",
		TaskID:    task.ID,
		Attempt:   task.AttemptCount,
		Status:    orchestra.TaskRunning,
		StartedAt: time.Now().UTC(),
	}
	if err := store.SaveExecution(ctx, exec); err != nil {
		t.Fatal(err)
	}

	// Verify task is running and execution persisted
	savedTask, err := store.GetTask(ctx, task.ID)
	if err != nil {
		t.Fatal(err)
	}
	if savedTask.Status != orchestra.TaskRunning {
		t.Fatalf("task status = %q, want %q", savedTask.Status, orchestra.TaskRunning)
	}

	persistedExec, err := store.GetExecution(ctx, exec.ID)
	if err != nil {
		t.Fatal(err)
	}
	if persistedExec.Status != orchestra.TaskRunning {
		t.Fatalf("execution status = %q, want %q", persistedExec.Status, orchestra.TaskRunning)
	}

	// Register active task for recovery semantics
	if err := store.RegisterActive(ctx, task.ID, exec.ID, "worker-1", time.Now()); err != nil {
		t.Fatal(err)
	}

	// Complete execution - simulates worker completing with evidence collection
	finishedAt := time.Now().UTC()
	persistedExec.FinishedAt = &finishedAt
	persistedExec.Status = orchestra.TaskAwaitingInspection
	persistedExec.EvidenceIDs = []string{"test-evidence"}
	if err := store.SaveExecution(ctx, persistedExec); err != nil {
		t.Fatal(err)
	}

	result, err = orchestra.ApplyTransition(savedTask, orchestra.TransitionCollectEvidence)
	if err != nil {
		t.Fatal(err)
	}
	savedTask = result.Task
	if err := store.SaveTask(ctx, savedTask); err != nil {
		t.Fatal(err)
	}

	event := orchestra.TaskEvent{
		TaskID:      task.ID,
		ExecutionID: exec.ID,
		Type:        orchestra.EventEvidenceCollected,
		From:        orchestra.TaskRunning,
		To:          orchestra.TaskAwaitingInspection,
		EvidenceIDs: []string{"test-evidence"},
		OccurredAt:  finishedAt,
	}
	if err := store.AppendEvent(ctx, event); err != nil {
		t.Fatal(err)
	}

	// Load task and verify awaiting inspection status
	loadedTask, err := store.GetTask(ctx, task.ID)
	if err != nil {
		t.Fatal(err)
	}
	if loadedTask.Status != orchestra.TaskAwaitingInspection {
		t.Fatalf("task status after completion = %q, want %q", loadedTask.Status, orchestra.TaskAwaitingInspection)
	}

	events, err := store.ListEvents(ctx, task.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) < 1 {
		t.Fatalf("expected at least 1 event, got %d", len(events))
	}
	foundCompletion := false
	for _, e := range events {
		if e.Type == orchestra.EventEvidenceCollected {
			foundCompletion = true
		}
	}
	if !foundCompletion {
		t.Fatal("missing evidence collected event")
	}

	// Simulate acceptance via transition
	result, err = orchestra.ApplyTransition(loadedTask, orchestra.TransitionInspectAccepted)
	if err != nil {
		t.Fatal(err)
	}
	finalTask := result.Task
	if err := store.SaveTask(ctx, finalTask); err != nil {
		t.Fatal(err)
	}

	finalLoaded, err := store.GetTask(ctx, finalTask.ID)
	if err != nil {
		t.Fatal(err)
	}
	if finalLoaded.Status != orchestra.TaskSuccess {
		t.Fatalf("final status = %q, want %q", finalLoaded.Status, orchestra.TaskSuccess)
	}

	// Execute recovery on non-existent interrupted task - should be no-op
	recovered, err := store.RecoverInterrupted(ctx, "test-recovery")
	if err != nil {
		t.Fatal(err)
	}
	if len(recovered) > 0 {
		t.Fatalf("recovered tasks = %v, expected none for successful task", recovered)
	}

	// Cancel a fresh ready task and verify persistence
	cancelTask := task
	cancelTask.ID = "cancel-task"
	cancelTask.Status = orchestra.TaskReady
	cancelTask.AttemptCount = 0
	if err := store.SaveTask(ctx, cancelTask); err != nil {
		t.Fatal(err)
	}

	cancelExec := orchestra.Execution{
		ID:        "cancel-task-1",
		TaskID:    cancelTask.ID,
		Attempt:   1,
		Status:    orchestra.TaskRunning,
		StartedAt: time.Now().UTC(),
	}
	if err := store.SaveExecution(ctx, cancelExec); err != nil {
		t.Fatal(err)
	}
	if err := store.RegisterActive(ctx, cancelTask.ID, cancelExec.ID, "worker-2", time.Now()); err != nil {
		t.Fatal(err)
	}

	cancelResult, err := orchestra.ApplyTransition(cancelTask, orchestra.TransitionCancel)
	if err != nil {
		t.Fatal(err)
	}
	cancelTask = cancelResult.Task
	if err := store.SaveTask(ctx, cancelTask); err != nil {
		t.Fatal(err)
	}

	cancelExec.FinishedAt = &finishedAt
	cancelExec.Status = orchestra.TaskCanceled
	if err := store.SaveExecution(ctx, cancelExec); err != nil {
		t.Fatal(err)
	}

	cancelEvent := orchestra.TaskEvent{
		TaskID:     cancelTask.ID,
		Type:       orchestra.EventTaskCanceled,
		From:       orchestra.TaskRunning,
		To:         orchestra.TaskCanceled,
		OccurredAt: finishedAt,
	}
	if err := store.AppendEvent(ctx, cancelEvent); err != nil {
		t.Fatal(err)
	}

	cancelLoaded, err := store.GetTask(ctx, cancelTask.ID)
	if err != nil {
		t.Fatal(err)
	}
	if cancelLoaded.Status != orchestra.TaskCanceled {
		t.Fatalf("canceled task status = %q, want %q", cancelLoaded.Status, orchestra.TaskCanceled)
	}

	canceledExec, err := store.GetExecution(ctx, cancelExec.ID)
	if err != nil {
		t.Fatal(err)
	}
	if canceledExec.Status != orchestra.TaskCanceled {
		t.Fatalf("canceled execution status = %q, want %q", canceledExec.Status, orchestra.TaskCanceled)
	}
	if canceledExec.FinishedAt == nil || canceledExec.FinishedAt.IsZero() {
		t.Fatal("canceled execution missing finished timestamp")
	}

	// List all tasks and verify both completed and canceled exist
	allTasks, err := store.ListTasks(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(allTasks) < 2 {
		t.Fatalf("expected at least 2 tasks, got %d", len(allTasks))
	}

	successFound := false
	cancelFound := false
	for _, t := range allTasks {
		if t.ID == "integration-task" && t.Status == orchestra.TaskSuccess {
			successFound = true
		}
		if t.ID == "cancel-task" && t.Status == orchestra.TaskCanceled {
			cancelFound = true
		}
	}
	if !successFound || !cancelFound {
		t.Fatalf("not all tasks found correctly, success=%v, cancel=%v", successFound, cancelFound)
	}
}

func TestSQLiteForeignKeysEnforced(t *testing.T) {
	path := filepath.Join(t.TempDir(), "fk-test.db")
	ctx := context.Background()
	store, err := Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	// Verify that the connection was opened with foreign-key enforcement.
	var fkOn int
	if err := store.db.QueryRowContext(ctx, `PRAGMA foreign_keys`).Scan(&fkOn); err != nil {
		t.Fatalf("failed to check FK pragma: %v", err)
	}
	if fkOn != 1 {
		t.Fatalf("foreign keys not enabled, got %d", fkOn)
	}
	if err := store.AppendEvent(ctx, orchestra.TaskEvent{TaskID: "nonexistent", Type: orchestra.EventTaskValidated, Sequence: 1}); err == nil {
		t.Fatal("expected foreign-key violation for nonexistent task")
	}
}

func TestSQLiteRecoveryExcludesOwnedTasks(t *testing.T) {
	path := filepath.Join(t.TempDir(), "recovery-test.db")
	ctx := context.Background()
	store, err := Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	// Create two running tasks - one registered, one not
	task1 := testTask()
	task1.ID = "owned-task"
	task1.Status = orchestra.TaskRunning
	task1.AttemptCount = 1
	if err := store.SaveTask(ctx, task1); err != nil {
		t.Fatal(err)
	}
	exec1 := orchestra.Execution{ID: "owned-exec", TaskID: task1.ID, Attempt: 1, Status: orchestra.TaskRunning}
	if err := store.SaveExecution(ctx, exec1); err != nil {
		t.Fatal(err)
	}

	task2 := testTask()
	task2.ID = "unowned-task"
	task2.Status = orchestra.TaskRunning
	task2.AttemptCount = 2 // Will fail on recovery
	if err := store.SaveTask(ctx, task2); err != nil {
		t.Fatal(err)
	}

	// Only register task1 as owned
	now := time.Now()
	if err := store.RegisterActive(ctx, task1.ID, "owned-exec", "worker-a", now); err != nil {
		t.Fatal(err)
	}

	// Recovery should only recover task1 (owned), not task2 (unowned)
	recovered, err := store.RecoverInterrupted(ctx, "test")
	if err != nil {
		t.Fatal(err)
	}
	if len(recovered) != 1 || recovered[0] != "owned-task" {
		t.Fatalf("recovered = %#v, want [\"owned-task\"]", recovered)
	}

	// Verify task1 became Ready (retryable)
	loaded1, err := store.GetTask(ctx, task1.ID)
	if err != nil {
		t.Fatal(err)
	}
	if loaded1.Status != orchestra.TaskReady {
		t.Fatalf("owned-task status = %q, want Ready", loaded1.Status)
	}

	// Verify task2 still Running (not recovered, no active registration)
	loaded2, err := store.GetTask(ctx, task2.ID)
	if err != nil {
		t.Fatal(err)
	}
	if loaded2.Status != orchestra.TaskRunning {
		t.Fatalf("unowned-task status = %q, want Running", loaded2.Status)
	}

	// Active table should have task2 but not task1
	var activeCount int
	err = store.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM orchestra_active_tasks WHERE task_id = ?`, string(task1.ID)).Scan(&activeCount)
	if err != nil {
		t.Fatal(err)
	}
	if activeCount != 0 {
		t.Fatalf("owned-task should be cleared from active table, count=%d", activeCount)
	}
}
