package harness

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"
)

type fakePiRunner struct {
	mu      sync.Mutex
	calls   []string
	runFunc func(ctx context.Context, cmd string, args []string, env []string, workDir string) (SandboxResult, error)
}

func (f *fakePiRunner) Run(ctx context.Context, cmd string, args []string, env []string, workDir string,
	stdoutLimit, stderrLimit int64, gracePeriod time.Duration) (SandboxResult, error) {
	f.mu.Lock()
	f.calls = append(f.calls, strings.Join(append([]string{cmd}, args...), " "))
	f.mu.Unlock()
	if f.runFunc != nil {
		return f.runFunc(ctx, cmd, args, env, workDir)
	}
	return SandboxResult{}, nil
}

func makeHandshakeJSON(seq uint64, corr string) string {
	msg, _ := NewMessage(MessageHandshake, corr, seq, HandshakePayload{
		WorkerID:      "fake-pi-worker",
		WorkerVersion: "1.0.0",
		Protocol:      WorkerProtocolVersion,
	})
	line, _ := EncodeLine(msg)
	return string(line)
}

func makeTaskJSON(seq uint64, corr string, toolName string) string {
	msg, _ := NewMessage(MessageTask, corr, seq, TaskEnvelope{
		ContractVersion: HarnessContractVersion,
		ExecutionID:     corr,
		TaskID:          "task-pi-1",
		WorktreeID:      "wt-pi-1",
		ProjectID:       "proj-pi-1",
		ToolName:        toolName,
		Input:           json.RawMessage(`{"prompt":"write code"}`),
		TimeoutMs:       5000,
	})
	line, _ := EncodeLine(msg)
	return string(line)
}

func makeProgressJSON(seq uint64, corr string, phase string, percent int) string {
	msg, _ := NewMessage(MessageProgress, corr, seq, ProgressPayload{
		Phase:   phase,
		Percent: percent,
		Message: "working",
	})
	line, _ := EncodeLine(msg)
	return string(line)
}

func makeEvidenceJSON(seq uint64, corr string, evID string) string {
	digest := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	msg, _ := NewMessage(MessageEvidence, corr, seq, EvidencePayload{
		EvidenceID: evID,
		Kind:       EvidenceKindCommandExecution,
		Digest:     digest,
		Summary:    "code file generated",
	})
	line, _ := EncodeLine(msg)
	return string(line)
}

func makeDiagnosticJSON(seq uint64, corr string, code string, msgText string) string {
	msg, _ := NewMessage(MessageDiagnostic, corr, seq, DiagnosticPayload{
		Level:   "info",
		Code:    code,
		Message: msgText,
	})
	line, _ := EncodeLine(msg)
	return string(line)
}

func makeTerminalJSON(seq uint64, corr string, status TerminalStatus, errMsg string, evIDs ...string) string {
	payload := TerminalPayload{
		Status:      status,
		Message:     "finished",
		EvidenceIDs: evIDs,
	}
	if errMsg != "" {
		payload.Error = &ToolError{
			Code:    "harness.adapter_failure",
			Message: errMsg,
		}
	}
	msg, _ := NewMessage(MessageTerminal, corr, seq, payload)
	line, _ := EncodeLine(msg)
	return string(line)
}

func makeShutdownJSON(seq uint64, corr string) string {
	msg, _ := NewMessage(MessageShutdown, corr, seq, ShutdownPayload{
		Reason: "session closed",
	})
	line, _ := EncodeLine(msg)
	return string(line)
}

func testPiEnvelope(worktreeRoot, toolName string, allowed bool) ExecutionEnvelope {
	input, _ := json.Marshal(map[string]string{"prompt": "generate tests"})
	effect := EffectDeny
	if allowed {
		effect = EffectAllow
	}
	return ExecutionEnvelope{
		ContractVersion: HarnessContractVersion,
		ExecutionID:     "exec-pi-1",
		TaskID:          "task-pi-1",
		WorktreeID:      "wt-pi-1",
		ProjectID:       "proj-pi-1",
		WorktreeRoot:    worktreeRoot,
		ToolName:        toolName,
		Input:           input,
		PolicyDecision: PolicyDecision{
			Allowed: allowed,
			Effect:  effect,
			Reason:  "policy evaluation for test",
		},
		Timeout: 30 * time.Second,
		TraceID: "trace-pi-1",
	}
}

func TestPiAdapterDescriptorAndCapabilities(t *testing.T) {
	root := t.TempDir()
	runner := &fakePiRunner{}
	adapter, err := NewPiAdapter(PiAdapterConfig{
		WorktreeRoot: root,
		Runner:       runner,
	})
	if err != nil {
		t.Fatalf("NewPiAdapter failed: %v", err)
	}

	if adapter.Kind() != ToolKindPi {
		t.Fatalf("Kind = %v, want %v", adapter.Kind(), ToolKindPi)
	}

	desc := adapter.Describe()
	if desc.Name != PiAdapterName {
		t.Fatalf("Describe().Name = %q, want %q", desc.Name, PiAdapterName)
	}
	if desc.Version != PiAdapterVersion {
		t.Fatalf("Describe().Version = %q, want %q", desc.Version, PiAdapterVersion)
	}
	if desc.Kind != ToolKindPi {
		t.Fatalf("Describe().Kind = %v, want %v", desc.Kind, ToolKindPi)
	}
	if desc.SchemaVersion != HarnessContractVersion {
		t.Fatalf("Describe().SchemaVersion = %q, want %q", desc.SchemaVersion, HarnessContractVersion)
	}

	expectedCaps := []ToolCapability{
		CapabilityCoding, CapabilityAnalysis, CapabilityRefactor,
		CapabilityDebug, CapabilityReview, CapabilityTestGen, CapabilityDocGen,
	}
	caps := adapter.Capabilities()
	if len(caps) != len(expectedCaps) {
		t.Fatalf("Capabilities count = %d, want %d", len(caps), len(expectedCaps))
	}
	for _, expected := range expectedCaps {
		found := false
		for _, c := range caps {
			if c == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("missing expected capability: %s", expected)
		}
	}
}

func TestPiAdapterStartupAndHealth(t *testing.T) {
	root := t.TempDir()
	runner := &fakePiRunner{
		runFunc: func(ctx context.Context, cmd string, args []string, env []string, workDir string) (SandboxResult, error) {
			if len(args) > 0 && args[0] == "--handshake" {
				return SandboxResult{
					Stdout: []byte(makeHandshakeJSON(1, "health-corr")),
				}, nil
			}
			return SandboxResult{}, nil
		},
	}
	adapter, err := NewPiAdapter(PiAdapterConfig{
		WorktreeRoot: root,
		Runner:       runner,
	})
	if err != nil {
		t.Fatalf("NewPiAdapter failed: %v", err)
	}

	ctx := NewExecutionContext(context.Background(), "trace-health")
	if err := adapter.Health(ctx); err != nil {
		t.Fatalf("Health() failed: %v", err)
	}

	// Test handshake failure
	badRunner := &fakePiRunner{
		runFunc: func(ctx context.Context, cmd string, args []string, env []string, workDir string) (SandboxResult, error) {
			return SandboxResult{
				Stdout: []byte("invalid json output"),
			}, nil
		},
	}
	badAdapter, err := NewPiAdapter(PiAdapterConfig{
		WorktreeRoot: root,
		Runner:       badRunner,
	})
	if err != nil {
		t.Fatalf("NewPiAdapter failed: %v", err)
	}
	if err := badAdapter.Health(ctx); !errors.Is(err, ErrAdapterUnhealthy) {
		t.Fatalf("expected ErrAdapterUnhealthy, got: %v", err)
	}
}

func TestPiAdapterRouterRegistration(t *testing.T) {
	root := t.TempDir()
	runner := &fakePiRunner{
		runFunc: func(ctx context.Context, cmd string, args []string, env []string, workDir string) (SandboxResult, error) {
			if len(args) > 0 && args[0] == "--handshake" {
				return SandboxResult{
					Stdout: []byte(makeHandshakeJSON(1, "router-corr")),
				}, nil
			}
			return SandboxResult{}, nil
		},
	}
	adapter, err := NewPiAdapter(PiAdapterConfig{
		WorktreeRoot: root,
		Runner:       runner,
	})
	if err != nil {
		t.Fatalf("NewPiAdapter failed: %v", err)
	}

	router := NewBasicRouter(DefaultRoutingPolicy())
	desc := PiExtendedDescriptor()
	if err := router.Register(adapter, desc); err != nil {
		t.Fatalf("router.Register failed: %v", err)
	}

	ctx := context.Background()
	sel, err := router.Find(ctx, []ToolCapability{CapabilityCoding, CapabilityAnalysis})
	if err != nil {
		t.Fatalf("router.Find failed: %v", err)
	}
	if sel.Adapter.Kind() != ToolKindPi {
		t.Fatalf("selected adapter kind = %v, want %v", sel.Adapter.Kind(), ToolKindPi)
	}
}

func TestPiAdapterStreamingEvidence(t *testing.T) {
	root := t.TempDir()
	corr := "exec-pi-1"
	streamOutput := makeHandshakeJSON(1, corr) +
		makeTaskJSON(2, corr, "coding") +
		makeProgressJSON(3, corr, "parsing", 25) +
		makeEvidenceJSON(4, corr, "ev-pi-1") +
		makeDiagnosticJSON(5, corr, "pi.info", "analyzed 42 files") +
		makeEvidenceJSON(6, corr, "ev-pi-2") +
		makeTerminalJSON(7, corr, TerminalSuccess, "", "ev-pi-1", "ev-pi-2") +
		makeShutdownJSON(8, corr)

	runner := &fakePiRunner{
		runFunc: func(ctx context.Context, cmd string, args []string, env []string, workDir string) (SandboxResult, error) {
			return SandboxResult{
				Stdout:      []byte(streamOutput),
				RedactedOut: []byte(streamOutput),
				ExitCode:    0,
			}, nil
		},
	}

	var streamedEvidence []EvidenceRecord
	var evMu sync.Mutex

	adapter, err := NewPiAdapter(PiAdapterConfig{
		WorktreeRoot: root,
		Runner:       runner,
		OnEvidence: func(rec EvidenceRecord) {
			evMu.Lock()
			streamedEvidence = append(streamedEvidence, rec)
			evMu.Unlock()
		},
	})
	if err != nil {
		t.Fatalf("NewPiAdapter failed: %v", err)
	}

	envelope := testPiEnvelope(root, "coding", true)
	ctx := NewExecutionContext(context.Background(), "trace-stream")
	result, err := adapter.Execute(ctx, envelope)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	if result.Status != ToolStatusSuccess {
		t.Fatalf("Status = %v, want %v", result.Status, ToolStatusSuccess)
	}
	if len(result.EvidenceIDs) < 2 {
		t.Fatalf("EvidenceIDs count = %d, want >= 2", len(result.EvidenceIDs))
	}

	evMu.Lock()
	defer evMu.Unlock()
	if len(streamedEvidence) < 2 {
		t.Fatalf("streamed evidence count = %d, want >= 2", len(streamedEvidence))
	}
}

func TestPiAdapterCancellation(t *testing.T) {
	root := t.TempDir()
	runner := &fakePiRunner{
		runFunc: func(ctx context.Context, cmd string, args []string, env []string, workDir string) (SandboxResult, error) {
			return SandboxResult{
				Canceled: true,
			}, ErrCanceled
		},
	}

	adapter, err := NewPiAdapter(PiAdapterConfig{
		WorktreeRoot: root,
		Runner:       runner,
	})
	if err != nil {
		t.Fatalf("NewPiAdapter failed: %v", err)
	}

	envelope := testPiEnvelope(root, "coding", true)
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Pre-cancel
	execCtx := NewExecutionContext(ctx, "trace-cancel")

	result, err := adapter.Execute(execCtx, envelope)
	if !errors.Is(err, ErrCanceled) {
		t.Fatalf("expected ErrCanceled, got: %v", err)
	}
	if result.Status != ToolStatusCanceled {
		t.Fatalf("Status = %v, want %v", result.Status, ToolStatusCanceled)
	}
}

func TestPiAdapterTimeout(t *testing.T) {
	root := t.TempDir()
	runner := &fakePiRunner{
		runFunc: func(ctx context.Context, cmd string, args []string, env []string, workDir string) (SandboxResult, error) {
			return SandboxResult{
				TimedOut: true,
			}, ErrTimeout
		},
	}

	adapter, err := NewPiAdapter(PiAdapterConfig{
		WorktreeRoot: root,
		Runner:       runner,
	})
	if err != nil {
		t.Fatalf("NewPiAdapter failed: %v", err)
	}

	envelope := testPiEnvelope(root, "coding", true)
	ctx := NewExecutionContext(context.Background(), "trace-timeout")

	result, err := adapter.Execute(ctx, envelope)
	if !errors.Is(err, ErrTimeout) {
		t.Fatalf("expected ErrTimeout, got: %v", err)
	}
	if result.Status != ToolStatusTimeout {
		t.Fatalf("Status = %v, want %v", result.Status, ToolStatusTimeout)
	}
}

func TestPiAdapterCrash(t *testing.T) {
	root := t.TempDir()

	// Crash case 1: Non-zero exit with execution error
	crashRunner := &fakePiRunner{
		runFunc: func(ctx context.Context, cmd string, args []string, env []string, workDir string) (SandboxResult, error) {
			return SandboxResult{
				ExitCode: 137,
			}, errors.New("process terminated by SIGKILL")
		},
	}
	adapter, err := NewPiAdapter(PiAdapterConfig{
		WorktreeRoot: root,
		Runner:       crashRunner,
	})
	if err != nil {
		t.Fatalf("NewPiAdapter failed: %v", err)
	}

	envelope := testPiEnvelope(root, "coding", true)
	ctx := NewExecutionContext(context.Background(), "trace-crash")

	result, err := adapter.Execute(ctx, envelope)
	if !errors.Is(err, ErrAdapterFailure) {
		t.Fatalf("expected ErrAdapterFailure, got: %v", err)
	}
	if result.Status != ToolStatusAdapterError && result.Status != ToolStatusFailed {
		t.Fatalf("Status = %v, want failed/adapter_error", result.Status)
	}

	// Crash case 2: Premature EOF before terminal message
	corr := "exec-pi-1"
	truncatedOutput := makeHandshakeJSON(1, corr) +
		makeTaskJSON(2, corr, "coding") +
		makeProgressJSON(3, corr, "crashed_here", 10)

	truncRunner := &fakePiRunner{
		runFunc: func(ctx context.Context, cmd string, args []string, env []string, workDir string) (SandboxResult, error) {
			return SandboxResult{
				Stdout: []byte(truncatedOutput),
			}, nil
		},
	}
	adapter2, err := NewPiAdapter(PiAdapterConfig{
		WorktreeRoot: root,
		Runner:       truncRunner,
	})
	if err != nil {
		t.Fatalf("NewPiAdapter failed: %v", err)
	}
	result2, err2 := adapter2.Execute(ctx, envelope)
	if !errors.Is(err2, ErrAdapterFailure) {
		t.Fatalf("expected ErrAdapterFailure on premature EOF, got: %v", err2)
	}
	if result2.Status != ToolStatusFailed && result2.Status != ToolStatusAdapterError {
		t.Fatalf("Status = %v, want failed", result2.Status)
	}
}

func TestPiAdapterMissingExecutable(t *testing.T) {
	root := t.TempDir()
	nonExistentPath := filepath.Join(t.TempDir(), "nonexistent", "secret_admin_token", "pi_binary")

	_, err := NewPiAdapter(PiAdapterConfig{
		WorktreeRoot:   root,
		ExecutablePath: nonExistentPath,
		Runner:         nil, // Use DefaultProcessRunner to check executable existence
	})
	if !errors.Is(err, ErrPiExecutableNotFound) {
		t.Fatalf("expected ErrPiExecutableNotFound, got: %v", err)
	}

	// Verify error string is sanitized and does not leak full paths or tokens
	if strings.Contains(err.Error(), "secret_admin_token") {
		t.Fatalf("error leaked secret path: %v", err)
	}
}

func TestPiAdapterPolicyDenial(t *testing.T) {
	root := t.TempDir()
	runner := &fakePiRunner{}
	adapter, err := NewPiAdapter(PiAdapterConfig{
		WorktreeRoot: root,
		Runner:       runner,
	})
	if err != nil {
		t.Fatalf("NewPiAdapter failed: %v", err)
	}

	envelope := testPiEnvelope(root, "coding", false) // PolicyDenied
	ctx := NewExecutionContext(context.Background(), "trace-policy")

	result, err := adapter.Execute(ctx, envelope)
	if !errors.Is(err, ErrPolicyDenied) {
		t.Fatalf("expected ErrPolicyDenied, got: %v", err)
	}
	if result.Status != ToolStatusPolicyDenied {
		t.Fatalf("Status = %v, want %v", result.Status, ToolStatusPolicyDenied)
	}

	// Verify runner was NEVER invoked on policy denial
	if len(runner.calls) > 0 {
		t.Fatalf("runner should not be called when policy is denied, got %d calls", len(runner.calls))
	}
}

func TestPiAdapterResourceLimits(t *testing.T) {
	gov := NewGovernor(WithBudgets(map[EngineClass]ResourceBudget{
		EngineClassPi: {MaxWorkers: 2, MaxMemoryMB: 1000, MaxCPUPriority: 2},
	}))

	ctx := context.Background()

	// Admit task 1
	res1, err := gov.Admit(ctx, TaskDescriptor{
		TaskID:            "pi-task-1",
		EngineClass:       EngineClassPi,
		EstimatedMemoryMB: 400,
	})
	if err != nil {
		t.Fatalf("Admit task 1 failed: %v", err)
	}
	defer res1.Release()

	// Admit task 2
	res2, err := gov.Admit(ctx, TaskDescriptor{
		TaskID:            "pi-task-2",
		EngineClass:       EngineClassPi,
		EstimatedMemoryMB: 400,
	})
	if err != nil {
		t.Fatalf("Admit task 2 failed: %v", err)
	}
	defer res2.Release()

	// Task 3 exceeds MaxWorkers (2), with a short timeout should fail with ErrCapacityExceeded
	timeoutCtx, cancel := context.WithTimeout(ctx, 20*time.Millisecond)
	defer cancel()

	_, err = gov.Admit(timeoutCtx, TaskDescriptor{
		TaskID:            "pi-task-3",
		EngineClass:       EngineClassPi,
		EstimatedMemoryMB: 400,
	})
	if !errors.Is(err, ErrCapacityExceeded) {
		t.Fatalf("expected ErrCapacityExceeded for worker pool cap, got: %v", err)
	}

	// Release task 1, now task 3 can be admitted
	res1.Release()

	res3, err := gov.Admit(ctx, TaskDescriptor{
		TaskID:            "pi-task-3",
		EngineClass:       EngineClassPi,
		EstimatedMemoryMB: 400,
	})
	if err != nil {
		t.Fatalf("Admit task 3 failed after release: %v", err)
	}
	res3.Release()
}

func TestPiAdapterWorktreeViolation(t *testing.T) {
	root := t.TempDir()
	otherRoot := t.TempDir()

	adapter, err := NewPiAdapter(PiAdapterConfig{
		WorktreeRoot: root,
		Runner:       &fakePiRunner{},
	})
	if err != nil {
		t.Fatalf("NewPiAdapter failed: %v", err)
	}

	// Mismatched worktree root
	envelope := testPiEnvelope(otherRoot, "coding", true)
	ctx := NewExecutionContext(context.Background(), "trace-wt")

	result, err := adapter.Execute(ctx, envelope)
	if !errors.Is(err, ErrWorktreeViolation) {
		t.Fatalf("expected ErrWorktreeViolation, got: %v", err)
	}
	if result.Status != ToolStatusSandboxError {
		t.Fatalf("Status = %v, want %v", result.Status, ToolStatusSandboxError)
	}
}

func TestPiAdapterUnsupportedCapability(t *testing.T) {
	root := t.TempDir()
	adapter, err := NewPiAdapter(PiAdapterConfig{
		WorktreeRoot: root,
		Runner:       &fakePiRunner{},
	})
	if err != nil {
		t.Fatalf("NewPiAdapter failed: %v", err)
	}

	envelope := testPiEnvelope(root, "completely_unsupported_capability_xyz", true)
	ctx := NewExecutionContext(context.Background(), "trace-cap")

	_, err = adapter.Execute(ctx, envelope)
	if !errors.Is(err, ErrNoEligibleAdapter) && !errors.Is(err, ErrPiCapabilityMissing) {
		t.Fatalf("expected ErrPiCapabilityMissing, got: %v", err)
	}
}

func TestPiAdapterRealProcessFixture(t *testing.T) {
	root := t.TempDir()
	corr := "exec-real-1"
	lines := []string{
		makeHandshakeJSON(1, corr),
		makeTaskJSON(2, corr, "coding"),
		makeProgressJSON(3, corr, "starting", 10),
		makeEvidenceJSON(4, corr, "ev-real-1"),
		makeTerminalJSON(5, corr, TerminalSuccess, "", "ev-real-1"),
		makeShutdownJSON(6, corr),
	}

	var scriptPath string
	if runtime.GOOS == "windows" {
		scriptPath = filepath.Join(root, "fake_pi.cmd")
		var sb strings.Builder
		sb.WriteString("@echo off\r\n")
		sb.WriteString("if \"%1\"==\"--handshake\" (\r\n")
		sb.WriteString(fmt.Sprintf("  echo %s\r\n", strings.ReplaceAll(lines[0], "\n", "")))
		sb.WriteString("  exit /b 0\r\n")
		sb.WriteString(")\r\n")
		for _, line := range lines {
			sb.WriteString(fmt.Sprintf("echo %s\r\n", strings.ReplaceAll(line, "\n", "")))
		}
		if err := os.WriteFile(scriptPath, []byte(sb.String()), 0o755); err != nil {
			t.Fatal(err)
		}
	} else {
		scriptPath = filepath.Join(root, "fake_pi.sh")
		var sb strings.Builder
		sb.WriteString("#!/bin/sh\n")
		sb.WriteString("if [ \"$1\" = \"--handshake\" ]; then\n")
		sb.WriteString(fmt.Sprintf("  echo '%s'\n", strings.ReplaceAll(lines[0], "\n", "")))
		sb.WriteString("  exit 0\n")
		sb.WriteString("fi\n")
		for _, line := range lines {
			sb.WriteString(fmt.Sprintf("echo '%s'\n", strings.ReplaceAll(line, "\n", "")))
		}
		if err := os.WriteFile(scriptPath, []byte(sb.String()), 0o755); err != nil {
			t.Fatal(err)
		}
	}

	adapter, err := NewPiAdapter(PiAdapterConfig{
		WorktreeRoot:   root,
		ExecutablePath: scriptPath,
		StartupTimeout: 5 * time.Second,
	})
	if err != nil {
		t.Fatalf("NewPiAdapter failed: %v", err)
	}

	ctx := NewExecutionContext(context.Background(), "trace-real")

	// Test real health check
	if err := adapter.Health(ctx); err != nil {
		t.Fatalf("real process Health() failed: %v", err)
	}

	// Test real execution
	envelope := testPiEnvelope(root, "coding", true)
	envelope.ExecutionID = corr
	res, err := adapter.Execute(ctx, envelope)
	if err != nil {
		t.Fatalf("real process Execute() failed: %v", err)
	}
	if res.Status != ToolStatusSuccess {
		t.Fatalf("Status = %v, want %v", res.Status, ToolStatusSuccess)
	}
	if len(res.EvidenceIDs) == 0 {
		t.Fatal("expected at least one evidence ID")
	}
}
