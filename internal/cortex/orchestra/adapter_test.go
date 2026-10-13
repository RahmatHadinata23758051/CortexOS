package orchestra_test

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/RahmatHadinata23758051/CortexOS/internal/cortex/application"
	"github.com/RahmatHadinata23758051/CortexOS/internal/cortex/harness"
	harnesssqlite "github.com/RahmatHadinata23758051/CortexOS/internal/cortex/harness/sqlite"
	"github.com/RahmatHadinata23758051/CortexOS/internal/cortex/orchestra"
	orchestrasqlite "github.com/RahmatHadinata23758051/CortexOS/internal/cortex/orchestra/sqlite"
)

// mockWorktreeInspector implements orchestra.WorktreeInspector for testing.
type mockWorktreeInspector struct {
	worktreeID string
	revision   string
	dirty      bool
	valid      bool
}

func (m *mockWorktreeInspector) InspectWorktreeState(_ context.Context, id string) (orchestra.WorktreeState, error) {
	if m.worktreeID != id {
		return orchestra.WorktreeState{WorktreeID: id, Valid: false}, nil
	}
	return orchestra.WorktreeState{
		WorktreeID: m.worktreeID,
		Revision:   m.revision,
		Dirty:      m.dirty,
		Valid:      m.valid,
	}, nil
}

func setupTestAdapter(t *testing.T) (application.OrchestraPort, *orchestrasqlite.Store, *harnesssqlite.Store) {
	t.Helper()
	dir := t.TempDir()
	ctx := context.Background()

	orchStore, err := orchestrasqlite.Open(ctx, filepath.Join(dir, "orchestra.db"))
	if err != nil {
		t.Fatalf("open orchestra db: %v", err)
	}
	t.Cleanup(func() { _ = orchStore.Close() })

	harnessStore, err := harnesssqlite.Open(ctx, filepath.Join(dir, "harness.db"))
	if err != nil {
		t.Fatalf("open harness db: %v", err)
	}
	t.Cleanup(func() { _ = harnessStore.Close() })

	wtInspector := &mockWorktreeInspector{
		worktreeID: "wt-1",
		revision:   "rev-1",
		dirty:      false,
		valid:      true,
	}

	adapter, err := orchestra.NewProductionAdapter(orchestra.ProductionAdapterConfig{
		TaskStore:         orchStore,
		HarnessStore:      harnessStore,
		WorktreeInspector: wtInspector,
		Inspector:         orchestra.MergeAuthority{Capability: orchestra.MergeCapability},
	})
	if err != nil {
		t.Fatalf("create production adapter: %v", err)
	}

	return adapter, orchStore, harnessStore
}

func seedTaskAwaitingInspection(t *testing.T, orchStore *orchestrasqlite.Store, harnessStore *harnesssqlite.Store, taskID orchestra.TaskID, wtID string) (orchestra.Task, orchestra.Execution) {
	t.Helper()
	ctx := context.Background()

	task := orchestra.Task{
		ID:                 taskID,
		ProjectID:          "p-1",
		WorktreeID:         wtID,
		Title:              "Test task",
		AcceptanceCriteria: []string{"passes audit"},
		Status:             orchestra.TaskAwaitingInspection,
		MaxAttempts:        3,
		AttemptCount:       1,
		CreatedAt:          time.Now().UTC(),
		UpdatedAt:          time.Now().UTC(),
		SchemaVersion:      orchestra.ContractVersion,
	}
	if err := orchStore.SaveTask(ctx, task); err != nil {
		t.Fatalf("save task: %v", err)
	}

	execID := orchestra.ExecutionID(string(taskID) + "-exec-1")
	exec := orchestra.Execution{
		ID:          execID,
		TaskID:      taskID,
		Attempt:     1,
		Status:      orchestra.TaskAwaitingInspection,
		StartedAt:   time.Now().UTC(),
		EvidenceIDs: []string{"ev-1"},
	}
	if err := orchStore.SaveExecution(ctx, exec); err != nil {
		t.Fatalf("save execution: %v", err)
	}

	evRecord, err := harness.NewEvidenceRecord(string(execID), string(taskID), wtID, harness.EvidenceKindCommandExecution, map[string]string{"tool": "lint", "status": "pass"})
	if err != nil {
		t.Fatalf("create evidence record: %v", err)
	}
	// Override ID to match execution
	evRecord.ID = "ev-1"
	if err := harnessStore.SaveEvidence(ctx, evRecord); err != nil {
		t.Fatalf("save evidence: %v", err)
	}

	return task, exec
}

func TestProductionAdapter_MergeTask_Success(t *testing.T) {
	adapter, orchStore, harnessStore := setupTestAdapter(t)
	ctx := context.Background()

	task, _ := seedTaskAwaitingInspection(t, orchStore, harnessStore, "task-1", "wt-1")

	// Call MergeTask
	merged, err := adapter.MergeTask(ctx, task.ID, "LGTM by inspector")
	if err != nil {
		t.Fatalf("expected merge success, got: %v", err)
	}

	if merged.Status != orchestra.TaskSuccess {
		t.Fatalf("expected status %s, got %s", orchestra.TaskSuccess, merged.Status)
	}

	// Verify persistence in SQLite
	persisted, err := orchStore.GetTask(ctx, task.ID)
	if err != nil {
		t.Fatalf("get persisted task: %v", err)
	}
	if persisted.Status != orchestra.TaskSuccess {
		t.Fatalf("persisted status = %s, want %s", persisted.Status, orchestra.TaskSuccess)
	}

	// Verify event appended
	events, err := orchStore.ListEvents(ctx, task.ID)
	if err != nil {
		t.Fatalf("list events: %v", err)
	}
	found := false
	for _, ev := range events {
		if ev.Type == orchestra.EventInspectionAccepted {
			found = true
			if ev.To != orchestra.TaskSuccess {
				t.Fatalf("event To = %s, want %s", ev.To, orchestra.TaskSuccess)
			}
			break
		}
	}
	if !found {
		t.Fatal("expected EventInspectionAccepted in timeline")
	}
}

func TestProductionAdapter_MergeTask_UpdatesPlanGraph(t *testing.T) {
	adapter, orchStore, harnessStore := setupTestAdapter(t)
	ctx := context.Background()

	parent, _ := seedTaskAwaitingInspection(t, orchStore, harnessStore, "parent-task", "wt-1")

	// Create child task depending on parent
	child := orchestra.Task{
		ID:                 "child-task",
		ProjectID:          parent.ProjectID,
		WorktreeID:         "wt-1",
		Title:              "Child task",
		AcceptanceCriteria: []string{"parent done"},
		Dependencies:       []orchestra.TaskID{parent.ID},
		Status:             orchestra.TaskDraft,
		MaxAttempts:        3,
		AttemptCount:       0,
		CreatedAt:          time.Now().UTC(),
		UpdatedAt:          time.Now().UTC(),
		SchemaVersion:      orchestra.ContractVersion,
	}
	if err := orchStore.SaveTask(ctx, child); err != nil {
		t.Fatalf("save child task: %v", err)
	}

	// Merge parent
	_, err := adapter.MergeTask(ctx, parent.ID, "parent merge")
	if err != nil {
		t.Fatalf("merge parent: %v", err)
	}

	// Child must now be TaskReady because dependency reached TaskSuccess
	updatedChild, err := orchStore.GetTask(ctx, child.ID)
	if err != nil {
		t.Fatalf("get child task: %v", err)
	}
	if updatedChild.Status != orchestra.TaskReady {
		t.Fatalf("child status = %s, want %s", updatedChild.Status, orchestra.TaskReady)
	}
}

func TestProductionAdapter_MergeTask_FailsClosedOnDirtyWorktree(t *testing.T) {
	dir := t.TempDir()
	ctx := context.Background()

	orchStore, _ := orchestrasqlite.Open(ctx, filepath.Join(dir, "orchestra.db"))
	defer orchStore.Close()
	harnessStore, _ := harnesssqlite.Open(ctx, filepath.Join(dir, "harness.db"))
	defer harnessStore.Close()

	// Dirty worktree
	wtInspector := &mockWorktreeInspector{
		worktreeID: "wt-dirty",
		revision:   "rev-1",
		dirty:      true,
		valid:      true,
	}

	adapter, _ := orchestra.NewProductionAdapter(orchestra.ProductionAdapterConfig{
		TaskStore:         orchStore,
		HarnessStore:      harnessStore,
		WorktreeInspector: wtInspector,
		Inspector:         orchestra.MergeAuthority{Capability: orchestra.MergeCapability},
	})

	task, _ := seedTaskAwaitingInspection(t, orchStore, harnessStore, "task-dirty", "wt-dirty")

	_, err := adapter.MergeTask(ctx, task.ID, "attempt merge on dirty worktree")
	if err == nil {
		t.Fatal("expected error on dirty worktree, got nil")
	}

	// Task must remain in awaiting inspection
	persisted, _ := orchStore.GetTask(ctx, task.ID)
	if persisted.Status != orchestra.TaskAwaitingInspection {
		t.Fatalf("status mutated on rejection: %s", persisted.Status)
	}
}

func TestProductionAdapter_MergeTask_FailsClosedOnMissingEvidence(t *testing.T) {
	adapter, orchStore, _ := setupTestAdapter(t)
	ctx := context.Background()

	// Seed task without evidence in harness store
	task := orchestra.Task{
		ID:                 "task-no-ev",
		ProjectID:          "p-1",
		WorktreeID:         "wt-1",
		Title:              "No evidence task",
		AcceptanceCriteria: []string{"passes audit"},
		Status:             orchestra.TaskAwaitingInspection,
		MaxAttempts:        3,
		AttemptCount:       1,
		CreatedAt:          time.Now().UTC(),
		UpdatedAt:          time.Now().UTC(),
		SchemaVersion:      orchestra.ContractVersion,
	}
	_ = orchStore.SaveTask(ctx, task)

	exec := orchestra.Execution{
		ID:          "task-no-ev-exec-1",
		TaskID:      task.ID,
		Attempt:     1,
		Status:      orchestra.TaskAwaitingInspection,
		StartedAt:   time.Now().UTC(),
		EvidenceIDs: []string{}, // No evidence
	}
	_ = orchStore.SaveExecution(ctx, exec)

	_, err := adapter.MergeTask(ctx, task.ID, "attempt merge")
	if err == nil {
		t.Fatal("expected error on missing evidence, got nil")
	}

	persisted, _ := orchStore.GetTask(ctx, task.ID)
	if persisted.Status != orchestra.TaskAwaitingInspection {
		t.Fatalf("status mutated: %s", persisted.Status)
	}
}
