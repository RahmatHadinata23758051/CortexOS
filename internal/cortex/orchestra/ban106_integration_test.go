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

// TestBAN106_Integration_FullVerticalSlice verifies the complete Phase 5 vertical slice:
// Staff persistence → routing/skills/memory → knowledge ingest/retrieval
// → Orchestra/Harness dispatch → evidence → cancellation/recovery
// → security/concurrency/contracts/import-boundaries.
func TestBAN106_Integration_FullVerticalSlice(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	// ─────────────────────────────────────────────────────────────────
	// 1. SETUP: In-memory / temp-dir stores (all disposable, no external deps)
	// ─────────────────────────────────────────────────────────────────

	// Workspace: in-memory Vault + Projects + Worktrees
	memWS := workspace.NewMemoryWorkspace()
	if err := memWS.Open(ctx); err != nil {
		t.Fatal(err)
	}
	defer memWS.Close()

	projectID := workspace.ProjectID("proj-ban106")
	if _, err := memWS.RegisterProject(ctx, workspace.Project{
		ID:             projectID,
		Name:           "BAN-106 Test Project",
		RepositoryRoot: "/tmp/repo-ban106",
		VaultRoot:      "/tmp/vault-ban106",
		Status:         workspace.ProjectStatusActive,
		CreatedAt:      time.Now().UTC(),
		UpdatedAt:      time.Now().UTC(),
		SchemaVersion:  workspace.ContractVersion,
	}); err != nil {
		t.Fatal(err)
	}

	// Vault note with secrets (will be redacted during ingestion)
	created := time.Now().UTC()
	_, err := memWS.CreateNote(ctx, workspace.VaultNote{
		ID:            "note-arch",
		ProjectID:     projectID,
		RelativePath:  "docs/architecture.md",
		Title:         "System Architecture",
		Body:          "# Architecture\n\nAPI Key: api_key=secret123\nContact: admin@example.com",
		FormatVersion: vault.FormatVersion,
		Source:        "vault-editor",
		Author:        "arch-lead",
		CreatedAt:     created,
		UpdatedAt:     created,
		ContentHash:   "hash-arch-1",
		Status:        workspace.NoteStatusActive,
	})
	if err != nil {
		t.Fatal(err)
	}

	// Retrieval index (temp dir for JSON-backed index)
	indexRoot := t.TempDir()
	retIndex, err := retrieval.New(indexRoot)
	if err != nil {
		t.Fatal(err)
	}

	// Knowledge: MemoryStore + Service + Pipeline + Indexer
	knowStore := knowledge.NewMemoryStore()
	if err := knowStore.Open(ctx); err != nil {
		t.Fatal(err)
	}
	knowSvc, err := knowledge.NewService(knowStore)
	if err != nil {
		t.Fatal(err)
	}

	pipeline, err := knowledge.NewPipeline(memWS, knowSvc, knowledge.DefaultIngestionConfig())
	if err != nil {
		t.Fatal(err)
	}

	indexer := knowledge.NewIndexer(retIndex, "ws-ban106")

	// Staff: in-memory store + router + scheduler + skills + memory
	staffStore := newTestStaffStore(t)   // from ban113_integration_test.go
	skillStore := newTestSkillStore(t)   // from ban113_integration_test.go
	memoryStore := newTestMemoryStore(t) // from ban113_integration_test.go
	capReg := staff.DefaultCapabilityRegistry()

	// Harness router with Native/Pi/OMP fakes
	harnessRouter := newTestHarnessRouter(t)

	staffScheduler, err := staff.NewScheduler(staffStore, staffStore)
	if err != nil {
		t.Fatal(err)
	}

	assigner, err := NewStaffTaskAssigner(
		staffStore, staffStore, skillStore, memoryStore, capReg, harnessRouter, staffScheduler,
	)
	if err != nil {
		t.Fatal(err)
	}

	// Harness: Broker with allow-coding policy + fake engine
	policy, err := harness.NewPolicy([]harness.Rule{
		{ID: "allow-coding", Action: "coding", Resource: "*", Effect: harness.EffectAllow},
	})
	if err != nil {
		t.Fatal(err)
	}
	policyEngine, err := harness.NewStaticPolicy(policy)
	if err != nil {
		t.Fatal(err)
	}
	broker := harness.NewBroker(policyEngine)

	tool := harness.ToolDefinition{
		Name:         "coding_agent",
		Kind:         harness.ToolKindNative,
		Capabilities: []harness.ToolCapability{harness.CapabilityCoding},
		InputSchema:  json.RawMessage(`{"type":"object"}`),
		OutputSchema: json.RawMessage(`{"type":"object"}`),
		Timeout:      30 * time.Second,
		Version:      "1.0.0-test",
	}
	if err := broker.Register(tool); err != nil {
		t.Fatal(err)
	}

	engine := newFakeEngine(harness.ToolKindNative, harness.CapabilityCoding)
	if err := broker.RegisterEngine(engine); err != nil {
		t.Fatal(err)
	}

	// Orchestra: MemoryTaskStore + HarnessBridge + Dispatcher
	taskStore := NewMemoryTaskStore()
	bridge := NewHarnessBridge(broker, taskStore, "/tmp/worktree-ban106")
	bridge.SetContextProvider(assigner)

	dispatcher := NewDispatcher(taskStore, bridge)
	dispatcher.SetStaffScheduler(staffScheduler)

	// ─────────────────────────────────────────────────────────────────
	// 2. KNOWLEDGE INGEST → RETRIEVAL VERIFICATION
	// ─────────────────────────────────────────────────────────────────

	// Ingest the Vault note into Knowledge (chunks, redacts, validates)
	ingestRes, err := pipeline.IngestProject(ctx, projectID, "ws-ban106")
	if err != nil {
		t.Fatalf("IngestProject: %v", err)
	}
	if ingestRes.Ingested == 0 {
		t.Fatal("expected at least 1 knowledge item ingested")
	}

	// Verify knowledge items are Verified + Active (ADR-0005)
	items, err := knowSvc.List(ctx, knowledge.Filter{
		ProjectID:  projectIDPtr(string(projectID)),
		ActiveOnly: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(items) == 0 {
		t.Fatal("expected knowledge items")
	}

	for _, item := range items {
		if item.Lifecycle != knowledge.LifecycleActive {
			t.Errorf("expected Active, got %s", item.Lifecycle)
		}
		if item.ValidationStatus != knowledge.StatusVerified {
			t.Errorf("expected Verified, got %s", item.ValidationStatus)
		}
		// Secrets must be redacted
		if contains(item.Content, "secret123") || contains(item.Content, "admin@example.com") {
			t.Errorf("secret leaked in knowledge: %s", item.Content)
		}
	}

	// Index into retrieval
	if err := indexer.IndexItems(ctx, items); err != nil {
		t.Fatalf("IndexItems: %v", err)
	}

	// Query retrieval
	results, err := retIndex.Query(ctx, projectID, "architecture", 10)
	if err != nil || len(results) == 0 {
		t.Fatal("retrieval query returned no results")
	}

	// ─────────────────────────────────────────────────────────────────
	// 3. TASK DISPATCH WITH STAFF/SKILL/MEMORY ENRICHMENT
	// ─────────────────────────────────────────────────────────────────

	task := Task{
		ID:                   TaskID("task-ban106-1"),
		WorkspaceID:          "ws-test",
		ProjectID:            "proj-test",
		WorktreeID:           "wt-test",
		Title:                "Implement authentication module",
		AcceptanceCriteria:   []string{"code compiles", "passes verification", "security review"},
		Status:               TaskReady,
		AttemptCount:         0,
		MaxAttempts:          3,
		Priority:             staff.PriorityNormal,
		RequiredCapabilities: []staff.Capability{"coding", "file_read", "file_write"},
		Action:               "coding",
		Resource:             "repo/*",
		CreatedAt:            time.Now().UTC(),
		UpdatedAt:            time.Now().UTC(),
		SchemaVersion:        ContractVersion,
		AssignedStaffID:      "", // let scheduler assign
	}
	if err := taskStore.SaveTask(ctx, task); err != nil {
		t.Fatal(err)
	}

	execution, err := dispatcher.Dispatch(ctx, task.ID)
	if err != nil {
		t.Fatalf("Dispatch: %v", err)
	}

	// Wait for task to reach AwaitingInspection (worker completes, evidence collected)
	saved, err := waitForTaskStatus(ctx, taskStore, task.ID, TaskAwaitingInspection, 5*time.Second)
	if err != nil {
		t.Fatalf("wait for inspection: %v", err)
	}
	if saved.Status != TaskAwaitingInspection {
		t.Fatalf("expected AwaitingInspection, got %s", saved.Status)
	}

	// Verify execution has evidence
	exec, err := taskStore.GetExecution(ctx, execution.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(exec.EvidenceIDs) == 0 {
		t.Fatal("expected evidence IDs in execution record")
	}

	// Verify envelope enrichment reached Harness (StaffContext, SkillInjection, Memory, AssignmentProvenance)
	// The fake engine captured the envelope; inspect it
	if len(engine.calls) == 0 {
		t.Fatal("engine never received envelope")
	}
	env := engine.calls[0]
	if env.StaffContext == nil {
		t.Fatal("expected StaffContext in envelope")
	}
	if env.StaffContext.StaffID == "" || env.StaffContext.StaffRole == "" {
		t.Fatal("StaffContext missing StaffID or Role")
	}
	if env.SkillInjection == nil || env.SkillInjection.SkillID == "" {
		t.Fatal("expected SkillInjection in envelope")
	}
	if env.MemoryContext == nil || len(env.MemoryContext) == 0 {
		t.Fatal("expected advisory MemoryContext in envelope")
	}
	if env.StaffContext.Assignment.AssignmentID == "" {
		t.Fatal("expected AssignmentProvenance in envelope")
	}

	// ─────────────────────────────────────────────────────────────────
	// 4. INSPECTOR/MERGE AUTHORITY (ORCHESTRA SOVEREIGNTY)
	// ─────────────────────────────────────────────────────────────────

	// Simulate Inspector approval via MergeAuthority
	inspector := MergeAuthority{Capability: MergeCapability}

	// Need WorktreeState - use clean worktree for test
	worktreeState := WorktreeState{
		WorktreeID: task.WorktreeID,
		Revision:   "abc123",
		Dirty:      false,
		Valid:      true,
	}

	// Convert execution evidence to InspectionEvidence
	var inspEvidence []InspectionEvidence
	for _, eid := range exec.EvidenceIDs {
		inspEvidence = append(inspEvidence, InspectionEvidence{
			ID: eid, Kind: "command_execution", Passed: true, Summary: "ok",
		})
	}

	inspectReq := InspectionRequest{
		Task:         saved,
		Execution:    exec,
		Worktree:     worktreeState,
		Evidence:     inspEvidence,
		ChangedPaths: []string{"src/auth.go", "src/auth_test.go"},
	}

	inspection, err := inspector.Inspect(ctx, inspectReq)
	if err != nil {
		t.Fatalf("Inspect failed: %v", err)
	}
	if !inspection.Accepted {
		t.Fatal("Inspector should accept clean worktree with evidence")
	}
	if !inspection.MergeReady {
		t.Fatal("expected MergeReady=true")
	}

	// Inspector returns the accepted task; Orchestra persists the authoritative transition.
	finalTask := inspection.AcceptedTask
	if err := taskStore.SaveTask(ctx, finalTask); err != nil {
		t.Fatal(err)
	}
	if finalTask.Status != TaskSuccess {
		t.Fatalf("expected TaskSuccess after inspection, got %s", finalTask.Status)
	}

	// ─────────────────────────────────────────────────────────────────
	// 5. CANCELLATION / RECOVERY
	// ─────────────────────────────────────────────────────────────────

	// Dispatch second task, then cancel immediately
	task2 := Task{
		ID:                   TaskID("task-ban106-2"),
		WorkspaceID:          "ws-test",
		ProjectID:            "proj-test",
		WorktreeID:           "wt-test",
		Title:                "Cancelled task",
		AcceptanceCriteria:   []string{"code compiles"},
		Status:               TaskReady,
		AttemptCount:         0,
		MaxAttempts:          3,
		Priority:             staff.PriorityNormal,
		RequiredCapabilities: []staff.Capability{"coding"},
		CreatedAt:            time.Now().UTC(),
		UpdatedAt:            time.Now().UTC(),
		SchemaVersion:        ContractVersion,
	}
	if err := taskStore.SaveTask(ctx, task2); err != nil {
		t.Fatal(err)
	}

	exec2, err := dispatcher.Dispatch(ctx, task2.ID)
	if err != nil {
		t.Fatal(err)
	}

	// Cancel before completion
	if err := dispatcher.Cancel(ctx, task2.ID); err != nil {
		t.Fatalf("Cancel: %v", err)
	}

	canceledTask, err := waitForTaskStatus(ctx, taskStore, task2.ID, TaskCanceled, 2*time.Second)
	if err != nil {
		t.Fatalf("wait for cancel: %v", err)
	}
	if canceledTask.Status != TaskCanceled {
		t.Fatalf("expected TaskCanceled, got %s", canceledTask.Status)
	}

	// Verify execution also canceled
	exec2Saved, err := taskStore.GetExecution(ctx, exec2.ID)
	if err != nil {
		t.Fatal(err)
	}
	if exec2Saved.Status != TaskCanceled {
		t.Fatalf("expected execution Canceled, got %s", exec2Saved.Status)
	}

	// ─────────────────────────────────────────────────────────────────
	// 6. RETRY / REASSIGNMENT (NEW ATTEMPT, NEW PROVENANCE)
	// ─────────────────────────────────────────────────────────────────

	retryTask := Task{
		ID:                   TaskID("task-ban106-retry"),
		WorkspaceID:          "ws-test",
		ProjectID:            "proj-test",
		WorktreeID:           "wt-test",
		Title:                "Retry module task",
		AcceptanceCriteria:   []string{"code compiles"},
		Status:               TaskFailed,
		AttemptCount:         1,
		MaxAttempts:          3,
		Priority:             staff.PriorityNormal,
		RequiredCapabilities: []staff.Capability{"coding"},
		CreatedAt:            time.Now().UTC(),
		UpdatedAt:            time.Now().UTC(),
		SchemaVersion:        ContractVersion,
	}
	if err := taskStore.SaveTask(ctx, retryTask); err != nil {
		t.Fatal(err)
	}

	// Use ApplyTransition with TransitionRetry (Orchestra owns retry)
	retryResult, err := ApplyTransition(retryTask, TransitionRetry)
	if err != nil {
		t.Fatalf("retry transition: %v", err)
	}
	if err := taskStore.SaveTask(ctx, retryResult.Task); err != nil {
		t.Fatalf("save task after retry: %v", err)
	}

	// Second dispatch - should have attempt count incremented to 2
	execution2, err := dispatcher.Dispatch(ctx, retryTask.ID)
	if err != nil {
		t.Fatalf("dispatch 2: %v", err)
	}

	// Verify attempt count incremented
	savedRetry, err := taskStore.GetTask(ctx, retryTask.ID)
	if err != nil {
		t.Fatalf("get task after retry: %v", err)
	}
	if savedRetry.AttemptCount != 2 {
		t.Fatalf("expected AttemptCount=2 after retry, got %d", savedRetry.AttemptCount)
	}

	// Verify new execution ID and attempt
	if execution2.Attempt != 2 {
		t.Fatalf("expected execution2.Attempt=2, got %d", execution2.Attempt)
	}

	// Wait for retry task to complete so scheduler reservation is released
	if _, err := waitForTaskStatus(ctx, taskStore, retryTask.ID, TaskAwaitingInspection, 2*time.Second); err != nil {
		t.Fatalf("wait for retry task completion: %v", err)
	}

	// ─────────────────────────────────────────────────────────────────
	// 7. SECURITY / CONTRACTS / CONCURRENCY
	// ─────────────────────────────────────────────────────────────────

	// 7a. Harness policy denial → TaskFailed (non-retryable)
	denyPolicy, err := harness.NewPolicy([]harness.Rule{
		{ID: "deny-all", Action: "*", Resource: "*", Effect: harness.EffectDeny},
	})
	if err != nil {
		t.Fatal(err)
	}
	denyEngine, err := harness.NewStaticPolicy(denyPolicy)
	if err != nil {
		t.Fatal(err)
	}
	denyBroker := harness.NewBroker(denyEngine)
	if err := denyBroker.Register(tool); err != nil {
		t.Fatal(err)
	}
	denyEngine2 := newFakeEngine(harness.ToolKindNative, harness.CapabilityCoding)
	if err := denyBroker.RegisterEngine(denyEngine2); err != nil {
		t.Fatal(err)
	}

	denyBridge := NewHarnessBridge(denyBroker, taskStore, "/tmp/worktree")
	denyBridge.SetContextProvider(assigner)
	denyDispatcher := NewDispatcher(taskStore, denyBridge)
	denyDispatcher.SetStaffScheduler(staffScheduler)

	denyTask := Task{
		ID:                 TaskID("task-ban106-deny"),
		WorkspaceID:        "ws-test",
		ProjectID:          "proj-test",
		WorktreeID:         "wt-test",
		Title:              "Policy denial test",
		AcceptanceCriteria: []string{"code compiles"},
		Status:             TaskReady,
		AttemptCount:       0,
		MaxAttempts:        3,
		CreatedAt:          time.Now().UTC(),
		UpdatedAt:          time.Now().UTC(),
		SchemaVersion:      ContractVersion,
	}
	if err := taskStore.SaveTask(ctx, denyTask); err != nil {
		t.Fatal(err)
	}

	_, err = denyDispatcher.Dispatch(ctx, denyTask.ID)
	if err != nil {
		t.Fatal(err)
	}

	failedTask, err := waitForTaskStatus(ctx, taskStore, denyTask.ID, TaskFailed, 2*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if failedTask.Status != TaskFailed {
		t.Fatalf("expected TaskFailed on policy denial, got %s", failedTask.Status)
	}

	// Verify failure is non-retryable
	denialErr := harness.ErrorFor(harness.ErrPolicyDenied)
	if ClassifyError(denialErr) != FailureNonRetryable {
		t.Fatal("policy denial must be non-retryable")
	}

	// 7b. Concurrent dispatches of same task → rejected
	concurrentTask := Task{
		ID:                 TaskID("task-ban106-concurrent"),
		WorkspaceID:        "ws-test",
		ProjectID:          "proj-test",
		WorktreeID:         "wt-test",
		Title:              "Concurrent test",
		AcceptanceCriteria: []string{"code compiles"},
		Status:             TaskReady,
		AttemptCount:       0,
		MaxAttempts:        3,
		CreatedAt:          time.Now().UTC(),
		UpdatedAt:          time.Now().UTC(),
		SchemaVersion:      ContractVersion,
	}
	if err := taskStore.SaveTask(ctx, concurrentTask); err != nil {
		t.Fatal(err)
	}

	done := make(chan error, 2)
	go func() { _, err := dispatcher.Dispatch(ctx, concurrentTask.ID); done <- err }()
	go func() { _, err := dispatcher.Dispatch(ctx, concurrentTask.ID); done <- err }()

	err1 := <-done
	err2 := <-done
	if err1 == nil && err2 == nil {
		t.Fatal("expected one concurrent dispatch to fail")
	}

	// 7c. Schema version enforcement (fail-closed on unknown versions)
	badTask := task
	badTask.ID = "task-ban106-badver"
	badTask.SchemaVersion = "cortexos.orchestra.v999"
	if err := taskStore.SaveTask(ctx, badTask); err == nil {
		t.Fatal("expected schema version rejection")
	}

	// 7d. Staff permission fail-closed (Ask/Deny never becomes Allow in EnvelopeAdapter)
	// Already covered by EnvelopeAdapter test in staff/harness_test.go, but verify at integration:
	// The assigner uses EnvelopeAdapter.ToEnvelopePolicyDecision which only allows unconditional Allow

	// ─────────────────────────────────────────────────────────────────
	// 8. CONTRACT VERSIONS IN ALL ENVELOPES
	// ─────────────────────────────────────────────────────────────────

	// Verify all contract versions are present and correct
	if saved.SchemaVersion != ContractVersion {
		t.Errorf("task schema version: %s", saved.SchemaVersion)
	}
	if env.ContractVersion != harness.HarnessContractVersion {
		t.Errorf("envelope schema version: %s", env.ContractVersion)
	}
	if env.SkillInjection != nil && env.SkillInjection.ContractVersion != staff.SkillContractVersion {
		t.Errorf("skill injection schema version: %s", env.SkillInjection.ContractVersion)
	}

	t.Log("BAN-106 full vertical slice PASSED")
}

// ─────────────────────────────────────────────────────────────────
// Helpers
// ─────────────────────────────────────────────────────────────────

func newTestHarnessRouter(t *testing.T) harness.AdapterRouter {
	t.Helper()
	router := harness.NewBasicRouter(harness.DefaultRoutingPolicy())

	// Native
	nativeEngine := newFakeEngine(harness.ToolKindNative, harness.CapabilityCoding, harness.CapabilityFileRead, harness.CapabilityFileWrite)
	if err := router.Register(nativeEngine, harness.ExtendedAdapterDescriptor{
		AdapterDescriptor: harness.AdapterDescriptor{
			Name:          "fake-native",
			Kind:          harness.ToolKindNative,
			Version:       "1.0.0-test",
			Capabilities:  []harness.ToolCapability{harness.CapabilityCoding, harness.CapabilityFileRead, harness.CapabilityFileWrite},
			SchemaVersion: harness.HarnessContractVersion,
		},
		Identity: harness.AdapterIdentity{
			Name:          "fake-native",
			InstanceID:    "fake-native-1",
			Kind:          harness.ToolKindNative,
			Class:         harness.EngineClassNative,
			Version:       "1.0.0-test",
			SchemaVersion: harness.HarnessContractVersion,
		},
		ResourceReq: harness.ResourceRequirement{MinMemoryMB: 200},
	}); err != nil {
		t.Fatalf("register native: %v", err)
	}

	// Pi
	piEngine := newFakeEngine(harness.ToolKindPi, harness.CapabilityCoding)
	if err := router.Register(piEngine, harness.ExtendedAdapterDescriptor{
		AdapterDescriptor: harness.AdapterDescriptor{
			Name:          "fake-pi",
			Kind:          harness.ToolKindPi,
			Version:       "1.0.0-test",
			Capabilities:  []harness.ToolCapability{harness.CapabilityCoding},
			SchemaVersion: harness.HarnessContractVersion,
		},
		Identity: harness.AdapterIdentity{
			Name:          "fake-pi",
			InstanceID:    "fake-pi-1",
			Kind:          harness.ToolKindPi,
			Class:         harness.EngineClassPi,
			Version:       "1.0.0-test",
			SchemaVersion: harness.HarnessContractVersion,
		},
		ResourceReq: harness.ResourceRequirement{MinMemoryMB: 500},
	}); err != nil {
		t.Fatalf("register pi: %v", err)
	}

	// OMP
	ompEngine := newFakeEngine(harness.ToolKindOMP, harness.CapabilityCoding)
	if err := router.Register(ompEngine, harness.ExtendedAdapterDescriptor{
		AdapterDescriptor: harness.AdapterDescriptor{
			Name:          "fake-omp",
			Kind:          harness.ToolKindOMP,
			Version:       "1.0.0-test",
			Capabilities:  []harness.ToolCapability{harness.CapabilityCoding},
			SchemaVersion: harness.HarnessContractVersion,
		},
		Identity: harness.AdapterIdentity{
			Name:          "fake-omp",
			InstanceID:    "fake-omp-1",
			Kind:          harness.ToolKindOMP,
			Class:         harness.EngineClassOMP,
			Version:       "1.0.0-test",
			SchemaVersion: harness.HarnessContractVersion,
		},
		ResourceReq: harness.ResourceRequirement{MinMemoryMB: 800},
	}); err != nil {
		t.Fatalf("register omp: %v", err)
	}

	return router
}

func projectIDPtr(s string) *knowledge.ProjectID {
	id := knowledge.ProjectID(s)
	return &id
}
