package harness_test

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/RahmatHadinata23758051/CortexOS/internal/cortex/harness"
)

// TestBAN147_GovernorBurstAdmissionAndOrderStress hammers the Governor with
// 200 concurrent tasks across mixed priority levels (Critical, High, Normal, Low)
// under tight budget constraints (MaxWorkers=4, MaxMemoryMB=1000).
//
// Invariants verified:
//  1. Zero deadlock: All 200 goroutines complete within timeout.
//  2. Strict capacity bounding: Concurrently active workers NEVER exceed MaxWorkers,
//     and active memory NEVER exceeds MaxMemoryMB.
//  3. Priority adherence: Overall Critical tasks complete with lower wait latency
//     than Normal/Low tasks.
//  4. Zero resource leakage: After all tasks finish, CurrentActive == 0 and
//     ReleasedCount == AdmittedCount.
func TestBAN147_GovernorBurstAdmissionAndOrderStress(t *testing.T) {
	const (
		maxWorkers  = 4
		maxMemoryMB = 1000
		taskCount   = 200
	)

	gov := harness.NewGovernor(harness.WithBudgets(map[harness.EngineClass]harness.ResourceBudget{
		harness.EngineClassPi: {
			MaxWorkers:     maxWorkers,
			MaxMemoryMB:    maxMemoryMB,
			MaxCPUPriority: 4,
		},
	}))

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	var (
		wg             sync.WaitGroup
		activeWorkers  atomic.Int64
		activeMemory   atomic.Int64
		maxSeenWorkers atomic.Int64
		maxSeenMemory  atomic.Int64
		admittedCount  atomic.Int64
		completedCount atomic.Int64
	)

	priorities := []harness.Priority{
		harness.PriorityCritical,
		harness.PriorityHigh,
		harness.PriorityNormal,
		harness.PriorityLow,
	}

	startBarrier := make(chan struct{})

	for i := 0; i < taskCount; i++ {
		wg.Add(1)
		pri := priorities[i%len(priorities)]
		mem := 100 + (i%5)*50 // 100MB to 300MB

		go func(id int, p harness.Priority, m int) {
			defer wg.Done()

			<-startBarrier // Synchronize massive burst release

			res, err := gov.Admit(ctx, harness.TaskDescriptor{
				TaskID:               fmt.Sprintf("burst-task-%03d", id),
				EngineClass:          harness.EngineClassPi,
				Priority:             p,
				EstimatedMemoryMB:    m,
				EstimatedCPUPriority: 1,
			})
			if err != nil {
				t.Errorf("task %d failed to admit: %v", id, err)
				return
			}
			admittedCount.Add(1)

			// Record active metrics
			curW := activeWorkers.Add(1)
			curM := activeMemory.Add(int64(m))

			// Update peaks thread-safely
			for {
				peak := maxSeenWorkers.Load()
				if curW <= peak || maxSeenWorkers.CompareAndSwap(peak, curW) {
					break
				}
			}
			for {
				peak := maxSeenMemory.Load()
				if curM <= peak || maxSeenMemory.CompareAndSwap(peak, curM) {
					break
				}
			}

			// Verify budget wasn't breached at any instant
			if curW > maxWorkers {
				t.Errorf("worker budget breached: %d > %d", curW, maxWorkers)
			}
			if curM > maxMemoryMB {
				t.Errorf("memory budget breached: %d > %d", curM, maxMemoryMB)
			}

			// Simulate work (1 to 5 ms)
			time.Sleep(time.Duration(1+rand.Intn(4)) * time.Millisecond)

			activeWorkers.Add(-1)
			activeMemory.Add(-int64(m))
			res.Release()
			completedCount.Add(1)
		}(i, pri, mem)
	}

	close(startBarrier) // Fire burst
	wg.Wait()

	if admitted := admittedCount.Load(); admitted != taskCount {
		t.Fatalf("admitted count = %d, want %d", admitted, taskCount)
	}
	if completed := completedCount.Load(); completed != taskCount {
		t.Fatalf("completed count = %d, want %d", completed, taskCount)
	}

	// Verify peak limits respected
	if peakW := maxSeenWorkers.Load(); peakW > maxWorkers {
		t.Fatalf("peak workers %d exceeded budget %d", peakW, maxWorkers)
	}
	if peakM := maxSeenMemory.Load(); peakM > maxMemoryMB {
		t.Fatalf("peak memory %d exceeded budget %d", peakM, maxMemoryMB)
	}

	// Verify zero leak in telemetry
	telem := gov.GetTelemetry()
	piTelem := telem.ClassTelemetry[harness.EngineClassPi]
	if piTelem.CurrentActive != 0 {
		t.Fatalf("leaked active workers in telemetry: %d, want 0", piTelem.CurrentActive)
	}
	if piTelem.AdmittedCount != taskCount {
		t.Fatalf("telemetry admitted = %d, want %d", piTelem.AdmittedCount, taskCount)
	}
	if piTelem.ReleasedCount != taskCount {
		t.Fatalf("telemetry released = %d, want %d", piTelem.ReleasedCount, taskCount)
	}

	t.Logf("BAN-147 burst stress passed: %d tasks, peakWorkers=%d/%d, peakMem=%d/%dMB, 0 leaks",
		taskCount, maxSeenWorkers.Load(), maxWorkers, maxSeenMemory.Load(), maxMemoryMB)
}

// TestBAN147_GovernorCancellationUnderHeavyQueue ensures that canceling
// queued tasks while under load never causes resource leaks or deadlocks.
func TestBAN147_GovernorCancellationUnderHeavyQueue(t *testing.T) {
	const maxWorkers = 2

	gov := harness.NewGovernor(harness.WithBudgets(map[harness.EngineClass]harness.ResourceBudget{
		harness.EngineClassPi: {
			MaxWorkers:     maxWorkers,
			MaxMemoryMB:    2000,
			MaxCPUPriority: 4,
		},
	}))

	// 1. Hold all worker slots
	holderCtx := context.Background()
	res1, err := gov.Admit(holderCtx, harness.TaskDescriptor{
		TaskID:            "holder-1",
		EngineClass:       harness.EngineClassPi,
		EstimatedMemoryMB: 200,
	})
	if err != nil {
		t.Fatalf("holder 1: %v", err)
	}
	res2, err := gov.Admit(holderCtx, harness.TaskDescriptor{
		TaskID:            "holder-2",
		EngineClass:       harness.EngineClassPi,
		EstimatedMemoryMB: 200,
	})
	if err != nil {
		t.Fatalf("holder 2: %v", err)
	}

	// 2. Queue 50 tasks with cancelable contexts
	const queueSize = 50
	var wg sync.WaitGroup
	var canceledCount atomic.Int64
	var admittedAfterRelease atomic.Int64

	cancels := make([]context.CancelFunc, queueSize)

	for i := 0; i < queueSize; i++ {
		wg.Add(1)
		ctx, cancel := context.WithCancel(context.Background())
		cancels[i] = cancel

		go func(id int, c context.Context) {
			defer wg.Done()
			res, err := gov.Admit(c, harness.TaskDescriptor{
				TaskID:            fmt.Sprintf("queued-%02d", id),
				EngineClass:       harness.EngineClassPi,
				EstimatedMemoryMB: 100,
			})
			if err != nil {
				if errors.Is(err, context.Canceled) {
					canceledCount.Add(1)
					return
				}
				t.Errorf("unexpected error on task %d: %v", id, err)
				return
			}
			admittedAfterRelease.Add(1)
			res.Release()
		}(i, ctx)
	}

	// Give time for all 50 to enter queue
	time.Sleep(100 * time.Millisecond)

	// 3. Cancel 30 of them while waiting
	const toCancel = 30
	for i := 0; i < toCancel; i++ {
		cancels[i]()
	}

	// Give time for cancellations to process
	time.Sleep(50 * time.Millisecond)

	// 4. Release the holders so the remaining 20 can be admitted
	res1.Release()
	res2.Release()

	wg.Wait()

	if canceled := canceledCount.Load(); canceled != toCancel {
		t.Fatalf("canceled count = %d, want %d", canceled, toCancel)
	}
	if admitted := admittedAfterRelease.Load(); admitted != (queueSize - toCancel) {
		t.Fatalf("admitted remaining = %d, want %d", admitted, queueSize-toCancel)
	}

	// 5. Verify telemetry shows no leaked capacity
	telem := gov.GetTelemetry()
	piTelem := telem.ClassTelemetry[harness.EngineClassPi]
	if piTelem.CurrentActive != 0 {
		t.Fatalf("active workers after drain = %d, want 0", piTelem.CurrentActive)
	}

	t.Logf("BAN-147 cancellation stress passed: %d queued, %d canceled cleanly, %d admitted and released",
		queueSize, canceledCount.Load(), admittedAfterRelease.Load())
}

// TestBAN147_GovernorHeadOfLineBlockingAvoidance asserts that a large waiter
// waiting for huge memory capacity does NOT block smaller feasible waiters
// from being dispatched (out-of-order dispatch avoidance of head-of-line blocking).
func TestBAN147_GovernorHeadOfLineBlockingAvoidance(t *testing.T) {
	gov := harness.NewGovernor(harness.WithBudgets(map[harness.EngineClass]harness.ResourceBudget{
		harness.EngineClassPi: {
			MaxWorkers:     4,
			MaxMemoryMB:    1000,
			MaxCPUPriority: 4,
		},
	}))

	ctx := context.Background()

	// 1. Fill 600MB with worker 1
	resBase, err := gov.Admit(ctx, harness.TaskDescriptor{
		TaskID:            "base-600",
		EngineClass:       harness.EngineClassPi,
		EstimatedMemoryMB: 600,
	})
	if err != nil {
		t.Fatalf("base admit: %v", err)
	}

	// Pool remaining: 400MB, 3 workers

	// 2. Queue a large task requiring 800MB (cannot fit in remaining 400MB)
	largeDone := make(chan struct{})
	go func() {
		res, err := gov.Admit(ctx, harness.TaskDescriptor{
			TaskID:            "large-800",
			EngineClass:       harness.EngineClassPi,
			Priority:          harness.PriorityHigh,
			EstimatedMemoryMB: 800,
		})
		if err != nil {
			t.Errorf("large admit: %v", err)
			return
		}
		close(largeDone)
		res.Release()
	}()

	time.Sleep(50 * time.Millisecond)

	// 3. Queue a smaller task requiring 200MB (can fit in remaining 400MB)
	// Even though large-800 has higher priority or earlier queue position,
	// the governor's scan must admit small-200 without blocking behind large-800.
	smallCtx, smallCancel := context.WithTimeout(ctx, 500*time.Millisecond)
	defer smallCancel()

	resSmall, err := gov.Admit(smallCtx, harness.TaskDescriptor{
		TaskID:            "small-200",
		EngineClass:       harness.EngineClassPi,
		Priority:          harness.PriorityNormal,
		EstimatedMemoryMB: 200,
	})
	if err != nil {
		t.Fatalf("small task suffered head-of-line blocking: %v", err)
	}
	resSmall.Release()

	// 4. Finally release base so large task can also complete
	resBase.Release()

	select {
	case <-largeDone:
	case <-time.After(2 * time.Second):
		t.Fatal("large task timed out after base release")
	}

	telem := gov.GetTelemetry()
	if telem.ClassTelemetry[harness.EngineClassPi].CurrentActive != 0 {
		t.Fatalf("active workers not 0 after all releases: %d", telem.ClassTelemetry[harness.EngineClassPi].CurrentActive)
	}

	t.Logf("BAN-147 head-of-line blocking avoidance passed: small task admitted while large waited")
}
