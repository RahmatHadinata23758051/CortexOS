package harness

import (
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

type fakePiCLIRunner struct {
	mu      sync.Mutex
	calls   [][]string
	runFunc func(ctx context.Context, cmd string, args []string, env []string, workDir string) (SandboxResult, error)
}

func (f *fakePiCLIRunner) Run(ctx context.Context, cmd string, args []string, env []string, workDir string,
	stdoutLimit, stderrLimit int64, gracePeriod time.Duration) (SandboxResult, error) {
	f.mu.Lock()
	f.calls = append(f.calls, append([]string{cmd}, args...))
	f.mu.Unlock()
	if f.runFunc != nil {
		return f.runFunc(ctx, cmd, args, env, workDir)
	}
	return SandboxResult{}, nil
}

func (f *fakePiCLIRunner) lastCall() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.calls) == 0 {
		return nil
	}
	return f.calls[len(f.calls)-1]
}

func TestPiCLIAdapterDescriptorAndCapabilities(t *testing.T) {
	tmp := t.TempDir()
	adapter, err := NewPiCLIAdapter(PiCLIAdapterConfig{
		ExecutablePath: "pi",
		WorktreeRoot:   tmp,
		Runner:         &fakePiCLIRunner{},
	})
	if err != nil {
		t.Fatalf("NewPiCLIAdapter failed: %v", err)
	}
	if adapter.Kind() != ToolKindPi {
		t.Fatalf("expected ToolKindPi, got %s", adapter.Kind())
	}
	desc := adapter.Describe()
	if desc.Name != PiCLIAdapterName {
		t.Fatalf("expected descriptor name %q, got %q", PiCLIAdapterName, desc.Name)
	}
	caps := adapter.Capabilities()
	if len(caps) == 0 {
		t.Fatalf("expected capabilities to be non-empty")
	}
}

func TestPiCLIAdapterHealthOffline(t *testing.T) {
	tmp := t.TempDir()
	runner := &fakePiCLIRunner{}
	adapter, err := NewPiCLIAdapter(PiCLIAdapterConfig{
		ExecutablePath: "pi",
		WorktreeRoot:   tmp,
		Runner:         runner,
	})
	if err != nil {
		t.Fatalf("NewPiCLIAdapter failed: %v", err)
	}
	if err := adapter.Health(nil); err != nil {
		t.Fatalf("expected health to pass without network/model call, got: %v", err)
	}
	if len(runner.calls) != 0 {
		t.Fatalf("expected health to be offline without process calls, got %d calls", len(runner.calls))
	}
}

func TestPiCLIAdapterExecuteSuccess(t *testing.T) {
	tmp := t.TempDir()
	stream := strings.Join([]string{
		`{"type":"message_start","message":{"role":"user","content":[{"type":"text","text":"Implement hello"}]}}`,
		`{"type":"message_start","message":{"role":"assistant","content":[]}}`,
		`{"type":"message_end","message":{"role":"assistant","content":[{"type":"text","text":"Implemented Hello in main.go"}]}}`,
		`{"type":"agent_end"}`,
	}, "\n")

	runner := &fakePiCLIRunner{
		runFunc: func(ctx context.Context, cmd string, args []string, env []string, workDir string) (SandboxResult, error) {
			return SandboxResult{
				Stdout:   []byte(stream),
				ExitCode: 0,
			}, nil
		},
	}

	var recorded []EvidenceRecord
	adapter, err := NewPiCLIAdapter(PiCLIAdapterConfig{
		ExecutablePath: "pi",
		WorktreeRoot:   tmp,
		Runner:         runner,
		OnEvidence: func(rec EvidenceRecord) {
			recorded = append(recorded, rec)
		},
	})
	if err != nil {
		t.Fatalf("NewPiCLIAdapter failed: %v", err)
	}

	envelope := ExecutionEnvelope{
		ContractVersion: HarnessContractVersion,
		ExecutionID:     "exec-picli-1",
		TaskID:          "task-picli-1",
		WorktreeID:      "wt-picli-1",
		ProjectID:       "proj-picli-1",
		WorktreeRoot:    tmp,
		ToolName:        "coding_agent",
		Input:           json.RawMessage(`{"title":"Implement hello","acceptanceCriteria":["writes main.go"]}`),
		Timeout:         5 * time.Second,
		PolicyDecision:  PolicyDecision{Allowed: true},
	}

	result, err := adapter.Execute(nil, envelope)
	if err != nil {
		t.Fatalf("adapter.Execute returned error: %v", err)
	}
	if result.Status != ToolStatusSuccess {
		t.Fatalf("expected ToolStatusSuccess, got %s", result.Status)
	}
	if len(result.EvidenceIDs) == 0 {
		t.Fatalf("expected at least one evidence ID")
	}
	if len(recorded) != 1 {
		t.Fatalf("expected 1 recorded evidence, got %d", len(recorded))
	}
	if recorded[0].Kind != EvidenceKindCommandExecution {
		t.Fatalf("expected kind command_execution, got %s", recorded[0].Kind)
	}

	call := runner.lastCall()
	if len(call) < 5 {
		t.Fatalf("expected at least 5 argv elements, got: %v", call)
	}
	if call[1] != "--print" || call[2] != "--mode" || call[3] != "json" || call[4] != "--no-session" {
		t.Fatalf("unexpected CLI flags: %v", call)
	}
	lastArg := call[len(call)-1]
	if !strings.Contains(lastArg, "Implement hello") || !strings.Contains(lastArg, "writes main.go") {
		t.Fatalf("prompt was not formatted properly into argv: %q", lastArg)
	}
}

func TestPiCLIAdapterExecuteModelAuthFailure(t *testing.T) {
	tmp := t.TempDir()
	stream := strings.Join([]string{
		`{"type":"message_start","message":{"role":"user","content":[{"type":"text","text":"Implement hello"}]}}`,
		`{"type":"message_start","message":{"role":"assistant","content":[],"errorMessage":"401 invalid x-api-key"}}`,
		`{"type":"message_end","message":{"role":"assistant","content":[],"errorMessage":"401 invalid x-api-key"}}`,
	}, "\n")

	runner := &fakePiCLIRunner{
		runFunc: func(ctx context.Context, cmd string, args []string, env []string, workDir string) (SandboxResult, error) {
			return SandboxResult{
				Stdout:   []byte(stream),
				ExitCode: 1,
			}, nil
		},
	}

	var recorded []EvidenceRecord
	adapter, err := NewPiCLIAdapter(PiCLIAdapterConfig{
		ExecutablePath: "pi",
		WorktreeRoot:   tmp,
		Runner:         runner,
		OnEvidence: func(rec EvidenceRecord) {
			recorded = append(recorded, rec)
		},
	})
	if err != nil {
		t.Fatalf("NewPiCLIAdapter failed: %v", err)
	}

	envelope := ExecutionEnvelope{
		ContractVersion: HarnessContractVersion,
		ExecutionID:     "exec-picli-2",
		TaskID:          "task-picli-2",
		WorktreeID:      "wt-picli-2",
		WorktreeRoot:    tmp,
		ToolName:        "coding_agent",
		Input:           json.RawMessage(`{"prompt":"Implement hello"}`),
		Timeout:         5 * time.Second,
		PolicyDecision:  PolicyDecision{Allowed: true},
	}

	result, err := adapter.Execute(nil, envelope)
	if err == nil {
		t.Fatalf("expected error on model failure, got nil")
	}
	if result.Status != ToolStatusAdapterError {
		t.Fatalf("expected ToolStatusAdapterError, got %s", result.Status)
	}
	if result.Error == nil {
		t.Fatalf("expected structured adapter error")
	}
	if len(result.EvidenceIDs) == 0 {
		t.Fatalf("expected evidence to be preserved even on failure")
	}
	if !strings.Contains(string(recorded[0].RedactedPayload), "401 invalid x-api-key") {
		t.Fatalf("expected provider failure in redacted evidence, got %s", recorded[0].RedactedPayload)
	}
	if len(recorded) != 1 {
		t.Fatalf("expected evidence to be recorded on failure, got %d", len(recorded))
	}
}

func TestPiCLIAdapterExecuteTimeout(t *testing.T) {
	tmp := t.TempDir()
	runner := &fakePiCLIRunner{
		runFunc: func(ctx context.Context, cmd string, args []string, env []string, workDir string) (SandboxResult, error) {
			return SandboxResult{TimedOut: true}, ErrTimeout
		},
	}

	adapter, err := NewPiCLIAdapter(PiCLIAdapterConfig{
		ExecutablePath: "pi",
		WorktreeRoot:   tmp,
		Runner:         runner,
	})
	if err != nil {
		t.Fatalf("NewPiCLIAdapter failed: %v", err)
	}

	envelope := ExecutionEnvelope{
		ContractVersion: HarnessContractVersion,
		ExecutionID:     "exec-picli-3",
		TaskID:          "task-picli-3",
		WorktreeID:      "wt-picli-3",
		WorktreeRoot:    tmp,
		ToolName:        "coding",
		Input:           json.RawMessage(`{"prompt":"Do work"}`),
		Timeout:         10 * time.Millisecond,
		PolicyDecision:  PolicyDecision{Allowed: true},
	}

	result, err := adapter.Execute(nil, envelope)
	if err == nil {
		t.Fatalf("expected timeout error, got nil")
	}
	if result.Status != ToolStatusTimeout {
		t.Fatalf("expected ToolStatusTimeout, got %s", result.Status)
	}
}

func TestPiCLIAdapterFindExecutable(t *testing.T) {
	// If pi is installed in the default location, findPiCLIExecutable should locate it.
	path, err := findPiCLIExecutable()
	if err != nil {
		t.Skipf("pi is not installed on this system: %v", err)
	}
	if !filepath.IsAbs(path) && !strings.Contains(path, "pi") {
		t.Fatalf("unexpected pi path: %s", path)
	}
}
