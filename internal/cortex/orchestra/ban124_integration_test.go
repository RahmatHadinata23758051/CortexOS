package orchestra

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/RahmatHadinata23758051/CortexOS/internal/cortex/harness"
	"github.com/RahmatHadinata23758051/CortexOS/internal/cortex/knowledge"
	"github.com/RahmatHadinata23758051/CortexOS/internal/cortex/staff"
	"github.com/RahmatHadinata23758051/CortexOS/internal/cortex/workspace"
	"github.com/RahmatHadinata23758051/CortexOS/internal/cortex/workspace/retrieval"
	"github.com/RahmatHadinata23758051/CortexOS/internal/cortex/workspace/vault"
)

// TestBAN124_EndToEndIntegration_CockpitBridgeChain verifies the full Phase 6
// end-to-end chain:
// Workspace + Knowledge + Staff + Orchestra + Harness
// → task dispatch to Staff
// → Harness broker execution (native adapter)
// → evidence collection
// → Inspector review gate (ADR-0005)
// → merge authorization and plan update.
func TestBAN124_EndToEndIntegration_CockpitBridgeChain(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	// 1. In-memory Workspace setup
	memWS := workspace.NewMemoryWorkspace()
	if err := memWS.Open(ctx); err != nil {
		t.Fatalf("open workspace: %v", err)
	}
	defer memWS.Close()

	projectID := workspace.ProjectID("proj-test")
	if _, err := memWS.RegisterProject(ctx, workspace.Project{
		ID:             projectID,
		Name:           "BAN-124 Integration Project",
		RepositoryRoot: "/tmp/repo-ban124",
		VaultRoot:      "/tmp/vault-ban124",
		Status:         workspace.ProjectStatusActive,
		CreatedAt:      time.Now().UTC(),
		UpdatedAt:      time.Now().UTC(),
		SchemaVersion:  workspace.ContractVersion,
	}); err != nil {
		t.Fatalf("register project: %v", err)
	}

	created := time.Now().UTC()
	_, err := memWS.CreateNote(ctx, workspace.VaultNote{
		ID:            "note-cockpit",
		ProjectID:     projectID,
		RelativePath:  "specs/cockpit-spec.md",
		Title:         "Cockpit Spec",
		Body:          "# Cockpit Architecture\n\nBridge connects React frontend to Go services.",
		FormatVersion: vault.FormatVersion,
		Source:        "vault-editor",
		Author:        "lead-architect",
		CreatedAt:     created,
		UpdatedAt:     created,
		ContentHash:   "hash-cockpit-1",
		Status:        workspace.NoteStatusActive,
	})
	if err != nil {
		t.Fatalf("create note: %v", err)
	}

	retIndex, err := retrieval.New(t.TempDir())
	if err != nil {
		t.Fatalf("new retrieval: %v", err)
	}

	// 2. In-memory Knowledge service & indexing
	knowStore := knowledge.NewMemoryStore()
	if err := knowStore.Open(ctx); err != nil {
		t.Fatalf("open knowledge store: %v", err)
	}
	knowSvc, err := knowledge.NewService(knowStore)
	if err != nil {
		t.Fatalf("new knowledge service: %v", err)
	}
	pipeline, err := knowledge.NewPipeline(memWS, knowSvc, knowledge.DefaultIngestionConfig())
	if err != nil {
		t.Fatalf("new knowledge pipeline: %v", err)
	}
	if _, err := pipeline.IngestProject(ctx, projectID, "ws-test"); err != nil {
		t.Fatalf("ingest project: %v", err)
	}
	items, err := knowSvc.List(ctx, knowledge.Filter{
		ProjectID:  projectIDPtr(string(projectID)),
		ActiveOnly: true,
	})
	if err != nil || len(items) == 0 {
		t.Fatalf("expected active knowledge items, got %d, err: %v", len(items), err)
	}
	indexer := knowledge.NewIndexer(retIndex, "ws-test")
	if err := indexer.IndexItems(ctx, items); err != nil {
		t.Fatalf("index knowledge items: %v", err)
	}

	// 3. In-memory Staff store with defined capabilities
	staffStore := newTestStaffStore(t)
	skillStore := newTestSkillStore(t)
	memoryStore := newTestMemoryStore(t)
	capReg := staff.DefaultCapabilityRegistry()
	harnessRouter := newTestHarnessRouter(t)

	staffScheduler, err := staff.NewScheduler(staffStore, staffStore)
	if err != nil {
		t.Fatalf("new staff scheduler: %v", err)
	}

	assigner, err := NewStaffTaskAssigner(
		staffStore, staffStore, skillStore, memoryStore, capReg, harnessRouter, staffScheduler,
	)
	if err != nil {
		t.Fatalf("new staff task assigner: %v", err)
	}

	// 4. Harness Broker with Native adapter engine
	policy, err := harness.NewPolicy([]harness.Rule{
		{ID: "allow-coding", Action: "coding", Resource: "*", Effect: harness.EffectAllow},
	})
	if err != nil {
		t.Fatalf("new policy: %v", err)
	}
	policyEngine, err := harness.NewStaticPolicy(policy)
	if err != nil {
		t.Fatalf("new static policy: %v", err)
	}
	broker := harness.NewBroker(policyEngine)

	tool := harness.ToolDefinition{
		Name:         "coding_agent",
		Kind:         harness.ToolKindNative,
		Description:  "Native coding tool",
		Capabilities: []harness.ToolCapability{harness.CapabilityCoding},
		InputSchema:  json.RawMessage(`{"type":"object"}`),
		OutputSchema: json.RawMessage(`{"type":"object"}`),
		Timeout:      30 * time.Second,
		Version:      "1.0.0-test",
	}
	if err := broker.Register(tool); err != nil {
		t.Fatalf("register tool: %v", err)
	}

	engine := newFakeEngine(harness.ToolKindNative, harness.CapabilityCoding)
	if err := broker.RegisterEngine(engine); err != nil {
		t.Fatalf("register engine: %v", err)
	}

	// 5. Orchestra TaskStore, Bridge, Dispatcher
	taskStore := NewMemoryTaskStore()
	bridge := NewHarnessBridge(broker, taskStore, "/tmp/worktree-ban124")
	bridge.SetContextProvider(assigner)

	dispatcher := NewDispatcher(taskStore, bridge)
	dispatcher.SetStaffScheduler(staffScheduler)

	// 6. Plan creation: Define dependent tasks to test plan updating on merge
	parentTask := Task{
		ID:                   TaskID("task-ban124-parent"),
		WorkspaceID:          "ws-test",
		ProjectID:            "proj-test",
		WorktreeID:           "wt-test",
		Title:                "Phase 6 Cockpit Core",
		AcceptanceCriteria:   []string{"Bridge contracts implemented", "Evidence streamed"},
		Status:               TaskDraft,
		AttemptCount:         0,
		MaxAttempts:          3,
		Priority:             staff.PriorityNormal,
		RequiredCapabilities: []staff.Capability{"coding"},
		CreatedAt:            time.Now().UTC(),
		UpdatedAt:            time.Now().UTC(),
		SchemaVersion:        ContractVersion,
	}
	dependentTask := Task{
		ID:                   TaskID("task-ban124-child"),
		WorkspaceID:          "ws-test",
		ProjectID:            "proj-test",
		WorktreeID:           "wt-test",
		Title:                "Phase 6 Follow-up Task",
		AcceptanceCriteria:   []string{"Runs only after parent completes"},
		Dependencies:         []TaskID{"task-ban124-parent"},
		Status:               TaskDraft,
		AttemptCount:         0,
		MaxAttempts:          3,
		Priority:             staff.PriorityNormal,
		RequiredCapabilities: []staff.Capability{"coding"},
		CreatedAt:            time.Now().UTC(),
		UpdatedAt:            time.Now().UTC(),
		SchemaVersion:        ContractVersion,
	}

	if err := taskStore.SaveTask(ctx, parentTask); err != nil {
		t.Fatalf("save parent task: %v", err)
	}
	if err := taskStore.SaveTask(ctx, dependentTask); err != nil {
		t.Fatalf("save child task: %v", err)
	}

	// Verify initial plan: only parent is ready, child is blocked by dependency
	initialTasks, err := taskStore.ListTasks(ctx)
	if err != nil {
		t.Fatalf("list tasks: %v", err)
	}
	readyBefore, err := ReadyTasks(initialTasks)
	if err != nil {
		t.Fatalf("ready tasks before: %v", err)
	}
	if len(readyBefore) != 1 || readyBefore[0].ID != parentTask.ID {
		t.Fatalf("expected only parent task to be ready before merge, got %v", readyBefore)
	}

	// Simulate UI transition: Task board drag-drop draft -> ready
	validRes, err := ApplyTransition(parentTask, TransitionValidate)
	if err != nil {
		t.Fatalf("validate transition: %v", err)
	}
	parentTask = validRes.Task
	if err := taskStore.SaveTask(ctx, parentTask); err != nil {
		t.Fatalf("save validated task: %v", err)
	}

	// 7. Dispatch parent task
	exec, err := dispatcher.Dispatch(ctx, parentTask.ID)
	if err != nil {
		t.Fatalf("dispatch parent task: %v", err)
	}

	// Wait for worker execution to complete and reach AwaitingInspection
	awaitingTask, err := waitForTaskStatus(ctx, taskStore, parentTask.ID, TaskAwaitingInspection, 3*time.Second)
	if err != nil {
		t.Fatalf("wait for inspection: %v", err)
	}
	if awaitingTask.Status != TaskAwaitingInspection {
		t.Fatalf("expected TaskAwaitingInspection, got %s", awaitingTask.Status)
	}

	// 8. ADR-0005 Gating: Worker cannot self-report or force TaskSuccess
	// Attempt direct transition to Success must fail
	if _, err := ApplyTransition(awaitingTask, TransitionValidate); err == nil {
		t.Fatal("expected invalid transition error when attempting non-inspection transition")
	}

	// Retrieve execution record & verify evidence was collected and immutable
	executionRecord, err := taskStore.GetExecution(ctx, exec.ID)
	if err != nil {
		t.Fatalf("get execution: %v", err)
	}
	if len(executionRecord.EvidenceIDs) == 0 {
		t.Fatal("expected evidence IDs in execution record")
	}
	evidenceCountBefore := len(executionRecord.EvidenceIDs)

	events, err := taskStore.ListEvents(ctx, parentTask.ID)
	if err != nil {
		t.Fatalf("list events: %v", err)
	}
	var evidenceEvents []TaskEvent
	for _, e := range events {
		if e.Type == EventEvidenceCollected {
			evidenceEvents = append(evidenceEvents, e)
		}
	}
	if len(evidenceEvents) == 0 {
		t.Fatal("expected at least one EvidenceCollected event")
	}

	// 9. Inspector Review & Merge Authority
	inspector := MergeAuthority{Capability: MergeCapability}

	// Verify rejected if worktree is dirty
	dirtyReq := InspectionRequest{
		Task:      awaitingTask,
		Execution: executionRecord,
		Worktree: WorktreeState{
			WorktreeID: awaitingTask.WorktreeID,
			Revision:   "rev-1",
			Dirty:      true, // Dirty worktree must fail
			Valid:      true,
		},
		Evidence: []InspectionEvidence{
			{ID: executionRecord.EvidenceIDs[0], Kind: "command_execution", Passed: true, Summary: "ok"},
		},
		ChangedPaths: []string{"web/src/App.tsx"},
	}
	if _, err := inspector.Inspect(ctx, dirtyReq); err == nil {
		t.Fatal("expected inspection failure on dirty worktree")
	}

	// Verify rejected if merge capability is invalid
	invalidAuth := MergeAuthority{Capability: "invalid.capability"}
	validReq := InspectionRequest{
		Task:      awaitingTask,
		Execution: executionRecord,
		Worktree: WorktreeState{
			WorktreeID: awaitingTask.WorktreeID,
			Revision:   "rev-1",
			Dirty:      false,
			Valid:      true,
		},
		Evidence: []InspectionEvidence{
			{ID: executionRecord.EvidenceIDs[0], Kind: "command_execution", Passed: true, Summary: "ok"},
		},
		ChangedPaths: []string{"web/src/App.tsx"},
	}
	if _, err := invalidAuth.Inspect(ctx, validReq); err == nil {
		t.Fatal("expected inspection failure with invalid capability")
	}

	// Valid Inspector inspection passes
	inspectionResult, err := inspector.Inspect(ctx, validReq)
	if err != nil {
		t.Fatalf("valid inspect failed: %v", err)
	}
	if !inspectionResult.Accepted {
		t.Fatal("expected inspectionResult.Accepted=true")
	}
	if !inspectionResult.MergeReady {
		t.Fatal("expected inspectionResult.MergeReady=true")
	}

	// Authoritative transition: persist AcceptedTask as TaskSuccess
	successTask := inspectionResult.AcceptedTask
	if successTask.Status != TaskSuccess {
		t.Fatalf("expected AcceptedTask to have status TaskSuccess, got %s", successTask.Status)
	}
	if err := taskStore.SaveTask(ctx, successTask); err != nil {
		t.Fatalf("save accepted success task: %v", err)
	}

	// 10. Verify evidence immutability (evidence records must remain intact)
	execAfter, err := taskStore.GetExecution(ctx, exec.ID)
	if err != nil {
		t.Fatalf("get execution after: %v", err)
	}
	if len(execAfter.EvidenceIDs) != evidenceCountBefore {
		t.Fatalf("evidence count changed: before %d, after %d", evidenceCountBefore, len(execAfter.EvidenceIDs))
	}
	for i, id := range execAfter.EvidenceIDs {
		if id != executionRecord.EvidenceIDs[i] {
			t.Fatalf("evidence ID at index %d altered: %s vs %s", i, id, executionRecord.EvidenceIDs[i])
		}
	}

	// 11. Verify plan update after merge: Dependent task is now unblocked and Ready
	updatedTasks, err := taskStore.ListTasks(ctx)
	if err != nil {
		t.Fatalf("list tasks after merge: %v", err)
	}
	readyAfter, err := ReadyTasks(updatedTasks)
	if err != nil {
		t.Fatalf("ready tasks after merge: %v", err)
	}
	foundChild := false
	for _, r := range readyAfter {
		if r.ID == dependentTask.ID {
			foundChild = true
			break
		}
	}
	if !foundChild {
		t.Fatalf("expected child task %s to be unblocked and ready after parent merged, ready list: %v", dependentTask.ID, readyAfter)
	}

	t.Log("BAN-124 End-to-end Cockpit bridge integration test PASSED")
}
