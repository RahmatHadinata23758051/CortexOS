// Package orchestra_test — Phase 10 stress suite.
//
// BAN-P10-STRESS: deterministic high-concurrency merge and dispatch stress tests
// exercising real SQLite compare-and-set (CAS) persistence semantics under load.
//
// These tests are intentionally bounded for Windows CI (32 goroutines max,
// in-memory + temp-dir SQLite). They are soak-probe style: they fail closed if
// more than one merge wins, if sequences collide, or if the governor admits
// beyond budget.
package orchestra_test

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/RahmatHadinata23758051/CortexOS/internal/cortex/harness"
	harnesssqlite "github.com/RahmatHadinata23758051/CortexOS/internal/cortex/harness/sqlite"
	"github.com/RahmatHadinata23758051/CortexOS/internal/cortex/orchestra"
	orchestrasqlite "github.com/RahmatHadinata23758051/CortexOS/internal/cortex/orchestra/sqlite"
)

// seedTaskAwaitingInspectionUnique seeds a task with a uniquely keyed evidence
// record so concurrent tasks never collide on evidence IDs.
func seedTaskAwaitingInspectionUnique(t *testing.T, orchStore *orchestrasqlite.Store, harnessStore *harnesssqlite.Store, taskID orchestra.TaskID, wtID string) (orchestra.Task, orchestra.Execution) {
	t.Helper()
	ctx := context.Background()

	task := orchestra.Task{
		ID:                 taskID,
		ProjectID:          "p-1",
		WorktreeID:         wtID,
		Title:              "Stress test task",
		AcceptanceCriteria: []string{"passes audit"},
		Status:             orchestra.TaskAwaitingInspection,
		MaxAttempts:        3,
		AttemptCount:       1,
		CreatedAt:          time.Now().UTC(),
		UpdatedAt:          time.Now().UTC(),
		SchemaVersion:      orchestra.ContractVersion,
	}
	if err := orchStore.SaveTask(ctx, task); err != nil {
		t.Fatalf("save task: %v", err)
	}

	execID := orchestra.ExecutionID(fmt.Sprintf("%s-1", taskID))
	evID := fmt.Sprintf("ev-%s", taskID)
	exec := orchestra.Execution{
		ID:          execID,
		TaskID:      taskID,
		Attempt:     1,
		Status:      orchestra.TaskAwaitingInspection,
		StartedAt:   time.Now().UTC(),
		EvidenceIDs: []string{evID},
	}
	if err := orchStore.SaveExecution(ctx, exec); err != nil {
		t.Fatalf("save execution: %v", err)
	}

	evRecord, err := harness.NewEvidenceRecord(string(execID), string(taskID), wtID, harness.EvidenceKindCommandExecution, map[string]string{"tool": "lint", "status": "pass"})
	if err != nil {
		t.Fatalf("create evidence record: %v", err)
	}
	evRecord.ID = evID
	if err := harnessStore.SaveEvidence(ctx, evRecord); err != nil {
		t.Fatalf("save evidence: %v", err)
	}

	return task, exec
}

// TestP10Stress_ConcurrentMergesExactlyOneWinner dispatches N independent tasks
// to awaiting-inspection and merges each from K racing goroutines. Exactly one
// merge per task must succeed; all other attempts must fail closed.
func TestP10Stress_ConcurrentMergesExactlyOneWinner(t *testing.T) {
	adapter, orchStore, harnessStore := setupTestAdapter(t)
	ctx := context.Background()

	const tasks = 8
	const racers = 8

	type seed struct {
		id orchestra.TaskID
	}
	// setupTestAdapter's mockWorktreeInspector pins wt-1; all seeded tasks must
	// use that worktree ID or inspection fails closed on identity mismatch.
	// Each task needs its OWN evidence record because Execution.EvidenceIDs is an
	// exact pinned set checked against the harness store — sharing "ev-1" across
	// tasks would trip the mismatch guard during concurrent merges.
	seeds := make([]seed, 0, tasks)
	for i := 0; i < tasks; i++ {
		id := orchestra.TaskID(fmt.Sprintf("p10-stress-%d", i))
		seedTaskAwaitingInspectionUnique(t, orchStore, harnessStore, id, "wt-1")
		seeds = append(seeds, seed{id: id})
	}

	var totalSuccesses atomic.Int64
	var totalFailures atomic.Int64
	var wg sync.WaitGroup

	start := time.Now()
	for _, s := range seeds {
		for r := 0; r < racers; r++ {
			wg.Add(1)
			go func(taskID orchestra.TaskID) {
				defer wg.Done()
				if _, err := adapter.MergeTask(ctx, taskID, "p10 stress race"); err != nil {
					totalFailures.Add(1)
					return
				}
				totalSuccesses.Add(1)
			}(s.id)
		}
	}
	wg.Wait()
	elapsed := time.Since(start)

	if got := totalSuccesses.Load(); got != tasks {
		t.Fatalf("expected exactly %d merge winners (one per task), got %d (failures=%d)", tasks, got, totalFailures.Load())
	}

	// Verify every task reached success and emitted exactly one accepted event.
	for _, s := range seeds {
		persisted, err := orchStore.GetTask(ctx, s.id)
		if err != nil {
			t.Fatalf("read merged task %s: %v", s.id, err)
		}
		if persisted.Status != orchestra.TaskSuccess {
			t.Fatalf("task %s status = %s, want success", s.id, persisted.Status)
		}
		events, err := orchStore.ListEvents(ctx, s.id)
		if err != nil {
			t.Fatalf("list events %s: %v", s.id, err)
		}
		accepted := 0
		for _, ev := range events {
			if ev.Type == orchestra.EventInspectionAccepted {
				accepted++
			}
		}
		if accepted != 1 {
			t.Fatalf("task %s accepted events = %d, want exactly one", s.id, accepted)
		}
	}

	t.Logf("P10 stress merges: tasks=%d racers/task=%d successes=%d failures=%d elapsed=%s",
		tasks, racers, totalSuccesses.Load(), totalFailures.Load(), elapsed.Round(time.Millisecond))
}

// TestP10Stress_SequentialThroughputSoak performs 200 sequential save→merge
// cycles against real SQLite persistence to catch handle leaks, WAL growth, or
// sequence drift. It asserts p50-ish pacing stays under a generous bound for
// laptop-class hardware.
func TestP10Stress_SequentialThroughputSoak(t *testing.T) {
	adapter, orchStore, harnessStore := setupTestAdapter(t)
	ctx := context.Background()

	const cycles = 200
	deadline := 60 * time.Second

	start := time.Now()
	for i := 0; i < cycles; i++ {
		if time.Since(start) > deadline {
			t.Fatalf("soak exceeded deadline after %d/%d cycles", i, cycles)
		}
		id := orchestra.TaskID(fmt.Sprintf("p10-soak-%d", i))
		task, _ := seedTaskAwaitingInspectionUnique(t, orchStore, harnessStore, id, "wt-1")
		if _, err := adapter.MergeTask(ctx, task.ID, "p10 soak"); err != nil {
			t.Fatalf("merge cycle %d: %v", i, err)
		}
	}
	elapsed := time.Since(start)
	perOp := elapsed / cycles
	t.Logf("P10 soak: cycles=%d elapsed=%s per-merge=%s", cycles, elapsed.Round(time.Millisecond), perOp.Round(time.Microsecond))

	// Sanity: last task persisted.
	last, err := orchStore.GetTask(ctx, "p10-soak-199")
	if err != nil {
		t.Fatalf("read last soak task: %v", err)
	}
	if last.Status != orchestra.TaskSuccess {
		t.Fatalf("last soak status = %s, want success", last.Status)
	}
}

// TestP10Stress_GovernorAdmissionUnderLoad asserts the resource governor keeps
// admitting in-budget tasks and rejecting over-budget tasks even when hammered
// concurrently by many goroutines.
func TestP10Stress_GovernorAdmissionUnderLoad(t *testing.T) {
	ctx := context.Background()
	piProfile := harness.DefaultResourceRequirements(harness.EngineClassPi)

	gov := harness.NewGovernor(
		harness.WithBudgets(map[harness.EngineClass]harness.ResourceBudget{
			harness.EngineClassPi: {MaxWorkers: 4, MaxMemoryMB: piProfile.PeakMemoryMB * 4, MaxCPUPriority: 8},
		}))

	const workers = 32
	var admitted atomic.Int64
	var rejected atomic.Int64
	var wg sync.WaitGroup

	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			inBudget := idx%2 == 0
			mem := piProfile.MinMemoryMB
			if !inBudget {
				mem = piProfile.PeakMemoryMB*4 + 1 // guaranteed over budget
			}
			res, err := gov.Admit(ctx, harness.TaskDescriptor{
				TaskID:               fmt.Sprintf("p10-gov-%d", idx),
				EngineClass:          harness.EngineClassPi,
				Priority:             harness.PriorityNormal,
				EstimatedMemoryMB:    mem,
				EstimatedCPUPriority: piProfile.CPUPriority,
			})
			if err != nil {
				rejected.Add(1)
				return
			}
			admitted.Add(1)
			res.Release()
		}(i)
	}
	wg.Wait()

	if got := admitted.Load(); got != workers/2 {
		t.Fatalf("expected %d admitted in-budget, got %d (rejected=%d)", workers/2, got, rejected.Load())
	}
	if got := rejected.Load(); got != workers/2 {
		t.Fatalf("expected %d rejected over-budget, got %d", workers/2, got)
	}
	t.Logf("P10 governor: admitted=%d rejected=%d workers=%d", admitted.Load(), rejected.Load(), workers)
}

// TestP10Stress_SQLiteReopenDurability commits merges, closes the store, reopens
// it, and verifies the terminal state survived a process restart boundary.
func TestP10Stress_SQLiteReopenDurability(t *testing.T) {
	ctx := context.Background()
	path := t.TempDir() + "/p10-durability.db"

	store, err := orchestrasqlite.Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}

	const n = 20
	for i := 0; i < n; i++ {
		id := orchestra.TaskID(fmt.Sprintf("p10-durable-%d", i))
		parent := orchestra.Task{
			ID: id, ProjectID: "p-1", WorktreeID: "wt-1",
			Title: "p10 durability", AcceptanceCriteria: []string{"survives restart"},
			Status: orchestra.TaskAwaitingInspection, AttemptCount: 1, MaxAttempts: 3,
			CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC(), SchemaVersion: orchestra.ContractVersion,
		}
		if err := store.SaveTask(ctx, parent); err != nil {
			_ = store.Close()
			t.Fatalf("save %s: %v", id, err)
		}
		if err := store.SaveExecution(ctx, orchestra.Execution{
			ID: orchestra.ExecutionID(string(id) + "-1"), TaskID: id, Attempt: 1,
			Status: orchestra.TaskAwaitingInspection, StartedAt: time.Now().UTC(),
		}); err != nil {
			_ = store.Close()
			t.Fatalf("save exec %s: %v", id, err)
		}
		res, _ := orchestra.ApplyTransition(parent, orchestra.TransitionInspectAccepted)
		res.Task.UpdatedAt = time.Now().UTC()
		acceptedAt := time.Now().UTC()
		if err := store.CommitMerge(ctx, orchestra.MergeCommit{
			MergedTask: res.Task,
			AcceptedEvent: orchestra.TaskEvent{
				TaskID: id, ExecutionID: orchestra.ExecutionID(string(id) + "-1"), Type: orchestra.EventInspectionAccepted,
				From: orchestra.TaskAwaitingInspection, To: orchestra.TaskSuccess,
				Message: "p10 durability", OccurredAt: acceptedAt,
			},
		}); err != nil {
			_ = store.Close()
			t.Fatalf("commit %s: %v", id, err)
		}
	}
	if err := store.Close(); err != nil {
		t.Fatalf("close store: %v", err)
	}

	reopened, err := orchestrasqlite.Open(ctx, path)
	if err != nil {
		t.Fatalf("reopen db: %v", err)
	}
	defer reopened.Close()

	for i := 0; i < n; i++ {
		id := orchestra.TaskID(fmt.Sprintf("p10-durable-%d", i))
		got, err := reopened.GetTask(ctx, id)
		if err != nil {
			t.Fatalf("get %s after reopen: %v", id, err)
		}
		if got.Status != orchestra.TaskSuccess {
			t.Fatalf("task %s after reopen = %s, want success", id, got.Status)
		}
	}
	t.Logf("P10 durability: %d merges survived close/reopen", n)
}
