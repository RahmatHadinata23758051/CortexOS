package staff

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

func TestSafeJSONSerialization(t *testing.T) {
	d := validDefinition()
	data, err := json.Marshal(d)
	if err != nil {
		t.Fatal(err)
	}
	encoded := string(data)
	for _, forbidden := range []string{"workerID", "processHandle", "apiKey", "providerOutput", "successVerdict"} {
		if strings.Contains(encoded, forbidden) {
			t.Errorf("serialized Staff contains forbidden field %q: %s", forbidden, encoded)
		}
	}
	var roundTrip Definition
	if err := json.Unmarshal(data, &roundTrip); err != nil {
		t.Fatal(err)
	}
	if roundTrip.ID != d.ID || roundTrip.Permissions[0].ID != "read-worktree" {
		t.Fatalf("unexpected round trip: %+v", roundTrip)
	}
}

func TestDefinitionMarshalRejectsInvalid(t *testing.T) {
	bad := validDefinition()
	bad.SchemaVersion = "cortexos.staff.v99"
	if _, err := json.Marshal(bad); err == nil {
		t.Fatal("expected marshaling invalid definition to fail")
	}
}

func TestDefinitionUnmarshalRejectsForbiddenKeyVariants(t *testing.T) {
	d := validDefinition()
	baseJSON, err := json.Marshal(d)
	if err != nil {
		t.Fatal(err)
	}

	forbiddenVariants := []string{
		"workerID",
		"workerid",
		"worker_id",
		"worker-id",
		"WORKER_ID",
		"processID",
		"processid",
		"process_id",
		"process-id",
		"PROCESS_ID",
		"pid",
		"PID",
		"Pid",
		"cliIdentity",
		"cli_identity",
		"cli-identity",
		"CLI_IDENTITY",
		"processHandle",
		"processhandle",
		"process_handle",
		"process-handle",
		"PROCESS_HANDLE",
		"secret",
		"secrets",
		"SECRET",
		"token",
		"tokens",
		"TOKEN",
		"apiKey",
		"apikey",
		"api_key",
		"api-key",
		"API_KEY",
		"password",
		"PASSWORD",
		"credential",
		"credentials",
		"CREDENTIALS",
		"rawProviderOut",
		"raw_provider_out",
		"providerOutput",
		"provider_output",
		"selfApprove",
		"self_approve",
		"mergeAuthority",
		"merge_authority",
		"successVerdict",
		"success_verdict",
		"executionID",
		"execution_id",
	}

	for _, key := range forbiddenVariants {
		t.Run(key, func(t *testing.T) {
			malicious := strings.Replace(string(baseJSON), `"permissions"`, fmt.Sprintf(`"%s":"injected","permissions"`, key), 1)
			var target Definition
			err := json.Unmarshal([]byte(malicious), &target)
			if ErrorCodeOf(err) != ErrInvalidRequest {
				t.Fatalf("expected forbidden field %q to fail with ErrInvalidRequest, got %v (code=%q)", key, err, ErrorCodeOf(err))
			}
		})
	}
}

func TestDefinitionUnmarshalRejectsMalformedJSON(t *testing.T) {
	var d Definition
	if err := json.Unmarshal([]byte(`{not valid json}`), &d); err == nil {
		t.Fatal("expected malformed JSON to fail unmarshaling")
	}
}

func TestJSONSafeSummaryOmitsSensitiveContractFields(t *testing.T) {
	d := validDefinition()
	summary, err := d.JSONSafe()
	if err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(summary)
	if err != nil {
		t.Fatal(err)
	}
	encoded := string(data)
	for _, forbidden := range []string{"permissions", "skills", "memory", "provider", "process"} {
		if strings.Contains(encoded, forbidden) {
			t.Errorf("summary contains sensitive field %q: %s", forbidden, encoded)
		}
	}

	// Verify fields present in summary
	if summary.ID != d.ID || summary.Name != d.Name || summary.Role != d.Role ||
		summary.WorkspaceID != d.Workspace.WorkspaceID || summary.Lifecycle != d.Lifecycle ||
		summary.Availability != d.Availability || summary.SchemaVersion != ContractVersion {
		t.Fatalf("summary does not match source definition: %+v", summary)
	}
}

func TestJSONSafeFailsClosedOnInvalidDefinition(t *testing.T) {
	bad := validDefinition()
	bad.SchemaVersion = "bad.version"
	_, err := bad.JSONSafe()
	if ErrorCodeOf(err) != ErrUnsupportedVersion {
		t.Fatalf("expected ErrUnsupportedVersion, got %v", err)
	}
}

func TestSummaryMarshalAndUnmarshal(t *testing.T) {
	d := validDefinition()
	summary := d.Summary()
	data, err := json.Marshal(summary)
	if err != nil {
		t.Fatal(err)
	}

	var decoded Summary
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.ID != summary.ID || decoded.Name != summary.Name || decoded.Role != summary.Role ||
		decoded.WorkspaceID != summary.WorkspaceID || decoded.Lifecycle != summary.Lifecycle {
		t.Fatalf("decoded summary mismatch: %+v", decoded)
	}

	// Summary unmarshaling rejects forbidden process/secret keys
	badSummary := strings.Replace(string(data), `"name"`, `"workerID":"w-1","name"`, 1)
	if err := json.Unmarshal([]byte(badSummary), &decoded); ErrorCodeOf(err) != ErrInvalidRequest {
		t.Fatalf("forbidden key in summary unexpectedly accepted: %v", err)
	}

	// Summary unmarshaling rejects smuggled sensitive definition fields
	for _, smuggled := range []string{"permissions", "skills", "memory"} {
		t.Run("smuggle_"+smuggled, func(t *testing.T) {
			badJSON := strings.Replace(string(data), `"name"`, fmt.Sprintf(`"%s":[],"name"`, smuggled), 1)
			if err := json.Unmarshal([]byte(badJSON), &decoded); ErrorCodeOf(err) != ErrInvalidRequest {
				t.Fatalf("smuggled %q in summary unexpectedly accepted: %v", smuggled, err)
			}
		})
	}

	// Summary unmarshaling rejects invalid enum fields
	invalidEnums := []struct {
		field    string
		badVal   string
		wantCode ErrorCode
	}{
		{"role", "archmage", ErrInvalidRequest},
		{"lifecycle", "running", ErrInvalidRequest},
		{"availability", "rebooting", ErrInvalidRequest},
		{"schemaVersion", "v99", ErrUnsupportedVersion},
	}
	for _, tc := range invalidEnums {
		t.Run(tc.field+"_"+tc.badVal, func(t *testing.T) {
			bad := strings.Replace(string(data), fmt.Sprintf(`"%s":"%s"`, tc.field, d.Role), fmt.Sprintf(`"%s":"%s"`, tc.field, tc.badVal), 1)
			if tc.field == "lifecycle" {
				bad = strings.Replace(string(data), fmt.Sprintf(`"%s":"%s"`, tc.field, d.Lifecycle), fmt.Sprintf(`"%s":"%s"`, tc.field, tc.badVal), 1)
			}
			if tc.field == "availability" {
				bad = strings.Replace(string(data), fmt.Sprintf(`"%s":"%s"`, tc.field, d.Availability), fmt.Sprintf(`"%s":"%s"`, tc.field, tc.badVal), 1)
			}
			if tc.field == "schemaVersion" {
				bad = strings.Replace(string(data), fmt.Sprintf(`"%s":"%s"`, tc.field, d.SchemaVersion), fmt.Sprintf(`"%s":"%s"`, tc.field, tc.badVal), 1)
			}
			var s Summary
			err := json.Unmarshal([]byte(bad), &s)
			if ErrorCodeOf(err) != tc.wantCode {
				t.Fatalf("field %q with value %q returned code %q, want %q; err=%v", tc.field, tc.badVal, ErrorCodeOf(err), tc.wantCode, err)
			}
		})
	}

	// Summary MarshalJSON rejects invalid fields
	badSummaryObj := summary
	badSummaryObj.SchemaVersion = "bad.version"
	if _, err := json.Marshal(badSummaryObj); ErrorCodeOf(err) != ErrUnsupportedVersion {
		t.Fatalf("expected ErrUnsupportedVersion on bad summary marshal, got %v", err)
	}

	badSummaryObj = summary
	badSummaryObj.Lifecycle = "invalid"
	if _, err := json.Marshal(badSummaryObj); ErrorCodeOf(err) != ErrInvalidRequest {
		t.Fatalf("expected ErrInvalidRequest on bad summary lifecycle, got %v", err)
	}

	badSummaryObj = summary
	badSummaryObj.ID = "invalid/id"
	if _, err := json.Marshal(badSummaryObj); ErrorCodeOf(err) != ErrInvalidRequest {
		t.Fatalf("expected ErrInvalidRequest on bad summary ID, got %v", err)
	}

	badSummaryObj = summary
	badSummaryObj.Name = "   "
	if _, err := json.Marshal(badSummaryObj); ErrorCodeOf(err) != ErrInvalidRequest {
		t.Fatalf("expected ErrInvalidRequest on whitespace summary name, got %v", err)
	}

	badSummaryObj = summary
	badSummaryObj.WorkspaceID = "ws/invalid"
	if _, err := json.Marshal(badSummaryObj); ErrorCodeOf(err) != ErrInvalidRequest {
		t.Fatalf("expected ErrInvalidRequest on bad summary workspace ID, got %v", err)
	}

	// Summary UnmarshalJSON rejects invalid ID, name, workspace ID
	invalidSummaryFields := []struct {
		name     string
		field    string
		badVal   string
		wantCode ErrorCode
	}{
		{"id", "id", "invalid/id", ErrInvalidRequest},
		{"name", "name", "   ", ErrInvalidRequest},
		{"workspaceId", "workspaceId", "ws/invalid", ErrInvalidRequest},
		{"projectId", "projectId", "proj/invalid", ErrInvalidRequest},
		{"worktreeId", "worktreeId", "wt/invalid", ErrInvalidRequest},
	}
	for _, tc := range invalidSummaryFields {
		t.Run(tc.name, func(t *testing.T) {
			bad := strings.Replace(string(data), fmt.Sprintf(`"%s":"%s"`, tc.field, d.Workspace.WorkspaceID), fmt.Sprintf(`"%s":"%s"`, tc.field, tc.badVal), 1)
			if tc.field == "id" {
				bad = strings.Replace(string(data), fmt.Sprintf(`"%s":"%s"`, tc.field, d.ID), fmt.Sprintf(`"%s":"%s"`, tc.field, tc.badVal), 1)
			}
			if tc.field == "name" {
				bad = strings.Replace(string(data), fmt.Sprintf(`"%s":"%s"`, tc.field, d.Name), fmt.Sprintf(`"%s":"%s"`, tc.field, tc.badVal), 1)
			}
			if tc.field == "projectId" {
				bad = strings.Replace(string(data), fmt.Sprintf(`"%s":"%s"`, tc.field, d.Workspace.ProjectID), fmt.Sprintf(`"%s":"%s"`, tc.field, tc.badVal), 1)
			}
			if tc.field == "worktreeId" {
				bad = strings.Replace(string(data), fmt.Sprintf(`"%s":"%s"`, tc.field, d.Workspace.WorktreeID), fmt.Sprintf(`"%s":"%s"`, tc.field, tc.badVal), 1)
			}
			var s Summary
			err := json.Unmarshal([]byte(bad), &s)
			if ErrorCodeOf(err) != tc.wantCode {
				t.Fatalf("field %q with value %q returned code %q, want %q; err=%v", tc.field, tc.badVal, ErrorCodeOf(err), tc.wantCode, err)
			}
		})
	}
}
