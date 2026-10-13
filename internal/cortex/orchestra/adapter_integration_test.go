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
	"github.com/RahmatHadinata23758051/CortexOS/internal/platform"
)

func TestEndToEnd_BridgeToProductionOrchestraAdapter(t *testing.T) {
	dir := t.TempDir()
	ctx := context.Background()

	// 1. Setup durable SQLite databases
	orchStore, err := orchestrasqlite.Open(ctx, filepath.Join(dir, "orchestra.db"))
	if err != nil {
		t.Fatalf("open orchestra db: %v", err)
	}
	defer orchStore.Close()

	harnessStore, err := harnesssqlite.Open(ctx, filepath.Join(dir, "harness.db"))
	if err != nil {
		t.Fatalf("open harness db: %v", err)
	}
	defer harnessStore.Close()

	// 2. Setup mock worktree inspector
	wtInspector := &mockWorktreeInspector{
		worktreeID: "wt-e2e",
		revision:   "commit-abc",
		dirty:      false,
		valid:      true,
	}

	// 3. Construct production adapter
	adapter, err := orchestra.NewProductionAdapter(orchestra.ProductionAdapterConfig{
		TaskStore:         orchStore,
		HarnessStore:      harnessStore,
		WorktreeInspector: wtInspector,
		Inspector:         orchestra.MergeAuthority{Capability: orchestra.MergeCapability},
	})
	if err != nil {
		t.Fatalf("create production adapter: %v", err)
	}

	// 4. Construct application service wired with production adapter
	appService := application.NewServiceWithOrchestra(adapter)

	// 5. Construct desktop bridge
	bridge := platform.NewBridge(appService)

	// 6. Seed task graph: parent task (awaiting inspection) + child task (draft, dependent on parent)
	parentTask := orchestra.Task{
		ID:                 "task-parent",
		ProjectID:          "proj-e2e",
		WorktreeID:         "wt-e2e",
		Title:              "Parent Task Awaiting Merge",
		AcceptanceCriteria: []string{"passes security and lint checks"},
		Status:             orchestra.TaskAwaitingInspection,
		MaxAttempts:        3,
		AttemptCount:       1,
		CreatedAt:          time.Now().UTC(),
		UpdatedAt:          time.Now().UTC(),
		SchemaVersion:      orchestra.ContractVersion,
	}
	if err := orchStore.SaveTask(ctx, parentTask); err != nil {
		t.Fatalf("save parent task: %v", err)
	}

	childTask := orchestra.Task{
		ID:                 "task-child",
		ProjectID:          "proj-e2e",
		WorktreeID:         "wt-e2e",
		Title:              "Child Task Blocked by Parent",
		AcceptanceCriteria: []string{"parent must merge first"},
		Dependencies:       []orchestra.TaskID{parentTask.ID},
		Status:             orchestra.TaskDraft,
		MaxAttempts:        3,
		AttemptCount:       0,
		CreatedAt:          time.Now().UTC(),
		UpdatedAt:          time.Now().UTC(),
		SchemaVersion:      orchestra.ContractVersion,
	}
	if err := orchStore.SaveTask(ctx, childTask); err != nil {
		t.Fatalf("save child task: %v", err)
	}

	// 7. Seed execution & evidence in harness store
	execID := orchestra.ExecutionID("task-parent-exec-1")
	exec := orchestra.Execution{
		ID:          execID,
		TaskID:      parentTask.ID,
		Attempt:     1,
		Status:      orchestra.TaskAwaitingInspection,
		StartedAt:   time.Now().UTC(),
		EvidenceIDs: []string{"ev-e2e-1"},
	}
	if err := orchStore.SaveExecution(ctx, exec); err != nil {
		t.Fatalf("save execution: %v", err)
	}

	evidenceRecord, err := harness.NewEvidenceRecord(string(execID), string(parentTask.ID), "wt-e2e", harness.EvidenceKindCommandExecution, map[string]string{
		"suite": "regression-gate",
		"pass":  "true",
	})
	if err != nil {
		t.Fatalf("create evidence record: %v", err)
	}
	evidenceRecord.ID = "ev-e2e-1"
	if err := harnessStore.SaveEvidence(ctx, evidenceRecord); err != nil {
		t.Fatalf("save evidence: %v", err)
	}

	// 8. Invoke desktop bridge command: MergeOrchestraTask
	mergeReq := application.OrchestraMergeRequest{
		SchemaVersion: application.OrchestraBridgeSchemaVersion,
		TaskID:        string(parentTask.ID),
		Note:          "Merged after review in Cockpit",
	}

	mergeResult, err := bridge.MergeOrchestraTask(mergeReq)
	if err != nil {
		t.Fatalf("bridge.MergeOrchestraTask failed: %v", err)
	}

	// 9. Assert bridge result
	if mergeResult.TaskID != string(parentTask.ID) {
		t.Fatalf("expected task ID %s, got %s", parentTask.ID, mergeResult.TaskID)
	}
	if mergeResult.Status != string(orchestra.TaskSuccess) {
		t.Fatalf("expected status %s, got %s", orchestra.TaskSuccess, mergeResult.Status)
	}

	// 10. Assert parent task is marked TaskSuccess in SQLite
	persistedParent, err := orchStore.GetTask(ctx, parentTask.ID)
	if err != nil {
		t.Fatalf("get parent task: %v", err)
	}
	if persistedParent.Status != orchestra.TaskSuccess {
		t.Fatalf("persisted parent status = %s, want %s", persistedParent.Status, orchestra.TaskSuccess)
	}

	// 11. Assert event timeline has EventInspectionAccepted
	events, err := orchStore.ListEvents(ctx, parentTask.ID)
	if err != nil {
		t.Fatalf("list events: %v", err)
	}
	var hasAcceptedEvent bool
	for _, e := range events {
		if e.Type == orchestra.EventInspectionAccepted {
			hasAcceptedEvent = true
			if e.To != orchestra.TaskSuccess {
				t.Fatalf("event To = %s, want %s", e.To, orchestra.TaskSuccess)
			}
			break
		}
	}
	if !hasAcceptedEvent {
		t.Fatal("expected EventInspectionAccepted in SQLite timeline")
	}

	// 12. Assert child task was automatically unblocked and transitioned to TaskReady
	persistedChild, err := orchStore.GetTask(ctx, childTask.ID)
	if err != nil {
		t.Fatalf("get child task: %v", err)
	}
	if persistedChild.Status != orchestra.TaskReady {
		t.Fatalf("child task status = %s, want %s (unblocked by parent merge)", persistedChild.Status, orchestra.TaskReady)
	}
}

func TestEndToEnd_BridgeFailsClosedOnDirtyWorktree(t *testing.T) {
	dir := t.TempDir()
	ctx := context.Background()

	orchStore, _ := orchestrasqlite.Open(ctx, filepath.Join(dir, "orchestra.db"))
	defer orchStore.Close()
	harnessStore, _ := harnesssqlite.Open(ctx, filepath.Join(dir, "harness.db"))
	defer harnessStore.Close()

	// Dirty worktree
	wtInspector := &mockWorktreeInspector{
		worktreeID: "wt-dirty",
		revision:   "commit-xyz",
		dirty:      true,
		valid:      true,
	}

	adapter, _ := orchestra.NewProductionAdapter(orchestra.ProductionAdapterConfig{
		TaskStore:         orchStore,
		HarnessStore:      harnessStore,
		WorktreeInspector: wtInspector,
		Inspector:         orchestra.MergeAuthority{Capability: orchestra.MergeCapability},
	})
	appService := application.NewServiceWithOrchestra(adapter)
	bridge := platform.NewBridge(appService)

	task := orchestra.Task{
		ID:                 "task-dirty-e2e",
		ProjectID:          "proj-e2e",
		WorktreeID:         "wt-dirty",
		Title:              "Dirty Worktree Task",
		AcceptanceCriteria: []string{"check cleanliness"},
		Status:             orchestra.TaskAwaitingInspection,
		MaxAttempts:        3,
		AttemptCount:       1,
		CreatedAt:          time.Now().UTC(),
		UpdatedAt:          time.Now().UTC(),
		SchemaVersion:      orchestra.ContractVersion,
	}
	_ = orchStore.SaveTask(ctx, task)

	// Attempt merge via bridge
	mergeReq := application.OrchestraMergeRequest{
		SchemaVersion: application.OrchestraBridgeSchemaVersion,
		TaskID:        string(task.ID),
		Note:          "Attempt merge on dirty tree",
	}

	_, err := bridge.MergeOrchestraTask(mergeReq)
	if err == nil {
		t.Fatal("expected fail-closed error for dirty worktree, got nil")
	}

	// Verify task in SQLite remains untouched in awaitingInspection
	persisted, _ := orchStore.GetTask(ctx, task.ID)
	if persisted.Status != orchestra.TaskAwaitingInspection {
		t.Fatalf("task status altered unexpectedly: %s", persisted.Status)
	}
}
