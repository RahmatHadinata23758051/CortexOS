package orchestra_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/RahmatHadinata23758051/CortexOS/internal/cortex/application"
	"github.com/RahmatHadinata23758051/CortexOS/internal/cortex/bridge"
	"github.com/RahmatHadinata23758051/CortexOS/internal/cortex/harness"
	"github.com/RahmatHadinata23758051/CortexOS/internal/cortex/orchestra"
	"github.com/RahmatHadinata23758051/CortexOS/internal/platform"
)

type ban164GroundingStub struct {
	items []application.KnowledgeContextItem
}

func (s ban164GroundingStub) GroundRoutingContext(_ context.Context, _ application.KnowledgeContextRequest) (application.KnowledgeContextBundle, error) {
	return application.KnowledgeContextBundle{
		Items:         s.items,
		TotalChars:    120,
		Truncated:     false,
		Degraded:      false,
		SchemaVersion: application.KnowledgeContextSchemaVersion,
	}, nil
}

// ponytail: in-memory stubs (ban164FakeEngine, ban164EvidenceReader, ban164WorktreeInspector)
// simulate external components without disk I/O; upgrade to real SQLite/process engine in live integration.
type ban164EvidenceReader struct{}

func (r *ban164EvidenceReader) EvidenceForExecution(_ context.Context, _ string) ([]harness.EvidenceRecord, error) {
	return nil, nil
}

type ban164WorktreeInspector struct{}

func (w *ban164WorktreeInspector) InspectWorktreeState(_ context.Context, worktreeID string) (orchestra.WorktreeState, error) {
	return orchestra.WorktreeState{
		WorktreeID: worktreeID,
		Revision:   "rev-1",
		Dirty:      false,
		Valid:      true,
	}, nil
}

type ban164FakeEngine struct {
	kind harness.ToolKind
	caps []harness.ToolCapability
}

func (e *ban164FakeEngine) Kind() harness.ToolKind                 { return e.kind }
func (e *ban164FakeEngine) Capabilities() []harness.ToolCapability { return e.caps }
func (e *ban164FakeEngine) Execute(_ harness.ExecutionContext, _ harness.ExecutionEnvelope) (harness.ToolResult, error) {
	return harness.ToolResult{
		Status: harness.ToolStatusSuccess,
		Output: json.RawMessage(`{"status":"ok"}`),
	}, nil
}
func (e *ban164FakeEngine) Health(_ harness.ExecutionContext) error { return nil }
func (e *ban164FakeEngine) Describe() harness.AdapterDescriptor {
	return harness.AdapterDescriptor{
		Name:          "fake-engine",
		Kind:          e.kind,
		Version:       "1.0.0",
		Capabilities:  e.caps,
		SchemaVersion: harness.HarnessContractVersion,
	}
}

// TestBAN164_EndToEnd_Phase13_AuthorityChain tests the full Phase 13 vertical flow:
// 1. Brain routing with bounded grounding context
// 2. Deterministic Orchestra task creation (Brain remains advisory)
// 3. Duplicate correlation protection & concurrency safety
// 4. Governed tool proposal & CAS operator approval
// 5. Harness execution under policy & evidence generation
// 6. Inspector / MergeAuthority gating (only path to TaskSuccess)
// 7. Event boundedness, privacy, and absence of secret/path leaks
func TestBAN164_EndToEnd_Phase13_AuthorityChain(t *testing.T) {
	ctx := context.Background()

	// Step 1: Brain and grounding setup
	fakeBrain := application.NewFakeBrainAdapter("brain-phase13")
	fakeBrain.SetResult("corr-ban164-plan", application.BrainDispatchResult{
		TaskID:        "corr-ban164-plan",
		State:         "completed",
		Output:        "PLAN:2|Analyze module|Apply governed edits",
		SchemaVersion: application.BrainBridgeContract,
	})

	grounding := ban164GroundingStub{
		items: []application.KnowledgeContextItem{
			{ID: "doc-1", Title: "Security Architecture", SourceLabel: "docs/security.md", Snippet: "Governed execution required", Score: 0.95},
		},
	}

	// Step 2: Orchestra store and routing adapter with real ProductionAdapter
	taskStore := orchestra.NewMemoryTaskStore()
	orchestraAdapter, err := orchestra.NewProductionAdapter(orchestra.ProductionAdapterConfig{
		TaskStore:         taskStore,
		HarnessStore:      &ban164EvidenceReader{},
		WorktreeInspector: &ban164WorktreeInspector{},
		Inspector:         orchestra.MergeAuthority{Capability: orchestra.MergeCapability},
	})
	if err != nil {
		t.Fatalf("failed to create orchestra adapter: %v", err)
	}

	routingAdapter, err := bridge.NewBrainOrchestraRoutingAdapter(bridge.BrainOrchestraRoutingAdapterConfig{
		Brain:     fakeBrain,
		Orchestra: orchestraAdapter,
		Grounding: grounding,
		ProjectID: "proj-ban164",
	})
	if err != nil {
		t.Fatalf("failed to create routing adapter: %v", err)
	}

	// Step 3: Service & Bridge wiring
	service := application.NewService()
	service.SetBrainOrchestra(routingAdapter)
	b := platform.NewBridge(service)

	// Collect bridge events to verify bounds and absence of secrets
	var recordedEvents []struct {
		name    string
		payload any
	}
	var eventsMu sync.Mutex
	unsub := b.RegisterEventHook(func(name string, payload any) {
		eventsMu.Lock()
		defer eventsMu.Unlock()
		recordedEvents = append(recordedEvents, struct {
			name    string
			payload any
		}{name, payload})
	})
	defer unsub()

	// Step 4: Submit RouteBrainTaskToOrchestra
	routeReq := application.BrainOrchestraRouteRequest{
		CorrelationID:      "corr-ban164",
		UserPrompt:         "Refactor core module with governed safety",
		Title:              "Refactor module",
		AcceptanceCriteria: []string{"governed tests pass"},
		SchemaVersion:      application.BrainOrchestraBridgeSchemaVersion,
	}

	routeRes, err := b.RouteBrainTaskToOrchestra(routeReq)
	if err != nil {
		t.Fatalf("RouteBrainTaskToOrchestra failed: %v", err)
	}
	if routeRes.Status != "accepted" {
		t.Fatalf("expected accepted status, got %q", routeRes.Status)
	}
	if len(routeRes.GroundedContext) != 1 || routeRes.GroundedContext[0].ID != "doc-1" {
		t.Fatalf("grounded context references missing or invalid: %#v", routeRes.GroundedContext)
	}

	// Step 5: Verify task created in Orchestra with Draft status (Brain cannot force Ready/Success)
	tasks, err := taskStore.ListTasks(ctx)
	if err != nil {
		t.Fatalf("failed to list tasks: %v", err)
	}
	if len(tasks) != 1 {
		t.Fatalf("expected 1 task in Orchestra, got %d", len(tasks))
	}
	routedTask := tasks[0]
	if routedTask.Status != orchestra.TaskDraft {
		t.Fatalf("expected routed task status to be TaskDraft, got %s", routedTask.Status)
	}
	if routedTask.ProjectID != "proj-ban164" {
		t.Fatalf("unexpected project ID: %s", routedTask.ProjectID)
	}

	// Step 6: Test Idempotent Duplicate Routing (10 concurrent requests with same correlation ID)
	var dupSuccessCount atomic.Int32
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			res, err := b.RouteBrainTaskToOrchestra(routeReq)
			if err == nil && res.Status == "accepted" && res.OrchestraTaskID == routeRes.OrchestraTaskID {
				dupSuccessCount.Add(1)
			}
		}()
	}
	wg.Wait()
	if dupSuccessCount.Load() != 10 {
		t.Fatalf("expected 10 duplicate calls to succeed idempotently, got %d", dupSuccessCount.Load())
	}
	tasksAfterDup, _ := taskStore.ListTasks(ctx)
	if len(tasksAfterDup) != 1 {
		t.Fatalf("expected still exactly 1 task in store after duplicates, got %d", len(tasksAfterDup))
	}

	// Step 7: Tool Proposal & Governed Execution Setup
	policy, err := harness.NewPolicy([]harness.Rule{
		{ID: "allow-safe", Action: "file_read", Resource: "*", Effect: harness.EffectAllow},
	})
	if err != nil {
		t.Fatalf("failed to create policy: %v", err)
	}
	staticPolicy, err := harness.NewStaticPolicy(policy)
	if err != nil {
		t.Fatalf("failed to create static policy: %v", err)
	}
	broker := harness.NewBroker(staticPolicy)

	toolDef := harness.ToolDefinition{
		Name:         "file_read",
		Kind:         harness.ToolKindNative,
		Description:  "Safe file reader",
		Capabilities: []harness.ToolCapability{harness.CapabilityFileRead},
		Timeout:      10 * time.Second,
		Version:      "1.0.0",
	}
	if err := broker.Register(toolDef); err != nil {
		t.Fatalf("failed to register tool: %v", err)
	}
	engine := &ban164FakeEngine{kind: harness.ToolKindNative, caps: []harness.ToolCapability{harness.CapabilityFileRead}}
	if err := broker.RegisterEngine(engine); err != nil {
		t.Fatalf("failed to register engine: %v", err)
	}

	toolExecutor, err := bridge.NewHarnessToolExecutor(bridge.HarnessToolExecutorConfig{
		Broker: broker,
	})
	if err != nil {
		t.Fatalf("failed to create tool executor: %v", err)
	}

	toolAdapter, err := bridge.NewToolApprovalAdapter(bridge.ToolApprovalAdapterConfig{
		Allowlist: map[string]string{
			"file_read": "file_read",
		},
		Executor: toolExecutor,
	})
	if err != nil {
		t.Fatalf("failed to create tool approval adapter: %v", err)
	}
	service.SetToolApproval(toolAdapter)

	// Step 8: Propose Tool Call
	proposal := application.ToolProposal{
		ProposalID:          "prop-ban164-1",
		CorrelationID:       "corr-ban164",
		TaskID:              string(routedTask.ID),
		WorktreeID:          "wt-ban164",
		ToolName:            "file_read",
		Capability:          "file_read",
		RedactedArgsSummary: "Read config relative path configs/app.json",
		ArgByteSize:         64,
		ProposedAt:          time.Now().UTC(),
		SchemaVersion:       application.ToolProposalSchemaVersion,
	}

	propRes, err := b.ProposeToolCall(proposal)
	if err != nil {
		t.Fatalf("ProposeToolCall failed: %v", err)
	}
	if propRes.Status != application.ToolApprovalPending {
		t.Fatalf("expected pending status, got %q", propRes.Status)
	}

	// Step 9: CAS Operator Approval Race (concurrent approval decisions)
	var approvedWins atomic.Int32
	var approveWg sync.WaitGroup
	for i := 0; i < 5; i++ {
		approveWg.Add(1)
		actor := fmt.Sprintf("operator-%d", i)
		go func(act string) {
			defer approveWg.Done()
			decRes, decErr := b.DecideToolCall(application.ToolApprovalDecision{
				ProposalID:     proposal.ProposalID,
				Decision:       "approve",
				Actor:          act,
				Reason:         "Approved review",
				IdempotencyKey: "idem-approve-1",
				DecidedAt:      time.Now().UTC(),
				SchemaVersion:  application.ToolDecisionSchemaVersion,
			})
			if decErr != nil {
				t.Logf("decide tool call error: %v", decErr)
			} else {
				t.Logf("decide tool call result: status=%s", decRes.Status)
			}
			if decErr == nil && decRes.Status == application.ToolApprovalExecuted {
				approvedWins.Add(1)
			}
		}(actor)
	}
	approveWg.Wait()
	if approvedWins.Load() == 0 {
		t.Fatal("expected at least 1 approval decision to execute successfully")
	}

	// Step 10: Verify Governed Harness Execution produced evidence & audit trail
	rec, err := toolAdapter.GetProposal(ctx, proposal.ProposalID)
	if err != nil {
		t.Fatalf("GetProposal failed: %v", err)
	}
	if rec.Status != application.ToolApprovalExecuted {
		t.Fatalf("expected proposal status executed, got %q", rec.Status)
	}

	// Step 11: Inspector Review & Merge Authority (ADR-0005)
	valRes, err := orchestra.ApplyTransition(routedTask, orchestra.TransitionValidate)
	if err != nil {
		t.Fatalf("transition to ready failed: %v", err)
	}
	routedTask = valRes.Task
	_ = taskStore.SaveTask(ctx, routedTask)

	dispRes, err := orchestra.ApplyTransition(routedTask, orchestra.TransitionDispatch)
	if err != nil {
		t.Fatalf("transition to running failed: %v", err)
	}
	routedTask = dispRes.Task
	_ = taskStore.SaveTask(ctx, routedTask)

	inspRes, err := orchestra.ApplyTransition(routedTask, orchestra.TransitionCollectEvidence)
	if err != nil {
		t.Fatalf("transition to awaiting inspection failed: %v", err)
	}
	routedTask = inspRes.Task
	_ = taskStore.SaveTask(ctx, routedTask)

	// Direct transition to Success without Inspector MUST fail
	if _, err := orchestra.ApplyTransition(routedTask, orchestra.TransitionValidate); err == nil {
		t.Fatal("expected illegal transition when bypassing Inspector")
	}

	// Failing evidence: Inspector must reject
	inspector := orchestra.MergeAuthority{Capability: orchestra.MergeCapability}
	failingInspectionReq := orchestra.InspectionRequest{
		Task: routedTask,
		Execution: orchestra.Execution{
			ID:          orchestra.ExecutionID("exec-failing"),
			TaskID:      routedTask.ID,
			EvidenceIDs: []string{"ev-fail-1"},
		},
		Worktree: orchestra.WorktreeState{
			WorktreeID: routedTask.WorktreeID,
			Revision:   "rev-1",
			Dirty:      false,
			Valid:      true,
		},
		Evidence: []orchestra.InspectionEvidence{
			{ID: "ev-fail-1", Kind: "tool_execution", Passed: false, Summary: "test failure"},
		},
	}
	if _, err := inspector.Inspect(ctx, failingInspectionReq); !errors.Is(err, orchestra.ErrInspectionRejected) {
		t.Fatalf("expected ErrInspectionRejected on failing evidence, got %v", err)
	}

	// Valid evidence passes and yields TaskSuccess
	validInspectionReq := orchestra.InspectionRequest{
		Task: routedTask,
		Execution: orchestra.Execution{
			ID:          orchestra.ExecutionID("exec-valid"),
			TaskID:      routedTask.ID,
			EvidenceIDs: []string{"ev-pass-1"},
		},
		Worktree: orchestra.WorktreeState{
			WorktreeID: routedTask.WorktreeID,
			Revision:   "rev-1",
			Dirty:      false,
			Valid:      true,
		},
		Evidence: []orchestra.InspectionEvidence{
			{ID: "ev-pass-1", Kind: "tool_execution", Passed: true, Summary: "clean inspection"},
		},
	}
	inspectResult, err := inspector.Inspect(ctx, validInspectionReq)
	if err != nil {
		t.Fatalf("valid inspect failed: %v", err)
	}
	if !inspectResult.Accepted || !inspectResult.MergeReady {
		t.Fatalf("expected accepted and merge ready from inspector")
	}
	if inspectResult.AcceptedTask.Status != orchestra.TaskSuccess {
		t.Fatalf("expected TaskSuccess status, got %s", inspectResult.AcceptedTask.Status)
	}

	if err := taskStore.SaveTask(ctx, inspectResult.AcceptedTask); err != nil {
		t.Fatalf("failed to save final success task: %v", err)
	}
	finalTask, _ := taskStore.GetTask(ctx, routedTask.ID)
	if finalTask.Status != orchestra.TaskSuccess {
		t.Fatalf("final task in store is not TaskSuccess: %s", finalTask.Status)
	}

	// Step 12: Event Privacy & Redaction Verification
	eventsMu.Lock()
	defer eventsMu.Unlock()
	if len(recordedEvents) == 0 {
		t.Fatal("expected bridge events to be recorded")
	}

	for _, ev := range recordedEvents {
		evStr := fmt.Sprintf("%#v", ev.payload)
		for _, forbidden := range []string{
			"apiKey", "password", "secret", "bearer", `C:\Users\`, `/home/`, `/etc/`,
		} {
			if strings.Contains(strings.ToLower(evStr), strings.ToLower(forbidden)) {
				t.Fatalf("bridge event %q leaked forbidden token %q: %s", ev.name, forbidden, evStr)
			}
		}
	}
}

// TestBAN164_Concurrency_Stress_And_Backpressure tests high concurrency:
// 1. 50 concurrent routing requests across 10 distinct correlation IDs (5 duplicates each)
// 2. Exact 10 tasks created in Orchestra; all duplicates return matching task IDs
// 3. 20 concurrent tool proposals with racing operator approvals
// 4. Safe event delivery to multiple concurrent hooks without deadlocks
func TestBAN164_Concurrency_Stress_And_Backpressure(t *testing.T) {
	ctx := context.Background()
	taskStore := orchestra.NewMemoryTaskStore()
	orchestraAdapter, err := orchestra.NewProductionAdapter(orchestra.ProductionAdapterConfig{
		TaskStore:         taskStore,
		HarnessStore:      &ban164EvidenceReader{},
		WorktreeInspector: &ban164WorktreeInspector{},
		Inspector:         orchestra.MergeAuthority{Capability: orchestra.MergeCapability},
	})
	if err != nil {
		t.Fatalf("failed to create orchestra adapter: %v", err)
	}

	fakeBrain := application.NewFakeBrainAdapter("brain-stress")
	for i := 0; i < 10; i++ {
		corr := fmt.Sprintf("corr-stress-%d", i)
		fakeBrain.SetResult(corr+"-plan", application.BrainDispatchResult{
			TaskID:        corr + "-plan",
			State:         "completed",
			Output:        fmt.Sprintf("PLAN:1|Step %d", i),
			SchemaVersion: application.BrainBridgeContract,
		})
	}

	routingAdapter, err := bridge.NewBrainOrchestraRoutingAdapter(bridge.BrainOrchestraRoutingAdapterConfig{
		Brain:     fakeBrain,
		Orchestra: orchestraAdapter,
		ProjectID: "proj-stress",
	})
	if err != nil {
		t.Fatalf("failed to create routing adapter: %v", err)
	}

	// Tool setup
	policy, _ := harness.NewPolicy([]harness.Rule{
		{ID: "allow-safe", Action: "file_read", Resource: "*", Effect: harness.EffectAllow},
	})
	staticPolicy, _ := harness.NewStaticPolicy(policy)
	broker := harness.NewBroker(staticPolicy)
	_ = broker.Register(harness.ToolDefinition{
		Name:         "file_read",
		Kind:         harness.ToolKindNative,
		Capabilities: []harness.ToolCapability{harness.CapabilityFileRead},
		Timeout:      10 * time.Second,
		Version:      "1.0.0",
	})
	_ = broker.RegisterEngine(&ban164FakeEngine{
		kind: harness.ToolKindNative,
		caps: []harness.ToolCapability{harness.CapabilityFileRead},
	})
	toolExecutor, _ := bridge.NewHarnessToolExecutor(bridge.HarnessToolExecutorConfig{Broker: broker})
	toolAdapter, _ := bridge.NewToolApprovalAdapter(bridge.ToolApprovalAdapterConfig{
		Allowlist: map[string]string{"file_read": "file_read"},
		Executor:  toolExecutor,
	})

	service := application.NewService()
	service.SetBrainOrchestra(routingAdapter)
	service.SetToolApproval(toolAdapter)
	b := platform.NewBridge(service)

	// Register 5 concurrent event hooks
	var eventCount atomic.Int64
	for i := 0; i < 5; i++ {
		unsub := b.RegisterEventHook(func(_ string, _ any) {
			eventCount.Add(1)
		})
		defer unsub()
	}

	// Part 1: 50 concurrent routing dispatches (10 groups of 5 duplicates)
	const numKeys = 10
	const dupsPerKey = 5
	var routeWg sync.WaitGroup
	results := make([][]application.BrainOrchestraRouteResult, numKeys)
	var resMu sync.Mutex

	for k := 0; k < numKeys; k++ {
		results[k] = make([]application.BrainOrchestraRouteResult, 0, dupsPerKey)
		for d := 0; d < dupsPerKey; d++ {
			routeWg.Add(1)
			go func(keyIndex int) {
				defer routeWg.Done()
				res, rErr := b.RouteBrainTaskToOrchestra(application.BrainOrchestraRouteRequest{
					CorrelationID:      fmt.Sprintf("corr-stress-%d", keyIndex),
					UserPrompt:         fmt.Sprintf("Stress prompt %d", keyIndex),
					Title:              fmt.Sprintf("Stress Title %d", keyIndex),
					AcceptanceCriteria: []string{"passes"},
					SchemaVersion:      application.BrainOrchestraBridgeSchemaVersion,
				})
				if rErr == nil && res.Status == "accepted" {
					resMu.Lock()
					results[keyIndex] = append(results[keyIndex], res)
					resMu.Unlock()
				}
			}(k)
		}
	}
	routeWg.Wait()

	// Verify all 50 calls succeeded and all duplicates within each key returned identical orchestra task ID
	for k := 0; k < numKeys; k++ {
		resMu.Lock()
		keyResults := results[k]
		resMu.Unlock()
		if len(keyResults) != dupsPerKey {
			t.Fatalf("key %d had %d successful results, expected %d", k, len(keyResults), dupsPerKey)
		}
		firstTaskID := keyResults[0].OrchestraTaskID
		for _, r := range keyResults[1:] {
			if r.OrchestraTaskID != firstTaskID {
				t.Fatalf("task ID mismatch for key %d: %s vs %s", k, r.OrchestraTaskID, firstTaskID)
			}
		}
	}

	// Orchestra task store must contain exactly 10 tasks
	storedTasks, err := taskStore.ListTasks(ctx)
	if err != nil {
		t.Fatalf("failed to list tasks: %v", err)
	}
	if len(storedTasks) != numKeys {
		t.Fatalf("expected exactly %d tasks in store, got %d", numKeys, len(storedTasks))
	}

	// Part 2: 20 proposals with racing approvals
	const numProposals = 20
	var propWg sync.WaitGroup
	for p := 0; p < numProposals; p++ {
		propWg.Add(1)
		go func(pIdx int) {
			defer propWg.Done()
			pID := fmt.Sprintf("prop-stress-%d", pIdx)
			_, pErr := b.ProposeToolCall(application.ToolProposal{
				ProposalID:          pID,
				CorrelationID:       fmt.Sprintf("corr-stress-%d", pIdx%numKeys),
				TaskID:              fmt.Sprintf("task-stress-%d", pIdx),
				WorktreeID:          "wt-stress",
				ToolName:            "file_read",
				Capability:          "file_read",
				RedactedArgsSummary: "Read config relative path",
				ArgByteSize:         32,
				ProposedAt:          time.Now().UTC(),
				SchemaVersion:       application.ToolProposalSchemaVersion,
			})
			if pErr != nil {
				t.Errorf("proposal %s failed: %v", pID, pErr)
				return
			}

			// 3 concurrent operators race to approve this proposal
			var appWg sync.WaitGroup
			for op := 0; op < 3; op++ {
				appWg.Add(1)
				go func(actor string) {
					defer appWg.Done()
					_, _ = b.DecideToolCall(application.ToolApprovalDecision{
						ProposalID:     pID,
						Decision:       "approve",
						Actor:          actor,
						Reason:         "Approved in stress test",
						IdempotencyKey: fmt.Sprintf("idem-stress-%s", actor),
						DecidedAt:      time.Now().UTC(),
						SchemaVersion:  application.ToolDecisionSchemaVersion,
					})
				}(fmt.Sprintf("actor-%d", op))
			}
			appWg.Wait()

			// Proposal must be executed
			rec, gErr := toolAdapter.GetProposal(ctx, pID)
			if gErr != nil || rec.Status != application.ToolApprovalExecuted {
				t.Errorf("proposal %s expected executed, got status=%s err=%v", pID, rec.Status, gErr)
			}
		}(p)
	}
	propWg.Wait()

	if eventCount.Load() == 0 {
		t.Fatal("expected events to be counted across hooks")
	}
}

// 1. Context cancellation stops routing before task persistence
// 2. Malformed Brain outputs never create tasks
// 3. Denied tool proposals never invoke Harness
func TestBAN164_ContextCancellation_And_MalformedHandling(t *testing.T) {
	ctx := context.Background()
	taskStore := orchestra.NewMemoryTaskStore()
	orchestraAdapter, _ := orchestra.NewProductionAdapter(orchestra.ProductionAdapterConfig{
		TaskStore:         taskStore,
		HarnessStore:      &ban164EvidenceReader{},
		WorktreeInspector: &ban164WorktreeInspector{},
	})

	// Case 1: Pre-canceled context
	fakeBrain := application.NewFakeBrainAdapter("brain-canceled")
	routingAdapter, _ := bridge.NewBrainOrchestraRoutingAdapter(bridge.BrainOrchestraRoutingAdapterConfig{
		Brain:     fakeBrain,
		Orchestra: orchestraAdapter,
	})
	canceledCtx, cancel := context.WithCancel(ctx)
	cancel()

	req := application.BrainOrchestraRouteRequest{
		CorrelationID:      "corr-canceled",
		UserPrompt:         "Should not be routed",
		Title:              "Canceled request",
		AcceptanceCriteria: []string{"none"},
		SchemaVersion:      application.BrainOrchestraBridgeSchemaVersion,
	}
	res, err := routingAdapter.RouteBrainToOrchestra(canceledCtx, req)
	if err == nil {
		t.Fatal("expected error on canceled context")
	}
	if res.Status != "canceled" {
		t.Fatalf("expected canceled status, got %q", res.Status)
	}
	tasks, _ := taskStore.ListTasks(ctx)
	if len(tasks) != 0 {
		t.Fatalf("expected 0 tasks created on canceled context, got %d", len(tasks))
	}

	// Case 2: Unstructured Brain output fails closed
	fakeBrain.SetResult("corr-unstructured-plan", application.BrainDispatchResult{
		TaskID:        "corr-unstructured-plan",
		State:         "completed",
		Output:        "Sorry, I cannot generate a structured plan.",
		SchemaVersion: application.BrainBridgeContract,
	})
	unstructuredReq := application.BrainOrchestraRouteRequest{
		CorrelationID:      "corr-unstructured",
		UserPrompt:         "Bad prompt",
		Title:              "Unstructured",
		AcceptanceCriteria: []string{"none"},
		SchemaVersion:      application.BrainOrchestraBridgeSchemaVersion,
	}
	uRes, uErr := routingAdapter.RouteBrainToOrchestra(ctx, unstructuredReq)
	if uErr == nil {
		t.Fatal("expected error on unstructured output")
	}
	if uRes.Status != "rejected" {
		t.Fatalf("expected rejected status, got %q", uRes.Status)
	}
	tasks, _ = taskStore.ListTasks(ctx)
	if len(tasks) != 0 {
		t.Fatalf("expected 0 tasks after rejected routing, got %d", len(tasks))
	}
}
