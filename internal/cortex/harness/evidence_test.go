package harness

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestComputeDigestDeterministic(t *testing.T) {
	data := []byte("hello world")
	first := ComputeDigest(data)
	second := ComputeDigest(data)

	if first != second {
		t.Fatalf("digest not deterministic: %s != %s", first, second)
	}
	if len(first) != 64 {
		t.Fatalf("SHA-256 digest length = %d, want 64", len(first))
	}
}

func TestNewEvidenceRecord(t *testing.T) {
	payload := map[string]string{
		"command": "go test ./...",
		"output":  "all tests passed",
	}

	record, err := NewEvidenceRecord("exec-1", "task-1", "wt-1", EvidenceKindCommandExecution, payload)
	if err != nil {
		t.Fatal(err)
	}

	if record.ID == "" {
		t.Fatal("evidence ID should not be empty")
	}
	if record.ContractVersion != HarnessContractVersion {
		t.Fatalf("contract version = %q", record.ContractVersion)
	}
	if record.ExecutionID != "exec-1" {
		t.Fatalf("execution ID = %q", record.ExecutionID)
	}
	if record.Kind != EvidenceKindCommandExecution {
		t.Fatalf("kind = %q", record.Kind)
	}
	if len(record.Digest) != 64 {
		t.Fatalf("digest length = %d", len(record.Digest))
	}
	if len(record.RedactedPayload) == 0 {
		t.Fatal("redacted payload should not be empty")
	}
	if record.CollectedAt.IsZero() {
		t.Fatal("collected time should be set")
	}
}

func TestNewEvidenceRecordRequiresIDs(t *testing.T) {
	for _, test := range []struct {
		name        string
		executionID string
		taskID      string
		worktreeID  string
	}{
		{"missing execution", "", "task-1", "wt-1"},
		{"missing task", "exec-1", "", "wt-1"},
		{"missing worktree", "exec-1", "task-1", ""},
	} {
		t.Run(test.name, func(t *testing.T) {
			_, err := NewEvidenceRecord(test.executionID, test.taskID, test.worktreeID, EvidenceKindCommandExecution, map[string]string{})
			if err == nil {
				t.Fatal("expected validation error")
			}
		})
	}
}

func TestEvidenceRecordSerialization(t *testing.T) {
	record, err := NewEvidenceRecord("exec-1", "task-1", "wt-1", EvidenceKindPolicyDecision, PolicyDecision{
		Allowed: true,
		Effect:  EffectAllow,
		Reason:  "matched rule",
	})
	if err != nil {
		t.Fatal(err)
	}

	data, err := json.Marshal(record)
	if err != nil {
		t.Fatal(err)
	}

	var decoded EvidenceRecord
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}

	if decoded.ID != record.ID {
		t.Fatalf("ID mismatch: %s != %s", decoded.ID, record.ID)
	}
	if decoded.Digest != record.Digest {
		t.Fatalf("digest mismatch")
	}
	if decoded.Kind != EvidenceKindPolicyDecision {
		t.Fatalf("kind = %q", decoded.Kind)
	}
}

func TestRedactStringSecrets(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		checks []string
	}{
		{
			name:   "api key",
			input:  "api_key: super-secret-value-12345",
			checks: []string{"super-secret-value-12345"},
		},
		{
			name:   "bearer token",
			input:  "Authorization: Bearer abcdefghijklmnop",
			checks: []string{"abcdefghijklmnop"},
		},
		{
			name:   "github token",
			input:  "token ghp_abcdefghijklmnopqrstuvwxyz1234567890",
			checks: []string{"ghp_abcdefghijklmnopqrstuvwxyz1234567890"},
		},
		{
			name:   "password",
			input:  "password = correct-horse-battery-staple",
			checks: []string{"correct-horse-battery-staple"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := RedactString(tt.input)
			for _, secret := range tt.checks {
				if strings.Contains(got, secret) {
					t.Fatalf("redacted string still contains secret %q: %q", secret, got)
				}
			}
			if !strings.Contains(got, "[REDACTED]") {
				t.Fatalf("redacted string missing marker: %q", got)
			}
		})
	}
}

func TestRedactStringPaths(t *testing.T) {
	input := `error opening C:\Users\alice\secret\file.txt`
	got := RedactString(input)

	if strings.Contains(got, `C:\Users\alice`) {
		t.Fatalf("redacted string contains absolute path: %q", got)
	}
	if !strings.Contains(got, ".../file.txt") {
		t.Fatalf("redacted string should preserve basename: %q", got)
	}
}

func TestRedactBytes(t *testing.T) {
	input := []byte(`{"token":"secret-value-12345"}`)
	got := RedactBytes(input)

	if strings.Contains(string(got), "secret-value-12345") {
		t.Fatalf("redacted bytes contain secret: %s", got)
	}
}

func TestEvidenceKinds(t *testing.T) {
	kinds := []EvidenceKind{
		EvidenceKindCommandExecution,
		EvidenceKindFileMutation,
		EvidenceKindPolicyDecision,
		EvidenceKindSandboxAudit,
		EvidenceKindAdapterHealth,
		EvidenceKindEngineDiagnostic,
	}
	for _, kind := range kinds {
		if kind == "" {
			t.Fatal("evidence kind should not be empty")
		}
	}
}
