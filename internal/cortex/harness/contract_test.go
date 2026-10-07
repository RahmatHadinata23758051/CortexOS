package harness

import (
	"context"
	"encoding/json"
	"testing"
	"time"
)

func TestHarnessContractVersion(t *testing.T) {
	if HarnessContractVersion != "cortexos.harness.v1" {
		t.Fatalf("contract version changed: %s", HarnessContractVersion)
	}
}

func TestToolDefinitionValidate(t *testing.T) {
	tests := []struct {
		name    string
		def     ToolDefinition
		wantErr bool
	}{
		{
			name: "valid native tool",
			def: ToolDefinition{
				Name:         "shell",
				Kind:         ToolKindNative,
				Description:  "Run a shell command",
				Capabilities: []ToolCapability{CapabilityShell},
				InputSchema:  json.RawMessage(`{"type":"object"}`),
				OutputSchema: json.RawMessage(`{"type":"object"}`),
				Timeout:      30 * time.Second,
				Version:      "1.0.0",
			},
			wantErr: false,
		},
		{
			name: "valid pi tool",
			def: ToolDefinition{
				Name:         "code",
				Kind:         ToolKindPi,
				Description:  "Pi coding agent",
				Capabilities: []ToolCapability{CapabilityFileRead, CapabilityFileWrite, CapabilityCodeNavigation},
				InputSchema:  json.RawMessage(`{"type":"object"}`),
				OutputSchema: json.RawMessage(`{"type":"object"}`),
				Timeout:      120 * time.Second,
				Version:      "1.0.0",
			},
			wantErr: false,
		},
		{
			name: "missing name",
			def: ToolDefinition{
				Kind:    ToolKindNative,
				Timeout: 10 * time.Second,
				Version: "1.0.0",
			},
			wantErr: true,
		},
		{
			name: "invalid kind",
			def: ToolDefinition{
				Name:    "test",
				Kind:    "invalid",
				Timeout: 10 * time.Second,
				Version: "1.0.0",
			},
			wantErr: true,
		},
		{
			name: "zero timeout",
			def: ToolDefinition{
				Name:    "test",
				Kind:    ToolKindNative,
				Timeout: 0,
				Version: "1.0.0",
			},
			wantErr: true,
		},
		{
			name: "missing version",
			def: ToolDefinition{
				Name:    "test",
				Kind:    ToolKindNative,
				Timeout: 10 * time.Second,
			},
			wantErr: true,
		},
		{
			name: "missing capabilities",
			def: ToolDefinition{
				Name:    "test",
				Kind:    ToolKindNative,
				Timeout: 10 * time.Second,
				Version: "1.0.0",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.def.Validate()
			if (err != nil) != tt.wantErr {
				t.Fatalf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestToolRequestValidate(t *testing.T) {
	validInput := json.RawMessage(`{"cmd": "echo hello"}`)

	tests := []struct {
		name    string
		req     ToolRequest
		wantErr bool
	}{
		{
			name: "valid request",
			req: ToolRequest{
				ContractVersion: HarnessContractVersion,
				ExecutionID:     "exec-123",
				TaskID:          "task-456",
				WorktreeID:      "wt-789",
				ToolName:        "shell",
				Input:           validInput,
				Timeout:         30 * time.Second,
				TraceID:         "trace-abc",
			},
			wantErr: false,
		},
		{
			name: "wrong contract version",
			req: ToolRequest{
				ContractVersion: "cortexos.harness.v2",
				ExecutionID:     "exec-123",
				TaskID:          "task-456",
				WorktreeID:      "wt-789",
				ToolName:        "shell",
				Input:           validInput,
				Timeout:         30 * time.Second,
			},
			wantErr: true,
		},
		{
			name: "missing execution ID",
			req: ToolRequest{
				ContractVersion: HarnessContractVersion,
				TaskID:          "task-456",
				WorktreeID:      "wt-789",
				ToolName:        "shell",
				Input:           validInput,
				Timeout:         30 * time.Second,
			},
			wantErr: true,
		},
		{
			name: "missing task ID",
			req: ToolRequest{
				ContractVersion: HarnessContractVersion,
				ExecutionID:     "exec-123",
				WorktreeID:      "wt-789",
				ToolName:        "shell",
				Input:           validInput,
				Timeout:         30 * time.Second,
			},
			wantErr: true,
		},
		{
			name: "missing worktree ID",
			req: ToolRequest{
				ContractVersion: HarnessContractVersion,
				ExecutionID:     "exec-123",
				TaskID:          "task-456",
				ToolName:        "shell",
				Input:           validInput,
				Timeout:         30 * time.Second,
			},
			wantErr: true,
		},
		{
			name: "missing tool name",
			req: ToolRequest{
				ContractVersion: HarnessContractVersion,
				ExecutionID:     "exec-123",
				TaskID:          "task-456",
				WorktreeID:      "wt-789",
				Input:           validInput,
				Timeout:         30 * time.Second,
			},
			wantErr: true,
		},
		{
			name: "zero timeout",
			req: ToolRequest{
				ContractVersion: HarnessContractVersion,
				ExecutionID:     "exec-123",
				TaskID:          "task-456",
				WorktreeID:      "wt-789",
				ToolName:        "shell",
				Input:           validInput,
				Timeout:         0,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.req.Validate()
			if (err != nil) != tt.wantErr {
				t.Fatalf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestExecutionContext(t *testing.T) {
	ctx := NewExecutionContext(context.Background(), "trace-123")

	if ctx.TraceID() != "trace-123" {
		t.Fatalf("TraceID() = %q, want %q", ctx.TraceID(), "trace-123")
	}

	select {
	case <-ctx.Done():
		t.Fatal("context should not be done")
	default:
	}

	ctx2 := NewExecutionContext(context.WithValue(context.Background(), "key", "val"), "trace-456")
	if ctx2.TraceID() != "trace-456" {
		t.Fatalf("TraceID() = %q, want %q", ctx2.TraceID(), "trace-456")
	}
}

func TestToolErrorIsRetriable(t *testing.T) {
	tests := []struct {
		code     string
		expected bool
	}{
		{"harness.timeout", true},
		{"harness.canceled", true},
		{"harness.sandbox_unavailable", true},
		{"harness.policy_denied", false},
		{"harness.policy_approval_required", false},
		{"harness.worktree_violation", false},
		{"harness.adapter_failure", false},
		{"harness.tool_not_found", false},
		{"harness.duplicate_tool", false},
		{"harness.invalid_contract", false},
	}

	for _, tt := range tests {
		t.Run(tt.code, func(t *testing.T) {
			err := &ToolError{Code: tt.code, Message: "test"}
			if err.IsRetriable() != tt.expected {
				t.Fatalf("IsRetriable() = %v, want %v", err.IsRetriable(), tt.expected)
			}
		})
	}
}

func TestToolErrorError(t *testing.T) {
	err := &ToolError{Code: "harness.test", Message: "test message", Details: "detail"}
	if err.Error() != "harness.test: test message (detail)" {
		t.Fatalf("Error() = %q", err.Error())
	}

	err2 := &ToolError{Code: "harness.test", Message: "test message"}
	if err2.Error() != "harness.test: test message" {
		t.Fatalf("Error() = %q", err2.Error())
	}
}

func TestExecutionEnvelope(t *testing.T) {
	env := ExecutionEnvelope{
		ContractVersion: HarnessContractVersion,
		ExecutionID:     "exec-1",
		TaskID:          "task-1",
		WorktreeID:      "wt-1",
		ProjectID:       "proj-1",
		WorktreeRoot:    "/worktrees/proj-1/wt-1",
		ToolName:        "shell",
		Input:           json.RawMessage(`{"cmd": "ls"}`),
		PolicyDecision: PolicyDecision{
			Allowed: true,
			Effect:  EffectAllow,
			Reason:  "test",
		},
		Timeout: 30 * time.Second,
		TraceID: "trace-1",
	}

	data, err := json.Marshal(env)
	if err != nil {
		t.Fatal(err)
	}

	var decoded ExecutionEnvelope
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}

	if decoded.ExecutionID != env.ExecutionID {
		t.Fatalf("ExecutionID mismatch: %s != %s", decoded.ExecutionID, env.ExecutionID)
	}
	if decoded.PolicyDecision.Effect != env.PolicyDecision.Effect {
		t.Fatalf("PolicyDecision.Effect mismatch")
	}
}

func TestToolResultContract(t *testing.T) {
	evidenceID := "ev_cmd_abc123"
	result := ToolResult{
		ContractVersion: HarnessContractVersion,
		ExecutionID:     "exec-1",
		TaskID:          "task-1",
		ToolName:        "shell",
		Status:          ToolStatusSuccess,
		Output:          json.RawMessage(`{"stdout":"hello\n"}`),
		EvidenceIDs:     []string{evidenceID},
		StartedAt:       time.Now().Add(-100 * time.Millisecond),
		CompletedAt:     time.Now(),
		DurationMs:      100,
		Redacted:        true,
	}

	data, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}

	var decoded ToolResult
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}

	if decoded.ContractVersion != HarnessContractVersion {
		t.Fatalf("ContractVersion changed: %s", decoded.ContractVersion)
	}
	if !decoded.Redacted {
		t.Fatal("result should be marked redacted")
	}
}

func TestPolicyDecision(t *testing.T) {
	decision := PolicyDecision{
		Allowed:     true,
		RuleID:      "rule-1",
		Effect:      EffectAllow,
		Reason:      "matched rule",
		RequiresAsk: false,
	}

	data, err := json.Marshal(decision)
	if err != nil {
		t.Fatal(err)
	}

	var decoded PolicyDecision
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}

	if decoded.Effect != EffectAllow {
		t.Fatalf("Effect = %q, want %q", decoded.Effect, EffectAllow)
	}
	if decoded.RequiresAsk {
		t.Fatal("RequiresAsk should be false for Allow")
	}
}

func TestAdapterDescriptor(t *testing.T) {
	desc := AdapterDescriptor{
		Name:          "native",
		Kind:          ToolKindNative,
		Version:       "1.0.0",
		Capabilities:  []ToolCapability{CapabilityShell, CapabilityFileRead},
		SchemaVersion: HarnessContractVersion,
	}

	data, err := json.Marshal(desc)
	if err != nil {
		t.Fatal(err)
	}

	var decoded AdapterDescriptor
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}

	if len(decoded.Capabilities) != 2 {
		t.Fatalf("Capabilities count = %d, want 2", len(decoded.Capabilities))
	}
}

func TestCapabilityString(t *testing.T) {
	if CapabilityShell.String() != "shell" {
		t.Fatalf("CapabilityShell.String() = %q, want %q", CapabilityShell.String(), "shell")
	}
}

func TestPolicyRequest(t *testing.T) {
	req := PolicyRequest{
		Action:   "shell",
		Resource: "wt-123",
		ToolName: "shell",
	}

	if req.Action != "shell" {
		t.Fatal("Action not set")
	}
	if req.Resource != "wt-123" {
		t.Fatal("Resource not set")
	}
	if req.ToolName != "shell" {
		t.Fatal("ToolName not set")
	}
}

func TestIsValidKind(t *testing.T) {
	if !isValidKind(ToolKindNative) {
		t.Fatal("native should be valid")
	}
	if !isValidKind(ToolKindPi) {
		t.Fatal("pi should be valid")
	}
	if !isValidKind(ToolKindOMP) {
		t.Fatal("omp should be valid")
	}
	if isValidKind("invalid") {
		t.Fatal("invalid should not be valid")
	}
}

func TestContextWithTrace(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	wrapped := NewExecutionContext(ctx, "trace-xyz")
	if wrapped.TraceID() != "trace-xyz" {
		t.Fatal("TraceID not preserved")
	}
	select {
	case <-wrapped.Done():
		// Expected - parent was canceled
	default:
		t.Fatal("Done() should be closed")
	}
}

func TestErrorFor(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		wantCode ErrorCode
	}{
		{"context canceled", context.Canceled, ErrorCodeCanceled},
		{"context deadline", context.DeadlineExceeded, ErrorCodeTimeout},
		{"policy denied", ErrPolicyDenied, ErrorCodePolicyDenied},
		{"policy ask", ErrPolicyAsk, ErrorCodePolicyAsk},
		{"worktree violation", ErrWorktreeViolation, ErrorCodeWorktreeViolation},
		{"sandbox error", ErrSandbox, ErrorCodeSandbox},
		{"tool not found", ErrToolNotFound, ErrorCodeToolNotFound},
		{"duplicate tool", ErrDuplicateTool, ErrorCodeDuplicateTool},
		{"invalid contract", ErrInvalidContract, ErrorCodeInvalidContract},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			coded := ErrorFor(tt.err)
			if coded.Code != tt.wantCode {
				t.Fatalf("Code = %v, want %v", coded.Code, tt.wantCode)
			}
		})
	}
}

func TestErrorForWrapsExisting(t *testing.T) {
	existing := &CodedError{Code: ErrorCodeTimeout, Message: "wrapped"}
	wrapped := ErrorFor(existing)
	if wrapped.Code != ErrorCodeTimeout {
		t.Fatalf("wrapped error code changed: %v", wrapped.Code)
	}
}

func TestErrorForNil(t *testing.T) {
	if ErrorFor(nil) != nil {
		t.Fatal("ErrorFor(nil) should return nil")
	}
}

func TestBrokerExecuteNilContext(t *testing.T) {
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

	req := ToolRequest{
		ContractVersion: HarnessContractVersion,
		ExecutionID:     "exec-1",
		TaskID:          "task-1",
		WorktreeID:      "wt-1",
		ToolName:        "shell",
		Input:           json.RawMessage(`{}`),
		Timeout:         30 * time.Second,
	}

	result, err := broker.Execute(nil, req)
	if err != nil {
		t.Fatalf("Execute with nil context should not error: %v", err)
	}
	if result.Status != ToolStatusSuccess {
		t.Fatalf("status = %v, want success", result.Status)
	}
}

func TestBrokerExecuteMissingPolicyEngine(t *testing.T) {
	broker := NewBroker(nil)

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

	req := ToolRequest{
		ContractVersion: HarnessContractVersion,
		ExecutionID:     "exec-1",
		TaskID:          "task-1",
		WorktreeID:      "wt-1",
		ToolName:        "shell",
		Input:           json.RawMessage(`{}`),
		Timeout:         30 * time.Second,
	}

	_, err := broker.Execute(NewExecutionContext(context.Background(), "trace-1"), req)
	if err == nil {
		t.Fatal("expected error for missing policy engine")
	}

	coded := ErrorFor(err)
	if coded.Code != ErrorCodePolicyDenied {
		t.Fatalf("code = %v, want %v", coded.Code, ErrorCodePolicyDenied)
	}
}
