package harness

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"
)

func enforcementTool(name string, caps ...ToolCapability) ToolDefinition {
	return ToolDefinition{
		Name:         name,
		Kind:         ToolKindNative,
		Capabilities: caps,
		InputSchema:  json.RawMessage(`{"type":"object"}`),
		OutputSchema: json.RawMessage(`{"type":"object"}`),
		Timeout:      time.Second,
		Version:      "1.0.0",
	}
}

func enforcementRequest(tool, action, execution string) ToolRequest {
	return ToolRequest{
		ContractVersion: HarnessContractVersion,
		ExecutionID:     execution,
		TaskID:          "task-enforcement",
		WorktreeID:      "worktree-enforcement",
		ToolName:        tool,
		Action:          action,
		Input:           json.RawMessage(`{"value":"safe"}`),
		Timeout:         time.Second,
		TraceID:         "trace-enforcement",
	}
}

func TestBrokerPolicyEnforcementPerCapability(t *testing.T) {
	policy := mustPolicy(Policy{Rules: []Rule{
		{ID: "allow-read", Action: "file_read", Resource: "*", Effect: EffectAllow, Priority: 20},
		{ID: "ask-write", Action: "file_write", Resource: "*", Effect: EffectAsk, Priority: 20},
		{ID: "deny-shell", Action: "shell", Resource: "*", Effect: EffectDeny, Priority: 20},
	}})
	broker := NewBroker(policy)
	engine := NewFakeEngine(ToolKindNative, CapabilityFileRead, CapabilityFileWrite, CapabilityShell)
	if err := broker.RegisterEngine(engine); err != nil {
		t.Fatal(err)
	}
	if err := broker.Register(enforcementTool("multi", CapabilityFileRead, CapabilityFileWrite, CapabilityShell)); err != nil {
		t.Fatal(err)
	}

	allowed, err := broker.Execute(nil, enforcementRequest("multi", "file_read", "exec-allow"))
	if err != nil || allowed.Status != ToolStatusSuccess {
		t.Fatalf("allow: result=%#v err=%v", allowed, err)
	}
	ask, err := broker.Execute(nil, enforcementRequest("multi", "file_write", "exec-ask"))
	if !errors.Is(err, ErrPolicyAsk) || ask.Status != ToolStatusPolicyDenied {
		t.Fatalf("ask: result=%#v err=%v", ask, err)
	}
	denied, err := broker.Execute(nil, enforcementRequest("multi", "shell", "exec-deny"))
	if !errors.Is(err, ErrPolicyDenied) || denied.Status != ToolStatusPolicyDenied {
		t.Fatalf("deny: result=%#v err=%v", denied, err)
	}
	if len(engine.Calls()) != 1 {
		t.Fatalf("ask/deny must not reach adapter; calls=%d", len(engine.Calls()))
	}
}

func TestBrokerAskRequiresExplicitApprovalBoundary(t *testing.T) {
	policy := mustPolicy(Policy{Rules: []Rule{{ID: "ask-shell", Action: "shell", Resource: "*", Effect: EffectAsk}}})
	broker := NewBroker(policy)
	engine := NewFakeEngine(ToolKindNative, CapabilityShell)
	if err := broker.RegisterEngine(engine); err != nil {
		t.Fatal(err)
	}
	if err := broker.Register(enforcementTool("shell", CapabilityShell)); err != nil {
		t.Fatal(err)
	}

	req := enforcementRequest("shell", "", "exec-approved")
	ctx := WithApproval(context.Background(), Approval{ExecutionID: req.ExecutionID, ToolName: req.ToolName, Action: "shell", Approver: "reviewer", GrantedAt: time.Now().UTC()})
	result, err := broker.Execute(NewExecutionContext(ctx, req.TraceID), req)
	if err != nil || result.Status != ToolStatusSuccess {
		t.Fatalf("approved ask: result=%#v err=%v", result, err)
	}
	if len(engine.Calls()) != 1 {
		t.Fatal("explicit approval should permit exactly one execution")
	}

	// Approval is single-use; a second request cannot silently inherit it.
	req.ExecutionID = "exec-no-approval"
	result, err = broker.Execute(nil, req)
	if !errors.Is(err, ErrPolicyAsk) || result.Status != ToolStatusPolicyDenied {
		t.Fatalf("approval must not be reusable: result=%#v err=%v", result, err)
	}
}

func TestBrokerRejectsCapabilityBypassAndUnknownRegistration(t *testing.T) {
	policy := mustPolicy(Policy{Rules: []Rule{{ID: "allow-all", Action: "*", Resource: "*", Effect: EffectAllow}}})
	broker := NewBroker(policy)
	if err := broker.Register(enforcementTool("invalid", ToolCapability("unregistered_capability"))); !errors.Is(err, ErrInvalidContract) {
		t.Fatalf("unknown capability registration error=%v", err)
	}
	engine := NewFakeEngine(ToolKindNative, CapabilityFileRead)
	if err := broker.RegisterEngine(engine); err != nil {
		t.Fatal(err)
	}
	if err := broker.Register(enforcementTool("reader", CapabilityFileRead)); err != nil {
		t.Fatal(err)
	}

	for _, action := range []string{"file_write", "unregistered_capability"} {
		result, err := broker.Execute(nil, enforcementRequest("reader", action, "exec-bypass-"+action))
		if err == nil || result.Status != ToolStatusPolicyDenied {
			t.Fatalf("bypass action %q: result=%#v err=%v", action, result, err)
		}
	}
	if len(engine.Calls()) != 0 {
		t.Fatal("capability bypass must not reach adapter")
	}
}

func TestBrokerMalformedAndUnknownRequestsAreFailClosedWithEvidence(t *testing.T) {
	policy := mustPolicy(Policy{Rules: []Rule{{ID: "allow-shell", Action: "shell", Resource: "*", Effect: EffectAllow}}})
	broker := NewBroker(policy)
	if err := broker.Register(enforcementTool("shell", CapabilityShell)); err != nil {
		t.Fatal(err)
	}

	malformed := enforcementRequest("shell", "", "exec-malformed")
	malformed.ContractVersion = "cortexos.harness.invalid"
	result, err := broker.Execute(nil, malformed)
	if !errors.Is(err, ErrInvalidContract) || result.Status != ToolStatusFailed || len(result.EvidenceIDs) != 1 {
		t.Fatalf("malformed request: result=%#v err=%v", result, err)
	}
	unknown := enforcementRequest("missing", "", "exec-unknown")
	result, err = broker.Execute(nil, unknown)
	if !errors.Is(err, ErrToolNotFound) || result.Status != ToolStatusFailed || len(result.EvidenceIDs) != 1 {
		t.Fatalf("unknown tool: result=%#v err=%v", result, err)
	}
}

func TestBrokerAuditEvidenceCompletenessAndRedaction(t *testing.T) {
	policy := mustPolicy(Policy{Rules: []Rule{
		{ID: "allow-shell", Action: "shell", Resource: "*", Effect: EffectAllow},
		{ID: "deny-write", Action: "file_write", Resource: "*", Effect: EffectDeny},
	}})
	broker := NewBroker(policy)
	engine := NewFakeEngine(ToolKindNative, CapabilityShell, CapabilityFileWrite)
	engine.SetExecuteFunc(func(ctx ExecutionContext, env ExecutionEnvelope) (ToolResult, error) {
		return ToolResult{}, errors.New("provider secret-token-12345678 failed")
	})
	if err := broker.RegisterEngine(engine); err != nil {
		t.Fatal(err)
	}
	if err := broker.Register(enforcementTool("shell", CapabilityShell)); err != nil {
		t.Fatal(err)
	}
	if err := broker.Register(enforcementTool("writer", CapabilityFileWrite)); err != nil {
		t.Fatal(err)
	}

	req := enforcementRequest("shell", "", "exec-failed")
	req.Input = json.RawMessage(`{"token":"secret-token-12345678"}`)
	if _, err := broker.Execute(nil, req); err == nil {
		t.Fatal("expected adapter failure")
	}
	if _, err := broker.Execute(nil, enforcementRequest("writer", "", "exec-denied-audit")); !errors.Is(err, ErrPolicyDenied) {
		t.Fatal("expected policy denial")
	}

	events := broker.AuditEvents()
	if len(events) < 3 {
		t.Fatalf("audit event count=%d, want policy and outcome evidence", len(events))
	}
	seenRule := false
	for _, event := range events {
		if event.ID == "" || event.Digest == "" || event.ContractVersion != HarnessContractVersion || event.ExecutionID == "" || event.TaskID == "" || event.WorktreeID == "" || event.SchemaVersion == "" || event.CollectedAt.IsZero() {
			t.Fatalf("incomplete immutable audit event: %#v", event)
		}
		payload := string(event.RedactedPayload)
		if strings.Contains(payload, "secret-token-12345678") {
			t.Fatalf("audit leaked secret: %s", payload)
		}
		if strings.Contains(payload, "allow-shell") {
			seenRule = true
		}
	}
	if !seenRule {
		t.Fatal("audit evidence must include deterministic matched-rule metadata")
	}
}

func TestBrokerCancellationProducesAuditEvidence(t *testing.T) {
	policy := mustPolicy(Policy{Rules: []Rule{{ID: "allow-shell", Action: "shell", Resource: "*", Effect: EffectAllow}}})
	broker := NewBroker(policy)
	engine := NewFakeEngine(ToolKindNative, CapabilityShell)
	engine.SetExecuteFunc(func(ctx ExecutionContext, env ExecutionEnvelope) (ToolResult, error) {
		<-ctx.Done()
		return ToolResult{}, ctx.Err()
	})
	if err := broker.RegisterEngine(engine); err != nil {
		t.Fatal(err)
	}
	if err := broker.Register(enforcementTool("shell", CapabilityShell)); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	result, err := broker.Execute(NewExecutionContext(ctx, "trace-cancel"), enforcementRequest("shell", "", "exec-canceled"))
	if !errors.Is(err, context.Canceled) || result.Status != ToolStatusCanceled {
		t.Fatalf("canceled execution: result=%#v err=%v", result, err)
	}
	found := false
	for _, event := range broker.AuditEventsForExecution("exec-canceled") {
		if strings.Contains(string(event.RedactedPayload), `"event":"canceled"`) {
			found = true
		}
	}
	if !found {
		t.Fatal("canceled execution must produce canceled audit evidence")
	}
}

func TestBrokerCapabilityRegistrationBindingAndDefaults(t *testing.T) {
	// Policy has no rules, so decision.RuleID is empty (default deny)
	policy := mustPolicy(Policy{})
	broker := NewBroker(policy)

	if _, ok := broker.Capability(CapabilityShell); !ok {
		t.Fatal("default capabilities must be registered on broker creation")
	}
	if len(broker.ListCapabilities()) < len(DefaultCapabilities()) {
		t.Fatal("ListCapabilities must include all default bindings")
	}

	err := broker.RegisterCapability(CapabilityBinding{Capability: ToolCapability("custom_cap"), Description: "custom", RequiresAsk: true})
	if err != nil {
		t.Fatalf("RegisterCapability failed: %v", err)
	}
	if binding, ok := broker.Capability(ToolCapability("custom_cap")); !ok || binding.RequiresAsk != true {
		t.Fatal("custom capability binding not persisted")
	}
	if err := broker.RegisterCapability(CapabilityBinding{Capability: "", Description: "invalid"}); !errors.Is(err, ErrInvalidContract) {
		t.Fatal("empty capability name must be rejected")
	}

	engine := NewFakeEngine(ToolKindNative, CapabilityShell)
	if err := broker.RegisterEngine(engine); err != nil {
		t.Fatal(err)
	}

	def := ToolDefinition{
		Name:         "shell",
		Kind:         ToolKindNative,
		Capabilities: []ToolCapability{CapabilityShell},
		InputSchema:  json.RawMessage(`{}`),
		OutputSchema: json.RawMessage(`{}`),
		Timeout:      time.Second,
		Version:      "1.0.0",
		RequiresAsk:  false,
	}
	if err := broker.Register(def); err != nil {
		t.Fatalf("tool registration failed: %v", err)
	}

	req := ToolRequest{
		ContractVersion: HarnessContractVersion,
		ExecutionID:     "exec-binding",
		TaskID:          "task-binding",
		WorktreeID:      "wt-binding",
		ToolName:        "shell",
		Action:          "",
		Input:           json.RawMessage(`{}`),
		Timeout:         time.Second,
	}

	// Without binding default, empty policy denies execution
	result, err := broker.Execute(nil, req)
	if err == nil || result.Status != ToolStatusPolicyDenied {
		t.Fatalf("empty policy should deny: result=%#v err=%v", result, err)
	}

	// Set capability binding to allow by default
	_ = broker.RegisterCapability(CapabilityBinding{Capability: CapabilityShell, DefaultEffect: EffectAllow})
	result, err = broker.Execute(nil, req)
	if err != nil || result.Status != ToolStatusSuccess {
		t.Fatalf("default binding allow should execute: result=%#v err=%v", result, err)
	}

	// Now set capability binding to deny by default
	_ = broker.RegisterCapability(CapabilityBinding{Capability: CapabilityShell, DefaultEffect: EffectDeny})
	result, err = broker.Execute(nil, req)
	if err == nil || result.Status != ToolStatusPolicyDenied {
		t.Fatalf("default binding deny should be enforced: result=%#v err=%v", result, err)
	}
}

func TestBrokerAskCannotOverrideDeny(t *testing.T) {
	policy := mustPolicy(Policy{Rules: []Rule{
		{ID: "deny-shell", Action: "shell", Resource: "*", Effect: EffectDeny, Priority: 10},
		{ID: "ask-shell", Action: "shell", Resource: "worktree-*", Effect: EffectAsk, Priority: 5},
	}})
	broker := NewBroker(policy)
	engine := NewFakeEngine(ToolKindNative, CapabilityShell)
	if err := broker.RegisterEngine(engine); err != nil {
		t.Fatal(err)
	}
	if err := broker.Register(enforcementTool("shell", CapabilityShell)); err != nil {
		t.Fatal(err)
	}

	approval := Approval{ExecutionID: "exec-deny-override", ToolName: "shell", Action: "shell", Approver: "admin", GrantedAt: time.Now().UTC()}
	ctx := WithApproval(context.Background(), approval)
	result, err := broker.Execute(NewExecutionContext(ctx, "trace"), enforcementRequest("shell", "", "exec-deny-override"))
	if !errors.Is(err, ErrPolicyDenied) || result.Status != ToolStatusPolicyDenied {
		t.Fatalf("deny must override approval: result=%#v err=%v", result, err)
	}
	if len(engine.Calls()) != 0 {
		t.Fatal("deny must prevent adapter execution despite approval")
	}
}

func TestBrokerApprovalRegistrationOnBroker(t *testing.T) {
	policy := mustPolicy(Policy{Rules: []Rule{{ID: "ask-shell", Action: "shell", Resource: "*", Effect: EffectAsk}}})
	broker := NewBroker(policy)
	engine := NewFakeEngine(ToolKindNative, CapabilityShell)
	if err := broker.RegisterEngine(engine); err != nil {
		t.Fatal(err)
	}
	if err := broker.Register(enforcementTool("shell", CapabilityShell)); err != nil {
		t.Fatal(err)
	}

	// Validation on broker.Approve
	if err := broker.Approve(Approval{ExecutionID: "", Approver: "admin"}); !errors.Is(err, ErrInvalidContract) {
		t.Fatal("approval without execution ID must be rejected")
	}
	if err := broker.Approve(Approval{ExecutionID: "exec-1", Approver: ""}); !errors.Is(err, ErrInvalidContract) {
		t.Fatal("approval without approver must be rejected")
	}

	// Register approval on broker directly
	req := enforcementRequest("shell", "shell", "exec-broker-approved")
	if err := broker.Approve(Approval{
		ExecutionID: req.ExecutionID,
		ToolName:    req.ToolName,
		Action:      "shell",
		Approver:    "sec-lead",
		Reason:      "approved for build",
	}); err != nil {
		t.Fatalf("Approve failed: %v", err)
	}

	result, err := broker.Execute(nil, req)
	if err != nil || result.Status != ToolStatusSuccess {
		t.Fatalf("broker-approved execution should succeed: result=%#v err=%v", result, err)
	}

	// Second execution with same ID must fail because approval was consumed (single-use)
	result, err = broker.Execute(nil, req)
	if !errors.Is(err, ErrPolicyAsk) || result.Status != ToolStatusPolicyDenied {
		t.Fatalf("approval must be single-use: result=%#v err=%v", result, err)
	}

	// Approval for mismatched tool must not match
	mismatchReq := enforcementRequest("shell", "shell", "exec-mismatch-tool")
	_ = broker.Approve(Approval{
		ExecutionID: mismatchReq.ExecutionID,
		ToolName:    "other_tool",
		Approver:    "sec-lead",
	})
	result, err = broker.Execute(nil, mismatchReq)
	if !errors.Is(err, ErrPolicyAsk) || result.Status != ToolStatusPolicyDenied {
		t.Fatalf("mismatched tool approval must not apply: result=%#v err=%v", result, err)
	}

	// Approval for mismatched action must not match
	mismatchActionReq := enforcementRequest("shell", "shell", "exec-mismatch-action")
	_ = broker.Approve(Approval{
		ExecutionID: mismatchActionReq.ExecutionID,
		ToolName:    "shell",
		Action:      "other_action",
		Approver:    "sec-lead",
	})
	result, err = broker.Execute(nil, mismatchActionReq)
	if !errors.Is(err, ErrPolicyAsk) || result.Status != ToolStatusPolicyDenied {
		t.Fatalf("mismatched action approval must not apply: result=%#v err=%v", result, err)
	}
}

func TestBrokerMalformedRequestsTable(t *testing.T) {
	policy := mustPolicy(Policy{Rules: []Rule{{ID: "allow-shell", Action: "shell", Resource: "*", Effect: EffectAllow}}})
	broker := NewBroker(policy)
	if err := broker.Register(enforcementTool("shell", CapabilityShell)); err != nil {
		t.Fatal(err)
	}

	validReq := enforcementRequest("shell", "", "exec-valid")
	tests := []struct {
		name   string
		mutate func(r *ToolRequest)
	}{
		{
			name:   "empty execution ID",
			mutate: func(r *ToolRequest) { r.ExecutionID = "" },
		},
		{
			name:   "empty task ID",
			mutate: func(r *ToolRequest) { r.TaskID = "" },
		},
		{
			name:   "empty worktree ID",
			mutate: func(r *ToolRequest) { r.WorktreeID = "" },
		},
		{
			name:   "empty tool name",
			mutate: func(r *ToolRequest) { r.ToolName = "" },
		},
		{
			name:   "zero timeout",
			mutate: func(r *ToolRequest) { r.Timeout = 0 },
		},
		{
			name:   "negative timeout",
			mutate: func(r *ToolRequest) { r.Timeout = -5 * time.Second },
		},
		{
			name:   "unsupported contract version",
			mutate: func(r *ToolRequest) { r.ContractVersion = "cortexos.harness.v999" },
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := validReq
			req.ExecutionID = "exec-" + strings.ReplaceAll(tt.name, " ", "-")
			tt.mutate(&req)

			result, err := broker.Execute(nil, req)
			if err == nil {
				t.Fatalf("expected error for %s", tt.name)
			}
			coded := ErrorFor(err)
			if coded.Code != ErrorCodeInvalidContract {
				t.Fatalf("code = %v, want %v", coded.Code, ErrorCodeInvalidContract)
			}
			if result.Status != ToolStatusFailed {
				t.Fatalf("status = %v, want failed", result.Status)
			}
			if len(result.EvidenceIDs) == 0 {
				t.Fatal("fail-closed execution must record audit evidence")
			}
		})
	}
}

func TestBrokerAuditEventsForExecutionFilter(t *testing.T) {
	policy := mustPolicy(Policy{Rules: []Rule{{ID: "allow-shell", Action: "shell", Resource: "*", Effect: EffectAllow}}})
	broker := NewBroker(policy)
	engine := NewFakeEngine(ToolKindNative, CapabilityShell)
	if err := broker.RegisterEngine(engine); err != nil {
		t.Fatal(err)
	}
	if err := broker.Register(enforcementTool("shell", CapabilityShell)); err != nil {
		t.Fatal(err)
	}

	_, _ = broker.Execute(nil, enforcementRequest("shell", "", "exec-alpha"))
	_, _ = broker.Execute(nil, enforcementRequest("shell", "", "exec-beta"))

	alphaEvents := broker.AuditEventsForExecution("exec-alpha")
	betaEvents := broker.AuditEventsForExecution("exec-beta")
	gammaEvents := broker.AuditEventsForExecution("exec-nonexistent")

	if len(alphaEvents) == 0 {
		t.Fatal("expected events for exec-alpha")
	}
	for _, e := range alphaEvents {
		if e.ExecutionID != "exec-alpha" {
			t.Fatalf("audit event has wrong execution ID: %s", e.ExecutionID)
		}
	}

	if len(betaEvents) == 0 {
		t.Fatal("expected events for exec-beta")
	}
	for _, e := range betaEvents {
		if e.ExecutionID != "exec-beta" {
			t.Fatalf("audit event has wrong execution ID: %s", e.ExecutionID)
		}
	}

	if len(gammaEvents) != 0 {
		t.Fatalf("expected 0 events for nonexistent execution, got %d", len(gammaEvents))
	}
}

func TestBrokerToolRequiresAskExplicitBoundary(t *testing.T) {
	policy := mustPolicy(Policy{Rules: []Rule{{ID: "allow-shell", Action: "shell", Resource: "*", Effect: EffectAllow}}})
	broker := NewBroker(policy)
	engine := NewFakeEngine(ToolKindNative, CapabilityShell)
	if err := broker.RegisterEngine(engine); err != nil {
		t.Fatal(err)
	}

	tool := enforcementTool("shell-ask", CapabilityShell)
	tool.RequiresAsk = true
	if err := broker.Register(tool); err != nil {
		t.Fatal(err)
	}

	// Even though policy allows, tool.RequiresAsk forces ask boundary
	req := enforcementRequest("shell-ask", "", "exec-tool-ask")
	result, err := broker.Execute(nil, req)
	if !errors.Is(err, ErrPolicyAsk) || result.Status != ToolStatusPolicyDenied {
		t.Fatalf("tool RequiresAsk must force ask: result=%#v err=%v", result, err)
	}

	// With explicit approval, it succeeds
	_ = broker.Approve(Approval{
		ExecutionID: req.ExecutionID,
		ToolName:    req.ToolName,
		Approver:    "gatekeeper",
	})
	result, err = broker.Execute(nil, req)
	if err != nil || result.Status != ToolStatusSuccess {
		t.Fatalf("tool RequiresAsk with approval must succeed: result=%#v err=%v", result, err)
	}
}
