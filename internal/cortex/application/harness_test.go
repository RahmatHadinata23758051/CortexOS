package application

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/RahmatHadinata23758051/CortexOS/internal/cortex/harness"
)

type harnessPortStub struct {
	caps       []HarnessCapabilityRecord
	workers    []HarnessWorkerRecord
	executions []HarnessExecutionRecord
	evidence   []HarnessEvidenceRecord
	err        error
}

func (s *harnessPortStub) ListCapabilities(ctx context.Context) ([]HarnessCapabilityRecord, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if s.err != nil {
		return nil, s.err
	}
	return s.caps, nil
}

func (s *harnessPortStub) ListWorkers(ctx context.Context) ([]HarnessWorkerRecord, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if s.err != nil {
		return nil, s.err
	}
	return s.workers, nil
}

func (s *harnessPortStub) ListActiveExecutions(ctx context.Context) ([]HarnessExecutionRecord, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if s.err != nil {
		return nil, s.err
	}
	return s.executions, nil
}

func (s *harnessPortStub) ListEvidence(ctx context.Context, executionID string) ([]HarnessEvidenceRecord, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if s.err != nil {
		return nil, s.err
	}
	var out []HarnessEvidenceRecord
	for _, e := range s.evidence {
		if e.ExecutionID == executionID {
			out = append(out, e)
		}
	}
	return out, nil
}

func TestHarnessBridgeCapabilities(t *testing.T) {
	stub := &harnessPortStub{
		caps: []HarnessCapabilityRecord{
			{Capability: "shell", Description: "Run shell in sandbox", RequiresAsk: true, DefaultEffect: "ask"},
			{Capability: "file_read", Description: "Read file", RequiresAsk: false, DefaultEffect: "allow"},
		},
	}
	svc := NewServiceWithHarness(stub)

	res, err := svc.ListHarnessCapabilities(context.Background(), HarnessCapabilitiesRequest{
		SchemaVersion: HarnessBridgeSchemaVersion,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(res) != 2 {
		t.Fatalf("expected 2 capabilities, got %d", len(res))
	}
	if res[0].Capability != "shell" || !res[0].RequiresAsk || res[0].SchemaVersion != HarnessBridgeSchemaVersion {
		t.Fatalf("unexpected capability summary: %+v", res[0])
	}
}

func TestHarnessBridgeWorkersOmitsForbiddenFields(t *testing.T) {
	now := time.Now().UTC()
	stub := &harnessPortStub{
		workers: []HarnessWorkerRecord{
			{
				ID:              "worker-1",
				InstanceID:      "inst-1",
				Kind:            "pi",
				Class:           "specialist",
				Version:         "1.0.0",
				WorkerSchema:    "cortexos.worker.v1",
				OwnerPID:        9999,
				OwnerProcessID:  "proc-secret-uuid-1234",
				RegisteredAt:    now,
				LastHeartbeatAt: now,
				Status:          "healthy",
				Metadata: map[string]any{
					"raw_secret": "my-secret-token",
					"host_path":  "/var/run/secret.sock",
				},
			},
		},
	}
	svc := NewServiceWithHarness(stub)

	workers, err := svc.ListHarnessWorkers(context.Background(), HarnessWorkersRequest{
		SchemaVersion: HarnessBridgeSchemaVersion,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(workers) != 1 {
		t.Fatalf("expected 1 worker, got %d", len(workers))
	}
	w := workers[0]
	if w.ID != "worker-1" || w.Status != "healthy" || w.SchemaVersion != HarnessBridgeSchemaVersion {
		t.Fatalf("unexpected worker summary: %+v", w)
	}

	// Verify JSON marshaling does NOT expose PID, ownerProcessID, or metadata
	data, err := json.Marshal(workers)
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}
	jsonStr := string(data)

	forbidden := []string{
		"9999", "proc-secret", "my-secret-token", "/var/run", "ownerPid", "ownerProcessId", "metadata", "OwnerPID", "OwnerProcessID",
	}
	for _, term := range forbidden {
		if strings.Contains(jsonStr, term) {
			t.Errorf("worker summary JSON contains forbidden data %q: %s", term, jsonStr)
		}
	}
}

func TestHarnessBridgeActiveExecutionsOmitsForbiddenFields(t *testing.T) {
	now := time.Now().UTC()
	stub := &harnessPortStub{
		executions: []HarnessExecutionRecord{
			{
				ID:            "exec-100",
				TaskID:        "task-abc",
				EngineClass:   "native",
				Priority:      "high",
				MemoryMB:      512,
				CPUPriority:   2,
				TraceID:       "trace-xyz",
				WorkerID:      "worker-1",
				CreatedAt:     now,
				WorktreePath:  "C:\\Secret\\Path\\Worktree",
				ProcessHandle: 123456,
				RawInput:      []byte(`{"token": "ghp_secrettoken12345"}`),
			},
		},
	}
	svc := NewServiceWithHarness(stub)

	execs, err := svc.ListActiveHarnessExecutions(context.Background(), HarnessActiveExecutionsRequest{
		SchemaVersion: HarnessBridgeSchemaVersion,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(execs) != 1 {
		t.Fatalf("expected 1 execution, got %d", len(execs))
	}
	e := execs[0]
	if e.ID != "exec-100" || e.TaskID != "task-abc" || e.WorkerID != "worker-1" || e.SchemaVersion != HarnessBridgeSchemaVersion {
		t.Fatalf("unexpected execution summary: %+v", e)
	}

	// Verify JSON serialization omits forbidden fields
	data, err := json.Marshal(execs)
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}
	jsonStr := string(data)

	forbidden := []string{
		"Secret", "Worktree", "123456", "ghp_secrettoken", "worktreePath", "processHandle", "rawInput",
	}
	for _, term := range forbidden {
		if strings.Contains(jsonStr, term) {
			t.Errorf("active executions JSON contains forbidden data %q: %s", term, jsonStr)
		}
	}
}

func TestHarnessBridgeEvidenceOmitsForbiddenFields(t *testing.T) {
	now := time.Now().UTC()
	stub := &harnessPortStub{
		evidence: []HarnessEvidenceRecord{
			{
				ID:              "ev-1",
				ExecutionID:     "exec-1",
				TaskID:          "task-1",
				WorktreeID:      "wt-1",
				Kind:            "command_execution",
				Digest:          "abcdef1234567890",
				RedactedPayload: []byte(`{"command": "go test"}`),
				RawProvider:     []byte(`{"raw": "provider internal secret"}`),
				Audit: map[string]any{
					"sql_query": "SELECT * FROM secret_tokens",
				},
				CollectedAt: now,
			},
		},
	}
	svc := NewServiceWithHarness(stub)

	evs, err := svc.GetHarnessEvidence(context.Background(), HarnessEvidenceRequest{
		SchemaVersion: HarnessBridgeSchemaVersion,
		ExecutionID:   "exec-1",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(evs) != 1 {
		t.Fatalf("expected 1 evidence, got %d", len(evs))
	}
	ev := evs[0]
	if ev.ID != "ev-1" || ev.Digest != "abcdef1234567890" || ev.SchemaVersion != HarnessBridgeSchemaVersion {
		t.Fatalf("unexpected evidence summary: %+v", ev)
	}

	// Verify JSON serialization contains only metadata
	data, err := json.Marshal(evs)
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}
	jsonStr := string(data)

	forbidden := []string{
		"go test", "provider internal secret", "SELECT", "secret_tokens", "payload", "audit", "sql", "RawProvider",
	}
	for _, term := range forbidden {
		if strings.Contains(jsonStr, term) {
			t.Errorf("evidence summary JSON contains forbidden data %q: %s", term, jsonStr)
		}
	}
}

func TestHarnessBridgeContextCancellation(t *testing.T) {
	stub := &harnessPortStub{}
	svc := NewServiceWithHarness(stub)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // canceled before call

	_, err := svc.ListHarnessCapabilities(ctx, HarnessCapabilitiesRequest{
		SchemaVersion: HarnessBridgeSchemaVersion,
	})
	if err == nil {
		t.Fatal("expected error on canceled context")
	}
	var bridgeErr *HarnessBridgeError
	if !errors.As(err, &bridgeErr) || bridgeErr.Code != "harness.canceled" {
		t.Fatalf("expected harness.canceled code, got %v", err)
	}
}

func TestHarnessBridgeDeadlineExceeded(t *testing.T) {
	stub := &harnessPortStub{}
	svc := NewServiceWithHarness(stub)

	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(-time.Hour))
	defer cancel()

	_, err := svc.ListHarnessWorkers(ctx, HarnessWorkersRequest{
		SchemaVersion: HarnessBridgeSchemaVersion,
	})
	if err == nil {
		t.Fatal("expected error on expired context")
	}
	var bridgeErr *HarnessBridgeError
	if !errors.As(err, &bridgeErr) || bridgeErr.Code != "harness.canceled" {
		t.Fatalf("expected harness.canceled code, got %v", err)
	}
}

func TestHarnessBridgeInvalidSchemaVersion(t *testing.T) {
	stub := &harnessPortStub{}
	svc := NewServiceWithHarness(stub)

	_, err := svc.ListHarnessCapabilities(context.Background(), HarnessCapabilitiesRequest{
		SchemaVersion: "cortexos.harness.v999",
	})
	if err == nil {
		t.Fatal("expected error for invalid schema version")
	}
	var bridgeErr *HarnessBridgeError
	if !errors.As(err, &bridgeErr) || bridgeErr.Code != "harness.unsupported_version" {
		t.Fatalf("expected harness.unsupported_version, got: %v", err)
	}
}

func TestHarnessBridgeMissingExecutionID(t *testing.T) {
	stub := &harnessPortStub{}
	svc := NewServiceWithHarness(stub)

	_, err := svc.GetHarnessEvidence(context.Background(), HarnessEvidenceRequest{
		SchemaVersion: HarnessBridgeSchemaVersion,
		ExecutionID:   "",
	})
	if err == nil {
		t.Fatal("expected error for empty execution ID")
	}
	var bridgeErr *HarnessBridgeError
	if !errors.As(err, &bridgeErr) || bridgeErr.Code != "harness.invalid_request" {
		t.Fatalf("expected harness.invalid_request, got: %v", err)
	}
}

func TestHarnessBridgeStoreUnavailable(t *testing.T) {
	svc := NewService() // harness service is nil

	_, err := svc.ListHarnessCapabilities(context.Background(), HarnessCapabilitiesRequest{
		SchemaVersion: HarnessBridgeSchemaVersion,
	})
	if err == nil {
		t.Fatal("expected error when harness port is nil")
	}
	var bridgeErr *HarnessBridgeError
	if !errors.As(err, &bridgeErr) || bridgeErr.Code != "harness.storage_unavailable" {
		t.Fatalf("expected harness.storage_unavailable, got: %v", err)
	}
}

func TestHarnessBridgeMappedErrorsAreGenericAndSafe(t *testing.T) {
	testCases := []struct {
		cause        error
		expectedCode string
		expectedMsg  string
	}{
		{harness.ErrNotFound, "harness.not_found", "harness record was not found"},
		{harness.ErrInvalidContract, "harness.invalid_request", "harness request is invalid"},
		{harness.ErrStoreUnavailable, "harness.storage_unavailable", "harness state is unavailable"},
		{harness.ErrOwnership, "harness.ownership", "harness operation not permitted"},
		{errors.New("SELECT secret FROM passwords WHERE 1=1"), "harness.internal", "harness request failed"},
	}

	for _, tc := range testCases {
		stub := &harnessPortStub{err: tc.cause}
		svc := NewServiceWithHarness(stub)

		_, err := svc.ListHarnessWorkers(context.Background(), HarnessWorkersRequest{
			SchemaVersion: HarnessBridgeSchemaVersion,
		})
		if err == nil {
			t.Fatalf("expected error for cause %v", tc.cause)
		}
		var bridgeErr *HarnessBridgeError
		if !errors.As(err, &bridgeErr) {
			t.Fatalf("expected HarnessBridgeError, got %T: %v", err, err)
		}
		if bridgeErr.Code != tc.expectedCode {
			t.Errorf("for cause %v, got code %q, want %q", tc.cause, bridgeErr.Code, tc.expectedCode)
		}
		if bridgeErr.Message != tc.expectedMsg {
			t.Errorf("for cause %v, got message %q, want %q", tc.cause, bridgeErr.Message, tc.expectedMsg)
		}
		// Prove that internal details (like SQL, secrets) never leak into message
		if strings.Contains(bridgeErr.Message, "SELECT") || strings.Contains(bridgeErr.Message, "password") {
			t.Errorf("leaked internal detail into error message: %s", bridgeErr.Message)
		}
	}
}
