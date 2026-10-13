package orchestra_test

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/RahmatHadinata23758051/CortexOS/internal/cortex/harness"
	harnesssqlite "github.com/RahmatHadinata23758051/CortexOS/internal/cortex/harness/sqlite"
	"github.com/RahmatHadinata23758051/CortexOS/internal/cortex/orchestra"
	orchestrasqlite "github.com/RahmatHadinata23758051/CortexOS/internal/cortex/orchestra/sqlite"
)

func TestProductionAdapter_RequiresAllDependencies(t *testing.T) {
	dir := t.TempDir()
	ctx := context.Background()

	orchStore, _ := orchestrasqlite.Open(ctx, filepath.Join(dir, "orchestra.db"))
	defer orchStore.Close()
	harnessStore, _ := harnesssqlite.Open(ctx, filepath.Join(dir, "harness.db"))
	defer harnessStore.Close()

	wt := &mockWorktreeInspector{worktreeID: "wt-1", valid: true}

	// Missing TaskStore
	if _, err := orchestra.NewProductionAdapter(orchestra.ProductionAdapterConfig{
		HarnessStore:      harnessStore,
		WorktreeInspector: wt,
	}); err == nil {
		t.Fatal("expected error when TaskStore is missing")
	}

	// Missing HarnessStore
	if _, err := orchestra.NewProductionAdapter(orchestra.ProductionAdapterConfig{
		TaskStore:         orchStore,
		WorktreeInspector: wt,
	}); err == nil {
		t.Fatal("expected error when HarnessStore is missing")
	}

	// Missing WorktreeInspector
	if _, err := orchestra.NewProductionAdapter(orchestra.ProductionAdapterConfig{
		TaskStore:    orchStore,
		HarnessStore: harnessStore,
	}); err == nil {
		t.Fatal("expected error when WorktreeInspector is missing")
	}
}

func TestProductionAdapter_FailsClosedOnMissingExecutionRecord(t *testing.T) {
	adapter, orchStore, _ := setupTestAdapter(t)
	ctx := context.Background()

	// Seed task awaiting inspection WITHOUT any execution record
	task := orchestra.Task{
		ID:                 "task-orphan",
		ProjectID:          "p-1",
		WorktreeID:         "wt-1",
		Title:              "Orphan Task",
		AcceptanceCriteria: []string{"audit"},
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

	// Attempt merge: must fail because execution record is missing
	if _, err := adapter.MergeTask(ctx, task.ID, "attempt merge"); err == nil {
		t.Fatal("expected error when execution record is missing, got nil")
	}

	// Task must remain awaiting inspection
	persisted, _ := orchStore.GetTask(ctx, task.ID)
	if persisted.Status != orchestra.TaskAwaitingInspection {
		t.Fatalf("status mutated on error: %s", persisted.Status)
	}
}

func TestProductionAdapter_FailsClosedOnEvidenceWorktreeMismatch(t *testing.T) {
	adapter, orchStore, harnessStore := setupTestAdapter(t)
	ctx := context.Background()

	taskID := orchestra.TaskID("task-mismatch")
	wtID := "wt-1"

	task := orchestra.Task{
		ID:                 taskID,
		ProjectID:          "p-1",
		WorktreeID:         wtID,
		Title:              "Mismatch Task",
		AcceptanceCriteria: []string{"audit"},
		Status:             orchestra.TaskAwaitingInspection,
		MaxAttempts:        3,
		AttemptCount:       1,
		CreatedAt:          time.Now().UTC(),
		UpdatedAt:          time.Now().UTC(),
		SchemaVersion:      orchestra.ContractVersion,
	}
	_ = orchStore.SaveTask(ctx, task)

	execID := orchestra.ExecutionID(string(taskID) + "-1")
	exec := orchestra.Execution{
		ID:          execID,
		TaskID:      taskID,
		Attempt:     1,
		Status:      orchestra.TaskAwaitingInspection,
		StartedAt:   time.Now().UTC(),
		EvidenceIDs: []string{"ev-tampered"},
	}
	_ = orchStore.SaveExecution(ctx, exec)

	// Create evidence pointing to a DIFFERENT worktree ("wt-tampered" instead of "wt-1")
	tamperedEv, _ := harness.NewEvidenceRecord(string(execID), string(taskID), "wt-tampered", harness.EvidenceKindCommandExecution, map[string]string{"result": "pass"})
	tamperedEv.ID = "ev-tampered"
	_ = harnessStore.SaveEvidence(ctx, tamperedEv)

	// Attempt merge: must fail closed because evidence worktree does not match task worktree
	if _, err := adapter.MergeTask(ctx, task.ID, "attempt merge"); err == nil {
		t.Fatal("expected rejection on evidence worktree mismatch, got nil")
	}

	persisted, _ := orchStore.GetTask(ctx, task.ID)
	if persisted.Status != orchestra.TaskAwaitingInspection {
		t.Fatalf("status mutated on error: %s", persisted.Status)
	}
}

func TestProductionAdapter_FailsClosedOnExtraEvidenceNotPinned(t *testing.T) {
	adapter, orchStore, harnessStore := setupTestAdapter(t)
	ctx := context.Background()

	taskID := orchestra.TaskID("task-extra-ev")
	wtID := "wt-1"

	task := orchestra.Task{
		ID:                 taskID,
		ProjectID:          "p-1",
		WorktreeID:         wtID,
		Title:              "Extra Evidence Task",
		AcceptanceCriteria: []string{"audit"},
		Status:             orchestra.TaskAwaitingInspection,
		MaxAttempts:        3,
		AttemptCount:       1,
		CreatedAt:          time.Now().UTC(),
		UpdatedAt:          time.Now().UTC(),
		SchemaVersion:      orchestra.ContractVersion,
	}
	_ = orchStore.SaveTask(ctx, task)

	execID := orchestra.ExecutionID(string(taskID) + "-1")
	exec := orchestra.Execution{
		ID:          execID,
		TaskID:      taskID,
		Attempt:     1,
		Status:      orchestra.TaskAwaitingInspection,
		StartedAt:   time.Now().UTC(),
		EvidenceIDs: []string{"ev-pinned-1"},
	}
	_ = orchStore.SaveExecution(ctx, exec)

	// Execution pins only ev-pinned-1, but harness store contains one pinned + one unpinned record.
	pinned, _ := harness.NewEvidenceRecord(string(execID), string(taskID), wtID, harness.EvidenceKindCommandExecution, map[string]string{"result": "pass"})
	pinned.ID = "ev-pinned-1"
	_ = harnessStore.SaveEvidence(ctx, pinned)

	unpinned, _ := harness.NewEvidenceRecord(string(execID), string(taskID), wtID, harness.EvidenceKindCommandExecution, map[string]string{"result": "pass"})
	unpinned.ID = "ev-unpinned"
	_ = harnessStore.SaveEvidence(ctx, unpinned)

	if _, err := adapter.MergeTask(ctx, task.ID, "attempt merge"); err == nil {
		t.Fatal("expected rejection on extra unpinned evidence, got nil")
	}

	persisted, _ := orchStore.GetTask(ctx, task.ID)
	if persisted.Status != orchestra.TaskAwaitingInspection {
		t.Fatalf("status mutated on error: %s", persisted.Status)
	}
}

type failingInspector struct{}

func (f failingInspector) Inspect(context.Context, orchestra.InspectionRequest) (orchestra.InspectionResult, error) {
	return orchestra.InspectionResult{}, orchestra.ErrInspectionRejected
}

func TestProductionAdapter_MergeTask_PersistsRejectionEvent(t *testing.T) {
	dir := t.TempDir()
	ctx := context.Background()

	orchStore, _ := orchestrasqlite.Open(ctx, filepath.Join(dir, "orchestra.db"))
	defer orchStore.Close()
	harnessStore, _ := harnesssqlite.Open(ctx, filepath.Join(dir, "harness.db"))
	defer harnessStore.Close()

	adapter, err := orchestra.NewProductionAdapter(orchestra.ProductionAdapterConfig{
		TaskStore:         orchStore,
		HarnessStore:      harnessStore,
		WorktreeInspector: &mockWorktreeInspector{worktreeID: "wt-1", valid: true},
		Inspector:         failingInspector{},
	})
	if err != nil {
		t.Fatalf("create adapter: %v", err)
	}

	task, _ := seedTaskAwaitingInspection(t, orchStore, harnessStore, "task-reject", "wt-1")

	if _, err := adapter.MergeTask(ctx, task.ID, "attempt merge"); err == nil {
		t.Fatal("expected inspection rejection, got nil")
	}

	events, err := orchStore.ListEvents(ctx, task.ID)
	if err != nil {
		t.Fatalf("list events: %v", err)
	}
	found := false
	for _, ev := range events {
		if ev.Type == orchestra.EventInspectionRejected {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("expected inspection.rejected event persisted on rejection")
	}
}
