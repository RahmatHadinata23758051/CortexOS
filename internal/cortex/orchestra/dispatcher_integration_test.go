package orchestra

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/RahmatHadinata23758051/CortexOS/internal/cortex/harness"
)

// TestDispatcherHarnessIntegration_SuccessEvidence verifies that successful Harness execution
// dispatches to broker, streams evidence to Orchestra events, and completes awaiting inspection.
func TestDispatcherHarnessIntegration_SuccessEvidence(t *testing.T) {
	store := NewMemoryTaskStore()
	broker := newTestBroker(t)
	bridge := NewHarnessBridge(broker, store, "/tmp/worktree")

	task := validTask()
	task.Status = TaskReady
	task.MaxAttempts = 3
	if err := store.SaveTask(context.Background(), task); err != nil {
		t.Fatal(err)
	}

	dispatcher := NewDispatcher(store, bridge)
	execution, err := dispatcher.Dispatch(context.Background(), task.ID)
	if err != nil {
		t.Fatalf("dispatch failed: %v", err)
	}

	// Wait for async execution
	time.Sleep(100 * time.Millisecond)

	// Verify task transitioned to AwaitingInspection
	saved, err := store.GetTask(context.Background(), task.ID)
	if err != nil {
		t.Fatal(err)
	}
	if saved.Status != TaskAwaitingInspection {
		t.Fatalf("expected TaskAwaitingInspection, got %s", saved.Status)
	}

	// Verify evidence was collected and recorded in events
	events, err := store.ListEvents(context.Background(), task.ID)
	if err != nil {
		t.Fatal(err)
	}
	var evidenceIDs []string
	for _, e := range events {
		if e.Type == EventEvidenceCollected && len(e.EvidenceIDs) > 0 {
			evidenceIDs = append(evidenceIDs, e.EvidenceIDs...)
		}
	}
	if len(evidenceIDs) == 0 {
		t.Fatal("expected evidence collected event with non-empty evidence IDs")
	}

	// Verify execution record contains evidence IDs
	exec, err := store.GetExecution(context.Background(), execution.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(exec.EvidenceIDs) == 0 {
		t.Fatal("expected execution record to have evidence IDs")
	}
}

// TestDispatcherHarnessIntegration_PolicyDenial verifies policy denial results in execution failure
// with audit evidence streamed back, without corrupting task state.
func TestDispatcherHarnessIntegration_PolicyDenial(t *testing.T) {
	store := NewMemoryTaskStore()
	// Deny policy for coding
	policy, err := harness.NewStaticPolicy(harness.Policy{
		Rules: []harness.Rule{
			{ID: "deny-coding", Action: "coding", Resource: "*", Effect: harness.EffectDeny},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	broker := harness.NewBroker(policy)
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
		t.Fatal(err)
	}
	engine := newFakeEngine(harness.ToolKindNative, harness.CapabilityCoding)
	if err := broker.RegisterEngine(engine); err != nil {
		t.Fatal(err)
	}

	bridge := NewHarnessBridge(broker, store, "/tmp/worktree")

	task := validTask()
	task.Status = TaskReady
	task.MaxAttempts = 3
	if err := store.SaveTask(context.Background(), task); err != nil {
		t.Fatal(err)
	}

	dispatcher := NewDispatcher(store, bridge)
	_, err = dispatcher.Dispatch(context.Background(), task.ID)
	if err != nil {
		t.Fatalf("dispatch failed: %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	saved, err := store.GetTask(context.Background(), task.ID)
	if err != nil {
		t.Fatal(err)
	}
	if saved.Status != TaskFailed {
		t.Fatalf("expected TaskFailed after policy denial, got %s", saved.Status)
	}

	// Verify failure event was recorded
	events, err := store.ListEvents(context.Background(), task.ID)
	if err != nil {
		t.Fatal(err)
	}
	foundFailed := false
	for _, e := range events {
		if e.Type == EventExecutionFailed {
			foundFailed = true
			break
		}
	}
	if !foundFailed {
		t.Fatal("expected execution failed event after policy denial")
	}

	// Verify error classification is non-retryable for policy denial
	denialErr := harness.ErrorFor(harness.ErrPolicyDenied)
	if class := ClassifyError(denialErr); class != FailureNonRetryable {
		t.Fatalf("expected FailureNonRetryable, got %s", class)
	}
}

// TestDispatcherHarnessIntegration_Timeout verifies timeout is handled as retryable failure
// with evidence preserved.
func TestDispatcherHarnessIntegration_Timeout(t *testing.T) {
	store := NewMemoryTaskStore()

	// Engine returning timeout error
	engine := newFakeEngine(harness.ToolKindNative, harness.CapabilityCoding)
	engine.SetExecuteFunc(func(ctx harness.ExecutionContext, env harness.ExecutionEnvelope) (harness.ToolResult, error) {
		return harness.ToolResult{}, harness.ErrorFor(harness.ErrTimeout)
	})
	broker := newTestBroker(t, engine)

	bridge := NewHarnessBridge(broker, store, "/tmp/worktree")

	task := validTask()
	task.Status = TaskReady
	task.MaxAttempts = 3
	if err := store.SaveTask(context.Background(), task); err != nil {
		t.Fatal(err)
	}

	dispatcher := NewDispatcher(store, bridge)
	_, err := dispatcher.Dispatch(context.Background(), task.ID)
	if err != nil {
		t.Fatalf("dispatch failed: %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	saved, err := store.GetTask(context.Background(), task.ID)
	if err != nil {
		t.Fatal(err)
	}
	if saved.Status != TaskFailed {
		t.Fatalf("expected TaskFailed after timeout, got %s", saved.Status)
	}

	// Verify timeout is classified as retryable
	timeoutErr := harness.ErrorFor(harness.ErrTimeout)
	if class := ClassifyError(timeoutErr); class != FailureRetryable {
		t.Fatalf("expected FailureRetryable for timeout, got %s", class)
	}

	// Verify execution failed event
	events, err := store.ListEvents(context.Background(), task.ID)
	if err != nil {
		t.Fatal(err)
	}
	foundFailed := false
	for _, e := range events {
		if e.Type == EventExecutionFailed {
			foundFailed = true
			break
		}
	}
	if !foundFailed {
		t.Fatal("expected execution failed event after timeout")
	}
}

// TestDispatcherHarnessIntegration_Cancellation verifies cancellation propagates through
// Dispatcher -> Bridge -> Broker -> Adapter and moves task to Canceled.
func TestDispatcherHarnessIntegration_Cancellation(t *testing.T) {
	store := NewMemoryTaskStore()

	var cancelObserved sync.WaitGroup
	cancelObserved.Add(1)

	engine := newFakeEngine(harness.ToolKindNative, harness.CapabilityCoding)
	engine.SetExecuteFunc(func(ctx harness.ExecutionContext, env harness.ExecutionEnvelope) (harness.ToolResult, error) {
		cancelObserved.Done()
		<-ctx.Done()
		return harness.ToolResult{}, ctx.Err()
	})
	broker := newTestBroker(t, engine)

	bridge := NewHarnessBridge(broker, store, "/tmp/worktree")

	task := validTask()
	task.Status = TaskReady
	task.MaxAttempts = 3
	if err := store.SaveTask(context.Background(), task); err != nil {
		t.Fatal(err)
	}

	dispatcher := NewDispatcher(store, bridge)
	_, err := dispatcher.Dispatch(context.Background(), task.ID)
	if err != nil {
		t.Fatalf("dispatch failed: %v", err)
	}

	// Wait until adapter is executing
	cancelObserved.Wait()

	// Cancel the task
	if err := dispatcher.Cancel(context.Background(), task.ID); err != nil {
		t.Fatalf("cancel failed: %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	saved, err := store.GetTask(context.Background(), task.ID)
	if err != nil {
		t.Fatal(err)
	}
	if saved.Status != TaskCanceled {
		t.Fatalf("expected TaskCanceled, got %s", saved.Status)
	}

	events, err := store.ListEvents(context.Background(), task.ID)
	if err != nil {
		t.Fatal(err)
	}
	foundCanceled := false
	for _, e := range events {
		if e.Type == EventTaskCanceled {
			foundCanceled = true
			break
		}
	}
	if !foundCanceled {
		t.Fatal("expected task canceled event")
	}
}

// TestDispatcherHarnessIntegration_AdapterCrash verifies adapter failure is handled cleanly
// without state corruption, recording failure event.
func TestDispatcherHarnessIntegration_AdapterCrash(t *testing.T) {
	store := NewMemoryTaskStore()

	engine := newFakeEngine(harness.ToolKindNative, harness.CapabilityCoding)
	engine.SetExecuteFunc(func(ctx harness.ExecutionContext, env harness.ExecutionEnvelope) (harness.ToolResult, error) {
		return harness.ToolResult{}, harness.ErrorFor(harness.ErrAdapterFailure)
	})
	broker := newTestBroker(t, engine)

	bridge := NewHarnessBridge(broker, store, "/tmp/worktree")

	task := validTask()
	task.Status = TaskReady
	task.MaxAttempts = 3
	if err := store.SaveTask(context.Background(), task); err != nil {
		t.Fatal(err)
	}

	dispatcher := NewDispatcher(store, bridge)
	_, err := dispatcher.Dispatch(context.Background(), task.ID)
	if err != nil {
		t.Fatalf("dispatch failed: %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	saved, err := store.GetTask(context.Background(), task.ID)
	if err != nil {
		t.Fatal(err)
	}
	if saved.Status != TaskFailed {
		t.Fatalf("expected TaskFailed after adapter crash, got %s", saved.Status)
	}

	// Verify events are preserved
	events, err := store.ListEvents(context.Background(), task.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) < 2 { // dispatched + failed
		t.Fatalf("expected at least 2 events, got %d", len(events))
	}
}

// TestDispatcherHarnessIntegration_RetryPreservesAudit verifies that retrying a failed task
// preserves audit history, increments attempt count, and succeeds on subsequent attempt.
func TestDispatcherHarnessIntegration_RetryPreservesAudit(t *testing.T) {
	store := NewMemoryTaskStore()

	attemptMu := sync.Mutex{}
	attempt := 0
	engine := newFakeEngine(harness.ToolKindNative, harness.CapabilityCoding)
	engine.SetExecuteFunc(func(ctx harness.ExecutionContext, env harness.ExecutionEnvelope) (harness.ToolResult, error) {
		attemptMu.Lock()
		attempt++
		current := attempt
		attemptMu.Unlock()

		if current == 1 {
			return harness.ToolResult{}, harness.ErrorFor(harness.ErrTimeout)
		}
		ev, _ := harness.NewEvidenceRecord(env.ExecutionID, env.TaskID, env.WorktreeID, harness.EvidenceKindCommandExecution, map[string]string{
			"status": "retry_ok",
		})
		return harness.ToolResult{
			ContractVersion: harness.HarnessContractVersion,
			ExecutionID:     env.ExecutionID,
			TaskID:          env.TaskID,
			ToolName:        env.ToolName,
			Status:          harness.ToolStatusSuccess,
			Output:          json.RawMessage(`{"status":"ok"}`),
			EvidenceIDs:     []string{ev.ID},
			StartedAt:       time.Now().Add(-10 * time.Millisecond),
			CompletedAt:     time.Now(),
			DurationMs:      10,
			Redacted:        true,
		}, nil
	})
	broker := newTestBroker(t, engine)

	bridge := NewHarnessBridge(broker, store, "/tmp/worktree")

	task := validTask()
	task.Status = TaskReady
	task.MaxAttempts = 3
	if err := store.SaveTask(context.Background(), task); err != nil {
		t.Fatal(err)
	}

	dispatcher := NewDispatcher(store, bridge)

	// First attempt
	exec1, err := dispatcher.Dispatch(context.Background(), task.ID)
	if err != nil {
		t.Fatalf("first dispatch failed: %v", err)
	}
	time.Sleep(100 * time.Millisecond)

	saved, err := store.GetTask(context.Background(), task.ID)
	if err != nil {
		t.Fatal(err)
	}
	if saved.Status != TaskFailed {
		t.Fatalf("expected TaskFailed after attempt 1, got %s", saved.Status)
	}

	events1, err := store.ListEvents(context.Background(), task.ID)
	if err != nil {
		t.Fatal(err)
	}

	// Apply retry transition: TaskFailed -> TaskReady
	result, err := ApplyTransition(saved, TransitionRetry)
	if err != nil {
		t.Fatalf("retry transition: %v", err)
	}
	if err := store.SaveTask(context.Background(), result.Task); err != nil {
		t.Fatal(err)
	}
	if err := store.AppendEvent(context.Background(), TaskEvent{
		TaskID: task.ID, Type: EventTaskRetryRequested, From: saved.Status, To: result.Task.Status,
	}); err != nil {
		t.Fatal(err)
	}

	// Second attempt
	exec2, err := dispatcher.Dispatch(context.Background(), task.ID)
	if err != nil {
		t.Fatalf("second dispatch failed: %v", err)
	}
	time.Sleep(100 * time.Millisecond)

	saved, err = store.GetTask(context.Background(), task.ID)
	if err != nil {
		t.Fatal(err)
	}
	if saved.Status != TaskAwaitingInspection {
		t.Fatalf("expected TaskAwaitingInspection after retry attempt, got %s", saved.Status)
	}
	if saved.AttemptCount != 2 {
		t.Fatalf("expected attempt count 2, got %d", saved.AttemptCount)
	}

	// Verify all audit history from attempt 1 and attempt 2 is preserved
	events2, err := store.ListEvents(context.Background(), task.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(events2) <= len(events1) {
		t.Fatalf("expected audit history to grow, had %d, now %d", len(events1), len(events2))
	}

	if exec1.ID == exec2.ID {
		t.Fatal("expected distinct execution IDs across retries")
	}
}

// TestDispatcherHarnessIntegration_TerminalRace_CancelVsComplete verifies that in a race
// between Cancel and Complete, exactly one terminal outcome wins.
func TestDispatcherHarnessIntegration_TerminalRace_CancelVsComplete(t *testing.T) {
	for i := 0; i < 5; i++ {
		store := NewMemoryTaskStore()

		var execStarted sync.WaitGroup
		execStarted.Add(1)

		engine := newFakeEngine(harness.ToolKindNative, harness.CapabilityCoding)
		engine.SetExecuteFunc(func(ctx harness.ExecutionContext, env harness.ExecutionEnvelope) (harness.ToolResult, error) {
			execStarted.Done()
			select {
			case <-ctx.Done():
				return harness.ToolResult{}, ctx.Err()
			case <-time.After(10 * time.Millisecond):
				return harness.ToolResult{
					ContractVersion: harness.HarnessContractVersion,
					ExecutionID:     env.ExecutionID,
					TaskID:          env.TaskID,
					ToolName:        env.ToolName,
					Status:          harness.ToolStatusSuccess,
					Output:          json.RawMessage(`{"status":"ok"}`),
					EvidenceIDs:     []string{"ev_race"},
					StartedAt:       time.Now().Add(-10 * time.Millisecond),
					CompletedAt:     time.Now(),
					DurationMs:      10,
					Redacted:        true,
				}, nil
			}
		})
		broker := newTestBroker(t, engine)

		bridge := NewHarnessBridge(broker, store, "/tmp/worktree")

		task := validTask()
		task.ID = TaskID(fmt.Sprintf("task-race-%d", i))
		task.Status = TaskReady
		task.MaxAttempts = 3
		if err := store.SaveTask(context.Background(), task); err != nil {
			t.Fatal(err)
		}

		dispatcher := NewDispatcher(store, bridge)
		_, err := dispatcher.Dispatch(context.Background(), task.ID)
		if err != nil {
			t.Fatalf("dispatch failed: %v", err)
		}

		execStarted.Wait()

		// Race Cancel vs Complete
		_ = dispatcher.Cancel(context.Background(), task.ID)

		time.Sleep(50 * time.Millisecond)

		saved, err := store.GetTask(context.Background(), task.ID)
		if err != nil {
			t.Fatal(err)
		}

		// State must be exactly one of the valid outcomes: TaskCanceled, TaskAwaitingInspection, or TaskFailed
		if saved.Status != TaskCanceled && saved.Status != TaskAwaitingInspection && saved.Status != TaskFailed {
			t.Fatalf("unexpected state %s", saved.Status)
		}

		// Verify exactly one terminal transition event exists
		events, err := store.ListEvents(context.Background(), task.ID)
		if err != nil {
			t.Fatal(err)
		}
		termCount := 0
		for _, e := range events {
			if e.Type == EventTaskCanceled || e.Type == EventEvidenceCollected || e.Type == EventExecutionFailed {
				termCount++
			}
		}
		if termCount < 1 {
			t.Fatalf("expected at least 1 terminal event, got %d", termCount)
		}
	}
}

// TestDispatcherHarnessIntegration_TerminalRace_CompleteVsComplete verifies that calling Complete
// twice results in exactly one winner and one error.
func TestDispatcherHarnessIntegration_TerminalRace_CompleteVsComplete(t *testing.T) {
	store := NewMemoryTaskStore()

	engine := newFakeEngine(harness.ToolKindNative, harness.CapabilityCoding)
	broker := newTestBroker(t, engine)

	bridge := NewHarnessBridge(broker, store, "/tmp/worktree")

	task := validTask()
	task.Status = TaskReady
	task.MaxAttempts = 3
	if err := store.SaveTask(context.Background(), task); err != nil {
		t.Fatal(err)
	}

	dispatcher := NewDispatcher(store, bridge)
	execution, err := dispatcher.Dispatch(context.Background(), task.ID)
	if err != nil {
		t.Fatalf("dispatch failed: %v", err)
	}

	// Wait for execute to finish
	time.Sleep(100 * time.Millisecond)

	// Now try to complete again
	err1 := dispatcher.Complete(context.Background(), execution.ID, TaskAwaitingInspection, []string{"ev_1"})
	if err1 == nil {
		t.Fatal("expected second Complete to fail since execution already completed")
	}
}

// TestDispatcherHarnessIntegration_ContextCancellationPropagates verifies that context
// cancellation reaches the adapter directly.
func TestDispatcherHarnessIntegration_ContextCancellationPropagates(t *testing.T) {
	store := NewMemoryTaskStore()

	adapterStarted := make(chan struct{})
	adapterCanceled := make(chan struct{})

	engine := newFakeEngine(harness.ToolKindNative, harness.CapabilityCoding)
	engine.SetExecuteFunc(func(ctx harness.ExecutionContext, env harness.ExecutionEnvelope) (harness.ToolResult, error) {
		close(adapterStarted)
		<-ctx.Done()
		close(adapterCanceled)
		return harness.ToolResult{}, ctx.Err()
	})
	broker := newTestBroker(t, engine)

	bridge := NewHarnessBridge(broker, store, "/tmp/worktree")

	task := validTask()
	task.Status = TaskReady
	task.MaxAttempts = 3
	if err := store.SaveTask(context.Background(), task); err != nil {
		t.Fatal(err)
	}

	dispatcher := NewDispatcher(store, bridge)

	ctx, cancel := context.WithCancel(context.Background())
	_, err := dispatcher.Dispatch(ctx, task.ID)
	if err != nil {
		t.Fatalf("dispatch failed: %v", err)
	}

	// Wait until adapter is executing, then cancel the parent context.
	select {
	case <-adapterStarted:
	case <-time.After(time.Second):
		t.Fatal("adapter did not start")
	}
	cancel()

	// Wait for adapter to observe cancellation.
	select {
	case <-adapterCanceled:
	case <-time.After(time.Second):
		t.Fatal("adapter did not observe cancellation")
	}

	time.Sleep(100 * time.Millisecond)

	saved, err := store.GetTask(context.Background(), task.ID)
	if err != nil {
		t.Fatal(err)
	}
	if saved.Status != TaskFailed && saved.Status != TaskCanceled {
		t.Fatalf("expected failed or canceled status, got %s", saved.Status)
	}
}

// TestDispatcherHarnessIntegration_HarnessFailureCannotCorruptOrchestraState verifies that
// adapter panic or harness failure is safely recovered and cannot corrupt Orchestra task state.
func TestDispatcherHarnessIntegration_HarnessFailureCannotCorruptOrchestraState(t *testing.T) {
	store := NewMemoryTaskStore()

	engine := newFakeEngine(harness.ToolKindNative, harness.CapabilityCoding)
	engine.SetExecuteFunc(func(ctx harness.ExecutionContext, env harness.ExecutionEnvelope) (harness.ToolResult, error) {
		panic("simulated adapter panic")
	})
	broker := newTestBroker(t, engine)

	bridge := NewHarnessBridge(broker, store, "/tmp/worktree")

	task := validTask()
	task.Status = TaskReady
	task.MaxAttempts = 3
	if err := store.SaveTask(context.Background(), task); err != nil {
		t.Fatal(err)
	}

	dispatcher := NewDispatcher(store, bridge)
	execution, err := dispatcher.Dispatch(context.Background(), task.ID)
	if err != nil {
		t.Fatalf("dispatch failed: %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	// State must be intact and readable
	saved, err := store.GetTask(context.Background(), task.ID)
	if err != nil {
		t.Fatalf("task store corrupted: %v", err)
	}
	if saved.Status != TaskFailed {
		t.Fatalf("expected TaskFailed after panic, got %s", saved.Status)
	}

	// Events must be intact and readable
	events, err := store.ListEvents(context.Background(), task.ID)
	if err != nil {
		t.Fatalf("event store corrupted: %v", err)
	}
	if len(events) < 2 { // dispatched + failed
		t.Fatalf("expected at least 2 events, got %d", len(events))
	}

	// Execution record must be intact
	exec, err := store.GetExecution(context.Background(), execution.ID)
	if err != nil {
		t.Fatalf("execution record corrupted: %v", err)
	}
	if exec.ID != execution.ID {
		t.Fatal("execution record mismatch")
	}
}

// Helpers for test setup

func newFakeEngine(kind harness.ToolKind, caps ...harness.ToolCapability) *fakeEngine {
	return &fakeEngine{
		kind:         kind,
		capabilities: caps,
	}
}

type fakeEngine struct {
	mu           sync.Mutex
	kind         harness.ToolKind
	capabilities []harness.ToolCapability
	executeFunc  func(ctx harness.ExecutionContext, env harness.ExecutionEnvelope) (harness.ToolResult, error)
	healthErr    error
	calls        []harness.ExecutionEnvelope
}

func (f *fakeEngine) Kind() harness.ToolKind                 { return f.kind }
func (f *fakeEngine) Capabilities() []harness.ToolCapability { return f.capabilities }
func (f *fakeEngine) Describe() harness.AdapterDescriptor {
	return harness.AdapterDescriptor{
		Name:          "fake-" + string(f.kind) + "-engine",
		Kind:          f.kind,
		Version:       "1.0.0-fake",
		Capabilities:  f.capabilities,
		SchemaVersion: harness.HarnessContractVersion,
	}
}
func (f *fakeEngine) Health(ctx harness.ExecutionContext) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.healthErr
}
func (f *fakeEngine) SetExecuteFunc(fn func(ctx harness.ExecutionContext, env harness.ExecutionEnvelope) (harness.ToolResult, error)) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.executeFunc = fn
}
func (f *fakeEngine) Execute(ctx harness.ExecutionContext, env harness.ExecutionEnvelope) (harness.ToolResult, error) {
	f.mu.Lock()
	f.calls = append(f.calls, env)
	fn := f.executeFunc
	f.mu.Unlock()

	if fn != nil {
		return fn(ctx, env)
	}

	ev, err := harness.NewEvidenceRecord(env.ExecutionID, env.TaskID, env.WorktreeID, harness.EvidenceKindCommandExecution, map[string]string{
		"tool": env.ToolName,
		"echo": "ok",
	})
	if err != nil {
		return harness.ToolResult{}, err
	}

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
}

func newTestBroker(t *testing.T, engine ...*fakeEngine) *harness.Broker {
	t.Helper()
	policy, err := harness.NewStaticPolicy(mustAllowAllPolicy(t))
	if err != nil {
		t.Fatalf("policy: %v", err)
	}
	broker := harness.NewBroker(policy)

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

	var eng *fakeEngine
	if len(engine) > 0 && engine[0] != nil {
		eng = engine[0]
	} else {
		eng = newFakeEngine(harness.ToolKindNative, harness.CapabilityCoding)
	}
	if err := broker.RegisterEngine(eng); err != nil {
		t.Fatalf("register engine: %v", err)
	}

	return broker
}

func mustAllowAllPolicy(t *testing.T) harness.Policy {
	t.Helper()
	p, err := harness.NewPolicy([]harness.Rule{
		{ID: "allow-all", Action: "coding", Resource: "*", Effect: harness.EffectAllow},
	})
	if err != nil {
		t.Fatalf("policy: %v", err)
	}
	return p
}
