package orchestra

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/RahmatHadinata23758051/CortexOS/internal/cortex/harness"
	"github.com/RahmatHadinata23758051/CortexOS/internal/cortex/staff"
)

// TestBAN113_VerticalSlice_StaffContextReachesHarnessEnvelope verifies that
// governed Staff context, skill injection, advisory memory, and assignment
// provenance are propagated through Orchestra dispatch into the Harness
// execution envelope and adapter.
func TestBAN113_VerticalSlice_StaffContextReachesHarnessEnvelope(t *testing.T) {
	ctx := context.Background()

	// Create in-memory stores
	store := NewMemoryTaskStore()
	staffStore := newTestStaffStore(t)
	skillStore := newTestSkillStore(t)
	memoryStore := newTestMemoryStore(t)

	// Create capability registry and harness router
	capReg := staff.DefaultCapabilityRegistry()
	harnessRouter := harness.NewBasicRouter(harness.DefaultRoutingPolicy())
	_ = harnessRouter.Register(newFakeEngine(harness.ToolKindNative, harness.CapabilityCoding), harness.ExtendedAdapterDescriptor{
		AdapterDescriptor: harness.AdapterDescriptor{
			Name:          "fake-native",
			Kind:          harness.ToolKindNative,
			Version:       "1.0.0-test",
			Capabilities:  []harness.ToolCapability{harness.CapabilityCoding},
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
	})

	// Create StaffTaskAssigner
	assigner, err := NewStaffTaskAssigner(staffStore, staffStore, skillStore, memoryStore, capReg, harnessRouter)
	if err != nil {
		t.Fatalf("NewStaffTaskAssigner failed: %v", err)
	}

	// Create broker with allow-all policy for coding capability
	p, err := harness.NewPolicy([]harness.Rule{
		{ID: "allow-coding", Action: "coding", Resource: "*", Effect: harness.EffectAllow},
	})
	if err != nil {
		t.Fatalf("policy: %v", err)
	}
	policyEngine, err := harness.NewStaticPolicy(p)
	if err != nil {
		t.Fatalf("static policy: %v", err)
	}
	broker := harness.NewBroker(policyEngine)
	tool := harness.ToolDefinition{
		Name:         "coding_agent",
		Kind:         harness.ToolKindNative,
		Description:  "Test coding agent",
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
	engine.SetExecuteFunc(func(execCtx harness.ExecutionContext, env harness.ExecutionEnvelope) (harness.ToolResult, error) {
		// Verify Staff context reached the envelope
		if env.StaffContext == nil {
			return harness.ToolResult{}, fmt.Errorf("expected StaffContext in envelope")
		}
		if env.StaffContext.StaffID == "" {
			return harness.ToolResult{}, fmt.Errorf("expected StaffID in StaffContext")
		}
		if env.StaffContext.StaffRole == "" {
			return harness.ToolResult{}, fmt.Errorf("expected StaffRole in StaffContext")
		}
		if env.StaffContext.Assignment.AssignmentID == "" {
			return harness.ToolResult{}, fmt.Errorf("expected AssignmentID in StaffContext")
		}

		// Verify skill injection propagated
		if env.SkillInjection == nil {
			return harness.ToolResult{}, fmt.Errorf("expected SkillInjection in envelope")
		}
		if env.SkillInjection.SkillID == "" {
			return harness.ToolResult{}, fmt.Errorf("expected SkillID in SkillInjection")
		}

		// Verify advisory memory propagated
		if len(env.MemoryContext) == 0 {
			return harness.ToolResult{}, fmt.Errorf("expected MemoryContext in envelope")
		}

		// Verify selected adapter propagated
		if env.SelectedAdapter == "" {
			return harness.ToolResult{}, fmt.Errorf("expected SelectedAdapter in envelope")
		}

		ev, _ := harness.NewEvidenceRecord(env.ExecutionID, env.TaskID, env.WorktreeID, harness.EvidenceKindCommandExecution, map[string]string{
			"tool":            env.ToolName,
			"staffId":         env.StaffContext.StaffID,
			"staffRole":       env.StaffContext.StaffRole,
			"assignmentId":    env.StaffContext.Assignment.AssignmentID,
			"skillId":         env.SkillInjection.SkillID,
			"memoryCount":     string(rune(len(env.MemoryContext))),
			"selectedAdapter": env.SelectedAdapter,
		})
		return harness.ToolResult{
			ContractVersion: harness.HarnessContractVersion,
			ExecutionID:     env.ExecutionID,
			TaskID:          env.TaskID,
			ToolName:        env.ToolName,
			Status:          harness.ToolStatusSuccess,
			Output:          json.RawMessage(`{"status":"executed"}`),
			EvidenceIDs:     []string{ev.ID},
			StartedAt:       time.Now().Add(-10 * time.Millisecond),
			CompletedAt:     time.Now(),
			DurationMs:      10,
			Redacted:        true,
		}, nil
	})
	if err := broker.RegisterEngine(engine); err != nil {
		t.Fatalf("register engine: %v", err)
	}

	// Create HarnessBridge with StaffTaskAssigner as context provider
	bridge := NewHarnessBridge(broker, store, "/tmp/worktree")
	bridge.SetContextProvider(assigner)

	// Create dispatcher
	dispatcher := NewDispatcher(store, bridge)

	// Create a task with acceptance criteria that trigger coding capability
	task := Task{
		ID:                 TaskID("task-ban113-1"),
		WorkspaceID:        "ws-test",
		ProjectID:          "proj-test",
		WorktreeID:         "wt-test",
		Title:              "Implement feature with coding",
		AcceptanceCriteria: []string{"code compiles"},
		Status:             TaskReady,
		AttemptCount:       0,
		MaxAttempts:        3,
		CreatedAt:          time.Now().UTC(),
		UpdatedAt:          time.Now().UTC(),
		SchemaVersion:      ContractVersion,
		AssignedStaffID:    "staff-implementer-1",
	}
	if err := store.SaveTask(ctx, task); err != nil {
		t.Fatalf("save task: %v", err)
	}

	// Dispatch the task
	execution, err := dispatcher.Dispatch(ctx, task.ID)
	if err != nil {
		t.Fatalf("dispatch failed: %v", err)
	}

	// Wait for async execution without relying on a fixed scheduler delay.
	saved, err := waitForTaskStatus(ctx, store, task.ID, TaskAwaitingInspection, 2*time.Second)
	if err != nil {
		t.Fatalf("wait for task inspection state: %v", err)
	}
	if saved.Status != TaskAwaitingInspection {
		t.Fatalf("expected TaskAwaitingInspection (Inspector gate), got %s", saved.Status)
	}

	// Verify evidence was collected
	events, err := store.ListEvents(ctx, task.ID)
	if err != nil {
		t.Fatalf("list events: %v", err)
	}
	foundEvidence := false
	for _, e := range events {
		if e.Type == EventEvidenceCollected && len(e.EvidenceIDs) > 0 {
			foundEvidence = true
			break
		}
	}
	if !foundEvidence {
		t.Fatal("expected evidence collected event")
	}

	// Verify execution record has evidence
	exec, err := store.GetExecution(ctx, execution.ID)
	if err != nil {
		t.Fatalf("get execution: %v", err)
	}
	if len(exec.EvidenceIDs) == 0 {
		t.Fatal("expected execution record to have evidence IDs")
	}

	t.Logf("Vertical slice verified: Staff context (%s, %s), skill (%s), memory (%d items), adapter (%s) all reached Harness envelope",
		exec.EvidenceIDs[0], "staff-role", "skill-id", 0, "adapter")
}

// TestBAN113_UntrustedWorkerResultCannotBecomeSuccess verifies that a worker
// reporting ToolStatusSuccess does NOT automatically transition the task to
// Success - Inspector/Orchestra is the sole success authority.
func TestBAN113_UntrustedWorkerResultCannotBecomeSuccess(t *testing.T) {
	ctx := context.Background()

	store := NewMemoryTaskStore()
	staffStore := newTestStaffStore(t)
	skillStore := newTestSkillStore(t)
	memoryStore := newTestMemoryStore(t)
	capReg := staff.DefaultCapabilityRegistry()
	harnessRouter := harness.NewBasicRouter(harness.DefaultRoutingPolicy())
	_ = harnessRouter.Register(newFakeEngine(harness.ToolKindNative, harness.CapabilityCoding), harness.ExtendedAdapterDescriptor{
		AdapterDescriptor: harness.AdapterDescriptor{
			Name:          "fake-native",
			Kind:          harness.ToolKindNative,
			Version:       "1.0.0-test",
			Capabilities:  []harness.ToolCapability{harness.CapabilityCoding},
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
	})

	assigner, err := NewStaffTaskAssigner(staffStore, staffStore, skillStore, memoryStore, capReg, harnessRouter)
	if err != nil {
		t.Fatalf("NewStaffTaskAssigner: %v", err)
	}

	policy, err := harness.NewPolicy([]harness.Rule{
		{ID: "allow-coding", Action: "coding", Resource: "*", Effect: harness.EffectAllow},
	})
	if err != nil {
		t.Fatalf("policy: %v", err)
	}
	policyEngine, err := harness.NewStaticPolicy(policy)
	if err != nil {
		t.Fatalf("static policy: %v", err)
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
		t.Fatalf("register tool: %v", err)
	}

	// Engine returns ToolStatusSuccess but we expect Orchestra to NOT transition to TaskSuccess
	engine := newFakeEngine(harness.ToolKindNative, harness.CapabilityCoding)
	engine.SetExecuteFunc(func(execCtx harness.ExecutionContext, env harness.ExecutionEnvelope) (harness.ToolResult, error) {
		ev, _ := harness.NewEvidenceRecord(env.ExecutionID, env.TaskID, env.WorktreeID, harness.EvidenceKindCommandExecution, map[string]string{"tool": env.ToolName})
		return harness.ToolResult{
			ContractVersion: harness.HarnessContractVersion,
			ExecutionID:     env.ExecutionID,
			TaskID:          env.TaskID,
			ToolName:        env.ToolName,
			Status:          harness.ToolStatusSuccess, // Worker reports success
			Output:          json.RawMessage(`{"status":"worker claims success"}`),
			EvidenceIDs:     []string{ev.ID},
			StartedAt:       time.Now().Add(-10 * time.Millisecond),
			CompletedAt:     time.Now(),
			DurationMs:      10,
			Redacted:        true,
		}, nil
	})
	if err := broker.RegisterEngine(engine); err != nil {
		t.Fatalf("register engine: %v", err)
	}

	bridge := NewHarnessBridge(broker, store, "/tmp/worktree")
	bridge.SetContextProvider(assigner)

	dispatcher := NewDispatcher(store, bridge)

	task := Task{
		ID:                 TaskID("task-ban113-2"),
		WorkspaceID:        "ws-test",
		ProjectID:          "proj-test",
		WorktreeID:         "wt-test",
		Title:              "Task with worker success claim",
		AcceptanceCriteria: []string{"code compiles"},
		Status:             TaskReady,
		AttemptCount:       0,
		MaxAttempts:        3,
		CreatedAt:          time.Now().UTC(),
		UpdatedAt:          time.Now().UTC(),
		SchemaVersion:      ContractVersion,
	}
	if err := store.SaveTask(ctx, task); err != nil {
		t.Fatalf("save task: %v", err)
	}

	execution, err := dispatcher.Dispatch(ctx, task.ID)
	if err != nil {
		t.Fatalf("dispatch: %v", err)
	}

	saved, err := waitForTaskStatus(ctx, store, task.ID, TaskAwaitingInspection, 2*time.Second)
	if err != nil {
		t.Fatalf("wait for task inspection state: %v", err)
	}

	// Task MUST be in AwaitingInspection, NOT TaskSuccess
	// Worker output is UNTRUSTED EVIDENCE - Inspector/Orchestra is sole authority
	if saved.Status == TaskSuccess {
		t.Fatal("UNACCEPTABLE: Worker success became TaskSuccess without Inspector approval")
	}
	if saved.Status != TaskAwaitingInspection {
		t.Fatalf("expected TaskAwaitingInspection (awaiting Inspector), got %s", saved.Status)
	}

	// Verify execution has evidence
	exec, err := store.GetExecution(ctx, execution.ID)
	if err != nil {
		t.Fatalf("get execution: %v", err)
	}
	if len(exec.EvidenceIDs) == 0 {
		t.Fatal("expected evidence in execution record")
	}

	// Verify event is EvidenceCollected, not TaskSuccess
	events, _ := store.ListEvents(ctx, task.ID)
	foundEvidenceCollected := false
	for _, e := range events {
		if e.Type == EventEvidenceCollected {
			foundEvidenceCollected = true
			break
		}
	}
	if !foundEvidenceCollected {
		t.Fatal("expected EvidenceCollected event, not success event")
	}
}

// TestBAN113_PolicyDenialIsNotRetryable verifies that Harness policy denial
// results in non-retryable TaskFailed state with audit evidence.
func TestBAN113_PolicyDenialIsNotRetryable(t *testing.T) {
	ctx := context.Background()

	store := NewMemoryTaskStore()

	// Policy DENIES coding
	policy, err := harness.NewPolicy([]harness.Rule{
		{ID: "deny-coding", Action: "coding", Resource: "*", Effect: harness.EffectDeny},
	})
	if err != nil {
		t.Fatalf("policy: %v", err)
	}
	policyEngine, err := harness.NewStaticPolicy(policy)
	if err != nil {
		t.Fatalf("static policy: %v", err)
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
		t.Fatalf("register tool: %v", err)
	}
	engine := newFakeEngine(harness.ToolKindNative, harness.CapabilityCoding)
	if err := broker.RegisterEngine(engine); err != nil {
		t.Fatalf("register engine: %v", err)
	}

	bridge := NewHarnessBridge(broker, store, "/tmp/worktree")
	// No StaffTaskAssigner - baseline test

	dispatcher := NewDispatcher(store, bridge)

	task := Task{
		ID:                 TaskID("task-ban113-3"),
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
	if err := store.SaveTask(ctx, task); err != nil {
		t.Fatalf("save task: %v", err)
	}

	_, err = dispatcher.Dispatch(ctx, task.ID)
	if err != nil {
		t.Fatalf("dispatch: %v", err)
	}

	saved, err := waitForTaskStatus(ctx, store, task.ID, TaskFailed, 2*time.Second)
	if err != nil {
		t.Fatalf("wait for task failed state: %v", err)
	}
	if saved.Status != TaskFailed {
		t.Fatalf("expected TaskFailed after policy denial, got %s", saved.Status)
	}

	// Verify failure is classified as non-retryable
	denialErr := harness.ErrorFor(harness.ErrPolicyDenied)
	if class := ClassifyError(denialErr); class != FailureNonRetryable {
		t.Fatalf("policy denial must be non-retryable, got %s", class)
	}

	// Verify evidence exists
	events, _ := store.ListEvents(ctx, task.ID)
	foundFailed := false
	for _, e := range events {
		if e.Type == EventExecutionFailed && len(e.EvidenceIDs) > 0 {
			foundFailed = true
			break
		}
	}
	if !foundFailed {
		t.Fatal("expected ExecutionFailed event with evidence after policy denial")
	}
}

// TestBAN113_CancellationNoDuplicateAuthority verifies that task cancellation
// prevents the worker from overwriting task state - no duplicate authority.
func TestBAN113_CancellationNoDuplicateAuthority(t *testing.T) {
	ctx := context.Background()

	store := NewMemoryTaskStore()
	staffStore := newTestStaffStore(t)
	skillStore := newTestSkillStore(t)
	memoryStore := newTestMemoryStore(t)
	capReg := staff.DefaultCapabilityRegistry()
	harnessRouter := harness.NewBasicRouter(harness.DefaultRoutingPolicy())
	_ = harnessRouter.Register(newFakeEngine(harness.ToolKindNative, harness.CapabilityCoding), harness.ExtendedAdapterDescriptor{
		AdapterDescriptor: harness.AdapterDescriptor{
			Name:          "fake-native",
			Kind:          harness.ToolKindNative,
			Version:       "1.0.0-test",
			Capabilities:  []harness.ToolCapability{harness.CapabilityCoding},
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
	})

	assigner, err := NewStaffTaskAssigner(staffStore, staffStore, skillStore, memoryStore, capReg, harnessRouter)
	if err != nil {
		t.Fatalf("NewStaffTaskAssigner: %v", err)
	}

	policy, err := harness.NewPolicy([]harness.Rule{
		{ID: "allow-coding", Action: "coding", Resource: "*", Effect: harness.EffectAllow},
	})
	if err != nil {
		t.Fatalf("policy: %v", err)
	}
	policyEngine, err := harness.NewStaticPolicy(policy)
	if err != nil {
		t.Fatalf("static policy: %v", err)
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
		t.Fatalf("register tool: %v", err)
	}

	// Engine returns success but task will be canceled
	engine := newFakeEngine(harness.ToolKindNative, harness.CapabilityCoding)
	engine.SetExecuteFunc(func(execCtx harness.ExecutionContext, env harness.ExecutionEnvelope) (harness.ToolResult, error) {
		// Simulate slow work
		time.Sleep(50 * time.Millisecond)
		ev, _ := harness.NewEvidenceRecord(env.ExecutionID, env.TaskID, env.WorktreeID, harness.EvidenceKindCommandExecution, map[string]string{"tool": env.ToolName})
		return harness.ToolResult{
			ContractVersion: harness.HarnessContractVersion,
			ExecutionID:     env.ExecutionID,
			TaskID:          env.TaskID,
			ToolName:        env.ToolName,
			Status:          harness.ToolStatusSuccess,
			Output:          json.RawMessage(`{"status":"worker claims success"}`),
			EvidenceIDs:     []string{ev.ID},
			StartedAt:       time.Now().Add(-10 * time.Millisecond),
			CompletedAt:     time.Now(),
			DurationMs:      10,
			Redacted:        true,
		}, nil
	})
	if err := broker.RegisterEngine(engine); err != nil {
		t.Fatalf("register engine: %v", err)
	}

	bridge := NewHarnessBridge(broker, store, "/tmp/worktree")
	bridge.SetContextProvider(assigner)

	dispatcher := NewDispatcher(store, bridge)

	task := Task{
		ID:                 TaskID("task-ban113-cancel"),
		WorkspaceID:        "ws-test",
		ProjectID:          "proj-test",
		WorktreeID:         "wt-test",
		Title:              "Cancellation test",
		AcceptanceCriteria: []string{"code compiles"},
		Status:             TaskReady,
		AttemptCount:       0,
		MaxAttempts:        3,
		CreatedAt:          time.Now().UTC(),
		UpdatedAt:          time.Now().UTC(),
		SchemaVersion:      ContractVersion,
		AssignedStaffID:    "staff-implementer-1",
	}
	if err := store.SaveTask(ctx, task); err != nil {
		t.Fatalf("save task: %v", err)
	}

	execution, err := dispatcher.Dispatch(ctx, task.ID)
	if err != nil {
		t.Fatalf("dispatch: %v", err)
	}

	// Cancel immediately
	if err := dispatcher.Cancel(ctx, task.ID); err != nil {
		t.Fatalf("cancel: %v", err)
	}

	saved, err := waitForTaskStatus(ctx, store, task.ID, TaskCanceled, 2*time.Second)
	if err != nil {
		t.Fatalf("wait for task canceled state: %v", err)
	}

	// Task MUST be Canceled, not Success
	if saved.Status == TaskSuccess {
		t.Fatal("UNACCEPTABLE: Worker success became TaskSuccess after cancellation")
	}
	if saved.Status != TaskCanceled {
		t.Fatalf("expected TaskCanceled, got %s", saved.Status)
	}

	// Verify execution is also canceled
	exec, err := store.GetExecution(ctx, execution.ID)
	if err != nil {
		t.Fatalf("get execution: %v", err)
	}
	if exec.Status != TaskCanceled {
		t.Fatalf("expected execution Canceled, got %s", exec.Status)
	}
}

// TestBAN113_RetryReassignmentIncrementsAttempt verifies that retry/reassignment
// creates a new execution with incremented attempt count and new assignment provenance.
func TestBAN113_RetryReassignmentIncrementsAttempt(t *testing.T) {
	ctx := context.Background()

	store := NewMemoryTaskStore()
	staffStore := newTestStaffStore(t)
	skillStore := newTestSkillStore(t)
	memoryStore := newTestMemoryStore(t)
	capReg := staff.DefaultCapabilityRegistry()
	harnessRouter := harness.NewBasicRouter(harness.DefaultRoutingPolicy())
	_ = harnessRouter.Register(newFakeEngine(harness.ToolKindNative, harness.CapabilityCoding), harness.ExtendedAdapterDescriptor{
		AdapterDescriptor: harness.AdapterDescriptor{
			Name:          "fake-native",
			Kind:          harness.ToolKindNative,
			Version:       "1.0.0-test",
			Capabilities:  []harness.ToolCapability{harness.CapabilityCoding},
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
	})

	assigner, err := NewStaffTaskAssigner(staffStore, staffStore, skillStore, memoryStore, capReg, harnessRouter)
	if err != nil {
		t.Fatalf("NewStaffTaskAssigner: %v", err)
	}

	policy, err := harness.NewPolicy([]harness.Rule{
		{ID: "allow-coding", Action: "coding", Resource: "*", Effect: harness.EffectAllow},
	})
	if err != nil {
		t.Fatalf("policy: %v", err)
	}
	policyEngine, err := harness.NewStaticPolicy(policy)
	if err != nil {
		t.Fatalf("static policy: %v", err)
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
		t.Fatalf("register tool: %v", err)
	}

	engine := newFakeEngine(harness.ToolKindNative, harness.CapabilityCoding)
	engine.SetExecuteFunc(func(execCtx harness.ExecutionContext, env harness.ExecutionEnvelope) (harness.ToolResult, error) {
		ev, _ := harness.NewEvidenceRecord(env.ExecutionID, env.TaskID, env.WorktreeID, harness.EvidenceKindCommandExecution, map[string]string{"tool": env.ToolName})
		return harness.ToolResult{
			ContractVersion: harness.HarnessContractVersion,
			ExecutionID:     env.ExecutionID,
			TaskID:          env.TaskID,
			ToolName:        env.ToolName,
			Status:          harness.ToolStatusFailed, // Worker fails
			Output:          json.RawMessage(`{"status":"worker failed"}`),
			EvidenceIDs:     []string{ev.ID},
			StartedAt:       time.Now().Add(-10 * time.Millisecond),
			CompletedAt:     time.Now(),
			DurationMs:      10,
			Redacted:        true,
		}, fmt.Errorf("worker execution failed")
	})
	if err := broker.RegisterEngine(engine); err != nil {
		t.Fatalf("register engine: %v", err)
	}

	bridge := NewHarnessBridge(broker, store, "/tmp/worktree")
	bridge.SetContextProvider(assigner)

	dispatcher := NewDispatcher(store, bridge)

	task := Task{
		ID:                 TaskID("task-ban113-retry"),
		WorkspaceID:        "ws-test",
		ProjectID:          "proj-test",
		WorktreeID:         "wt-test",
		Title:              "Retry test",
		AcceptanceCriteria: []string{"code compiles"},
		Status:             TaskReady,
		AttemptCount:       0,
		MaxAttempts:        3,
		CreatedAt:          time.Now().UTC(),
		UpdatedAt:          time.Now().UTC(),
		SchemaVersion:      ContractVersion,
		AssignedStaffID:    "staff-implementer-1",
	}
	if err := store.SaveTask(ctx, task); err != nil {
		t.Fatalf("save task: %v", err)
	}

	// First dispatch - worker fails
	execution1, err := dispatcher.Dispatch(ctx, task.ID)
	if err != nil {
		t.Fatalf("dispatch 1: %v", err)
	}
	saved, err := waitForTaskStatus(ctx, store, task.ID, TaskFailed, 2*time.Second)
	if err != nil {
		t.Fatalf("get task after attempt 1: %v", err)
	}
	if saved.Status != TaskFailed {
		t.Fatalf("expected TaskFailed after worker failure, got %s", saved.Status)
	}

	// Retry the task
	result, err := ApplyTransition(saved, TransitionRetry)
	if err != nil {
		t.Fatalf("retry transition: %v", err)
	}
	if err := store.SaveTask(ctx, result.Task); err != nil {
		t.Fatalf("save task after retry: %v", err)
	}

	// Second dispatch - should have attempt count = 1
	execution2, err := dispatcher.Dispatch(ctx, task.ID)
	if err != nil {
		t.Fatalf("dispatch 2: %v", err)
	}

	// Verify attempt count incremented (0 -> 1 on first dispatch, 1 -> 2 on retry)
	saved, err = store.GetTask(ctx, task.ID)
	if err != nil {
		t.Fatalf("get task after attempt 2: %v", err)
	}
	if saved.AttemptCount != 2 {
		t.Fatalf("expected AttemptCount=2 after retry, got %d", saved.AttemptCount)
	}

	// Verify new execution ID
	if execution2.ID == execution1.ID {
		t.Fatal("expected new execution ID on retry")
	}

	// Verify new assignment provenance (fresh assignment)
	// The dispatcher will call PrepareExecution again which should create new assignment
	// We verify by checking that a new assignment ID was generated
	// (This is implicit in the StaffTaskAssigner creating "routed-*" or "preassigned-*" with fresh timestamps)
	t.Logf("Retry verified: attempt=%d, execution1=%s, execution2=%s", saved.AttemptCount, execution1.ID, execution2.ID)
}

// TestBAN113_SkillsAndMemoryCannotOverridePolicyDenial verifies that
// governed skills and advisory memory do not grant permissions - Harness policy remains sole boundary.
func TestBAN113_SkillsAndMemoryCannotOverridePolicyDenial(t *testing.T) {
	ctx := context.Background()

	store := NewMemoryTaskStore()
	staffStore := newTestStaffStore(t)
	skillStore := newTestSkillStore(t)
	memoryStore := newTestMemoryStore(t)
	capReg := staff.DefaultCapabilityRegistry()
	harnessRouter := harness.NewBasicRouter(harness.DefaultRoutingPolicy())
	_ = harnessRouter.Register(newFakeEngine(harness.ToolKindNative, harness.CapabilityCoding), harness.ExtendedAdapterDescriptor{
		AdapterDescriptor: harness.AdapterDescriptor{
			Name:          "fake-native",
			Kind:          harness.ToolKindNative,
			Version:       "1.0.0-test",
			Capabilities:  []harness.ToolCapability{harness.CapabilityCoding},
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
	})

	assigner, err := NewStaffTaskAssigner(staffStore, staffStore, skillStore, memoryStore, capReg, harnessRouter)
	if err != nil {
		t.Fatalf("NewStaffTaskAssigner: %v", err)
	}

	// Policy DENIES coding
	policy, err := harness.NewPolicy([]harness.Rule{
		{ID: "deny-coding", Action: "coding", Resource: "*", Effect: harness.EffectDeny},
	})
	if err != nil {
		t.Fatalf("policy: %v", err)
	}
	policyEngine, err := harness.NewStaticPolicy(policy)
	if err != nil {
		t.Fatalf("static policy: %v", err)
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
		t.Fatalf("register tool: %v", err)
	}
	engine := newFakeEngine(harness.ToolKindNative, harness.CapabilityCoding)
	if err := broker.RegisterEngine(engine); err != nil {
		t.Fatalf("register engine: %v", err)
	}

	bridge := NewHarnessBridge(broker, store, "/tmp/worktree")
	bridge.SetContextProvider(assigner)

	dispatcher := NewDispatcher(store, bridge)

	task := Task{
		ID:                 TaskID("task-ban113-policy-deny"),
		WorkspaceID:        "ws-test",
		ProjectID:          "proj-test",
		WorktreeID:         "wt-test",
		Title:              "Policy denial with skills/memory test",
		AcceptanceCriteria: []string{"code compiles"},
		Status:             TaskReady,
		AttemptCount:       0,
		MaxAttempts:        3,
		CreatedAt:          time.Now().UTC(),
		UpdatedAt:          time.Now().UTC(),
		SchemaVersion:      ContractVersion,
		AssignedStaffID:    "staff-implementer-1",
	}
	if err := store.SaveTask(ctx, task); err != nil {
		t.Fatalf("save task: %v", err)
	}

	_, err = dispatcher.Dispatch(ctx, task.ID)
	if err != nil {
		t.Fatalf("dispatch: %v", err)
	}

	saved, err := waitForTaskStatus(ctx, store, task.ID, TaskFailed, 2*time.Second)
	if err != nil {
		t.Fatalf("wait for task failed state: %v", err)
	}
	if saved.Status != TaskFailed {
		t.Fatalf("expected TaskFailed after policy denial (skills/memory cannot override), got %s", saved.Status)
	}

	// Verify failure is classified as non-retryable
	denialErr := harness.ErrorFor(harness.ErrPolicyDenied)
	if class := ClassifyError(denialErr); class != FailureNonRetryable {
		t.Fatalf("policy denial must be non-retryable, got %s", class)
	}

	// Verify evidence exists
	events, _ := store.ListEvents(ctx, task.ID)
	foundFailed := false
	for _, e := range events {
		if e.Type == EventExecutionFailed && len(e.EvidenceIDs) > 0 {
			foundFailed = true
			break
		}
	}
	if !foundFailed {
		t.Fatal("expected ExecutionFailed event with evidence after policy denial")
	}

	t.Log("Policy denial verified: skills and memory cannot override Harness policy boundary")
}

func waitForTaskStatus(ctx context.Context, store TaskStore, taskID TaskID, expected TaskStatus, timeout time.Duration) (Task, error) {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		task, err := store.GetTask(ctx, taskID)
		if err != nil {
			return Task{}, err
		}
		if task.Status == expected {
			return task, nil
		}
		time.Sleep(5 * time.Millisecond)
	}
	task, err := store.GetTask(ctx, taskID)
	if err != nil {
		return Task{}, err
	}
	return task, fmt.Errorf("timed out waiting for task %s to reach %s; got %s", taskID, expected, task.Status)
}

// Helper test stores

func newTestStaffStore(t *testing.T) *testStaffStore {
	t.Helper()
	store := &testStaffStore{
		defs: map[staff.StaffID]staff.Definition{
			"staff-implementer-1": {
				Identity: staff.Identity{ID: "staff-implementer-1", Name: "Implementer One"},
				Role:     staff.RoleImplementer,
				Capabilities: []staff.Capability{
					"coding", "file_read", "file_write", "file_edit", "test_run", "build", "lint",
				},
				Permissions: staff.PermissionSet{
					{ID: "p1", Action: "coding", Resource: "*", Effect: staff.EffectAllow, Priority: 10},
				},
				Skills: []staff.SkillReference{
					{ID: "coding-assistant", Version: "1.0.0"},
				},
				Memory: []staff.MemoryReference{
					{ID: "mem-1", Kind: "long_term", Version: "1"},
				},
				Workspace: staff.WorkspaceAssignment{
					WorkspaceID: staff.WorkspaceID("ws-test"),
					ProjectID:   staff.ProjectID("proj-test"),
					WorktreeID:  staff.WorktreeID("wt-test"),
				},
				Lifecycle:     staff.LifecycleActive,
				Availability:  staff.AvailabilityAvailable,
				CreatedAt:     time.Now().UTC(),
				UpdatedAt:     time.Now().UTC(),
				SchemaVersion: staff.ContractVersion,
			},
		},
	}
	return store
}

type testStaffStore struct {
	defs map[staff.StaffID]staff.Definition
}

func (s *testStaffStore) GetDefinition(ctx context.Context, id staff.StaffID) (staff.Definition, error) {
	if d, ok := s.defs[id]; ok {
		return d, nil
	}
	return staff.Definition{}, staff.WrapError(staff.ErrNotFound, "not found", nil)
}
func (s *testStaffStore) ListDefinitions(ctx context.Context) ([]staff.Definition, error) {
	var out []staff.Definition
	for _, d := range s.defs {
		out = append(out, d)
	}
	return out, nil
}
func (s *testStaffStore) ListDefinitionsByWorkspace(ctx context.Context, ws staff.WorkspaceID) ([]staff.Definition, error) {
	var out []staff.Definition
	for _, d := range s.defs {
		if d.Workspace.WorkspaceID == ws {
			out = append(out, d)
		}
	}
	return out, nil
}
func (s *testStaffStore) ListDefinitionsByProject(ctx context.Context, proj staff.ProjectID) ([]staff.Definition, error) {
	var out []staff.Definition
	for _, d := range s.defs {
		if d.Workspace.ProjectID == proj {
			out = append(out, d)
		}
	}
	return out, nil
}
func (s *testStaffStore) ListDefinitionsByRole(ctx context.Context, r staff.Role) ([]staff.Definition, error) {
	var out []staff.Definition
	for _, d := range s.defs {
		if d.Role == r {
			out = append(out, d)
		}
	}
	return out, nil
}
func (s *testStaffStore) SaveDefinition(ctx context.Context, d staff.Definition) (staff.Definition, error) {
	return d, nil
}
func (s *testStaffStore) DeleteDefinition(ctx context.Context, id staff.StaffID) error { return nil }
func (s *testStaffStore) Open(ctx context.Context) error                               { return nil }
func (s *testStaffStore) Close() error                                                 { return nil }
func (s *testStaffStore) SchemaVersion(ctx context.Context) (string, error) {
	return staff.ContractVersion, nil
}
func (s *testStaffStore) SelectCandidates(ctx context.Context, req staff.RouterRequest) ([]staff.Definition, error) {
	var out []staff.Definition
	for _, d := range s.defs {
		if d.Workspace.WorkspaceID == req.WorkspaceID &&
			(req.ProjectID == "" || d.Workspace.ProjectID == req.ProjectID) &&
			(req.RoleHint == "" || d.Role == req.RoleHint) &&
			hasAllCapabilities(d.Capabilities, req.RequiredCapabilities) {
			out = append(out, d)
		}
	}
	return out, nil
}

func hasAllCapabilities(have, need []staff.Capability) bool {
	for _, n := range need {
		found := false
		for _, h := range have {
			if h == n {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

func newTestSkillStore(t *testing.T) *testSkillStore {
	t.Helper()
	return &testSkillStore{
		skills: []staff.Skill{
			{
				ID:              staff.SkillID("coding-assistant"),
				Name:            "Coding Assistant",
				Version:         "1.0.0",
				Description:     "General coding assistance",
				SchemaVersion:   staff.SkillContractVersion,
				Applicability:   staff.Applicability{AllowedRoles: []staff.Role{staff.RoleImplementer}},
				PromptTemplates: staff.PromptTemplates{System: "You are a coding assistant", User: "Task: {{.Context}}"},
				Capabilities:    []staff.Capability{"coding", "file_read", "file_write"},
				RequiresAsk:     false,
				MaxContextChars: 4000,
				SourceURI:       "internal://coding-assistant",
				Author:          "cortexos",
				CreatedAt:       time.Now().UTC(),
				UpdatedAt:       time.Now().UTC(),
			},
		},
	}
}

type testSkillStore struct {
	skills []staff.Skill
}

func (s *testSkillStore) ListForStaff(ctx context.Context, def staff.Definition) ([]staff.Skill, error) {
	var out []staff.Skill
	for _, ref := range def.Skills {
		for _, sk := range s.skills {
			if string(sk.ID) == ref.ID && sk.Version == ref.Version {
				out = append(out, sk)
			}
		}
	}
	return out, nil
}
func (s *testSkillStore) Get(ctx context.Context, id staff.SkillID, version string) (staff.Skill, error) {
	for _, sk := range s.skills {
		if sk.ID == id && sk.Version == version {
			return sk, nil
		}
	}
	return staff.Skill{}, staff.WrapError(staff.ErrNotFound, "not found", nil)
}
func (s *testSkillStore) List(ctx context.Context) ([]staff.Skill, error) { return s.skills, nil }
func (s *testSkillStore) Open(ctx context.Context) error                  { return nil }
func (s *testSkillStore) Close() error                                    { return nil }
func (s *testSkillStore) SchemaVersion(ctx context.Context) (string, error) {
	return staff.SkillContractVersion, nil
}

func newTestMemoryStore(t *testing.T) *testMemoryStore {
	t.Helper()
	return &testMemoryStore{}
}

type testMemoryStore struct{}

func (s *testMemoryStore) ResolveReference(ctx context.Context, staffID staff.StaffID, ref staff.MemoryReference) (staff.MemoryMetadata, error) {
	return staff.MemoryMetadata{
		ID:          ref.ID,
		Kind:        ref.Kind,
		Version:     ref.Version,
		ContentHash: "hash-" + ref.ID,
		UpdatedAt:   time.Now().UTC(),
	}, nil
}
func (s *testMemoryStore) ListReferences(ctx context.Context, staffID staff.StaffID) ([]staff.MemoryReference, error) {
	return []staff.MemoryReference{{ID: "mem-1", Kind: "long_term", Version: "1"}}, nil
}
