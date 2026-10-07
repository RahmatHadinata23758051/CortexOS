package sqlite

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/RahmatHadinata23758051/CortexOS/internal/cortex/harness"
)

func openStore(t *testing.T) *Store {
	t.Helper()
	path := t.TempDir() + "/harness.db"
	store, err := Open(context.Background(), path)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	return store
}

func testWorker(owner string) WorkerState {
	now := time.Date(2026, 10, 7, 12, 0, 0, 0, time.UTC)
	return WorkerState{
		ID: "worker-1", InstanceID: "instance-1", Kind: harness.ToolKindPi,
		Class: harness.EngineClassPi, Version: "1.0", SchemaVersion: harness.WorkerProtocolVersion,
		OwnerPID: 1234, OwnerProcessID: owner, RegisteredAt: now, LastHeartbeatAt: now,
		Status: "healthy", Metadata: map[string]any{"path": `C:\Users\private\repo`, "token": "secret-value"},
	}
}

func TestWorkerOwnershipPreventsForeignMutation(t *testing.T) {
	store := openStore(t)
	ctx := context.Background()
	if err := store.RegisterWorker(ctx, testWorker("process-a")); err != nil {
		t.Fatalf("register worker: %v", err)
	}

	if err := store.HeartbeatWorker(ctx, "worker-1", "process-b", time.Now()); !errors.Is(err, ErrOwnership) {
		t.Fatalf("foreign heartbeat error = %v, want ownership error", err)
	}
	if err := store.DeleteWorker(ctx, "worker-1", "process-b"); !errors.Is(err, ErrOwnership) {
		t.Fatalf("foreign delete error = %v, want ownership error", err)
	}
	if err := store.DeleteWorker(ctx, "worker-1", "process-a"); err != nil {
		t.Fatalf("owner delete: %v", err)
	}
	if err := store.HeartbeatWorker(ctx, "worker-1", "process-a", time.Now()); !errors.Is(err, ErrNotFound) {
		t.Fatalf("heartbeat after delete = %v, want not found", err)
	}
}

func TestReservationForeignReleaseAndIdempotency(t *testing.T) {
	store := openStore(t)
	ctx := context.Background()
	if err := store.RegisterWorker(ctx, testWorker("process-a")); err != nil {
		t.Fatal(err)
	}
	if err := store.SaveReservation(ctx, ReservationState{
		ID: "reservation-1", TaskID: "task-1", EngineClass: harness.EngineClassPi,
		Priority: harness.PriorityNormal, MemoryMB: 100, CPUPriority: 1, TraceID: "trace",
		OwnerWorkerID: "worker-1", OwnerProcessID: "process-a", Active: true,
	}); err != nil {
		t.Fatalf("save reservation: %v", err)
	}

	if err := store.ReleaseReservation(ctx, "reservation-1", "process-b", time.Now()); !errors.Is(err, ErrOwnership) {
		t.Fatalf("foreign release = %v, want ownership error", err)
	}
	if err := store.ReleaseReservation(ctx, "reservation-1", "process-a", time.Now()); err != nil {
		t.Fatalf("release: %v", err)
	}
	if err := store.ReleaseReservation(ctx, "reservation-1", "process-a", time.Now()); err != nil {
		t.Fatalf("idempotent release: %v", err)
	}
}

func TestEvidenceIsRedactedAndReferenceOnly(t *testing.T) {
	store := openStore(t)
	ctx := context.Background()
	record, err := harness.NewEvidenceRecord("exec-1", "task-1", "worktree-1", harness.EvidenceKindCommandExecution, map[string]string{
		"summary": `token=super-secret-token C:\Users\private\output.txt`,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := store.SaveEvidence(ctx, record); err != nil {
		t.Fatalf("save evidence: %v", err)
	}
	records, err := store.EvidenceForExecution(ctx, "exec-1")
	if err != nil {
		t.Fatalf("read evidence: %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("evidence count = %d, want 1", len(records))
	}
	payload := string(records[0].RedactedPayload)
	if payload == "" || containsAny(payload, "super-secret-token", `C:\Users\private`) {
		t.Fatalf("evidence not redacted: %q", payload)
	}
}

func TestCrashRecoveryIdempotentNoSuccess(t *testing.T) {
	store := openStore(t)
	ctx := context.Background()
	if err := store.RegisterWorker(ctx, testWorker("process-a")); err != nil {
		t.Fatal(err)
	}
	if err := store.SaveReservation(ctx, ReservationState{
		ID: "reservation-1", TaskID: "task-1", EngineClass: harness.EngineClassPi,
		OwnerWorkerID: "worker-1", OwnerProcessID: "process-a", Active: true,
	}); err != nil {
		t.Fatal(err)
	}
	when := time.Date(2026, 10, 7, 12, 30, 0, 0, time.UTC)
	ids, err := store.RecoverInterrupted(ctx, when)
	if err != nil {
		t.Fatalf("recover: %v", err)
	}
	if len(ids) != 1 || ids[0] != "worker-1" {
		t.Fatalf("recovered ids = %#v", ids)
	}
	ids, err = store.RecoverInterrupted(ctx, when)
	if err != nil {
		t.Fatalf("repeat recover: %v", err)
	}
	if len(ids) != 0 {
		t.Fatalf("repeat recovery = %#v, want empty", ids)
	}

	var status string
	if err := store.DB().QueryRowContext(ctx, `SELECT status FROM harness_workers WHERE id='worker-1'`).Scan(&status); err != nil {
		t.Fatal(err)
	}
	if status != "interrupted" {
		t.Errorf("status = %q, want interrupted", status)
	}
	var active int
	if err := store.DB().QueryRowContext(ctx, `SELECT active FROM harness_reservations WHERE id='reservation-1'`).Scan(&active); err != nil {
		t.Fatal(err)
	}
	if active != 0 {
		t.Errorf("reservation active = %d, want 0", active)
	}

	// Verify no evidence was mutated
	var evidenceCount int
	if err := store.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM harness_evidence`).Scan(&evidenceCount); err != nil {
		t.Fatal(err)
	}
	if evidenceCount != 0 {
		t.Errorf("evidence count = %d, want 0 (recovery must not create evidence)", evidenceCount)
	}
}

func TestConcurrentRecoverySerialized(t *testing.T) {
	store := openStore(t)
	ctx := context.Background()
	if err := store.RegisterWorker(ctx, testWorker("process-a")); err != nil {
		t.Fatal(err)
	}
	results := make(chan []string, 2)
	errs := make(chan error, 2)
	for i := 0; i < 2; i++ {
		go func() {
			ids, err := store.RecoverInterrupted(ctx, time.Now().UTC())
			results <- ids
			errs <- err
		}()
	}
	var recovered int
	for i := 0; i < 2; i++ {
		if err := <-errs; err != nil {
			t.Fatalf("concurrent recovery: %v", err)
		}
		recovered += len(<-results)
	}
	if recovered != 1 {
		t.Errorf("total recovered workers = %d, want 1", recovered)
	}
}

func TestConcurrentOpen(t *testing.T) {
	path := t.TempDir() + "/concurrent_open.db"
	ctx := context.Background()
	const count = 4
	stores := make([]*Store, count)
	errs := make(chan error, count)
	for i := 0; i < count; i++ {
		idx := i
		go func() {
			s, err := Open(ctx, path)
			if err == nil {
				stores[idx] = s
			}
			errs <- err
		}()
	}
	for i := 0; i < count; i++ {
		if err := <-errs; err != nil {
			t.Fatalf("concurrent open error: %v", err)
		}
	}
	for _, s := range stores {
		if s != nil {
			_ = s.Close()
		}
	}
}

func TestPolicyAndToolPersistence(t *testing.T) {
	store := openStore(t)
	ctx := context.Background()
	policy, err := harness.NewPolicy([]harness.Rule{{ID: "r1", Action: "shell", Resource: "build", Effect: harness.EffectAllow, Priority: 10}})
	if err != nil {
		t.Fatal(err)
	}
	if err := store.SavePolicy(ctx, PolicyState{ID: "pol-1", Name: "DefaultPolicy", ContractVersion: harness.PolicyContractVersion, Rules: policy, Active: true}); err != nil {
		t.Fatalf("save policy: %v", err)
	}
	gotPol, err := store.GetPolicy(ctx, "pol-1")
	if err != nil {
		t.Fatalf("get policy: %v", err)
	}
	if gotPol.Name != "DefaultPolicy" || len(gotPol.Rules.Rules) != 1 || gotPol.Rules.Rules[0].ID != "r1" {
		t.Fatalf("policy mismatch: %+v", gotPol)
	}
	toolDef := harness.ToolDefinition{
		Name: "test-runner", Kind: harness.ToolKindNative,
		Description:  `Runs tests at C:\\secret\\path with token=secret123456`,
		Capabilities: []harness.ToolCapability{harness.CapabilityTestRun},
		InputSchema:  []byte(`{"type":"object"}`), OutputSchema: []byte(`{"type":"object"}`),
		Timeout: 10 * time.Second, Version: "1.0.0",
	}
	if err := store.SaveTool(ctx, ToolState{ID: "tool-1", Definition: toolDef, Active: true}); err != nil {
		t.Fatalf("save tool: %v", err)
	}
	var desc string
	if err := store.DB().QueryRowContext(ctx, `SELECT description FROM harness_tools WHERE id='tool-1'`).Scan(&desc); err != nil {
		t.Fatal(err)
	}
	if containsAny(desc, "secret123456", `C:\\secret`) {
		t.Fatalf("tool description contains unredacted secrets or path: %q", desc)
	}
}

func TestHeartbeatOwnership(t *testing.T) {
	store := openStore(t)
	ctx := context.Background()
	if err := store.RegisterWorker(ctx, testWorker("proc-1")); err != nil {
		t.Fatal(err)
	}
	if err := store.RecordHeartbeat(ctx, "hb-1", "worker-1", "proc-2", true, 128, 1, time.Now(), nil); !errors.Is(err, ErrOwnership) {
		t.Fatalf("expected ErrOwnership for foreign heartbeat, got: %v", err)
	}
	if err := store.RecordHeartbeat(ctx, "hb-1", "worker-1", "proc-1", true, 128, 1, time.Now(), map[string]any{"token": "secret-heartbeat-token"}); err != nil {
		t.Fatalf("owning heartbeat failed: %v", err)
	}
	var details string
	if err := store.DB().QueryRowContext(ctx, `SELECT details_json FROM harness_heartbeats WHERE id='hb-1'`).Scan(&details); err != nil {
		t.Fatal(err)
	}
	if containsAny(details, "secret-heartbeat-token") {
		t.Fatalf("heartbeat details contains secret: %q", details)
	}
}

func containsAny(value string, needles ...string) bool {
	for _, needle := range needles {
		for i := 0; i+len(needle) <= len(value); i++ {
			if value[i:i+len(needle)] == needle {
				return true
			}
		}
	}
	return false
}
