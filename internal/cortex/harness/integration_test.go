package harness_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/RahmatHadinata23758051/CortexOS/internal/cortex/harness"
	"github.com/RahmatHadinata23758051/CortexOS/internal/cortex/orchestra"
)

// TestHarnessFullPathFixture is the disposable Phase 4 quality gate. It uses
// only in-memory fakes and a temporary worktree, so it is provider-independent.
func TestHarnessFullPathFixture(t *testing.T) {
	root := t.TempDir()
	worktree := filepath.Join(root, "worktree")
	if err := os.Mkdir(worktree, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(worktree, "fixture.txt"), []byte("fixture"), 0o600); err != nil {
		t.Fatal(err)
	}

	sandbox, err := harness.NewSandbox(harness.SandboxConfig{
		WorktreeRoot:     worktree,
		StdoutLimitBytes: 32,
		StderrLimitBytes: 32,
		Timeout:          time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}

	t.Run("jsonl worker fixture", func(t *testing.T) {
		handshake, err := harness.NewMessage(harness.MessageHandshake, "fixture", 1, harness.HandshakePayload{
			WorkerID: "fake-worker", WorkerVersion: "test", Protocol: harness.WorkerProtocolVersion,
		})
		if err != nil {
			t.Fatal(err)
		}
		health, err := harness.NewMessage(harness.MessageHealth, "fixture", 2, harness.HealthPayload{Healthy: true, Status: "ready"})
		if err != nil {
			t.Fatal(err)
		}
		first, err := harness.EncodeLine(handshake)
		if err != nil {
			t.Fatal(err)
		}
		second, err := harness.EncodeLine(health)
		if err != nil {
			t.Fatal(err)
		}
		fixture := append(first, second...)
		for _, line := range bytes.Split(bytes.TrimSpace(fixture), []byte("\n")) {
			message, err := harness.DecodeLine(line)
			if err != nil {
				t.Fatalf("decode JSONL fixture: %v", err)
			}
			if message.CorrelationID != "fixture" {
				t.Fatalf("correlation id = %q", message.CorrelationID)
			}
		}
	})

	t.Run("sandbox security and cleanup", func(t *testing.T) {
		for _, path := range []string{"../outside", `..\outside`, filepath.Join(root, "outside")} {
			if _, err := sandbox.ValidatePath(path); !errors.Is(err, harness.ErrWorktreeViolation) {
				t.Errorf("ValidatePath(%q) error = %v, want worktree violation", path, err)
			}
		}
		inside, err := sandbox.ValidatePath("fixture.txt")
		if err != nil || !strings.HasPrefix(inside, worktree) {
			t.Fatalf("valid path rejected: %q, %v", inside, err)
		}

		filtered, err := sandbox.FilterEnvironment([]string{"PATH=/bin", "HOME=/tmp", "API_TOKEN=secret-value", "SECRET_KEY=another-secret"})
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(strings.Join(filtered, "\n"), "SECRET") || strings.Contains(strings.Join(filtered, "\n"), "TOKEN") {
			t.Fatalf("secret environment leaked: %v", filtered)
		}

		runner := &fixtureRunner{stdout: []byte("API_KEY=secretvalue" + strings.Repeat("x", 100)), stderr: []byte("password=secretvalue")}
		result, err := sandbox.ExecuteWithSandbox(harness.NewExecutionContext(context.Background(), "sandbox-test"), runner, "fixture", nil, os.Environ(), ".")
		if err != nil {
			t.Fatal(err)
		}
		if len(result.Stdout) > 32 {
			t.Fatalf("output boundary failed: truncated=%v bytes=%d redacted=%q", result.Truncated, len(result.Stdout), result.RedactedOut)
		}

		started := make(chan struct{})
		cleaned := make(chan struct{})
		cancelRunner := &fixtureRunner{run: func(ctx context.Context, _ string, _ []string, _ []string, _ string, _, _ int64, _ time.Duration) (harness.SandboxResult, error) {
			close(started)
			<-ctx.Done()
			close(cleaned)
			return harness.SandboxResult{Canceled: true}, ctx.Err()
		}}
		ctx, cancel := context.WithCancel(context.Background())
		go func() { <-started; cancel() }()
		_, err = sandbox.ExecuteWithSandbox(harness.NewExecutionContext(ctx, "cleanup"), cancelRunner, "fixture", nil, nil, ".")
		if err == nil {
			t.Fatal("canceled process returned nil error")
		}
		select {
		case <-cleaned:
		case <-time.After(time.Second):
			t.Fatal("process runner was not cleaned up")
		}
	})

	t.Run("broker policy, bypass, and approval", func(t *testing.T) {
		broker := newFixtureBroker(t, newFixtureEngine())
		request := fixtureRequest("coding_agent", "coding")
		result, err := broker.Execute(harness.NewExecutionContext(context.Background(), request.TraceID), request)
		if err != nil || result.Status != harness.ToolStatusSuccess {
			t.Fatalf("allowed request: status=%s err=%v", result.Status, err)
		}

		request.Action = string(harness.CapabilityFileWrite)
		result, err = broker.Execute(harness.NewExecutionContext(context.Background(), request.TraceID), request)
		if err == nil || result.Status != harness.ToolStatusPolicyDenied {
			t.Fatalf("capability bypass was not denied: status=%s err=%v", result.Status, err)
		}

		askPolicy, err := harness.NewStaticPolicy(mustPolicy(t, harness.Rule{ID: "ask", Action: "coding", Resource: "*", Effect: harness.EffectAsk}))
		if err != nil {
			t.Fatal(err)
		}
		askBroker := harness.NewBroker(askPolicy)
		if err := registerFixtureToolAndEngine(askBroker, newFixtureEngine()); err != nil {
			t.Fatal(err)
		}
		request = fixtureRequest("coding_agent", "coding")
		if result, err := askBroker.Execute(harness.NewExecutionContext(context.Background(), request.TraceID), request); err == nil || result.Status != harness.ToolStatusPolicyDenied {
			t.Fatalf("unapproved ask was not denied: status=%s err=%v", result.Status, err)
		}
		approved := harness.WithApproval(context.Background(), harness.Approval{ExecutionID: request.ExecutionID, Approver: "fixture-reviewer"})
		result, err = askBroker.Execute(harness.NewExecutionContext(approved, request.TraceID), request)
		if err != nil || result.Status != harness.ToolStatusSuccess {
			t.Fatalf("approved ask failed: status=%s err=%v", result.Status, err)
		}
	})

	t.Run("governor admission release and concurrent reservations", func(t *testing.T) {
		governor := harness.NewGovernor(harness.WithBudgets(map[harness.EngineClass]harness.ResourceBudget{
			harness.EngineClassNative: {MaxWorkers: 2, MaxMemoryMB: 200, MaxCPUPriority: 2},
		}))
		request := func(id string) *harness.Reservation {
			res, err := governor.Admit(context.Background(), harness.TaskDescriptor{TaskID: id, EngineClass: harness.EngineClassNative, Priority: harness.PriorityNormal, EstimatedMemoryMB: 100, EstimatedCPUPriority: 1})
			if err != nil {
				t.Fatalf("admission %s failed: %v", id, err)
			}
			return res
		}
		res1 := request("capacity-1")
		res2 := request("capacity-2")
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
		defer cancel()
		if _, err := governor.Admit(ctx, harness.TaskDescriptor{TaskID: "capacity-waiter", EngineClass: harness.EngineClassNative, Priority: harness.PriorityNormal, EstimatedMemoryMB: 100, EstimatedCPUPriority: 1}); !errors.Is(err, harness.ErrCapacityExceeded) {
			t.Fatalf("expected queued admission timeout, got %v", err)
		}
		res1.Release()
		res2.Release()
		stats := governor.GetTelemetry()
		class := stats.ClassTelemetry[harness.EngineClassNative]
		if stats.TotalAdmitted != 2 || stats.TotalReleased != 2 || class.CurrentActive != 0 || class.PeakActiveWorkers > 2 {
			t.Fatalf("invalid governor accounting: %+v class=%+v", stats, class)
		}
	})

	t.Run("orchestra dispatch inspector recovery and terminal race", func(t *testing.T) {
		store := orchestra.NewMemoryTaskStore()
		broker := newFixtureBroker(t, newFixtureEngine())
		bridge := orchestra.NewHarnessBridge(broker, store, worktree)
		dispatcher := orchestra.NewDispatcher(store, bridge, "fixture-worker")
		task := fixtureTask("e2e", []string{"build the fixture"})
		if err := store.SaveTask(context.Background(), task); err != nil {
			t.Fatal(err)
		}
		execution, err := dispatcher.Dispatch(context.Background(), task.ID)
		if err != nil {
			t.Fatal(err)
		}
		awaitStatus(t, store, task.ID, orchestra.TaskAwaitingInspection)
		var running sync.WaitGroup
		running.Add(1)
		activeEngine := newFixtureEngine()
		activeEngine.SetExecuteFunc(func(ctx harness.ExecutionContext, env harness.ExecutionEnvelope) (harness.ToolResult, error) {
			running.Done()
			time.Sleep(20 * time.Millisecond)
			return harness.ToolResult{
				ContractVersion: harness.HarnessContractVersion,
				ExecutionID:     env.ExecutionID,
				TaskID:          env.TaskID,
				ToolName:        env.ToolName,
				Status:          harness.ToolStatusSuccess,
				Output:          json.RawMessage(`{"status":"ok"}`),
				Redacted:        true,
			}, nil
		})
		activeStore := orchestra.NewMemoryTaskStore()
		activeDispatcher := orchestra.NewDispatcher(activeStore, orchestra.NewHarnessBridge(newFixtureBroker(t, activeEngine), activeStore, worktree), "fixture-worker")
		ownershipTask := fixtureTask("owner", []string{"build"})
		if err := activeStore.SaveTask(context.Background(), ownershipTask); err != nil {
			t.Fatal(err)
		}
		activeExec, err := activeDispatcher.Dispatch(context.Background(), ownershipTask.ID)
		if err != nil {
			t.Fatal(err)
		}
		running.Wait()
		if !activeDispatcher.VerifyWorkerIdentity(ownershipTask.ID, activeExec.ID, "fixture-worker") || activeDispatcher.VerifyWorkerIdentity(ownershipTask.ID, activeExec.ID, "other-worker") {
			t.Fatal("recovery ownership check failed")
		}
		saved, _ := store.GetTask(context.Background(), task.ID)
		exec, _ := store.GetExecution(context.Background(), execution.ID)
		if len(exec.EvidenceIDs) == 0 {
			t.Fatal("dispatch produced no evidence")
		}
		inspection, err := (&orchestra.MergeAuthority{Capability: orchestra.MergeCapability}).Inspect(context.Background(), orchestra.InspectionRequest{
			Task: saved, Execution: exec, Worktree: orchestra.WorktreeState{WorktreeID: task.WorktreeID, Valid: true},
			Evidence: []orchestra.InspectionEvidence{{ID: exec.EvidenceIDs[0], Kind: "command_execution", Passed: true}},
		})
		if err != nil || !inspection.Accepted {
			t.Fatalf("inspector rejected governed evidence: accepted=%v err=%v", inspection.Accepted, err)
		}

		var started atomic.Bool
		blocking := newFixtureEngine()
		blocking.SetExecuteFunc(func(ctx harness.ExecutionContext, env harness.ExecutionEnvelope) (harness.ToolResult, error) {
			started.Store(true)
			<-ctx.Done()
			return harness.ToolResult{}, ctx.Err()
		})
		racingStore := orchestra.NewMemoryTaskStore()
		racingBroker := newFixtureBroker(t, blocking)
		racingDispatcher := orchestra.NewDispatcher(racingStore, orchestra.NewHarnessBridge(racingBroker, racingStore, worktree))
		raceTask := fixtureTask("race", []string{"build"})
		if err := racingStore.SaveTask(context.Background(), raceTask); err != nil {
			t.Fatal(err)
		}
		raceExec, err := racingDispatcher.Dispatch(context.Background(), raceTask.ID)
		if err != nil {
			t.Fatal(err)
		}
		for !started.Load() {
			time.Sleep(time.Millisecond)
		}
		var wg sync.WaitGroup
		wg.Add(2)
		go func() { defer wg.Done(); _ = racingDispatcher.Cancel(context.Background(), raceTask.ID) }()
		go func() {
			defer wg.Done()
			_ = racingDispatcher.Complete(context.Background(), raceExec.ID, orchestra.TaskAwaitingInspection, []string{"race-evidence"})
		}()
		wg.Wait()
		finalTask, err := racingStore.GetTask(context.Background(), raceTask.ID)
		if err != nil || (finalTask.Status != orchestra.TaskCanceled && finalTask.Status != orchestra.TaskAwaitingInspection) {
			t.Fatalf("terminal race corrupted state: task=%+v err=%v", finalTask, err)
		}
	})

	t.Run("timeout crash retry recovery", func(t *testing.T) {
		var calls atomic.Int32
		engine := newFixtureEngine()
		engine.SetExecuteFunc(func(_ harness.ExecutionContext, env harness.ExecutionEnvelope) (harness.ToolResult, error) {
			if calls.Add(1) == 1 {
				return harness.ToolResult{}, harness.ErrorFor(harness.ErrTimeout)
			}
			ev, _ := harness.NewEvidenceRecord(env.ExecutionID, env.TaskID, env.WorktreeID, harness.EvidenceKindCommandExecution, map[string]string{"recovered": "true"})
			return harness.ToolResult{ContractVersion: harness.HarnessContractVersion, ExecutionID: env.ExecutionID, TaskID: env.TaskID, ToolName: env.ToolName, Status: harness.ToolStatusSuccess, EvidenceIDs: []string{ev.ID}, Redacted: true}, nil
		})
		store := orchestra.NewMemoryTaskStore()
		dispatcher := orchestra.NewDispatcher(store, orchestra.NewHarnessBridge(newFixtureBroker(t, engine), store, worktree))
		task := fixtureTask("retry", []string{"build"})
		if err := store.SaveTask(context.Background(), task); err != nil {
			t.Fatal(err)
		}
		if _, err := dispatcher.Dispatch(context.Background(), task.ID); err != nil {
			t.Fatal(err)
		}
		awaitStatus(t, store, task.ID, orchestra.TaskFailed)
		failed, _ := store.GetTask(context.Background(), task.ID)
		retried, err := orchestra.ApplyTransition(failed, orchestra.TransitionRetry)
		if err != nil {
			t.Fatal(err)
		}
		if err := store.SaveTask(context.Background(), retried.Task); err != nil {
			t.Fatal(err)
		}
		if _, err := dispatcher.Dispatch(context.Background(), task.ID); err != nil {
			t.Fatal(err)
		}
		awaitStatus(t, store, task.ID, orchestra.TaskAwaitingInspection)
		if calls.Load() != 2 {
			t.Fatalf("expected one retry, got %d calls", calls.Load())
		}
		if orchestra.ClassifyError(harness.ErrorFor(harness.ErrTimeout)) != orchestra.FailureRetryable {
			t.Fatal("timeout was not classified retryable")
		}
	})

	t.Run("architecture and imports", func(t *testing.T) {
		var _ harness.ToolBroker = harness.NewBroker(nil)
		var _ harness.PolicyEngine = (*harness.StaticPolicy)(nil)
		var _ harness.EngineAdapter = (*harness.FakeEngine)(nil)
		var _ harness.Sandbox = sandbox
		var _ orchestra.Executor = orchestra.NewHarnessBridge(harness.NewBroker(nil), orchestra.NewMemoryTaskStore(), worktree)
		var _ orchestra.Inspector = (*orchestra.MergeAuthority)(nil)
	})
}

func fixtureTask(suffix string, criteria []string) orchestra.Task {
	now := time.Now().UTC()
	return orchestra.Task{ID: orchestra.TaskID("phase4-" + suffix + "-" + fmt.Sprint(now.UnixNano())), ProjectID: "fixture-project", WorktreeID: "fixture-worktree", Title: "Phase 4 fixture", AcceptanceCriteria: criteria, Status: orchestra.TaskReady, MaxAttempts: 3, CreatedAt: now, UpdatedAt: now, SchemaVersion: orchestra.ContractVersion}
}

func fixtureRequest(tool, action string) harness.ToolRequest {
	return harness.ToolRequest{ContractVersion: harness.HarnessContractVersion, ExecutionID: "fixture-execution-" + fmt.Sprint(time.Now().UnixNano()), TaskID: "fixture-task", WorktreeID: "fixture-worktree", ToolName: tool, Action: action, Input: json.RawMessage(`{"fixture":true}`), Timeout: time.Second, TraceID: "fixture-trace"}
}

func mustPolicy(t *testing.T, rules ...harness.Rule) harness.Policy {
	t.Helper()
	policy, err := harness.NewPolicy(rules)
	if err != nil {
		t.Fatal(err)
	}
	return policy
}

func newFixtureEngine() *harness.FakeEngine {
	return harness.NewFakeEngine(harness.ToolKindNative,
		harness.CapabilityCoding,
		harness.CapabilityBuild,
		harness.CapabilityTestRun,
		harness.CapabilityLint,
	)
}

func newFixtureBroker(t *testing.T, engine *harness.FakeEngine) *harness.Broker {
	t.Helper()
	policy, err := harness.NewStaticPolicy(mustPolicy(t,
		harness.Rule{ID: "allow-coding", Action: "coding", Resource: "*", Effect: harness.EffectAllow},
		harness.Rule{ID: "allow-test", Action: "test_run", Resource: "*", Effect: harness.EffectAllow},
		harness.Rule{ID: "allow-build", Action: "build", Resource: "*", Effect: harness.EffectAllow},
		harness.Rule{ID: "allow-lint", Action: "lint", Resource: "*", Effect: harness.EffectAllow},
	))
	if err != nil {
		t.Fatal(err)
	}
	broker := harness.NewBroker(policy)
	if err := registerFixtureToolAndEngine(broker, engine); err != nil {
		t.Fatal(err)
	}
	return broker
}

func registerFixtureToolAndEngine(broker *harness.Broker, engine *harness.FakeEngine) error {
	tools := []harness.ToolDefinition{
		{Name: "coding_agent", Kind: harness.ToolKindNative, Description: "fixture", Capabilities: []harness.ToolCapability{harness.CapabilityCoding}, InputSchema: json.RawMessage(`{"type":"object"}`), OutputSchema: json.RawMessage(`{"type":"object"}`), Timeout: time.Second, Version: "fixture"},
		{Name: "test_runner", Kind: harness.ToolKindNative, Description: "fixture", Capabilities: []harness.ToolCapability{harness.CapabilityTestRun}, InputSchema: json.RawMessage(`{"type":"object"}`), OutputSchema: json.RawMessage(`{"type":"object"}`), Timeout: time.Second, Version: "fixture"},
		{Name: "build_runner", Kind: harness.ToolKindNative, Description: "fixture", Capabilities: []harness.ToolCapability{harness.CapabilityBuild}, InputSchema: json.RawMessage(`{"type":"object"}`), OutputSchema: json.RawMessage(`{"type":"object"}`), Timeout: time.Second, Version: "fixture"},
		{Name: "lint_runner", Kind: harness.ToolKindNative, Description: "fixture", Capabilities: []harness.ToolCapability{harness.CapabilityLint}, InputSchema: json.RawMessage(`{"type":"object"}`), OutputSchema: json.RawMessage(`{"type":"object"}`), Timeout: time.Second, Version: "fixture"},
	}
	for _, tool := range tools {
		if err := broker.Register(tool); err != nil {
			return err
		}
	}
	return broker.RegisterEngine(engine)
}

func awaitStatus(t *testing.T, store *orchestra.MemoryTaskStore, id orchestra.TaskID, want orchestra.TaskStatus) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		task, err := store.GetTask(context.Background(), id)
		if err == nil && task.Status == want {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	task, _ := store.GetTask(context.Background(), id)
	t.Fatalf("task %s did not reach %s; got %s", id, want, task.Status)
}

type fixtureRunner struct {
	stdout, stderr []byte
	run            func(context.Context, string, []string, []string, string, int64, int64, time.Duration) (harness.SandboxResult, error)
}

func (r *fixtureRunner) Run(ctx context.Context, command string, args, env []string, workDir string, stdoutLimit, stderrLimit int64, grace time.Duration) (harness.SandboxResult, error) {
	if r.run != nil {
		return r.run(ctx, command, args, env, workDir, stdoutLimit, stderrLimit, grace)
	}
	out := r.stdout
	truncated := false
	if stdoutLimit > 0 && int64(len(out)) > stdoutLimit {
		out = out[:stdoutLimit]
		truncated = true
	}
	errOut := r.stderr
	if stderrLimit > 0 && int64(len(errOut)) > stderrLimit {
		errOut = errOut[:stderrLimit]
		truncated = true
	}
	return harness.SandboxResult{Stdout: out, Stderr: errOut, Truncated: truncated}, nil
}
