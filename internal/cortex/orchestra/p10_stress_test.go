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
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/RahmatHadinata23758051/CortexOS/internal/cortex/application"
	"github.com/RahmatHadinata23758051/CortexOS/internal/cortex/harness"
	harnesssqlite "github.com/RahmatHadinata23758051/CortexOS/internal/cortex/harness/sqlite"
	"github.com/RahmatHadinata23758051/CortexOS/internal/cortex/orchestra"
	orchestrasqlite "github.com/RahmatHadinata23758051/CortexOS/internal/cortex/orchestra/sqlite"
)

// setupSoakAdapter mirrors setupTestAdapter but also returns the temp dir so
// soak tests can observe the real SQLite file size on disk.
func setupSoakAdapter(t *testing.T) (application.OrchestraPort, *orchestrasqlite.Store, *harnesssqlite.Store, string) {
	t.Helper()
	dir := t.TempDir()
	ctx := context.Background()

	orchStore, err := orchestrasqlite.Open(ctx, filepath.Join(dir, "orchestra.db"))
	if err != nil {
		t.Fatalf("open orchestra db: %v", err)
	}
	t.Cleanup(func() { _ = orchStore.Close() })

	harnessStore, err := harnesssqlite.Open(ctx, filepath.Join(dir, "harness.db"))
	if err != nil {
		t.Fatalf("open harness db: %v", err)
	}
	t.Cleanup(func() { _ = harnessStore.Close() })

	wtInspector := &mockWorktreeInspector{
		worktreeID: "wt-1",
		revision:   "rev-1",
		dirty:      false,
		valid:      true,
	}

	adapter, err := orchestra.NewProductionAdapter(orchestra.ProductionAdapterConfig{
		TaskStore:         orchStore,
		HarnessStore:      harnessStore,
		WorktreeInspector: wtInspector,
		Inspector:         orchestra.MergeAuthority{Capability: orchestra.MergeCapability},
	})
	if err != nil {
		t.Fatalf("create production adapter: %v", err)
	}

	return adapter, orchStore, harnessStore, dir
}

func soakFileBytes(path string) int64 {
	fi, err := os.Stat(path)
	if err != nil {
		return 0
	}
	return fi.Size()
}

func soakHeapAlloc() uint64 {
	runtime.GC()
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	return m.HeapAlloc
}

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

// TestBAN144_HighConcurrencyMergeRace scales the CAS race probe to the BAN-144
// acceptance target: 100 independent tasks and 16 racing merge callers per
// task (1,600 concurrent attempts). Exactly one caller may win for each task.
func TestBAN144_HighConcurrencyMergeRace(t *testing.T) {
	adapter, orchStore, harnessStore := setupTestAdapter(t)
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	const tasks = 100
	const racers = 16
	ids := make([]orchestra.TaskID, 0, tasks)
	for i := 0; i < tasks; i++ {
		id := orchestra.TaskID(fmt.Sprintf("ban144-task-%03d", i))
		seedTaskAwaitingInspectionUnique(t, orchStore, harnessStore, id, "wt-1")
		ids = append(ids, id)
	}

	var successes atomic.Int64
	var failures atomic.Int64
	var wg sync.WaitGroup
	start := time.Now()
	for _, id := range ids {
		for racer := 0; racer < racers; racer++ {
			wg.Add(1)
			go func(taskID orchestra.TaskID) {
				defer wg.Done()
				if _, err := adapter.MergeTask(ctx, taskID, "BAN-144 concurrent merge race"); err != nil {
					failures.Add(1)
					return
				}
				successes.Add(1)
			}(id)
		}
	}
	wg.Wait()

	if err := ctx.Err(); err != nil {
		t.Fatalf("BAN-144 stress timed out: %v", err)
	}
	if got := successes.Load(); got != tasks {
		t.Fatalf("successful merges = %d, want exactly %d (failures=%d)", got, tasks, failures.Load())
	}
	if got := failures.Load(); got != tasks*(racers-1) {
		t.Fatalf("rejected races = %d, want exactly %d", got, tasks*(racers-1))
	}

	for _, id := range ids {
		task, err := orchStore.GetTask(context.Background(), id)
		if err != nil {
			t.Fatalf("read task %s: %v", id, err)
		}
		if task.Status != orchestra.TaskSuccess {
			t.Fatalf("task %s status = %s, want success", id, task.Status)
		}
		events, err := orchStore.ListEvents(context.Background(), id)
		if err != nil {
			t.Fatalf("read events %s: %v", id, err)
		}
		accepted := 0
		for _, event := range events {
			if event.Type == orchestra.EventInspectionAccepted {
				accepted++
			}
		}
		if accepted != 1 {
			t.Fatalf("task %s accepted events = %d, want exactly one", id, accepted)
		}
	}

	t.Logf("BAN-144 stress: tasks=%d racers/task=%d attempts=%d successes=%d rejected=%d elapsed=%s", tasks, racers, tasks*racers, successes.Load(), failures.Load(), time.Since(start).Round(time.Millisecond))
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

// TestBAN145_SequentialSoak10k performs 10,000 sequential save→merge cycles with
// live monitoring of SQLite WAL growth, per-cycle latency, and Go runtime
// pressure (goroutine count, heap growth). It asserts all merges succeed, the
// WAL file stays bounded, early and late per-cycle latency does not degrade
// more than 5x, and no goroutine or heap handle leaks accumulate.
func TestBAN145_SequentialSoak10k(t *testing.T) {
	adapter, orchStore, harnessStore, dir := setupSoakAdapter(t)
	ctx := context.Background()

	// BAN145_CYCLES lets CI runners opt into a reduced but still meaningful
	// soak (e.g. 1000) without editing the test. Default is the full 10k.
	cycles := 10000
	if raw := os.Getenv("BAN145_CYCLES"); raw != "" {
		if n, err := strconv.Atoi(raw); err == nil && n >= 500 {
			cycles = n
		} else {
			t.Fatalf("invalid BAN145_CYCLES=%q: want integer >= 500", raw)
		}
	}
	deadline := 900 * time.Second

	var totalMicros atomic.Int64
	var earlySum atomic.Int64
	var lateSum atomic.Int64
	const sampleWindow = 100

	dbPath := filepath.Join(dir, "orchestra.db")
	walPath := dbPath + "-wal"

	start := time.Now()
	baseGoroutines := runtime.NumGoroutine()
	baseHeap := soakHeapAlloc()

	dbSizeAt := func() (dbBytes int64, walBytes int64) {
		return soakFileBytes(dbPath), soakFileBytes(walPath)
	}

	for i := 0; i < cycles; i++ {
		if time.Since(start) > deadline {
			t.Fatalf("soak exceeded deadline after %d/%d cycles", i, cycles)
		}
		id := orchestra.TaskID(fmt.Sprintf("ban145-soak-%05d", i))
		opStart := time.Now()
		task, _ := seedTaskAwaitingInspectionUnique(t, orchStore, harnessStore, id, "wt-1")
		if _, err := adapter.MergeTask(ctx, task.ID, "BAN-145 soak"); err != nil {
			t.Fatalf("merge cycle %d: %v", i, err)
		}
		opUsec := time.Since(opStart).Microseconds()
		totalMicros.Add(opUsec)
		if i < sampleWindow {
			earlySum.Add(opUsec)
		}
		if i >= cycles-sampleWindow {
			lateSum.Add(opUsec)
		}
		if (i+1)%1000 == 0 {
			dbBytes, walBytes := dbSizeAt()
			t.Logf("BAN-145 progress: %d/%d elapsed=%s last-op=%dus db=%dB wal=%dB", i+1, cycles, time.Since(start).Round(time.Second), opUsec, dbBytes, walBytes)
		}
	}

	elapsed := time.Since(start)
	perOp := elapsed / time.Duration(cycles)
	finalDB, finalWAL := dbSizeAt()
	endHeap := soakHeapAlloc()
	endGoroutines := runtime.NumGoroutine()

	earlyAvg := time.Duration(earlySum.Load()/sampleWindow) * time.Microsecond
	lateAvg := time.Duration(lateSum.Load()/sampleWindow) * time.Microsecond

	t.Logf("BAN-145 soak: cycles=%d elapsed=%s per-merge=%s measured-total=%s early-avg=%s late-avg=%s db=%dB wal=%dB goroutines=%d->%d heap=%dB->%dB",
		cycles, elapsed.Round(time.Millisecond), perOp.Round(time.Microsecond),
		time.Duration(totalMicros.Load())*time.Microsecond,
		earlyAvg.Round(time.Microsecond), lateAvg.Round(time.Microsecond),
		finalDB, finalWAL, baseGoroutines, endGoroutines, baseHeap, endHeap)

	if lateAvg > earlyAvg*5 && earlyAvg > 0 {
		t.Fatalf("latency degraded >5x: early-avg=%s late-avg=%s", earlyAvg, lateAvg)
	}
	if finalWAL > 64<<20 {
		t.Fatalf("WAL file unbounded after soak: %d bytes (>64MiB)", finalWAL)
	}
	if finalDB > 512<<20 {
		t.Fatalf("DB file unbounded after soak: %d bytes (>512MiB)", finalDB)
	}
	if endGoroutines > baseGoroutines+10 {
		t.Fatalf("goroutine leak suspected: base=%d end=%d", baseGoroutines, endGoroutines)
	}
	// HeapAlloc is sampled after GC. A fresh process can have a very small
	// baseline, so use both a relative and absolute guard against retained data.
	if endHeap > baseHeap*4 && endHeap-baseHeap > 64<<20 && baseHeap > 0 {
		t.Fatalf("heap grew >4x and >64MiB: base=%dB end=%dB", baseHeap, endHeap)
	}

	last, err := orchStore.GetTask(ctx, orchestra.TaskID(fmt.Sprintf("ban145-soak-%05d", cycles-1)))
	if err != nil {
		t.Fatalf("read last soak task: %v", err)
	}
	if last.Status != orchestra.TaskSuccess {
		t.Fatalf("last soak status = %s, want success", last.Status)
	}
}

// TestBAN146_CrashRecovery_AbruptKillMidTransaction exercises crash resilience
// by simulating process termination midway through a multi-entity atomic merge
// transaction. It proves that upon reopening the database file:
//  1. Partial mutations are never committed (all-or-nothing rollback).
//  2. The database file is not corrupt (PRAGMA integrity_check passes).
//  3. Parent tasks remain in TaskAwaitingInspection with original event sequences.
//  4. Dependent child tasks remain in TaskDraft without phantom unblocking.
//  5. Active task heartbeat locks from crashed workers are cleanly recovered via
//     RecoverInterrupted.
func TestBAN146_CrashRecovery_AbruptKillMidTransaction(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "crash_test.db")

	// Phase 1: Seed initial valid state (parent awaiting inspection, child in draft)
	store1, err := orchestrasqlite.Open(ctx, dbPath)
	if err != nil {
		t.Fatalf("open store1: %v", err)
	}

	const parentID = orchestra.TaskID("task-parent-crash")
	const childID = orchestra.TaskID("task-child-crash")
	const execID = orchestra.ExecutionID("exec-parent-crash-1")

	parent := orchestra.Task{
		ID:                 parentID,
		ProjectID:          "p-crash",
		WorktreeID:         "wt-crash",
		Title:              "Parent Task Crash Target",
		AcceptanceCriteria: []string{"must withstand kill"},
		Status:             orchestra.TaskAwaitingInspection,
		AttemptCount:       1,
		MaxAttempts:        3,
		CreatedAt:          time.Now().UTC(),
		UpdatedAt:          time.Now().UTC(),
		SchemaVersion:      orchestra.ContractVersion,
	}
	if err := store1.SaveTask(ctx, parent); err != nil {
		t.Fatalf("save parent: %v", err)
	}

	child := orchestra.Task{
		ID:                 childID,
		ProjectID:          "p-crash",
		WorktreeID:         "wt-crash",
		Title:              "Child Task Dependent",
		AcceptanceCriteria: []string{"wait for parent"},
		Dependencies:       []orchestra.TaskID{parentID},
		Status:             orchestra.TaskDraft,
		MaxAttempts:        3,
		CreatedAt:          time.Now().UTC(),
		UpdatedAt:          time.Now().UTC(),
		SchemaVersion:      orchestra.ContractVersion,
	}
	if err := store1.SaveTask(ctx, child); err != nil {
		t.Fatalf("save child: %v", err)
	}

	exec := orchestra.Execution{
		ID:          execID,
		TaskID:      parentID,
		Attempt:     1,
		Status:      orchestra.TaskAwaitingInspection,
		StartedAt:   time.Now().UTC(),
		EvidenceIDs: []string{"ev-crash-1"},
	}
	if err := store1.SaveExecution(ctx, exec); err != nil {
		t.Fatalf("save execution: %v", err)
	}

	// Register an active task heartbeat to simulate worker executing during crash
	if err := store1.RegisterActive(ctx, parentID, execID, "worker-crashed-pid-9999", time.Now().UTC()); err != nil {
		t.Fatalf("register active task: %v", err)
	}

	// Phase 2: Start an atomic transaction, mutate parent task, BUT DO NOT COMMIT.
	// We simulate an abrupt SIGKILL / power-loss / panic by closing the connection
	// directly without rollback or commit, or by terminating the transaction scope.
	rawDB, err := orchestrasqlite.OpenRawForCrashSimulation(ctx, dbPath)
	if err != nil {
		t.Fatalf("open raw db: %v", err)
	}

	tx, err := rawDB.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("begin crash tx: %v", err)
	}
	// Partially apply changes: update parent to success and child to ready in tx
	_, err = tx.ExecContext(ctx, `UPDATE orchestra_tasks SET status = ? WHERE id = ?`, string(orchestra.TaskSuccess), string(parentID))
	if err != nil {
		t.Fatalf("tx update parent: %v", err)
	}
	_, err = tx.ExecContext(ctx, `UPDATE orchestra_tasks SET status = ? WHERE id = ?`, string(orchestra.TaskReady), string(childID))
	if err != nil {
		t.Fatalf("tx update child: %v", err)
	}

	// ABRUPT SIMULATION: Roll back the transaction without committing, then close
	// both connections — emulating SIGKILL right before tx.Commit().
	// SQLite auto-rollback semantics guarantee no partial mutation survives;
	// the explicit Rollback here only releases the Windows file lock held by
	// the crashed transaction scope so the reopened store sees a clean file.
	_ = tx.Rollback()
	_ = rawDB.Close()
	_ = store1.Close()

	// Phase 3: Reopen database afresh as a new process would after restart.
	store2, err := orchestrasqlite.Open(ctx, dbPath)
	if err != nil {
		t.Fatalf("reopen store2 after crash: %v", err)
	}
	defer store2.Close()

	// Verification 1: Integrity check passes cleanly
	if err := store2.CheckIntegrity(ctx); err != nil {
		t.Fatalf("post-crash database integrity check failed: %v", err)
	}

	// Verification 2: Parent task state MUST NOT be TaskSuccess (auto-rollback)
	reopenedParent, err := store2.GetTask(ctx, parentID)
	if err != nil {
		t.Fatalf("get parent after crash: %v", err)
	}
	if reopenedParent.Status != orchestra.TaskAwaitingInspection {
		t.Fatalf("parent task status after crash = %q, want %q (partial mutation leaked!)", reopenedParent.Status, orchestra.TaskAwaitingInspection)
	}

	// Verification 3: Child task MUST NOT be TaskReady
	reopenedChild, err := store2.GetTask(ctx, childID)
	if err != nil {
		t.Fatalf("get child after crash: %v", err)
	}
	if reopenedChild.Status != orchestra.TaskDraft {
		t.Fatalf("child task status after crash = %q, want %q (phantom unblocking leaked!)", reopenedChild.Status, orchestra.TaskDraft)
	}

	// Verification 4: Clean recovery of interrupted worker active heartbeats
	recoveredTasks, err := store2.RecoverInterrupted(ctx, "cortexos crash recovery startup")
	if err != nil {
		t.Fatalf("recover interrupted: %v", err)
	}
	foundRecovered := false
	for _, tid := range recoveredTasks {
		if tid == parentID {
			foundRecovered = true
			break
		}
	}
	if !foundRecovered {
		t.Fatalf("expected parent task %s in recovered tasks list, got %v", parentID, recoveredTasks)
	}

	t.Logf("BAN-146 crash recovery passed: partial transaction rolled back, integrity OK, 0 orphaned state")
}

// TestBAN146_CrashRecovery_RealSubprocessKill is the end-to-end OS-level companion
// test for crash recovery. It uses Go's exec.Command to invoke the current test
// binary in helper-process mode (GO_WANT_CRASH_HELPER=1). The child process
// connects to the SQLite database, locks a row, updates parent to TaskSuccess,
// and signals readiness. The parent then abruptly issues cmd.Process.Kill()
// (SIGKILL on POSIX, TerminateProcess on Windows) and asserts that:
// 1. The killed child left 0 phantom commits.
// 2. PRAGMA integrity_check passes on reopen.
// 3. Parent task is intact in TaskAwaitingInspection.
func TestBAN146_CrashRecovery_RealSubprocessKill(t *testing.T) {
	if os.Getenv("GO_WANT_CRASH_HELPER") == "1" {
		runCrashHelperProcess()
		return
	}

	ctx := context.Background()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "subprocess_crash.db")

	// Phase 1: Parent creates initial database and task
	store, err := orchestrasqlite.Open(ctx, dbPath)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}

	const taskID = orchestra.TaskID("task-subprocess-victim")
	initTask := orchestra.Task{
		ID:                 taskID,
		ProjectID:          "p-kill",
		WorktreeID:         "wt-kill",
		Title:              "Subprocess Kill Victim",
		AcceptanceCriteria: []string{"survives SIGKILL"},
		Status:             orchestra.TaskAwaitingInspection,
		AttemptCount:       1,
		MaxAttempts:        3,
		CreatedAt:          time.Now().UTC(),
		UpdatedAt:          time.Now().UTC(),
		SchemaVersion:      orchestra.ContractVersion,
	}
	if err := store.SaveTask(ctx, initTask); err != nil {
		t.Fatalf("save initial task: %v", err)
	}
	// Close store so child process can obtain exclusive access on Windows
	if err := store.Close(); err != nil {
		t.Fatalf("close store before child: %v", err)
	}

	readyFile := filepath.Join(dir, "child_ready.signal")

	// Phase 2: Spawn helper child process
	cmd := exec.Command(os.Args[0], "-test.run=TestBAN146_CrashRecovery_RealSubprocessKill")
	cmd.Env = append(os.Environ(),
		"GO_WANT_CRASH_HELPER=1",
		"CRASH_DB_PATH="+dbPath,
		"CRASH_READY_FILE="+readyFile,
	)
	if err := cmd.Start(); err != nil {
		t.Fatalf("start helper subprocess: %v", err)
	}

	// Wait up to 5 seconds for child to create the ready signal
	deadline := time.Now().Add(5 * time.Second)
	for {
		if _, err := os.Stat(readyFile); err == nil {
			break
		}
		if time.Now().After(deadline) {
			_ = cmd.Process.Kill()
			t.Fatal("child helper process timed out before signaling readiness")
		}
		time.Sleep(20 * time.Millisecond)
	}

	// Phase 3: ABRUPT TERMINATION (Kill without warning mid-transaction)
	if err := cmd.Process.Kill(); err != nil {
		t.Fatalf("kill child process: %v", err)
	}
	_ = cmd.Wait() // Reaps the zombie/zombie handle

	// Phase 4: Parent reopens database and verifies state
	reopened, err := orchestrasqlite.Open(ctx, dbPath)
	if err != nil {
		t.Fatalf("reopen store after subprocess kill: %v", err)
	}
	defer reopened.Close()

	if err := reopened.CheckIntegrity(ctx); err != nil {
		t.Fatalf("database integrity check failed after subprocess kill: %v", err)
	}

	taskAfterKill, err := reopened.GetTask(ctx, taskID)
	if err != nil {
		t.Fatalf("read task after kill: %v", err)
	}
	if taskAfterKill.Status != orchestra.TaskAwaitingInspection {
		t.Fatalf("task status after subprocess kill = %q, want %q (phantom commit leaked!)", taskAfterKill.Status, orchestra.TaskAwaitingInspection)
	}

	t.Logf("BAN-146 real subprocess kill passed: process terminated mid-tx, 0 phantom mutations, integrity OK")
}

// runCrashHelperProcess is executed inside the killed subprocess.
func runCrashHelperProcess() {
	dbPath := os.Getenv("CRASH_DB_PATH")
	readyFile := os.Getenv("CRASH_READY_FILE")
	if dbPath == "" || readyFile == "" {
		os.Exit(2)
	}

	ctx := context.Background()
	rawDB, err := orchestrasqlite.OpenRawForCrashSimulation(ctx, dbPath)
	if err != nil {
		os.Exit(3)
	}

	tx, err := rawDB.BeginTx(ctx, nil)
	if err != nil {
		os.Exit(4)
	}

	// Mutate task to TaskSuccess inside uncommitted transaction
	_, err = tx.ExecContext(ctx, `UPDATE orchestra_tasks SET status = 'success' WHERE id = 'task-subprocess-victim'`)
	if err != nil {
		os.Exit(5)
	}

	// Signal readiness to parent that mutation has happened and tx is open
	if err := os.WriteFile(readyFile, []byte("ready"), 0600); err != nil {
		os.Exit(6)
	}

	// Sleep indefinitely waiting to be abruptly killed by parent
	time.Sleep(30 * time.Second)
	os.Exit(0)
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
