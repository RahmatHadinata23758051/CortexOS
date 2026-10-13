package orchestra_test

import (
	"context"
	"path/filepath"
	"testing"

	harnesssqlite "github.com/RahmatHadinata23758051/CortexOS/internal/cortex/harness/sqlite"
	"github.com/RahmatHadinata23758051/CortexOS/internal/cortex/orchestra"
	orchestrasqlite "github.com/RahmatHadinata23758051/CortexOS/internal/cortex/orchestra/sqlite"
	"github.com/RahmatHadinata23758051/CortexOS/internal/logging"
)



func TestProductionAdapter_MergeTask_EmitsStructuredLogs(t *testing.T) {
	capturingLogger := &logging.CapturingLogger{}
	adapter, orchStore, harnessStore := setupTestAdapterWithLogger(t, capturingLogger)
	ctx := context.Background()

	task, _ := seedTaskAwaitingInspection(t, orchStore, harnessStore, "log-task-1", "wt-1")

	merged, err := adapter.MergeTask(ctx, task.ID, "smoke merge note")
	if err != nil {
		t.Fatalf("expected successful merge, got error: %v", err)
	}
	if merged.Status != orchestra.TaskSuccess {
		t.Fatalf("merged status = %s, want success", merged.Status)
	}

	records := capturingLogger.Records()
	if len(records) == 0 {
		t.Fatal("expected structured log records, got none")
	}

	// Verify required events and canonical fields
	eventsSeen := make(map[string]bool)
	for _, rec := range records {
		if ev, ok := rec.Fields["event"].(string); ok {
			eventsSeen[ev] = true
		}
		// Every record must have task_id
		if _, ok := rec.Fields["task_id"]; !ok {
			t.Errorf("record missing task_id field: %+v", rec)
		}
	}

	requiredEvents := []string{
		"orchestra_merge_started",
		"orchestra_merge_worktree_resolved",
		"orchestra_merge_evidence_resolved",
		"orchestra_merge_committed",
	}
	for _, ev := range requiredEvents {
		if !eventsSeen[ev] {
			t.Errorf("expected to observe event %q in logs, got events: %+v", ev, eventsSeen)
		}
	}
}

func TestProductionAdapter_MergeTask_Rejected_EmitsStructuredLogs(t *testing.T) {
	capturingLogger := &logging.CapturingLogger{}
	adapter, orchStore, _ := setupTestAdapterWithLogger(t, capturingLogger)
	ctx := context.Background()

	// Seed task with invalid status (TaskDraft)
	task := seedDraftTask(t, orchStore, "rejected-log-task")

	_, err := adapter.MergeTask(ctx, task.ID, "note")
	if err == nil {
		t.Fatal("expected merge to fail for draft task, got nil error")
	}

	records := capturingLogger.Records()
	foundInvalidStatus := false
	for _, rec := range records {
		if rec.Fields["event"] == "orchestra_merge_invalid_status" {
			foundInvalidStatus = true
			if rec.Fields["current_status"] != orchestra.TaskDraft {
				t.Errorf("expected current_status = draft, got %v", rec.Fields["current_status"])
			}
			if _, ok := rec.Fields["duration_ms"]; !ok {
				t.Error("expected duration_ms in invalid_status record")
			}
		}
	}
	if !foundInvalidStatus {
		t.Error("expected orchestra_merge_invalid_status event in logs")
	}
}

func setupTestAdapterWithLogger(t *testing.T, l logging.Logger) (*orchestra.ProductionAdapter, *orchestrasqlite.Store, *harnesssqlite.Store) {
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
		Logger:            l,
	})
	if err != nil {
		t.Fatalf("create custom adapter: %v", err)
	}
	return adapter, orchStore, harnessStore
}

func seedDraftTask(t *testing.T, store orchestra.TaskStore, id string) orchestra.Task {
	t.Helper()
	task := orchestra.Task{
		ID:                 orchestra.TaskID(id),
		ProjectID:          "p-1",
		WorktreeID:         "wt-1",
		Title:              "Draft task",
		AcceptanceCriteria: []string{"none"},
		Status:             orchestra.TaskDraft,
		MaxAttempts:        3,
		SchemaVersion:      orchestra.ContractVersion,
	}
	if err := store.SaveTask(context.Background(), task); err != nil {
		t.Fatalf("save draft task: %v", err)
	}
	return task
}
