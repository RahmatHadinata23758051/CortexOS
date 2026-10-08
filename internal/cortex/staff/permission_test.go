package staff

import (
	"encoding/json"
	"testing"
)

func TestPermissionSetValidate(t *testing.T) {
	valid := PermissionSet{
		{ID: "p1", Action: "read", Resource: "src/*", Effect: EffectAllow, Priority: 1},
		{ID: "p2", Action: "write", Resource: "tests/*", Effect: EffectAsk},
		{ID: "p3", Action: "delete", Resource: "build/*", Effect: EffectDeny, Priority: 10},
	}
	if err := valid.Validate(); err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name     string
		perms    PermissionSet
		wantCode ErrorCode
	}{
		{"missing id", PermissionSet{{ID: "", Action: "read", Resource: "x", Effect: EffectAllow}}, ErrInvalidPermission},
		{"id with invalid chars", PermissionSet{{ID: "p/1", Action: "read", Resource: "x", Effect: EffectAllow}}, ErrInvalidPermission},
		{"missing action", PermissionSet{{ID: "p", Action: "", Resource: "x", Effect: EffectAllow}}, ErrInvalidPermission},
		{"action with invalid chars", PermissionSet{{ID: "p", Action: "act/ion", Resource: "x", Effect: EffectAllow}}, ErrInvalidPermission},
		{"missing resource", PermissionSet{{ID: "p", Action: "read", Resource: "", Effect: EffectAllow}}, ErrInvalidPermission},
		{"whitespace resource", PermissionSet{{ID: "p", Action: "read", Resource: "   ", Effect: EffectAllow}}, ErrInvalidPermission},
		{"negative priority", PermissionSet{{ID: "p", Action: "read", Resource: "x", Effect: EffectAllow, Priority: -1}}, ErrInvalidPermission},
		{"unsafe resource with traversal ../", PermissionSet{{ID: "p", Action: "read", Resource: "../secret", Effect: EffectAllow}}, ErrInvalidPermission},
		{"unsafe resource with mid-traversal", PermissionSet{{ID: "p", Action: "read", Resource: "src/../secret", Effect: EffectAllow}}, ErrInvalidPermission},
		{"unsafe resource with Windows traversal", PermissionSet{{ID: "p", Action: "read", Resource: `src\..\secret`, Effect: EffectAllow}}, ErrInvalidPermission},
		{"unsafe resource with control char", PermissionSet{{ID: "p", Action: "read", Resource: "src/\x00secret", Effect: EffectAllow}}, ErrInvalidPermission},
		{"unknown effect", PermissionSet{{ID: "p", Action: "read", Resource: "x", Effect: "maybe"}}, ErrInvalidPermission},
		{"empty effect", PermissionSet{{ID: "p", Action: "read", Resource: "x", Effect: ""}}, ErrInvalidPermission},
		{"duplicate id", PermissionSet{{ID: "p", Action: "read", Resource: "x", Effect: EffectAllow}, {ID: "p", Action: "write", Resource: "y", Effect: EffectDeny}}, ErrInvalidPermission},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := test.perms.Validate()
			if ErrorCodeOf(err) != test.wantCode {
				t.Fatalf("error code = %q, want %q; err=%v", ErrorCodeOf(err), test.wantCode, err)
			}
		})
	}
}

func TestPermissionEvaluationOrderingAndPriority(t *testing.T) {
	perms := PermissionSet{
		{ID: "low", Action: "read", Resource: "src/*", Effect: EffectDeny, Priority: 1},
		{ID: "high", Action: "read", Resource: "src/main.go", Effect: EffectAllow, Priority: 10},
	}
	decision, err := perms.Evaluate("read", "src/main.go")
	if err != nil || decision.Effect != EffectAllow || decision.PermissionID != "high" {
		t.Fatalf("high priority not selected: %+v, %v", decision, err)
	}
	decision, err = perms.Evaluate("read", "src/other.go")
	if err != nil || decision.Effect != EffectDeny || decision.PermissionID != "low" {
		t.Fatalf("low priority not selected: %+v, %v", decision, err)
	}

	// Tie-breaking by permission ID ascending when priorities are equal
	tiePerms := PermissionSet{
		{ID: "rule-b", Action: "read", Resource: "src/*", Effect: EffectDeny, Priority: 5},
		{ID: "rule-a", Action: "read", Resource: "src/*", Effect: EffectAllow, Priority: 5},
	}
	decision, err = tiePerms.Evaluate("read", "src/file.go")
	if err != nil || decision.Effect != EffectAllow || decision.PermissionID != "rule-a" {
		t.Fatalf("expected tie-breaker to select rule-a by ID asc: %+v, %v", decision, err)
	}
}

func TestPermissionEvaluationFailClosed(t *testing.T) {
	perms := PermissionSet{
		{ID: "allow-read", Action: "read", Resource: "src/*", Effect: EffectAllow, Priority: 1},
		{ID: "ask-write", Action: "write", Resource: "src/*", Effect: EffectAsk},
	}

	// Case insensitivity and Windows path normalization
	decision, err := perms.Evaluate("READ", `src\main.go`)
	if err != nil || decision.Effect != EffectAllow || decision.PermissionID != "allow-read" {
		t.Fatalf("unexpected allow decision: %+v, %v", decision, err)
	}

	// Ask decision
	decision, err = perms.Evaluate("write", "src/main.go")
	if err != nil || decision.Effect != EffectAsk || !decision.RequiresAsk {
		t.Fatalf("unexpected ask decision: %+v, %v", decision, err)
	}

	// Unmatched action -> default deny
	decision, err = perms.Evaluate("delete", "src/main.go")
	if err != nil || decision.Effect != EffectDeny || decision.RequiresAsk || decision.PermissionID != "" {
		t.Fatalf("unexpected default deny on unmatched action: %+v, %v", decision, err)
	}

	// Unmatched resource -> default deny
	decision, err = perms.Evaluate("read", "secret/key.txt")
	if err != nil || decision.Effect != EffectDeny || decision.RequiresAsk || decision.PermissionID != "" {
		t.Fatalf("unexpected default deny on unmatched resource: %+v, %v", decision, err)
	}

	// Empty action or resource -> default deny with invalid request error
	dec, err := perms.Evaluate("", "src/main.go")
	if ErrorCodeOf(err) != ErrInvalidRequest || dec.Effect != EffectDeny {
		t.Fatalf("expected ErrInvalidRequest and deny on empty action: %+v, %v", dec, err)
	}
	dec, err = perms.Evaluate("read", "")
	if ErrorCodeOf(err) != ErrInvalidRequest || dec.Effect != EffectDeny {
		t.Fatalf("expected ErrInvalidRequest and deny on empty resource: %+v, %v", dec, err)
	}

	// Invalid permission set -> default deny with invalid permission error
	badPerms := PermissionSet{{ID: "bad", Action: "read", Resource: "x", Effect: "unknown"}}
	dec, err = badPerms.Evaluate("read", "x")
	if ErrorCodeOf(err) != ErrInvalidPermission || dec.Effect != EffectDeny {
		t.Fatalf("expected ErrInvalidPermission and deny on invalid set: %+v, %v", dec, err)
	}
}

func TestWildcardMatching(t *testing.T) {
	tests := []struct {
		pattern  string
		resource string
		want     bool
	}{
		{"src/*", "src/main.go", true},
		{"src/*", "src/pkg/sub.go", true},
		{"*.go", "main.go", true},
		{"*.go", "main.rs", false},
		{"tool/?", "tool/x", true},
		{"tool/?", "tool/xy", false},
		{"git status *", "git status", true}, // Harness trailing " *" compatibility
		{"git status *", "git status --short", true},
		{"git push *", "git status", false},
		{"exact/match", "exact/match", true},
		{"exact/match", "exact/match/extra", false},
	}
	for _, tc := range tests {
		t.Run(tc.pattern+"_"+tc.resource, func(t *testing.T) {
			got := wildcardMatch(tc.pattern, tc.resource)
			if got != tc.want {
				t.Fatalf("wildcardMatch(%q, %q) = %v, want %v", tc.pattern, tc.resource, got, tc.want)
			}
		})
	}
}

func TestValidateExecutionPermission(t *testing.T) {
	valid := Decision{
		ContractVersion: ContractVersion,
		Action:          "read",
		Resource:        "src/a",
		Effect:          EffectAllow,
		PermissionID:    "allow-rule",
		Reason:          "matched",
		RequiresAsk:     false,
	}
	if err := ValidateExecutionPermission(valid, "read", "src/a"); err != nil {
		t.Fatalf("valid execution permission rejected: %v", err)
	}

	// Normalization in action/resource check
	if err := ValidateExecutionPermission(valid, "READ", `src\a`); err != nil {
		t.Fatalf("action/resource normalization failed in ValidateExecutionPermission: %v", err)
	}

	tests := []struct {
		name     string
		decision Decision
		action   string
		resource string
		wantCode ErrorCode
	}{
		{"wrong contract version", Decision{ContractVersion: "cortexos.staff.v99", Action: "read", Resource: "src/a", Effect: EffectAllow, PermissionID: "r1"}, "read", "src/a", ErrUnsupportedVersion},
		{"mismatched action", Decision{ContractVersion: ContractVersion, Action: "write", Resource: "src/a", Effect: EffectAllow, PermissionID: "r1"}, "read", "src/a", ErrInvalidPermission},
		{"mismatched resource", Decision{ContractVersion: ContractVersion, Action: "read", Resource: "src/b", Effect: EffectAllow, PermissionID: "r1"}, "read", "src/a", ErrInvalidPermission},
		{"effect ask rejected", Decision{ContractVersion: ContractVersion, Action: "read", Resource: "src/a", Effect: EffectAsk, PermissionID: "r1", RequiresAsk: true}, "read", "src/a", ErrPermissionDenied},
		{"effect deny rejected", Decision{ContractVersion: ContractVersion, Action: "read", Resource: "src/a", Effect: EffectDeny, PermissionID: "r1"}, "read", "src/a", ErrPermissionDenied},
		{"empty permission id rejected", Decision{ContractVersion: ContractVersion, Action: "read", Resource: "src/a", Effect: EffectAllow, PermissionID: ""}, "read", "src/a", ErrPermissionDenied},
		{"requiresAsk true rejected", Decision{ContractVersion: ContractVersion, Action: "read", Resource: "src/a", Effect: EffectAllow, PermissionID: "r1", RequiresAsk: true}, "read", "src/a", ErrPermissionDenied},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateExecutionPermission(tc.decision, tc.action, tc.resource)
			if ErrorCodeOf(err) != tc.wantCode {
				t.Fatalf("error code = %q, want %q; err=%v", ErrorCodeOf(err), tc.wantCode, err)
			}
		})
	}
}

func TestPermissionJSONRoundTrip(t *testing.T) {
	perms := PermissionSet{
		{ID: "p1", Action: "read", Resource: "src/*", Effect: EffectAllow, Priority: 1},
		{ID: "p2", Action: "write", Resource: "out/*", Effect: EffectAsk, Priority: 2},
	}
	data, err := json.Marshal(perms)
	if err != nil {
		t.Fatal(err)
	}
	var decoded PermissionSet
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}
	if len(decoded) != 2 || decoded[0].ID != "p1" || decoded[1].ID != "p2" {
		t.Fatalf("unexpected decoded: %+v", decoded)
	}

	// Marshaling invalid permission set fails
	invalidPerms := PermissionSet{{ID: "bad", Action: "read", Resource: "../secret", Effect: EffectAllow}}
	if _, err := json.Marshal(invalidPerms); err == nil {
		t.Fatal("expected marshaling invalid permission set to fail")
	}

	// Unmarshaling invalid permission set fails
	invalidJSON := `[{"id":"bad","action":"read","resource":"../secret","effect":"allow"}]`
	if err := json.Unmarshal([]byte(invalidJSON), &decoded); err == nil {
		t.Fatal("expected unmarshaling invalid permission set to fail")
	}
}
