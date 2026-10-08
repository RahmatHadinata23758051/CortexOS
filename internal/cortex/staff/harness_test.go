package staff

import (
	"encoding/json"
	"testing"

	"github.com/RahmatHadinata23758051/CortexOS/internal/cortex/harness"
)

func TestPolicyTranslatorCompatibility(t *testing.T) {
	permissions := PermissionSet{
		{ID: "read", Action: "read", Resource: "src/*", Effect: EffectAllow, Priority: 2},
		{ID: "shell", Action: "shell", Resource: "git push *", Effect: EffectAsk},
	}
	policy, err := (PolicyTranslator{}).ToHarnessPolicy(permissions)
	if err != nil {
		t.Fatal(err)
	}
	decision, err := policy.Evaluate(harness.Request{Action: "read", Resource: "src/main.go"})
	if err != nil || decision.Effect != harness.EffectAllow || decision.RuleID != "read" {
		t.Fatalf("unexpected Harness decision: %+v, %v", decision, err)
	}
	decision, err = policy.Evaluate(harness.Request{Action: "shell", Resource: "git push origin main"})
	if err != nil || decision.Effect != harness.EffectAsk {
		t.Fatalf("unexpected Harness ask decision: %+v, %v", decision, err)
	}

	// Translation retains priority, scope, and effect for Harness evaluation.
	highPriority, err := (PolicyTranslator{}).ToHarnessPolicy(PermissionSet{
		{ID: "broad-deny", Action: "read", Resource: "src/*", Effect: EffectDeny, Priority: 1},
		{ID: "specific-allow", Action: "read", Resource: "src/main.go", Effect: EffectAllow, Priority: 10},
	})
	if err != nil {
		t.Fatal(err)
	}
	decision, err = highPriority.Evaluate(harness.Request{Action: "read", Resource: "src/main.go"})
	if err != nil || decision.Effect != harness.EffectAllow || decision.RuleID != "specific-allow" {
		t.Fatalf("priority was not retained by Harness translation: %+v, %v", decision, err)
	}
}

func TestPolicyTranslatorRejectsInvalidSet(t *testing.T) {
	_, err := (PolicyTranslator{}).ToHarnessPolicy(PermissionSet{{ID: "bad", Action: "read", Resource: "../secret", Effect: EffectAllow}})
	if ErrorCodeOf(err) != ErrInvalidPermission {
		t.Fatalf("error code = %q, want %q; err=%v", ErrorCodeOf(err), ErrInvalidPermission, err)
	}
}

func TestEnvelopeAdapterFailsClosed(t *testing.T) {
	adapter := EnvelopeAdapter{}
	allowed := Decision{ContractVersion: ContractVersion, Action: "read", Resource: "src/a", Effect: EffectAllow, PermissionID: "p", Reason: "matched"}
	policyDecision, err := adapter.ToEnvelopePolicyDecision(allowed, "read", "src/a")
	if err != nil || !policyDecision.Allowed || policyDecision.Effect != harness.EffectAllow {
		t.Fatalf("unexpected allowed policy decision: %+v, %v", policyDecision, err)
	}
	if policyDecision.RequiresAsk || policyDecision.RuleID != "p" {
		t.Fatalf("allowed policy decision lost safe fields: %+v", policyDecision)
	}
	for _, decision := range []Decision{
		{ContractVersion: ContractVersion, Action: "read", Resource: "src/a", Effect: EffectAsk, PermissionID: "p", RequiresAsk: true},
		{ContractVersion: ContractVersion, Action: "read", Resource: "src/a", Effect: EffectDeny, PermissionID: "p"},
		{ContractVersion: "cortexos.staff.v99", Action: "read", Resource: "src/a", Effect: EffectAllow, PermissionID: "p"},
		{ContractVersion: ContractVersion, Action: "read", Resource: "src/a", Effect: EffectAllow},
	} {
		policyDecision, err = adapter.ToEnvelopePolicyDecision(decision, "read", "src/a")
		if err != nil {
			t.Fatal(err)
		}
		if policyDecision.Allowed || policyDecision.Effect != harness.EffectDeny || policyDecision.RequiresAsk {
			t.Fatalf("unsafe decision became allowed: %+v", policyDecision)
		}
	}
}

func TestFromHarnessDecision(t *testing.T) {
	translator := PolicyTranslator{}
	staffDecision, err := translator.FromHarnessDecision(harness.Decision{
		ContractVersion: harness.PolicyContractVersion,
		Action:          "read", Resource: "src/a", Effect: harness.EffectAllow, RuleID: "rule", Reason: "matched",
	}, "read", "src/a")
	if err != nil || staffDecision.Effect != EffectAllow || staffDecision.PermissionID != "rule" {
		t.Fatalf("unexpected Staff decision: %+v, %v", staffDecision, err)
	}

	for _, effect := range []harness.Effect{harness.EffectAsk, harness.EffectDeny, harness.Effect("unknown")} {
		staffDecision, err = translator.FromHarnessDecision(harness.Decision{
			ContractVersion: harness.PolicyContractVersion,
			Action:          "write", Resource: "src/a", Effect: effect, RuleID: "rule", Reason: "decision",
		}, "write", "src/a")
		if err != nil {
			t.Fatal(err)
		}
		if effect == harness.EffectAsk && (staffDecision.Effect != EffectAsk || !staffDecision.RequiresAsk) {
			t.Fatalf("Harness ask was not preserved: %+v", staffDecision)
		}
		if effect == harness.EffectDeny && staffDecision.Effect != EffectDeny {
			t.Fatalf("Harness deny was not preserved: %+v", staffDecision)
		}
		if effect == harness.Effect("unknown") && staffDecision.Effect != EffectDeny {
			t.Fatalf("unknown Harness effect did not fail closed: %+v", staffDecision)
		}
	}
}

func TestEnvelopePolicyDecisionIsJSONSafe(t *testing.T) {
	adapter := EnvelopeAdapter{}
	decision, err := adapter.ToEnvelopePolicyDecision(
		Decision{ContractVersion: ContractVersion, Action: "read", Resource: "src/a", Effect: EffectAllow, PermissionID: "rule", Reason: "matched"},
		"read", "src/a",
	)
	if err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(decision)
	if err != nil {
		t.Fatal(err)
	}
	var decoded harness.PolicyDecision
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded != decision {
		t.Fatalf("policy decision round trip mismatch: got %+v want %+v", decoded, decision)
	}
}
