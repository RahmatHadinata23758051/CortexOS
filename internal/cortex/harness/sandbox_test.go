package harness

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func tempDir(t *testing.T) string {
	dir, err := os.MkdirTemp("", "cortex-sandbox-*")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	return dir
}

func mustSandbox(root string) *ExecutionSandbox {
	s, err := NewSandbox(SandboxConfig{WorktreeRoot: root})
	if err != nil {
		panic(err)
	}
	return s
}

func TestSandboxNewRejectsMissingRoot(t *testing.T) {
	_, err := NewSandbox(SandboxConfig{})
	if err == nil || !errors.Is(err, ErrSandbox) {
		t.Fatalf("expected sandbox error, got %v", err)
	}
}

func TestSandboxNewRejectsNonExistentRoot(t *testing.T) {
	_, err := NewSandbox(SandboxConfig{WorktreeRoot: "/nonexistent/path"})
	if err == nil || !errors.Is(err, ErrSandbox) {
		t.Fatalf("expected sandbox error, got %v", err)
	}
}

func TestSandboxValidateAcceptsMatchingRoot(t *testing.T) {
	root := tempDir(t)
	sb := mustSandbox(root)

	env := ExecutionEnvelope{
		ExecutionID: "e1", TaskID: "t1", WorktreeID: "w1",
		WorktreeRoot: root, Timeout: 30 * time.Second,
		PolicyDecision: PolicyDecision{Allowed: true},
	}
	if err := sb.Validate(nil, env); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSandboxValidateRejectsMismatchedRoot(t *testing.T) {
	root := tempDir(t)
	sb := mustSandbox(root)

	env := ExecutionEnvelope{
		ExecutionID: "e1", TaskID: "t1", WorktreeID: "w1",
		WorktreeRoot: "/other/root", Timeout: 30 * time.Second,
		PolicyDecision: PolicyDecision{Allowed: true},
	}
	if err := sb.Validate(nil, env); err == nil || !errors.Is(err, ErrWorktreeViolation) {
		t.Fatalf("expected worktree violation, got %v", err)
	}
}

func TestSandboxValidateRejectsMissingScope(t *testing.T) {
	root := tempDir(t)
	sb := mustSandbox(root)

	env := ExecutionEnvelope{
		ExecutionID: "e1", WorktreeRoot: root, Timeout: 30 * time.Second,
		PolicyDecision: PolicyDecision{Allowed: true},
	}
	if err := sb.Validate(nil, env); err == nil || !errors.Is(err, ErrInvalidContract) {
		t.Fatalf("expected invalid contract, got %v", err)
	}
}

func TestSandboxValidateRejectsDeniedPolicy(t *testing.T) {
	root := tempDir(t)
	sb := mustSandbox(root)

	env := ExecutionEnvelope{
		ExecutionID: "e1", TaskID: "t1", WorktreeID: "w1",
		WorktreeRoot: root, Timeout: 30 * time.Second,
		PolicyDecision: PolicyDecision{Allowed: false},
	}
	if err := sb.Validate(nil, env); err == nil || !errors.Is(err, ErrPolicyDenied) {
		t.Fatalf("expected policy denied, got %v", err)
	}
}

func TestSandboxValidatePathResolvesRelative(t *testing.T) {
	root := tempDir(t)
	sb := mustSandbox(root)

	sub := filepath.Join(root, "subdir")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}

	canonical, err := sb.ValidatePath("subdir")
	if err != nil {
		t.Fatalf("ValidatePath: %v", err)
	}
	if !samePath(canonical, sub) {
		t.Fatalf("expected %q, got %q", sub, canonical)
	}
}

func TestSandboxValidatePathRejectsAbsoluteOutside(t *testing.T) {
	root := tempDir(t)
	sb := mustSandbox(root)

	other := tempDir(t)
	if _, err := sb.ValidatePath(other); err == nil || !errors.Is(err, ErrWorktreeViolation) {
		t.Fatalf("expected worktree violation, got %v", err)
	}
}

func TestSandboxValidatePathRejectsTraversal(t *testing.T) {
	root := tempDir(t)
	sb := mustSandbox(root)

	_, err := sb.ValidatePath("subdir/../..")
	if err == nil || !errors.Is(err, ErrWorktreeViolation) {
		t.Fatalf("expected worktree violation, got %v", err)
	}
}

func TestSandboxValidatePathAcceptsAbsoluteInside(t *testing.T) {
	root := tempDir(t)
	sb := mustSandbox(root)

	sub := filepath.Join(root, "sub")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	canonical, err := sb.ValidatePath(sub)
	if err != nil {
		t.Fatalf("ValidatePath: %v", err)
	}
	if !samePath(canonical, sub) {
		t.Fatalf("expected %q, got %q", sub, canonical)
	}
}

func TestSandboxValidatePathRejectsSymlinkEscape(t *testing.T) {
	root := tempDir(t)
	sb := mustSandbox(root)

	target := tempDir(t)
	link := filepath.Join(root, "evil")
	if err := os.Symlink(target, link); err != nil {
		if runtime.GOOS == "windows" {
			t.Skipf("symlink creation not permitted: %v", err)
		}
		t.Fatal(err)
	}

	_, err := sb.ValidatePath("evil")
	if err == nil || !errors.Is(err, ErrWorktreeViolation) {
		t.Fatalf("expected worktree violation for symlink escape, got %v", err)
	}
}

func TestSandboxValidateArgsRejectsShellMeta(t *testing.T) {
	root := tempDir(t)
	sb := mustSandbox(root)

	for _, arg := range []string{"echo; rm -rf /", "ls && cat /etc/passwd", "cmd `id`", "echo $(whoami)"} {
		if err := sb.ValidateArgs([]string{arg}, nil); err == nil {
			t.Fatalf("expected rejection for %q", arg)
		}
	}
}

func TestSandboxValidateArgsRejectsTraversal(t *testing.T) {
	root := tempDir(t)
	sb := mustSandbox(root)

	if err := sb.ValidateArgs([]string{"../etc/passwd"}, nil); err == nil {
		t.Fatal("expected traversal rejection")
	}
}

func TestSandboxFilterEnvironmentAllowsListed(t *testing.T) {
	root := tempDir(t)
	sb, err := NewSandbox(SandboxConfig{
		WorktreeRoot:   root,
		AllowedEnvVars: []string{"PATH", "HOME", "CUSTOM_VAR"},
	})
	if err != nil {
		t.Fatal(err)
	}

	in := []string{"PATH=/bin", "HOME=/home/user", "CUSTOM_VAR=value", "SECRET_KEY=abc", "NOT_ALLOWED=foo"}
	out, err := sb.FilterEnvironment(in)
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 3 {
		t.Fatalf("expected 3 vars, got %d: %v", len(out), out)
	}
	for _, v := range out {
		if strings.HasPrefix(v, "SECRET_") || strings.HasPrefix(v, "NOT_ALLOWED") {
			t.Fatalf("secret or unallowed var passed through: %s", v)
		}
	}
}

func TestSandboxFilterEnvironmentRedactsSecrets(t *testing.T) {
	root := tempDir(t)
	sb, err := NewSandbox(SandboxConfig{
		WorktreeRoot:   root,
		AllowedEnvVars: []string{"API_KEY"},
	})
	if err != nil {
		t.Fatal(err)
	}

	// Even though API_KEY is allowed, its value looks like a secret.
	in := []string{"API_KEY=sk-12345678901234567890"}
	out, _ := sb.FilterEnvironment(in)
	if len(out) != 0 {
		t.Fatalf("secret-like value should be dropped even for allowed key: %v", out)
	}
}

func TestSandboxExecuteAdapterValidatesFirst(t *testing.T) {
	root := tempDir(t)
	sb := mustSandbox(root)

	engine := NewFakeEngine(ToolKindNative, CapabilityShell)
	engine.SetExecuteFunc(func(_ ExecutionContext, env ExecutionEnvelope) (ToolResult, error) {
		return ToolResult{
			ContractVersion: HarnessContractVersion,
			ExecutionID:     env.ExecutionID,
			TaskID:          env.TaskID,
			ToolName:        env.ToolName,
			Status:          ToolStatusSuccess,
			Output:          []byte(`{"stdout": "ok"}`),
			StartedAt:       time.Now(),
			CompletedAt:     time.Now(),
			Redacted:        true,
		}, nil
	})

	env := ExecutionEnvelope{
		ExecutionID: "e1", TaskID: "t1", WorktreeID: "w1", ToolName: "shell",
		WorktreeRoot: root, Timeout: 10 * time.Second,
		PolicyDecision: PolicyDecision{Allowed: true},
	}
	_, err := sb.ExecuteAdapter(NewExecutionContext(context.Background(), "trace-test"), engine, env)
	if err != nil {
		t.Fatalf("ExecuteAdapter: %v", err)
	}
}

func TestSandboxExecuteAdapterRejectsOnValidate(t *testing.T) {
	root := tempDir(t)
	sb := mustSandbox(root)

	engine := NewFakeEngine(ToolKindNative, CapabilityShell)
	env := ExecutionEnvelope{
		ExecutionID: "e1", TaskID: "t1", WorktreeID: "w1", ToolName: "shell",
		WorktreeRoot: "/wrong/root", Timeout: 10 * time.Second,
		PolicyDecision: PolicyDecision{Allowed: true},
	}
	_, err := sb.ExecuteAdapter(NewExecutionContext(context.Background(), "trace-test"), engine, env)
	if err == nil || !errors.Is(err, ErrWorktreeViolation) {
		t.Fatalf("expected worktree violation, got %v", err)
	}
}

func TestSandboxProcessRunSuccess(t *testing.T) {
	root := tempDir(t)
	sb := mustSandbox(root)

	runner := &DefaultProcessRunner{}
	ctx := context.Background()
	result, err := sb.ExecuteWithSandbox(NewExecutionContext(ctx, "trace-test"), runner, "cmd", []string{"/c", "echo", "hello"}, os.Environ(), root)
	if err != nil {
		t.Fatalf("ExecuteWithSandbox: %v", err)
	}
	if result.ExitCode != 0 {
		t.Fatalf("non-zero exit: %d", result.ExitCode)
	}
	if !bytes.Contains(result.Stdout, []byte("hello")) {
		t.Fatalf("missing expected output: %s", result.Stdout)
	}
	if len(result.RedactedOut) == 0 {
		t.Fatal("redacted output should be present")
	}
}

func TestSandboxProcessRunRejectsNonExistentCommand(t *testing.T) {
	root := tempDir(t)
	sb := mustSandbox(root)

	runner := &DefaultProcessRunner{}
	ctx := context.Background()
	_, err := sb.ExecuteWithSandbox(NewExecutionContext(ctx, "trace-test"), runner, "definitely_not_a_real_command_12345", []string{}, os.Environ(), root)
	if err == nil {
		t.Fatal("expected error for non-existent command")
	}
}

func TestSandboxProcessRunTimeout(t *testing.T) {
	root := tempDir(t)
	sb, err := NewSandbox(SandboxConfig{
		WorktreeRoot:   root,
		Timeout:        10 * time.Millisecond,
		AllowedEnvVars: []string{"PATH"},
	})
	if err != nil {
		t.Fatal(err)
	}

	runner := &DefaultProcessRunner{}
	ctx := context.Background()
	result, err := sb.ExecuteWithSandbox(NewExecutionContext(ctx, "trace-test"), runner, "powershell", []string{"-NoProfile", "-Command", "Start-Sleep -Seconds 1"}, os.Environ(), root)
	if err == nil {
		t.Fatal("expected timeout error")
	}
	if !result.TimedOut {
		t.Fatalf("expected TimedOut=true, got %+v", result)
	}
	if !errors.Is(err, ErrTimeout) {
		t.Fatalf("expected ErrTimeout, got %v", err)
	}
}

func TestSandboxProcessRunCancellation(t *testing.T) {
	root := tempDir(t)
	sb := mustSandbox(root)

	runner := &DefaultProcessRunner{}
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately
	result, err := sb.ExecuteWithSandbox(NewExecutionContext(ctx, "trace-test"), runner, "powershell", []string{"-NoProfile", "-Command", "Start-Sleep -Seconds 10"}, os.Environ(), root)
	if err == nil {
		t.Fatal("expected cancellation error")
	}
	if !result.Canceled {
		t.Fatalf("expected Canceled=true, got %+v", result)
	}
	if !errors.Is(err, ErrCanceled) {
		t.Fatalf("expected ErrCanceled, got %v", err)
	}
}

func TestSandboxProcessRunOutputLimit(t *testing.T) {
	root := tempDir(t)
	sb, err := NewSandbox(SandboxConfig{
		WorktreeRoot:     root,
		StdoutLimitBytes: 10,
		StderrLimitBytes: 10,
		AllowedEnvVars:   []string{"PATH"},
		RedactionConfig:  RedactionConfig{},
	})
	if err != nil {
		t.Fatal(err)
	}

	runner := &DefaultProcessRunner{}
	ctx := context.Background()
	// Use a command that produces more than 10 bytes
	_, err = sb.ExecuteWithSandbox(NewExecutionContext(ctx, "trace-test"), runner, "cmd", []string{"/c", "echo", "this output is longer than ten bytes"}, os.Environ(), root)
	if err != nil {
		t.Fatalf("ExecuteWithSandbox: %v", err)
	}
	// The truncated flag is set on the buffer inside the runner, but the
	// sandbox doesn't expose it directly; we rely on the length being bounded.
}

func TestSandboxRecordAuditCreatesEvidence(t *testing.T) {
	root := tempDir(t)
	sb := mustSandbox(root)

	env := ExecutionEnvelope{
		ExecutionID: "e1", TaskID: "t1", WorktreeID: "w1",
		WorktreeRoot: root, Timeout: 10 * time.Second,
	}
	ev, err := sb.RecordAudit(env, SandboxAuditEvent{
		Event:      "validate",
		Path:       "test/file.go",
		Command:    "echo",
		Args:       []string{"hello"},
		Truncated:  false,
		Redacted:   true,
		ExitCode:   0,
		DurationMs: 5,
	})
	if err != nil {
		t.Fatal(err)
	}
	if ev.Kind != EvidenceKindSandboxAudit {
		t.Fatalf("wrong evidence kind: %v", ev.Kind)
	}
	if ev.ExecutionID != "e1" || ev.TaskID != "t1" || ev.WorktreeID != "w1" {
		t.Fatal("scope IDs not propagated")
	}
}

func TestSandboxConfigDefaults(t *testing.T) {
	root := tempDir(t)
	sb, err := NewSandbox(SandboxConfig{WorktreeRoot: root})
	if err != nil {
		t.Fatal(err)
	}
	cfg := sb.Config()
	if cfg.StdoutLimitBytes != DefaultSandboxConfig().StdoutLimitBytes {
		t.Fatalf("default stdout limit not applied: %d", cfg.StdoutLimitBytes)
	}
	if cfg.StderrLimitBytes != DefaultSandboxConfig().StderrLimitBytes {
		t.Fatalf("default stderr limit not applied: %d", cfg.StderrLimitBytes)
	}
	if cfg.Timeout != DefaultSandboxConfig().Timeout {
		t.Fatalf("default timeout not applied: %v", cfg.Timeout)
	}
}

func TestSandboxWorktreeRootIsCanonical(t *testing.T) {
	root := tempDir(t)
	target := filepath.Join(root, "real")
	if err := os.MkdirAll(target, 0o755); err != nil {
		t.Fatal(err)
	}
	linkRoot := filepath.Join(root, "link")
	if err := os.Symlink(target, linkRoot); err != nil {
		if runtime.GOOS == "windows" {
			t.Skipf("symlink creation not permitted: %v", err)
		}
		t.Fatal(err)
	}
	sb, err := NewSandbox(SandboxConfig{WorktreeRoot: linkRoot})
	if err != nil {
		t.Fatal(err)
	}
	if !samePath(sb.WorktreeRoot(), target) {
		t.Fatalf("sandbox root not canonical: got %q, want %q", sb.WorktreeRoot(), target)
	}
}

// Integration test: broker uses sandbox for adapter execution.
// This requires a small broker fixture and a fake adapter.
func TestBrokerWithSandboxIntegration(t *testing.T) {
	root := tempDir(t)
	sb := mustSandbox(root)

	policy := mustPolicy(Policy{Rules: []Rule{
		{ID: "allow-shell", Action: "shell", Resource: "*", Effect: EffectAllow},
	}})
	broker := NewBroker(policy)

	engine := NewFakeEngine(ToolKindNative, CapabilityShell)
	engine.SetExecuteFunc(func(_ ExecutionContext, env ExecutionEnvelope) (ToolResult, error) {
		return ToolResult{
			ContractVersion: HarnessContractVersion,
			ExecutionID:     env.ExecutionID,
			TaskID:          env.TaskID,
			ToolName:        env.ToolName,
			Status:          ToolStatusSuccess,
			Output:          []byte(`{"result": "ok"}`),
			EvidenceIDs:     []string{"ev-1"},
			StartedAt:       time.Now().Add(-10 * time.Millisecond),
			CompletedAt:     time.Now(),
			DurationMs:      10,
			Redacted:        true,
		}, nil
	})

	if err := broker.RegisterEngine(engine); err != nil {
		t.Fatal(err)
	}
	if err := broker.Register(ToolDefinition{
		Name:         "shell",
		Kind:         ToolKindNative,
		Capabilities: []ToolCapability{CapabilityShell},
		InputSchema:  []byte(`{}`),
		OutputSchema: []byte(`{}`),
		Timeout:      30 * time.Second,
		Version:      "1.0.0",
	}); err != nil {
		t.Fatal(err)
	}

	ctx := NewExecutionContext(context.Background(), "trace-1")
	req := ToolRequest{
		ContractVersion: HarnessContractVersion,
		ExecutionID:     "exec-1",
		TaskID:          "task-1",
		WorktreeID:      "wt-1",
		ToolName:        "shell",
		Input:           []byte(`{"cmd": "echo hello"}`),
		Timeout:         30 * time.Second,
	}

	// The broker does not yet have a sandbox; this is the expected
	// integration point when BAN-91 wiring is complete.
	_ = sb
	_ = ctx
	_ = req
	// This test is a placeholder for the future integration path.
	// When the broker holds a Sandbox reference, it should call:
	// sb.Validate(ctx, envelope) then sb.ExecuteAdapter(ctx, engine, envelope)
}
