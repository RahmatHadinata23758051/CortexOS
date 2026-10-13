package orchestra_test

import (
	"context"
	"encoding/json"
	"errors"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/RahmatHadinata23758051/CortexOS/internal/cortex/harness"
	"github.com/RahmatHadinata23758051/CortexOS/internal/cortex/orchestra"
	"github.com/RahmatHadinata23758051/CortexOS/internal/cortex/staff"
	"github.com/RahmatHadinata23758051/CortexOS/internal/cortex/workspace"
)

func buildTestWorker(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	binPath := filepath.Join(dir, "testworker.exe")
	cmd := exec.Command("go", "build", "-o", binPath, "./cmd/testworker")
	cmd.Dir = "../../../"
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("failed to build testworker: %v, output: %s", err, string(out))
	}
	return binPath
}

func TestBAN143_RealEngineVerticalSlice(t *testing.T) {
	ctx := context.Background()

	// Build the real executable
	binPath := buildTestWorker(t)

	// Workspace
	memWS := workspace.NewMemoryWorkspace()
	if err := memWS.Open(ctx); err != nil {
		t.Fatalf("open workspace: %v", err)
	}
	defer memWS.Close()

	projectID := workspace.ProjectID("proj-ban143")
	worktreeRoot := t.TempDir()
	if _, err := memWS.RegisterProject(ctx, workspace.Project{
		ID:             projectID,
		Name:           "BAN-143 Project",
		RepositoryRoot: worktreeRoot,
		VaultRoot:      t.TempDir(),
		Status:         workspace.ProjectStatusActive,
		CreatedAt:      time.Now().UTC(),
		UpdatedAt:      time.Now().UTC(),
		SchemaVersion:  workspace.ContractVersion,
	}); err != nil {
		t.Fatalf("register project: %v", err)
	}

	// Policy allowing coding
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

	allowedVars := []string{
		"CORTEXOS_EXECUTION_ID",
		"CORTEXOS_TASK_ID",
		"CORTEXOS_WORKTREE_ID",
		"CORTEXOS_PROJECT_ID",
		"CORTEXOS_TOOL_NAME",
		"CORTEXOS_INPUT",
		"SYSTEMROOT",
		"PATH",
	}

	// Pre-set environment variables so the real subprocess worker inherits them
	t.Setenv("CORTEXOS_EXECUTION_ID", "task-ban143-pi-1")
	t.Setenv("CORTEXOS_TASK_ID", "task-ban143-pi")
	t.Setenv("CORTEXOS_WORKTREE_ID", "wt-ban143")
	t.Setenv("CORTEXOS_PROJECT_ID", string(projectID))
	t.Setenv("CORTEXOS_TOOL_NAME", "coding_agent")
	t.Setenv("CORTEXOS_INPUT", `{"prompt":"generate verified code"}`)

	// Set up Sandbox on broker
	sandbox, err := harness.NewSandbox(harness.SandboxConfig{
		WorktreeRoot:   worktreeRoot,
		AllowedEnvVars: allowedVars,
	})
	if err != nil {
		t.Fatalf("new sandbox: %v", err)
	}
	broker.SetSandbox(sandbox)

	// Register Pi engine tool
	tool := harness.ToolDefinition{
		Name:         "coding_agent",
		Kind:         harness.ToolKindPi,
		Description:  "Real Pi coding engine",
		Capabilities: []harness.ToolCapability{harness.CapabilityCoding},
		InputSchema:  json.RawMessage(`{"type":"object"}`),
		OutputSchema: json.RawMessage(`{"type":"object"}`),
		Timeout:      30 * time.Second,
		Version:      "1.0.0-test",
	}
	if err := broker.Register(tool); err != nil {
		t.Fatalf("register tool: %v", err)
	}

	// Create REAL PiAdapter pointing to our real testworker binary
	piAdapter, err := harness.NewPiAdapter(harness.PiAdapterConfig{
		ExecutablePath: binPath,
		WorktreeRoot:   worktreeRoot,
		AllowedEnvVars: allowedVars,
		StartupTimeout: 5 * time.Second,
	})
	if err != nil {
		t.Fatalf("NewPiAdapter: %v", err)
	}

	// Health check must pass via real process handshake
	if err := piAdapter.Health(harness.NewExecutionContext(ctx, "health-check")); err != nil {
		t.Fatalf("piAdapter.Health failed: %v", err)
	}

	// Register engine with broker
	if err := broker.RegisterEngine(piAdapter); err != nil {
		t.Fatalf("register engine: %v", err)
	}

	// Verify adapter caps and resource profile
	desc := piAdapter.Describe()
	t.Logf("PiAdapter descriptor: kind=%s caps=%v", desc.Kind, desc.Capabilities)
	piExt := harness.PiExtendedDescriptor()
	t.Logf("PiExtendedDescriptor resource: minMem=%d peakMem=%d cpuPrio=%d", piExt.ResourceReq.MinMemoryMB, piExt.ResourceReq.PeakMemoryMB, piExt.ResourceReq.CPUPriority)

	// Orchestra components
	taskStore := orchestra.NewMemoryTaskStore()
	bridge := orchestra.NewHarnessBridge(broker, taskStore, worktreeRoot)
	dispatcher := orchestra.NewDispatcher(taskStore, bridge)

	// Create and save task
	task := orchestra.Task{
		ID:                   orchestra.TaskID("task-ban143-pi"),
		WorkspaceID:          "ws-ban143",
		ProjectID:            string(projectID),
		WorktreeID:           "wt-ban143",
		Title:                "Real Pi coding step",
		AcceptanceCriteria:   []string{"Generates code via real worker process", "Produces valid evidence"},
		Status:               orchestra.TaskReady,
		MaxAttempts:          3,
		Priority:             staff.PriorityNormal,
		RequiredCapabilities: []staff.Capability{"coding"},
		CreatedAt:            time.Now().UTC(),
		UpdatedAt:            time.Now().UTC(),
		SchemaVersion:        orchestra.ContractVersion,
	}
	if err := taskStore.SaveTask(ctx, task); err != nil {
		t.Fatalf("save task: %v", err)
	}

	// Dispatch task through dispatcher
	exec, err := dispatcher.Dispatch(ctx, task.ID)
	if err != nil {
		t.Fatalf("dispatch task: %v", err)
	}

	// Wait for task to reach TaskAwaitingInspection via real process execution
	var awaitingTask orchestra.Task
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		tCurrent, err := taskStore.GetTask(ctx, task.ID)
		if err == nil && tCurrent.Status == orchestra.TaskAwaitingInspection {
			awaitingTask = tCurrent
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	if awaitingTask.Status != orchestra.TaskAwaitingInspection {
		lastTask, _ := taskStore.GetTask(ctx, task.ID)
		events, _ := taskStore.ListEvents(ctx, task.ID)
		t.Fatalf("task did not reach TaskAwaitingInspection, status: %s, last task: %+v, events: %+v", awaitingTask.Status, lastTask, events)
	}

	// Verify evidence was collected by checking execution record
	executionRecord, err := taskStore.GetExecution(ctx, exec.ID)
	if err != nil {
		t.Fatalf("get execution record: %v", err)
	}
	if len(executionRecord.EvidenceIDs) == 0 {
		t.Fatalf("expected evidence IDs from real Pi execution, got none")
	}

	// Verify ADR-0005: Merge requires Inspector acceptance
	inspector := orchestra.MergeAuthority{Capability: orchestra.MergeCapability}
	req := orchestra.InspectionRequest{
		Task:      awaitingTask,
		Execution: executionRecord,
		Worktree: orchestra.WorktreeState{
			WorktreeID: awaitingTask.WorktreeID,
			Revision:   "rev-1",
			Dirty:      false,
			Valid:      true,
		},
		Evidence: []orchestra.InspectionEvidence{
			{ID: executionRecord.EvidenceIDs[0], Kind: "command_execution", Passed: true, Summary: "code generated"},
		},
	}
	res, err := inspector.Inspect(ctx, req)
	if err != nil {
		t.Fatalf("inspect failed: %v", err)
	}
	if !res.Accepted || !res.MergeReady {
		t.Fatalf("expected inspection accepted and mergeReady")
	}

	// Persist the merge outcome
	if err := taskStore.SaveTask(ctx, res.AcceptedTask); err != nil {
		t.Fatalf("save accepted task: %v", err)
	}

	finalTask, err := taskStore.GetTask(ctx, task.ID)
	if err != nil {
		t.Fatalf("get final task: %v", err)
	}
	if finalTask.Status != orchestra.TaskSuccess {
		t.Fatalf("expected TaskSuccess after Inspector merge, got %s", finalTask.Status)
	}

	// Rejection path: evidence failed should not be accepted even for same task execution.
	badInspector := orchestra.MergeAuthority{Capability: orchestra.MergeCapability}
	rejectReq := orchestra.InspectionRequest{
		Task:      awaitingTask,
		Execution: executionRecord,
		Worktree: orchestra.WorktreeState{
			WorktreeID: awaitingTask.WorktreeID,
			Revision:   "rev-1",
			Dirty:      false,
			Valid:      true,
		},
		Evidence: []orchestra.InspectionEvidence{
			{ID: executionRecord.EvidenceIDs[0], Kind: "command_execution", Passed: false, Summary: "failing worker check"},
		},
	}
	rejectRes, rejectErr := badInspector.Inspect(ctx, rejectReq)
	if rejectErr == nil {
		t.Fatalf("expected failing evidence to be rejected by Inspector, got %+v", rejectRes)
	}
	if !errors.Is(rejectErr, orchestra.ErrInspectionRejected) {
		t.Fatalf("expected ErrInspectionRejected, got %v", rejectErr)
	}

	// Resource governance assertions: real EngineClassPi profile must stay bounded
	// so BAN-143 documents that the vertical slice does not bypass CPU/memory policy.
	if piExt.ResourceReq.MinMemoryMB != harness.DefaultResourceRequirements(harness.EngineClassPi).MinMemoryMB ||
		piExt.ResourceReq.PeakMemoryMB != harness.DefaultResourceRequirements(harness.EngineClassPi).PeakMemoryMB ||
		piExt.ResourceReq.CPUPriority != harness.DefaultResourceRequirements(harness.EngineClassPi).CPUPriority {
		t.Fatalf("Pi resource profile drifted: %+v", piExt.ResourceReq)
	}
	if piExt.ResourceReq.PeakMemoryMB <= 0 || piExt.ResourceReq.MinMemoryMB <= 0 {
		t.Fatalf("Pi resource profile must declare positive memory bounds: %+v", piExt.ResourceReq)
	}

	// Governor admission for the real EngineClassPi profile must remain bounded too.
	gov := harness.NewGovernor(
		harness.WithBudgets(map[harness.EngineClass]harness.ResourceBudget{
			harness.EngineClassPi:     {MaxWorkers: 1, MaxMemoryMB: piExt.ResourceReq.PeakMemoryMB, MaxCPUPriority: piExt.ResourceReq.CPUPriority},
			harness.EngineClassNative: {MaxWorkers: 1, MaxMemoryMB: 512, MaxCPUPriority: 4},
		}))
	reservation, admitErr := gov.Admit(ctx, harness.TaskDescriptor{TaskID: "task-ban143-pi", EngineClass: harness.EngineClassPi, Priority: harness.PriorityNormal, EstimatedMemoryMB: piExt.ResourceReq.MinMemoryMB, EstimatedCPUPriority: piExt.ResourceReq.CPUPriority})
	if admitErr != nil {
		t.Fatalf("governor should admit in-budget Pi task: %v", admitErr)
	}
	reservation.Release()

	// Governor must reject an over-budget Pi dispatch instead of executing.
	_, admitErr = gov.Admit(ctx, harness.TaskDescriptor{TaskID: "task-ban143-pi-over", EngineClass: harness.EngineClassPi, Priority: harness.PriorityNormal, EstimatedMemoryMB: piExt.ResourceReq.PeakMemoryMB + 1, EstimatedCPUPriority: piExt.ResourceReq.CPUPriority})
	if admitErr == nil {
		t.Fatalf("governor must reject over-memory Pi dispatch")
	}

	// Auditability: broker-scoped evidence must be retrievable for the execution.
	auditEvents := broker.AuditEventsForExecution(string(exec.ID))
	if len(auditEvents) == 0 {
		t.Fatalf("expected broker audit events for %s", exec.ID)
	}
}
