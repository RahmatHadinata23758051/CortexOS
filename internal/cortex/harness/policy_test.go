package harness

import (
	"errors"
	"testing"
)

func TestPolicyLastMatchingRuleWins(t *testing.T) {
	policy := Policy{Rules: []Rule{
		{ID: "default-shell", Action: "shell", Resource: "*", Effect: EffectAsk},
		{ID: "allow-git-status", Action: "shell", Resource: "git status *", Effect: EffectAllow},
		{ID: "deny-push", Action: "shell", Resource: "git push *", Effect: EffectDeny},
	}}

	decision, err := policy.Evaluate(Request{Action: "SHELL", Resource: "git status --short"})
	if err != nil {
		t.Fatal(err)
	}
	if decision.Effect != EffectAllow || decision.RuleID != "allow-git-status" {
		t.Fatalf("decision = %#v", decision)
	}

	decision, err = policy.Evaluate(Request{Action: "shell", Resource: "git push origin main"})
	if err != nil {
		t.Fatal(err)
	}
	if decision.Effect != EffectDeny || decision.RuleID != "deny-push" {
		t.Fatalf("decision = %#v", decision)
	}
}

func TestPolicyDefaultsToDeny(t *testing.T) {
	decision, err := (Policy{}).Evaluate(Request{Action: "read", Resource: "notes/project.md"})
	if err != nil {
		t.Fatal(err)
	}
	if decision.Effect != EffectDeny || decision.RuleID != "" {
		t.Fatalf("decision = %#v", decision)
	}
}

func TestPolicyRejectsInvalidRulesAndRequests(t *testing.T) {
	_, err := (Policy{Rules: []Rule{{ID: "bad", Action: "shell", Resource: "*", Effect: "maybe"}}}).Evaluate(Request{Action: "shell", Resource: "echo hi"})
	if !errors.Is(err, ErrInvalidPolicy) {
		t.Fatalf("expected invalid policy, got %v", err)
	}
	_, err = (Policy{}).Evaluate(Request{Action: "", Resource: "x"})
	if !errors.Is(err, ErrInvalidRequest) {
		t.Fatalf("expected invalid request, got %v", err)
	}
}

func TestWildcardMatchIsWholeValueAndSupportsQuestionMark(t *testing.T) {
	for _, test := range []struct {
		pattern string
		value   string
		want    bool
	}{
		{"git status *", "git status", true},
		{"git status *", "git status --short", true},
		{"git push *", "git status", false},
		{"tool/?", "tool/x", true},
		{"tool/?", "tool/xy", false},
	} {
		if got := wildcardMatch(test.pattern, test.value); got != test.want {
			t.Errorf("wildcardMatch(%q, %q) = %v, want %v", test.pattern, test.value, got, test.want)
		}
	}
}
