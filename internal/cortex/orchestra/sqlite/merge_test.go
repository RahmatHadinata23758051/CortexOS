package sqlite

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/RahmatHadinata23758051/CortexOS/internal/cortex/orchestra"
)

func seedMergeTask(t *testing.T, store *Store, id orchestra.TaskID, deps []orchestra.TaskID) {
	t.Helper()
	task := orchestra.Task{
		ID:                 id,
		ProjectID:          "p-merge",
		WorktreeID:         "wt-merge",
		Title:              "Merge commit test",
		AcceptanceCriteria: []string{"passes atomicity"},
		Dependencies:       deps,
		Status: func() orchestra.TaskStatus {
			if len(deps) > 0 {
				return orchestra.TaskDraft
			}
			return orchestra.TaskAwaitingInspection
		}(),
		AttemptCount:  1,
		MaxAttempts:   3,
		CreatedAt:     time.Now().UTC(),
		UpdatedAt:     time.Now().UTC(),
		SchemaVersion: orchestra.ContractVersion,
	}
	if err := store.SaveTask(context.Background(), task); err != nil {
		t.Fatalf("save task %s: %v", id, err)
	}
}

func TestCommitMerge_AtomicSuccess(t *testing.T) {
	path := filepath.Join(t.TempDir(), "orchestra.db")
	ctx := context.Background()
	store, err := Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	seedMergeTask(t, store, "parent", nil)
	if err := store.SaveExecution(ctx, orchestra.Execution{ID: "parent-1", TaskID: "parent", Attempt: 1, Status: orchestra.TaskAwaitingInspection, StartedAt: time.Now().UTC()}); err != nil {
		t.Fatalf("save parent execution: %v", err)
	}
	seedMergeTask(t, store, "child", []orchestra.TaskID{"parent"})
	seedMergeTask(t, store, "child-2", []orchestra.TaskID{"parent"})

	parent, _ := store.GetTask(ctx, "parent")
	child, _ := store.GetTask(ctx, "child")
	child2, _ := store.GetTask(ctx, "child-2")

	res, _ := orchestra.ApplyTransition(parent, orchestra.TransitionInspectAccepted)
	childRes, _ := orchestra.ApplyTransition(child, orchestra.TransitionValidate)
	child2Res, _ := orchestra.ApplyTransition(child2, orchestra.TransitionValidate)

	now := time.Now().UTC()
	res.Task.UpdatedAt = now
	childRes.Task.UpdatedAt = now
	child2Res.Task.UpdatedAt = now

	commit := orchestra.MergeCommit{
		MergedTask: res.Task,
		AcceptedEvent: orchestra.TaskEvent{
			TaskID: "parent", ExecutionID: "parent-1", Type: orchestra.EventInspectionAccepted,
			From: orchestra.TaskAwaitingInspection, To: orchestra.TaskSuccess,
			Message: "atomic merge", EvidenceIDs: []string{"ev-1"}, OccurredAt: now,
		},
		UnblockedTasks: []orchestra.Task{childRes.Task, child2Res.Task},
		ValidatedEvents: []orchestra.TaskEvent{
			{TaskID: "child", Type: orchestra.EventTaskValidated, From: orchestra.TaskDraft, To: orchestra.TaskReady, Message: "unblocked", OccurredAt: now},
			{TaskID: "child-2", Type: orchestra.EventTaskValidated, From: orchestra.TaskDraft, To: orchestra.TaskReady, Message: "unblocked", OccurredAt: now},
		},
	}

	if err := store.CommitMerge(ctx, commit); err != nil {
		t.Fatalf("CommitMerge failed: %v", err)
	}

	// Verify persistence
	merged, _ := store.GetTask(ctx, "parent")
	if merged.Status != orchestra.TaskSuccess {
		t.Fatalf("merged status = %s, want success", merged.Status)
	}
	c1, _ := store.GetTask(ctx, "child")
	if c1.Status != orchestra.TaskReady {
		t.Fatalf("child status = %s, want ready", c1.Status)
	}
	c2, _ := store.GetTask(ctx, "child-2")
	if c2.Status != orchestra.TaskReady {
		t.Fatalf("child-2 status = %s, want ready", c2.Status)
	}
	events, _ := store.ListEvents(ctx, "parent")
	found := false
	for _, e := range events {
		if e.Type == orchestra.EventInspectionAccepted {
			found = true
		}
	}
	if !found {
		t.Fatal("expected inspection accepted event")
	}
	ce, _ := store.ListEvents(ctx, "child")
	if len(ce) == 0 {
		t.Fatal("expected validated event for child")
	}
}

func TestCommitMerge_ConflictRejectsConcurrentStateChange(t *testing.T) {
	path := filepath.Join(t.TempDir(), "orchestra.db")
	ctx := context.Background()
	store, _ := Open(ctx, path)
	defer store.Close()

	seedMergeTask(t, store, "parent-conflict", nil)

	parent, _ := store.GetTask(ctx, "parent-conflict")
	res, _ := orchestra.ApplyTransition(parent, orchestra.TransitionInspectAccepted)
	now := time.Now().UTC()
	res.Task.UpdatedAt = now

	// Simulate concurrent merge: change status out from under commit
	parent.Status = orchestra.TaskSuccess
	parent.UpdatedAt = now
	redirect := parent
	// intentionally set terminal before commit
	if err := store.SaveTask(ctx, redirect); err != nil {
		t.Fatalf("simulate concurrent merge: %v", err)
	}

	commit := orchestra.MergeCommit{
		MergedTask: res.Task,
		AcceptedEvent: orchestra.TaskEvent{
			TaskID: "parent-conflict", Type: orchestra.EventInspectionAccepted,
			From: orchestra.TaskAwaitingInspection, To: orchestra.TaskSuccess,
			Message: "stale merge", OccurredAt: now,
		},
	}

	if err := store.CommitMerge(ctx, commit); err == nil {
		t.Fatal("expected merge conflict error, got nil")
	}

	events, _ := store.ListEvents(ctx, "parent-conflict")
	for _, e := range events {
		if e.Type == orchestra.EventInspectionAccepted {
			t.Fatal("stale commit inserted event despite conflict")
		}
	}
}

func TestCommitMerge_RollsBackOnDependentTaskFailure(t *testing.T) {
	path := filepath.Join(t.TempDir(), "orchestra.db")
	ctx := context.Background()
	store, _ := Open(ctx, path)
	defer store.Close()

	seedMergeTask(t, store, "parent-rb", nil)
	if err := store.SaveExecution(ctx, orchestra.Execution{ID: "parent-rb-1", TaskID: "parent-rb", Attempt: 1, Status: orchestra.TaskAwaitingInspection, StartedAt: time.Now().UTC()}); err != nil {
		t.Fatalf("save parent execution: %v", err)
	}
	seedMergeTask(t, store, "child-rb", []orchestra.TaskID{"parent-rb"})

	parent, _ := store.GetTask(ctx, "parent-rb")
	child, _ := store.GetTask(ctx, "child-rb")

	res, _ := orchestra.ApplyTransition(parent, orchestra.TransitionInspectAccepted)
	childRes, _ := orchestra.ApplyTransition(child, orchestra.TransitionValidate)
	now := time.Now().UTC()
	res.Task.UpdatedAt = now
	childRes.Task.UpdatedAt = now

	// Intentionally poison the payload with an invalid dependent status so validation fails
	childRes.Task.Status = orchestra.TaskRunning

	commit := orchestra.MergeCommit{
		MergedTask: res.Task,
		AcceptedEvent: orchestra.TaskEvent{
			TaskID: "parent-rb", ExecutionID: "parent-rb-1", Type: orchestra.EventInspectionAccepted,
			From: orchestra.TaskAwaitingInspection, To: orchestra.TaskSuccess,
			Message: "poisoned merge", OccurredAt: now,
		},
		UnblockedTasks: []orchestra.Task{childRes.Task},
		ValidatedEvents: []orchestra.TaskEvent{
			{TaskID: "child-rb", Type: orchestra.EventTaskValidated, From: orchestra.TaskDraft, To: orchestra.TaskReady, Message: "unblocked", OccurredAt: now},
		},
	}

	if err := store.CommitMerge(ctx, commit); err == nil {
		t.Fatal("expected commit validation error, got nil")
	}

	// Verify no partial changes: parent must remain awaiting inspection, no events
	merged, _ := store.GetTask(ctx, "parent-rb")
	if merged.Status != orchestra.TaskAwaitingInspection {
		t.Fatalf("parent changed despite rollback: %s", merged.Status)
	}
	events, _ := store.ListEvents(ctx, "parent-rb")
	for _, e := range events {
		if e.Type == orchestra.EventInspectionAccepted {
			t.Fatal("partial accepted event persisted despite rollback")
		}
	}
	dep, _ := store.GetTask(ctx, "child-rb")
	if dep.Status != orchestra.TaskDraft {
		t.Fatalf("dependent changed despite rollback: %s", dep.Status)
	}
}

func TestCommitMerge_MemoryStoreRoundTrip(t *testing.T) {
	ctx := context.Background()
	store := orchestra.NewMemoryTaskStore()

	parent := orchestra.Task{
		ID: "mem-parent", ProjectID: "p", WorktreeID: "wt", Title: "memory merge",
		AcceptanceCriteria: []string{"criteria"}, Status: orchestra.TaskAwaitingInspection,
		AttemptCount: 1, MaxAttempts: 3, CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC(),
		SchemaVersion: orchestra.ContractVersion,
	}
	child := orchestra.Task{
		ID: "mem-child", ProjectID: "p", WorktreeID: "wt", Title: "dependent",
		AcceptanceCriteria: []string{"criteria"}, Dependencies: []orchestra.TaskID{"mem-parent"},
		Status: orchestra.TaskDraft, AttemptCount: 0, MaxAttempts: 3,
		CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC(), SchemaVersion: orchestra.ContractVersion,
	}
	_ = store.SaveTask(ctx, parent)
	_ = store.SaveTask(ctx, child)

	res, _ := orchestra.ApplyTransition(parent, orchestra.TransitionInspectAccepted)
	cRes, _ := orchestra.ApplyTransition(child, orchestra.TransitionValidate)
	now := time.Now().UTC()
	res.Task.UpdatedAt = now
	cRes.Task.UpdatedAt = now

	commit := orchestra.MergeCommit{
		MergedTask: res.Task,
		AcceptedEvent: orchestra.TaskEvent{
			TaskID: "mem-parent", Type: orchestra.EventInspectionAccepted,
			From: orchestra.TaskAwaitingInspection, To: orchestra.TaskSuccess,
			Message: "memory merge", OccurredAt: now,
		},
		UnblockedTasks: []orchestra.Task{cRes.Task},
		ValidatedEvents: []orchestra.TaskEvent{
			{TaskID: "mem-child", Type: orchestra.EventTaskValidated, From: orchestra.TaskDraft, To: orchestra.TaskReady, Message: "unblocked", OccurredAt: now},
		},
	}
	if err := store.CommitMerge(ctx, commit); err != nil {
		t.Fatalf("memory commit: %v", err)
	}

	merged, _ := store.GetTask(ctx, "mem-parent")
	if merged.Status != orchestra.TaskSuccess {
		t.Fatalf("merged status = %s", merged.Status)
	}
	dep, _ := store.GetTask(ctx, "mem-child")
	if dep.Status != orchestra.TaskReady {
		t.Fatalf("child status = %s", dep.Status)
	}
}
