package application

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/RahmatHadinata23758051/CortexOS/internal/cortex/harness"
	harnesssqlite "github.com/RahmatHadinata23758051/CortexOS/internal/cortex/harness/sqlite"
)

func TestHarnessAdapterWithSQLiteStore(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "harness.db")

	store, err := harnesssqlite.Open(ctx, dbPath)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer store.Close()

	policy, err := harness.NewStaticPolicy(harness.Policy{
		Rules: []harness.Rule{
			{ID: "allow-all", Action: "*", Resource: "*", Effect: harness.EffectAllow},
		},
	})
	if err != nil {
		t.Fatalf("policy: %v", err)
	}
	broker := harness.NewBroker(policy)

	adapter := NewHarnessAdapter(store, broker)

	// 1. ListCapabilities
	caps, err := adapter.ListCapabilities(ctx)
	if err != nil {
		t.Fatalf("ListCapabilities: %v", err)
	}
	if len(caps) == 0 {
		t.Fatal("expected non-empty capabilities from broker")
	}

	// 2. Register a worker in store, list it
	now := time.Now().UTC()
	err = store.RegisterWorker(ctx, harnesssqlite.WorkerState{
		ID:              "worker-sqlite-1",
		InstanceID:      "inst-sqlite-1",
		Kind:            harness.ToolKindNative,
		Class:           harness.EngineClassNative,
		Version:         "1.0.0",
		SchemaVersion:   "cortexos.worker.v1",
		OwnerPID:        1234,
		OwnerProcessID:  "proc-1234",
		RegisteredAt:    now,
		LastHeartbeatAt: now,
		Status:          "healthy",
		Metadata:        map[string]any{"env": "test"},
	})
	if err != nil {
		t.Fatalf("RegisterWorker: %v", err)
	}

	workers, err := adapter.ListWorkers(ctx)
	if err != nil {
		t.Fatalf("ListWorkers: %v", err)
	}
	if len(workers) != 1 || workers[0].ID != "worker-sqlite-1" {
		t.Fatalf("unexpected workers: %+v", workers)
	}

	// 3. Save a reservation in store, list active executions
	err = store.SaveReservation(ctx, harnesssqlite.ReservationState{
		ID:             "res-1",
		TaskID:         "task-1",
		EngineClass:    harness.EngineClassNative,
		Priority:       harness.PriorityNormal,
		MemoryMB:       256,
		CPUPriority:    1,
		TraceID:        "trace-1",
		OwnerWorkerID:  "worker-sqlite-1",
		OwnerProcessID: "proc-1234",
		CreatedAt:      now,
		Active:         true,
	})
	if err != nil {
		t.Fatalf("SaveReservation: %v", err)
	}

	execs, err := adapter.ListActiveExecutions(ctx)
	if err != nil {
		t.Fatalf("ListActiveExecutions: %v", err)
	}
	if len(execs) != 1 || execs[0].ID != "res-1" || execs[0].TaskID != "task-1" {
		t.Fatalf("unexpected active executions: %+v", execs)
	}

	// 4. Save evidence in store, list evidence
	ev, err := harness.NewEvidenceRecord("exec-1", "task-1", "wt-1", harness.EvidenceKindCommandExecution, map[string]string{"result": "ok"})
	if err != nil {
		t.Fatalf("NewEvidenceRecord: %v", err)
	}
	err = store.SaveEvidence(ctx, ev)
	if err != nil {
		t.Fatalf("SaveEvidence: %v", err)
	}

	evidence, err := adapter.ListEvidence(ctx, "exec-1")
	if err != nil {
		t.Fatalf("ListEvidence: %v", err)
	}
	if len(evidence) != 1 || evidence[0].ID != ev.ID {
		t.Fatalf("unexpected evidence: %+v", evidence)
	}

	// 5. Test context cancellation
	cancCtx, cancel := context.WithCancel(ctx)
	cancel()
	if _, err := adapter.ListWorkers(cancCtx); err == nil {
		t.Fatal("expected error on canceled context")
	}
}
