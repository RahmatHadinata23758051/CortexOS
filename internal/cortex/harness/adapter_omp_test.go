package harness

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

type fakeOMPRunner struct {
	mu    sync.Mutex
	calls int
	args  [][]string
	run   func(context.Context, string, []string, []string, string) (SandboxResult, error)
}

func (r *fakeOMPRunner) Run(ctx context.Context, cmd string, args []string, env []string, workDir string, stdoutLimit, stderrLimit int64, grace time.Duration) (SandboxResult, error) {
	r.mu.Lock()
	r.calls++
	r.args = append(r.args, append([]string(nil), args...))
	r.mu.Unlock()
	if r.run != nil {
		return r.run(ctx, cmd, args, env, workDir)
	}
	return SandboxResult{}, nil
}

func ompHandshake(correlation string, sequence uint64) string {
	message, _ := NewMessage(MessageHandshake, correlation, sequence, HandshakePayload{
		WorkerID: "fake-omp-worker", WorkerVersion: OMPAdapterVersion, Protocol: WorkerProtocolVersion,
	})
	line, _ := EncodeLine(message)
	return string(line)
}

func ompTask(correlation string, sequence uint64, task TaskEnvelope) string {
	message, _ := NewMessage(MessageTask, correlation, sequence, task)
	line, _ := EncodeLine(message)
	return string(line)
}

func ompEvidence(correlation string, sequence uint64, id string) string {
	message, _ := NewMessage(MessageEvidence, correlation, sequence, EvidencePayload{
		EvidenceID: id,
		Kind:       EvidenceKindCommandExecution,
		Digest:     strings.Repeat("a", 64),
		Summary:    "specialist analysis completed",
	})
	line, _ := EncodeLine(message)
	return string(line)
}

func ompTerminal(correlation string, sequence uint64, status TerminalStatus, evidence ...string) string {
	message, _ := NewMessage(MessageTerminal, correlation, sequence, TerminalPayload{Status: status, EvidenceIDs: evidence})
	line, _ := EncodeLine(message)
	return string(line)
}

func ompShutdown(correlation string, sequence uint64) string {
	message, _ := NewMessage(MessageShutdown, correlation, sequence, ShutdownPayload{Reason: "fixture complete"})
	line, _ := EncodeLine(message)
	return string(line)
}

func testOMPEnvelope(root, tool string, allowed bool) ExecutionEnvelope {
	input, _ := json.Marshal(map[string]string{"prompt": "recover the failed task"})
	return ExecutionEnvelope{
		ContractVersion: HarnessContractVersion,
		ExecutionID:     "omp-execution-1",
		TaskID:          "omp-task-1",
		WorktreeID:      "omp-worktree-1",
		ProjectID:       "omp-project-1",
		WorktreeRoot:    root,
		ToolName:        tool,
		Input:           input,
		PolicyDecision: PolicyDecision{Allowed: allowed, Effect: func() Effect {
			if allowed {
				return EffectAllow
			}
			return EffectDeny
		}()},
		Timeout: 2 * time.Second,
		TraceID: "omp-trace-1",
	}
}

func newTestOMPAdapter(t *testing.T, runner ProcessRunner, config OMPAdapterConfig) *OMPAdapter {
	t.Helper()
	config.WorktreeRoot = t.TempDir()
	config.Runner = runner
	adapter, err := NewOMPAdapter(config)
	if err != nil {
		t.Fatalf("NewOMPAdapter: %v", err)
	}
	return adapter
}

func TestOMPAdapterCapabilityDiscovery(t *testing.T) {
	adapter := newTestOMPAdapter(t, &fakeOMPRunner{}, OMPAdapterConfig{})
	if adapter.Kind() != ToolKindOMP {
		t.Fatalf("Kind() = %q, want %q", adapter.Kind(), ToolKindOMP)
	}
	descriptor := adapter.Describe()
	if descriptor.Name != OMPAdapterName || descriptor.Version != OMPAdapterVersion || descriptor.Kind != ToolKindOMP {
		t.Fatalf("unexpected descriptor: %+v", descriptor)
	}
	want := OMPCapabilities()
	if len(adapter.Capabilities()) != len(want) {
		t.Fatalf("capability count = %d, want %d", len(adapter.Capabilities()), len(want))
	}
	for _, capability := range want {
		found := false
		for _, actual := range adapter.Capabilities() {
			if actual == capability {
				found = true
			}
		}
		if !found {
			t.Errorf("missing capability %q", capability)
		}
	}
}

func TestOMPAdapterSpecialistRouting(t *testing.T) {
	root := t.TempDir()
	runner := &fakeOMPRunner{run: func(ctx context.Context, cmd string, args []string, env []string, workDir string) (SandboxResult, error) {
		return SandboxResult{Stdout: []byte(ompHandshake("health", 1))}, nil
	}}
	adapter, err := NewOMPAdapter(OMPAdapterConfig{WorktreeRoot: root, Runner: runner})
	if err != nil {
		t.Fatal(err)
	}
	router := NewBasicRouter(DefaultRoutingPolicy())
	if err := router.Register(adapter, OMPExtendedDescriptor()); err != nil {
		t.Fatalf("Register: %v", err)
	}
	selection, err := router.Find(context.Background(), []ToolCapability{CapabilitySpecialist})
	if err != nil {
		t.Fatalf("Find: %v", err)
	}
	if selection.Adapter.Kind() != ToolKindOMP {
		t.Fatalf("selected %q, want omp", selection.Adapter.Kind())
	}

	// A policy that excludes OMP must not route specialist work to it.
	denied := NewBasicRouter(RoutingPolicy{AllowedClasses: []EngineClass{EngineClassPi}, RequireHealthEnforcement: true})
	if err := denied.Register(adapter, OMPExtendedDescriptor()); err != nil {
		t.Fatal(err)
	}
	if _, err := denied.Find(context.Background(), []ToolCapability{CapabilitySpecialist}); !errors.Is(err, ErrPolicyDeniedEngine) {
		t.Fatalf("Find with denied OMP error = %v, want policy denial", err)
	}
}

func TestOMPAdapterBoundedEscalation(t *testing.T) {
	runner := &fakeOMPRunner{run: func(context.Context, string, []string, []string, string) (SandboxResult, error) {
		return SandboxResult{ExitCode: 137}, fmt.Errorf("fixture crash")
	}}
	adapter := newTestOMPAdapter(t, runner, OMPAdapterConfig{MaxEscalations: 99, RetryLimit: 99})
	result, err := adapter.Execute(NewExecutionContext(context.Background(), "trace"), testOMPEnvelope(adapter.worktreeRoot, "specialist", true))
	if !errors.Is(err, ErrAdapterFailure) {
		t.Fatalf("Execute error = %v, want adapter failure", err)
	}
	runner.mu.Lock()
	calls := runner.calls
	runner.mu.Unlock()
	if calls != HardOMPMaxAttempts {
		t.Fatalf("process calls = %d, want hard cap %d", calls, HardOMPMaxAttempts)
	}
	if result.Status != ToolStatusAdapterError && result.Status != ToolStatusFailed {
		t.Fatalf("status = %q, want adapter error or failed", result.Status)
	}
}

func TestOMPAdapterUnavailableAllowsFallback(t *testing.T) {
	// Case 1: Missing executable during NewOMPAdapter (DefaultProcessRunner)
	nonExistent := filepath.Join(t.TempDir(), "nonexistent", "omp_bin")
	_, err := NewOMPAdapter(OMPAdapterConfig{WorktreeRoot: t.TempDir(), ExecutablePath: nonExistent})
	if !errors.Is(err, ErrAdapterUnhealthy) || !errors.Is(err, ErrOMPExecutableNotFound) {
		t.Fatalf("NewOMPAdapter missing executable = %v, want ErrOMPExecutableNotFound", err)
	}

	// Case 2: Health check handshake failure with fake runner
	badHandshakeRunner := &fakeOMPRunner{run: func(context.Context, string, []string, []string, string) (SandboxResult, error) {
		return SandboxResult{Stdout: []byte("corrupt-handshake")}, nil
	}}
	adapter := newTestOMPAdapter(t, badHandshakeRunner, OMPAdapterConfig{})
	if err := adapter.Health(NewExecutionContext(context.Background(), "health")); !errors.Is(err, ErrAdapterUnhealthy) || !errors.Is(err, ErrOMPHandshakeFailed) {
		t.Fatalf("Health corrupted handshake = %v, want ErrOMPHandshakeFailed", err)
	}

	// Case 3: Router fallback when OMP is unhealthy and backup is healthy
	router := NewBasicRouter(DefaultRoutingPolicy())
	if err := router.Register(adapter, OMPExtendedDescriptor()); err != nil {
		t.Fatal(err)
	}
	// Register a healthy backup specialist adapter
	healthyRunner := &fakeOMPRunner{run: func(context.Context, string, []string, []string, string) (SandboxResult, error) {
		return SandboxResult{Stdout: []byte(ompHandshake("health", 1))}, nil
	}}
	backupAdapter := newTestOMPAdapter(t, healthyRunner, OMPAdapterConfig{})
	backupDesc := OMPExtendedDescriptor()
	backupDesc.Identity.InstanceID = "omp-backup"
	if err := router.Register(backupAdapter, backupDesc); err != nil {
		t.Fatal(err)
	}

	selection, err := router.Find(context.Background(), []ToolCapability{CapabilitySpecialist})
	if err != nil {
		t.Fatalf("router.Find with fallback failed: %v", err)
	}
	if selection.Descriptor.Identity.InstanceID != "omp-backup" {
		t.Fatalf("router selected %q, want fallback omp-backup", selection.Descriptor.Identity.InstanceID)
	}
}

func TestOMPAdapterStreamingEvidence(t *testing.T) {
	root := t.TempDir()
	correlation := "omp-execution-1"
	task := TaskEnvelope{ContractVersion: HarnessContractVersion, ExecutionID: correlation, TaskID: "omp-task-1", WorktreeID: "omp-worktree-1", ProjectID: "omp-project-1", ToolName: "security_audit", Input: json.RawMessage(`{"prompt":"audit"}`), TimeoutMs: 2000}
	stream := ompHandshake(correlation, 1) + ompTask(correlation, 2, task) + ompEvidence(correlation, 3, "omp-evidence-1") + ompTerminal(correlation, 4, TerminalSuccess, "omp-evidence-1") + ompShutdown(correlation, 5)
	var evidence []EvidenceRecord
	runner := &fakeOMPRunner{run: func(context.Context, string, []string, []string, string) (SandboxResult, error) {
		return SandboxResult{Stdout: []byte(stream), RedactedOut: []byte(stream), ExitCode: 0}, nil
	}}
	adapter, err := NewOMPAdapter(OMPAdapterConfig{WorktreeRoot: root, Runner: runner, OnEvidence: func(record EvidenceRecord) { evidence = append(evidence, record) }})
	if err != nil {
		t.Fatal(err)
	}
	result, err := adapter.Execute(NewExecutionContext(context.Background(), "trace"), testOMPEnvelope(root, "security_audit", true))
	if err != nil || result.Status != ToolStatusSuccess {
		t.Fatalf("Execute = (%+v, %v), want success", result, err)
	}
	if len(result.EvidenceIDs) != 1 || len(evidence) != 1 {
		t.Fatalf("evidence IDs/callbacks = %v/%d, want one each", result.EvidenceIDs, len(evidence))
	}
}

func TestOMPAdapterCancellationTimeoutAndCrash(t *testing.T) {
	root := t.TempDir()
	cancelRunner := &fakeOMPRunner{}
	adapter, err := NewOMPAdapter(OMPAdapterConfig{WorktreeRoot: root, Runner: cancelRunner})
	if err != nil {
		t.Fatal(err)
	}
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	result, err := adapter.Execute(NewExecutionContext(cancelled, "cancel"), testOMPEnvelope(root, "recovery", true))
	if !errors.Is(err, ErrCanceled) || result.Status != ToolStatusCanceled {
		t.Fatalf("cancellation = (%+v, %v), want canceled", result, err)
	}

	timeoutRunner := &fakeOMPRunner{run: func(context.Context, string, []string, []string, string) (SandboxResult, error) {
		return SandboxResult{TimedOut: true}, ErrTimeout
	}}
	timeoutAdapter := newTestOMPAdapter(t, timeoutRunner, OMPAdapterConfig{})
	result, err = timeoutAdapter.Execute(NewExecutionContext(context.Background(), "timeout"), testOMPEnvelope(timeoutAdapter.worktreeRoot, "deep_debug", true))
	if !errors.Is(err, ErrTimeout) || result.Status != ToolStatusTimeout {
		t.Fatalf("timeout = (%+v, %v), want timeout", result, err)
	}

	crashRunner := &fakeOMPRunner{run: func(context.Context, string, []string, []string, string) (SandboxResult, error) {
		return SandboxResult{ExitCode: 1}, errors.New("fixture process crash")
	}}
	crashAdapter := newTestOMPAdapter(t, crashRunner, OMPAdapterConfig{})
	result, err = crashAdapter.Execute(NewExecutionContext(context.Background(), "crash"), testOMPEnvelope(crashAdapter.worktreeRoot, "architecture_review", true))
	if !errors.Is(err, ErrAdapterFailure) {
		t.Fatalf("crash error = %v, want adapter failure", err)
	}
}

func TestOMPAdapterRejectsUntrustedEvidence(t *testing.T) {
	root := t.TempDir()
	correlation := "omp-execution-1"
	task := TaskEnvelope{ContractVersion: HarnessContractVersion, ExecutionID: correlation, TaskID: "omp-task-1", WorktreeID: "omp-worktree-1", ProjectID: "omp-project-1", ToolName: "specialist", Input: json.RawMessage(`{"prompt":"inspect"}`), TimeoutMs: 2000}
	// The worker claims a record belonging to a different execution. The JSONL
	// envelope is valid, but the evidence is not trusted by the adapter.
	record := EvidenceRecord{ID: "wrong-id", Kind: EvidenceKindCommandExecution, Digest: strings.Repeat("b", 64), ExecutionID: "other-execution", TaskID: task.TaskID, WorktreeID: task.WorktreeID}
	message, _ := NewMessage(MessageEvidence, correlation, 3, EvidencePayload{EvidenceID: "omp-evidence-1", Kind: EvidenceKindCommandExecution, Digest: strings.Repeat("b", 64), Record: &record})
	line, _ := EncodeLine(message)
	stream := ompHandshake(correlation, 1) + ompTask(correlation, 2, task) + string(line) + ompTerminal(correlation, 4, TerminalSuccess)
	runner := &fakeOMPRunner{run: func(context.Context, string, []string, []string, string) (SandboxResult, error) {
		return SandboxResult{Stdout: []byte(stream)}, nil
	}}
	adapter, err := NewOMPAdapter(OMPAdapterConfig{WorktreeRoot: root, Runner: runner})
	if err != nil {
		t.Fatal(err)
	}
	result, err := adapter.Execute(NewExecutionContext(context.Background(), "trace"), testOMPEnvelope(root, "specialist", true))
	if !errors.Is(err, ErrAdapterFailure) || result.Status == ToolStatusSuccess {
		t.Fatalf("untrusted evidence = (%+v, %v), want adapter failure and no success", result, err)
	}
}
