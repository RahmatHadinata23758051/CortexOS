package harness

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"
)

func mustPolicy(policy Policy) *StaticPolicy {
	p, err := NewStaticPolicy(policy)
	if err != nil {
		panic(err)
	}
	return p
}

func TestBrokerRegisterAndGet(t *testing.T) {
	policy := mustPolicy(Policy{Rules: []Rule{
		{ID: "r1", Action: "shell", Resource: "*", Effect: EffectAllow},
	}})
	broker := NewBroker(policy)

	def := ToolDefinition{
		Name:         "shell",
		Kind:         ToolKindNative,
		Description:  "Run shell command",
		Capabilities: []ToolCapability{CapabilityShell},
		InputSchema:  json.RawMessage(`{"type":"object"}`),
		OutputSchema: json.RawMessage(`{"type":"object"}`),
		Timeout:      30 * time.Second,
		Version:      "1.0.0",
	}

	if err := broker.Register(def); err != nil {
		t.Fatalf("Register: %v", err)
	}

	got, ok := broker.Get("shell")
	if !ok {
		t.Fatal("tool not found after register")
	}
	if got.Name != "shell" {
		t.Fatalf("name = %q, want %q", got.Name, "shell")
	}
	if got.Kind != ToolKindNative {
		t.Fatalf("kind = %v, want %v", got.Kind, ToolKindNative)
	}
}

func TestBrokerDuplicateRegister(t *testing.T) {
	policy := mustPolicy(Policy{Rules: []Rule{
		{ID: "r1", Action: "shell", Resource: "*", Effect: EffectAllow},
	}})
	broker := NewBroker(policy)

	def := ToolDefinition{
		Name:         "shell",
		Kind:         ToolKindNative,
		Capabilities: []ToolCapability{CapabilityShell},
		InputSchema:  json.RawMessage(`{}`),
		OutputSchema: json.RawMessage(`{}`),
		Timeout:      30 * time.Second,
		Version:      "1.0.0",
	}

	if err := broker.Register(def); err != nil {
		t.Fatal(err)
	}

	err := broker.Register(def)
	if !errors.Is(err, ErrDuplicateTool) {
		t.Fatalf("expected duplicate error, got %v", err)
	}
}

func TestBrokerUnregister(t *testing.T) {
	policy := mustPolicy(Policy{Rules: []Rule{
		{ID: "r1", Action: "shell", Resource: "*", Effect: EffectAllow},
	}})
	broker := NewBroker(policy)

	def := ToolDefinition{
		Name:         "shell",
		Kind:         ToolKindNative,
		Capabilities: []ToolCapability{CapabilityShell},
		InputSchema:  json.RawMessage(`{}`),
		OutputSchema: json.RawMessage(`{}`),
		Timeout:      30 * time.Second,
		Version:      "1.0.0",
	}

	if err := broker.Register(def); err != nil {
		t.Fatal(err)
	}

	if err := broker.Unregister("shell"); err != nil {
		t.Fatal(err)
	}

	if _, ok := broker.Get("shell"); ok {
		t.Fatal("tool should be unregistered")
	}
}

func TestBrokerUnregisterNotFound(t *testing.T) {
	policy := mustPolicy(Policy{Rules: []Rule{
		{ID: "r1", Action: "shell", Resource: "*", Effect: EffectAllow},
	}})
	broker := NewBroker(policy)

	err := broker.Unregister("nonexistent")
	if !errors.Is(err, ErrToolNotFound) {
		t.Fatalf("expected tool not found, got %v", err)
	}
}

func TestBrokerList(t *testing.T) {
	policy := mustPolicy(Policy{Rules: []Rule{
		{ID: "r1", Action: "shell", Resource: "*", Effect: EffectAllow},
	}})
	broker := NewBroker(policy)

	for _, name := range []string{"shell", "read", "write"} {
		if err := broker.Register(ToolDefinition{
			Name:         name,
			Kind:         ToolKindNative,
			Capabilities: []ToolCapability{CapabilityShell},
			InputSchema:  json.RawMessage(`{}`),
			OutputSchema: json.RawMessage(`{}`),
			Timeout:      30 * time.Second,
			Version:      "1.0.0",
		}); err != nil {
			t.Fatal(err)
		}
	}

	list := broker.List()
	if len(list) != 3 {
		t.Fatalf("list length = %d, want 3", len(list))
	}
}

func TestBrokerRegisterEngine(t *testing.T) {
	policy := mustPolicy(Policy{Rules: []Rule{
		{ID: "r1", Action: "shell", Resource: "*", Effect: EffectAllow},
	}})
	broker := NewBroker(policy)

	engine := NewFakeEngine(ToolKindNative, CapabilityShell)
	if err := broker.RegisterEngine(engine); err != nil {
		t.Fatal(err)
	}

	got, ok := broker.Engine(ToolKindNative)
	if !ok {
		t.Fatal("engine not found")
	}
	if got.Kind() != ToolKindNative {
		t.Fatalf("kind = %v, want %v", got.Kind(), ToolKindNative)
	}
}

func TestBrokerDuplicateEngine(t *testing.T) {
	policy := mustPolicy(Policy{Rules: []Rule{
		{ID: "r1", Action: "shell", Resource: "*", Effect: EffectAllow},
	}})
	broker := NewBroker(policy)

	if err := broker.RegisterEngine(NewFakeEngine(ToolKindNative)); err != nil {
		t.Fatal(err)
	}

	err := broker.RegisterEngine(NewFakeEngine(ToolKindNative))
	if err == nil {
		t.Fatal("expected error for duplicate engine")
	}
}

func TestBrokerExecuteSuccess(t *testing.T) {
	policy := mustPolicy(Policy{Rules: []Rule{
		{ID: "r1", Action: "shell", Resource: "*", Effect: EffectAllow},
	}})
	broker := NewBroker(policy)

	engine := NewFakeEngine(ToolKindNative, CapabilityShell)
	engine.SetExecuteFunc(func(ctx ExecutionContext, env ExecutionEnvelope) (ToolResult, error) {
		return ToolResult{
			ContractVersion: HarnessContractVersion,
			ExecutionID:     env.ExecutionID,
			TaskID:          env.TaskID,
			ToolName:        env.ToolName,
			Status:          ToolStatusSuccess,
			Output:          json.RawMessage(`{"result": "ok"}`),
			EvidenceIDs:     []string{"ev-123"},
			StartedAt:       time.Now().Add(-50 * time.Millisecond),
			CompletedAt:     time.Now(),
			DurationMs:      50,
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
		InputSchema:  json.RawMessage(`{}`),
		OutputSchema: json.RawMessage(`{}`),
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
		Input:           json.RawMessage(`{"cmd": "echo hello"}`),
		Timeout:         30 * time.Second,
	}

	result, err := broker.Execute(ctx, req)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}

	if result.Status != ToolStatusSuccess {
		t.Fatalf("status = %v, want success", result.Status)
	}
	if result.ExecutionID != "exec-1" {
		t.Fatalf("executionID = %q", result.ExecutionID)
	}
	if len(result.EvidenceIDs) != 1 {
		t.Fatalf("evidence count = %d", len(result.EvidenceIDs))
	}
	if !result.Redacted {
		t.Fatal("result should be redacted")
	}
}

func TestBrokerExecutePolicyDenied(t *testing.T) {
	policy := mustPolicy(Policy{Rules: []Rule{
		{ID: "r1", Action: "shell", Resource: "*", Effect: EffectDeny},
	}})
	broker := NewBroker(policy)

	if err := broker.Register(ToolDefinition{
		Name:         "shell",
		Kind:         ToolKindNative,
		Capabilities: []ToolCapability{CapabilityShell},
		InputSchema:  json.RawMessage(`{}`),
		OutputSchema: json.RawMessage(`{}`),
		Timeout:      30 * time.Second,
		Version:      "1.0.0",
	}); err != nil {
		t.Fatal(err)
	}

	if err := broker.RegisterEngine(NewFakeEngine(ToolKindNative, CapabilityShell)); err != nil {
		t.Fatal(err)
	}

	ctx := NewExecutionContext(context.Background(), "trace-1")
	req := ToolRequest{
		ContractVersion: HarnessContractVersion,
		ExecutionID:     "exec-1",
		TaskID:          "task-1",
		WorktreeID:      "wt-1",
		ToolName:        "shell",
		Input:           json.RawMessage(`{"cmd": "echo hello"}`),
		Timeout:         30 * time.Second,
	}

	_, err := broker.Execute(ctx, req)
	if err == nil {
		t.Fatal("expected error for denied policy")
	}

	coded := ErrorFor(err)
	if coded.Code != ErrorCodePolicyDenied {
		t.Fatalf("code = %v, want %v", coded.Code, ErrorCodePolicyDenied)
	}
}

func TestBrokerExecutePolicyAsk(t *testing.T) {
	policy := mustPolicy(Policy{Rules: []Rule{
		{ID: "r1", Action: "shell", Resource: "*", Effect: EffectAsk},
	}})
	broker := NewBroker(policy)

	if err := broker.Register(ToolDefinition{
		Name:         "shell",
		Kind:         ToolKindNative,
		Capabilities: []ToolCapability{CapabilityShell},
		InputSchema:  json.RawMessage(`{}`),
		OutputSchema: json.RawMessage(`{}`),
		Timeout:      30 * time.Second,
		Version:      "1.0.0",
	}); err != nil {
		t.Fatal(err)
	}

	if err := broker.RegisterEngine(NewFakeEngine(ToolKindNative, CapabilityShell)); err != nil {
		t.Fatal(err)
	}

	ctx := NewExecutionContext(context.Background(), "trace-1")
	req := ToolRequest{
		ContractVersion: HarnessContractVersion,
		ExecutionID:     "exec-1",
		TaskID:          "task-1",
		WorktreeID:      "wt-1",
		ToolName:        "shell",
		Input:           json.RawMessage(`{"cmd": "echo hello"}`),
		Timeout:         30 * time.Second,
	}

	_, err := broker.Execute(ctx, req)
	if err == nil {
		t.Fatal("expected error for ask policy")
	}

	coded := ErrorFor(err)
	if coded.Code != ErrorCodePolicyAsk {
		t.Fatalf("code = %v, want %v", coded.Code, ErrorCodePolicyAsk)
	}
}

func TestBrokerExecuteToolNotFound(t *testing.T) {
	policy := mustPolicy(Policy{Rules: []Rule{
		{ID: "r1", Action: "shell", Resource: "*", Effect: EffectAllow},
	}})
	broker := NewBroker(policy)

	if err := broker.RegisterEngine(NewFakeEngine(ToolKindNative, CapabilityShell)); err != nil {
		t.Fatal(err)
	}

	ctx := NewExecutionContext(context.Background(), "trace-1")
	req := ToolRequest{
		ContractVersion: HarnessContractVersion,
		ExecutionID:     "exec-1",
		TaskID:          "task-1",
		WorktreeID:      "wt-1",
		ToolName:        "nonexistent",
		Input:           json.RawMessage(`{}`),
		Timeout:         30 * time.Second,
	}

	_, err := broker.Execute(ctx, req)
	if err == nil {
		t.Fatal("expected error for missing tool")
	}

	coded := ErrorFor(err)
	if coded.Code != ErrorCodeToolNotFound {
		t.Fatalf("code = %v, want %v", coded.Code, ErrorCodeToolNotFound)
	}
}

func TestBrokerExecuteNoEngine(t *testing.T) {
	policy := mustPolicy(Policy{Rules: []Rule{
		{ID: "r1", Action: "file_read", Resource: "*", Effect: EffectAllow},
	}})
	broker := NewBroker(policy)

	if err := broker.Register(ToolDefinition{
		Name:         "code",
		Kind:         ToolKindPi,
		Capabilities: []ToolCapability{CapabilityFileRead, CapabilityFileWrite},
		InputSchema:  json.RawMessage(`{}`),
		OutputSchema: json.RawMessage(`{}`),
		Timeout:      120 * time.Second,
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
		ToolName:        "code",
		Input:           json.RawMessage(`{}`),
		Timeout:         120 * time.Second,
	}

	_, err := broker.Execute(ctx, req)
	if err == nil {
		t.Fatal("expected error for missing engine")
	}

	coded := ErrorFor(err)
	if coded.Code != ErrorCodeAdapterFailure {
		t.Fatalf("code = %v, want %v", coded.Code, ErrorCodeAdapterFailure)
	}
}

func TestBrokerExecuteTimeout(t *testing.T) {
	policy := mustPolicy(Policy{Rules: []Rule{
		{ID: "r1", Action: "shell", Resource: "*", Effect: EffectAllow},
	}})
	broker := NewBroker(policy)

	engine := NewFakeEngine(ToolKindNative, CapabilityShell)
	engine.SetExecuteFunc(func(ctx ExecutionContext, env ExecutionEnvelope) (ToolResult, error) {
		select {
		case <-ctx.Done():
			return ToolResult{}, ctx.Err()
		case <-time.After(200 * time.Millisecond):
			return ToolResult{}, nil
		}
	})

	if err := broker.RegisterEngine(engine); err != nil {
		t.Fatal(err)
	}

	if err := broker.Register(ToolDefinition{
		Name:         "shell",
		Kind:         ToolKindNative,
		Capabilities: []ToolCapability{CapabilityShell},
		InputSchema:  json.RawMessage(`{}`),
		OutputSchema: json.RawMessage(`{}`),
		Timeout:      30 * time.Second,
		Version:      "1.0.0",
	}); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	req := ToolRequest{
		ContractVersion: HarnessContractVersion,
		ExecutionID:     "exec-1",
		TaskID:          "task-1",
		WorktreeID:      "wt-1",
		ToolName:        "shell",
		Input:           json.RawMessage(`{}`),
		Timeout:         10 * time.Millisecond,
	}

	_, err := broker.Execute(NewExecutionContext(ctx, "trace-1"), req)
	if err == nil {
		t.Fatal("expected timeout error")
	}

	coded := ErrorFor(err)
	if coded.Code != ErrorCodeTimeout {
		t.Fatalf("code = %v, want %v", coded.Code, ErrorCodeTimeout)
	}
	if !coded.Retryable {
		t.Fatal("timeout should be retryable")
	}
}

func TestBrokerExecuteCancellation(t *testing.T) {
	policy := mustPolicy(Policy{Rules: []Rule{
		{ID: "r1", Action: "shell", Resource: "*", Effect: EffectAllow},
	}})
	broker := NewBroker(policy)

	engine := NewFakeEngine(ToolKindNative, CapabilityShell)
	engine.SetExecuteFunc(func(ctx ExecutionContext, env ExecutionEnvelope) (ToolResult, error) {
		select {
		case <-ctx.Done():
			return ToolResult{}, ctx.Err()
		case <-time.After(200 * time.Millisecond):
			return ToolResult{}, nil
		}
	})

	if err := broker.RegisterEngine(engine); err != nil {
		t.Fatal(err)
	}

	if err := broker.Register(ToolDefinition{
		Name:         "shell",
		Kind:         ToolKindNative,
		Capabilities: []ToolCapability{CapabilityShell},
		InputSchema:  json.RawMessage(`{}`),
		OutputSchema: json.RawMessage(`{}`),
		Timeout:      30 * time.Second,
		Version:      "1.0.0",
	}); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	req := ToolRequest{
		ContractVersion: HarnessContractVersion,
		ExecutionID:     "exec-1",
		TaskID:          "task-1",
		WorktreeID:      "wt-1",
		ToolName:        "shell",
		Input:           json.RawMessage(`{}`),
		Timeout:         30 * time.Second,
	}

	_, err := broker.Execute(NewExecutionContext(ctx, "trace-1"), req)
	if err == nil {
		t.Fatal("expected cancellation error")
	}

	coded := ErrorFor(err)
	if coded.Code != ErrorCodeCanceled {
		t.Fatalf("code = %v, want %v", coded.Code, ErrorCodeCanceled)
	}
	if !coded.Retryable {
		t.Fatal("cancellation should be retryable")
	}
}

func TestBrokerExecuteInvalidContractVersion(t *testing.T) {
	policy := mustPolicy(Policy{Rules: []Rule{
		{ID: "r1", Action: "shell", Resource: "*", Effect: EffectAllow},
	}})
	broker := NewBroker(policy)

	if err := broker.Register(ToolDefinition{
		Name:         "shell",
		Kind:         ToolKindNative,
		Capabilities: []ToolCapability{CapabilityShell},
		InputSchema:  json.RawMessage(`{}`),
		OutputSchema: json.RawMessage(`{}`),
		Timeout:      30 * time.Second,
		Version:      "1.0.0",
	}); err != nil {
		t.Fatal(err)
	}

	if err := broker.RegisterEngine(NewFakeEngine(ToolKindNative, CapabilityShell)); err != nil {
		t.Fatal(err)
	}

	ctx := NewExecutionContext(context.Background(), "trace-1")
	req := ToolRequest{
		ContractVersion: "cortexos.harness.v99",
		ExecutionID:     "exec-1",
		TaskID:          "task-1",
		WorktreeID:      "wt-1",
		ToolName:        "shell",
		Input:           json.RawMessage(`{}`),
		Timeout:         30 * time.Second,
	}

	_, err := broker.Execute(ctx, req)
	if err == nil {
		t.Fatal("expected error for invalid contract version")
	}

	coded := ErrorFor(err)
	if coded.Code != ErrorCodeInvalidContract {
		t.Fatalf("code = %v, want %v", coded.Code, ErrorCodeInvalidContract)
	}
}

func TestBrokerPolicyAccess(t *testing.T) {
	policy := mustPolicy(Policy{Rules: []Rule{
		{ID: "r1", Action: "shell", Resource: "*", Effect: EffectAllow},
	}})
	broker := NewBroker(policy)

	p := broker.Policy()
	if p == nil {
		t.Fatal("policy should not be nil")
	}

	current := p.CurrentPolicy()
	if len(current.Rules) != 1 {
		t.Fatalf("policy rules = %d, want 1", len(current.Rules))
	}
}
