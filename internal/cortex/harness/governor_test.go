package harness

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestGovernorDefaults(t *testing.T) {
	g := NewGovernor()
	if g == nil {
		t.Fatal("NewGovernor returned nil")
	}
	if g.clock == nil {
		t.Fatal("clock not initialized")
	}
	budgets := g.GetClassBudget(EngineClassPi)
	if budgets.MaxWorkers != 4 {
		t.Errorf("default Pi MaxWorkers = %d, want 4", budgets.MaxWorkers)
	}
	if budgets.MaxMemoryMB != 2400 {
		t.Errorf("default Pi MaxMemoryMB = %d, want 2400", budgets.MaxMemoryMB)
	}
	nativeBudget := g.GetClassBudget(EngineClassNative)
	if nativeBudget.MaxWorkers != 0 {
		t.Errorf("default Native MaxWorkers = %d, want 0", nativeBudget.MaxWorkers)
	}
	ompBudget := g.GetClassBudget(EngineClassOMP)
	if ompBudget.MaxWorkers != 1 {
		t.Errorf("default OMP MaxWorkers = %d, want 1", ompBudget.MaxWorkers)
	}
}

func TestAdmitImmediateNativeUnlimited(t *testing.T) {
	g := NewGovernor(WithBudgets(map[EngineClass]ResourceBudget{
		EngineClassNative: {MaxWorkers: 0, MaxMemoryMB: 0, MaxCPUPriority: 0},
	}))
	res, err := g.Admit(context.Background(), TaskDescriptor{
		TaskID:               "t1",
		EngineClass:          EngineClassNative,
		Priority:             PriorityNormal,
		EstimatedMemoryMB:    100,
		EstimatedCPUPriority: 5,
	})
	if err != nil {
		t.Fatalf("Admit native: %v", err)
	}
	if res == nil {
		t.Fatal("reservation is nil")
	}
	if res.TaskID != "t1" {
		t.Errorf("TaskID = %q, want t1", res.TaskID)
	}
	if res.EngineClass != EngineClassNative {
		t.Errorf("EngineClass = %v, want native", res.EngineClass)
	}
	res.Release()
}

func TestAdmitRejectsTaskExceedingBudget(t *testing.T) {
	g := NewGovernor(WithBudgets(map[EngineClass]ResourceBudget{
		EngineClassPi: {MaxWorkers: 2, MaxMemoryMB: 500, MaxCPUPriority: 2},
	}))
	_, err := g.Admit(context.Background(), TaskDescriptor{
		TaskID:               "too-big",
		EngineClass:          EngineClassPi,
		Priority:             PriorityNormal,
		EstimatedMemoryMB:    600,
		EstimatedCPUPriority: 1,
	})
	if err == nil {
		t.Fatal("expected error for task exceeding memory budget")
	}
	if !errors.Is(err, ErrCapacityExceeded) {
		t.Errorf("error = %v, want ErrCapacityExceeded", err)
	}
}

func TestAdmitBlocksUntilCapacityFrees(t *testing.T) {
	g := NewGovernor(WithBudgets(map[EngineClass]ResourceBudget{
		EngineClassPi: {MaxWorkers: 1, MaxMemoryMB: 500, MaxCPUPriority: 2},
	}))

	res1, err := g.Admit(context.Background(), TaskDescriptor{
		TaskID:               "t1",
		EngineClass:          EngineClassPi,
		Priority:             PriorityNormal,
		EstimatedMemoryMB:    200,
		EstimatedCPUPriority: 1,
	})
	if err != nil {
		t.Fatalf("first admit: %v", err)
	}

	type result struct {
		res *Reservation
		err error
	}
	resultCh := make(chan result, 1)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	go func() {
		res, err := g.Admit(ctx, TaskDescriptor{
			TaskID:               "t2",
			EngineClass:          EngineClassPi,
			Priority:             PriorityNormal,
			EstimatedMemoryMB:    200,
			EstimatedCPUPriority: 1,
		})
		resultCh <- result{res: res, err: err}
	}()

	// Wait briefly to ensure t2 is waiting
	time.Sleep(50 * time.Millisecond)

	// Release res1 to allow t2 to be admitted
	res1.Release()

	select {
	case r := <-resultCh:
		if r.err != nil {
			t.Fatalf("second admit failed: %v", r.err)
		}
		if r.res == nil {
			t.Fatal("second reservation is nil")
		}
		r.res.Release()
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for t2 admission after res1 released")
	}
}

func TestPriorityOrderAdmission(t *testing.T) {
	g := NewGovernor(WithBudgets(map[EngineClass]ResourceBudget{
		EngineClassPi: {MaxWorkers: 1, MaxMemoryMB: 500, MaxCPUPriority: 2},
	}))

	resHeld, err := g.Admit(context.Background(), TaskDescriptor{
		TaskID:               "holder",
		EngineClass:          EngineClassPi,
		Priority:             PriorityLow,
		EstimatedMemoryMB:    200,
		EstimatedCPUPriority: 1,
	})
	if err != nil {
		t.Fatalf("holder admit: %v", err)
	}

	orderCh := make(chan string, 3)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	var wg sync.WaitGroup
	wg.Add(3)

	// Enqueue in low, high, critical order
	go func() {
		defer wg.Done()
		res, err := g.Admit(ctx, TaskDescriptor{
			TaskID:               "low",
			EngineClass:          EngineClassPi,
			Priority:             PriorityLow,
			EstimatedMemoryMB:    200,
			EstimatedCPUPriority: 1,
		})
		if err == nil {
			orderCh <- "low"
			res.Release()
		}
	}()
	time.Sleep(20 * time.Millisecond)

	go func() {
		defer wg.Done()
		res, err := g.Admit(ctx, TaskDescriptor{
			TaskID:               "high",
			EngineClass:          EngineClassPi,
			Priority:             PriorityHigh,
			EstimatedMemoryMB:    200,
			EstimatedCPUPriority: 1,
		})
		if err == nil {
			orderCh <- "high"
			res.Release()
		}
	}()
	time.Sleep(20 * time.Millisecond)

	go func() {
		defer wg.Done()
		res, err := g.Admit(ctx, TaskDescriptor{
			TaskID:               "critical",
			EngineClass:          EngineClassPi,
			Priority:             PriorityCritical,
			EstimatedMemoryMB:    200,
			EstimatedCPUPriority: 1,
		})
		if err == nil {
			orderCh <- "critical"
			res.Release()
		}
	}()
	time.Sleep(30 * time.Millisecond)

	// Release holder to start the chain
	resHeld.Release()

	wg.Wait()
	close(orderCh)

	admitted := make([]string, 0, 3)
	for id := range orderCh {
		admitted = append(admitted, id)
	}

	if len(admitted) != 3 {
		t.Fatalf("expected 3 admitted, got %d: %v", len(admitted), admitted)
	}
	if admitted[0] != "critical" || admitted[1] != "high" || admitted[2] != "low" {
		t.Errorf("admission order = %v, want [critical high low]", admitted)
	}
}

func TestFIFOOrderWithinPriority(t *testing.T) {
	g := NewGovernor(WithBudgets(map[EngineClass]ResourceBudget{
		EngineClassPi: {MaxWorkers: 1, MaxMemoryMB: 500, MaxCPUPriority: 2},
	}))

	resHeld, err := g.Admit(context.Background(), TaskDescriptor{
		TaskID:               "holder",
		EngineClass:          EngineClassPi,
		Priority:             PriorityNormal,
		EstimatedMemoryMB:    200,
		EstimatedCPUPriority: 1,
	})
	if err != nil {
		t.Fatalf("holder admit: %v", err)
	}

	orderCh := make(chan string, 2)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		res, err := g.Admit(ctx, TaskDescriptor{
			TaskID:               "first",
			EngineClass:          EngineClassPi,
			Priority:             PriorityNormal,
			EstimatedMemoryMB:    200,
			EstimatedCPUPriority: 1,
		})
		if err == nil {
			orderCh <- "first"
			res.Release()
		}
	}()
	time.Sleep(25 * time.Millisecond)

	go func() {
		defer wg.Done()
		res, err := g.Admit(ctx, TaskDescriptor{
			TaskID:               "second",
			EngineClass:          EngineClassPi,
			Priority:             PriorityNormal,
			EstimatedMemoryMB:    200,
			EstimatedCPUPriority: 1,
		})
		if err == nil {
			orderCh <- "second"
			res.Release()
		}
	}()
	time.Sleep(25 * time.Millisecond)

	resHeld.Release()
	wg.Wait()
	close(orderCh)

	admitted := make([]string, 0, 2)
	for id := range orderCh {
		admitted = append(admitted, id)
	}

	if len(admitted) != 2 {
		t.Fatalf("expected 2 admitted, got %d", len(admitted))
	}
	if admitted[0] != "first" || admitted[1] != "second" {
		t.Errorf("FIFO order = %v, want [first second]", admitted)
	}
}

func TestReleaseIsIdempotent(t *testing.T) {
	g := NewGovernor(WithBudgets(map[EngineClass]ResourceBudget{
		EngineClassNative: {MaxWorkers: 1, MaxMemoryMB: 100, MaxCPUPriority: 1},
	}))
	res, err := g.Admit(context.Background(), TaskDescriptor{
		TaskID:               "t1",
		EngineClass:          EngineClassNative,
		Priority:             PriorityNormal,
		EstimatedMemoryMB:    50,
		EstimatedCPUPriority: 1,
	})
	if err != nil {
		t.Fatalf("admit: %v", err)
	}
	res.Release()
	res.Release() // second call must not panic or decrement twice
	res.Release() // third call must not panic
	if !res.IsReleased() {
		t.Error("IsReleased false after multiple releases")
	}

	snap := g.GetTelemetry()
	if snap.TotalReleased != 1 {
		t.Errorf("TotalReleased = %d, want 1", snap.TotalReleased)
	}
}

func TestCancellationReturnsContextCanceled(t *testing.T) {
	g := NewGovernor(WithBudgets(map[EngineClass]ResourceBudget{
		EngineClassPi: {MaxWorkers: 1, MaxMemoryMB: 500, MaxCPUPriority: 2},
	}))

	resHeld, err := g.Admit(context.Background(), TaskDescriptor{
		TaskID:               "holder",
		EngineClass:          EngineClassPi,
		Priority:             PriorityLow,
		EstimatedMemoryMB:    200,
		EstimatedCPUPriority: 1,
	})
	if err != nil {
		t.Fatalf("holder admit: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		_, err := g.Admit(ctx, TaskDescriptor{
			TaskID:               "waiter",
			EngineClass:          EngineClassPi,
			Priority:             PriorityNormal,
			EstimatedMemoryMB:    200,
			EstimatedCPUPriority: 1,
		})
		done <- err
	}()

	time.Sleep(50 * time.Millisecond)
	cancel()

	select {
	case err := <-done:
		if err == nil {
			t.Fatal("expected error from canceled context")
		}
		if !errors.Is(err, context.Canceled) {
			t.Errorf("error = %v, want context.Canceled", err)
		}
	case <-time.After(time.Second):
		t.Fatal("Admit did not return after cancellation")
	}

	resHeld.Release()
}

func TestTimeoutReturnsErrCapacityExceeded(t *testing.T) {
	g := NewGovernor(WithBudgets(map[EngineClass]ResourceBudget{
		EngineClassPi: {MaxWorkers: 1, MaxMemoryMB: 500, MaxCPUPriority: 2},
	}))

	resHeld, err := g.Admit(context.Background(), TaskDescriptor{
		TaskID:               "holder",
		EngineClass:          EngineClassPi,
		Priority:             PriorityLow,
		EstimatedMemoryMB:    200,
		EstimatedCPUPriority: 1,
	})
	if err != nil {
		t.Fatalf("holder admit: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	_, err = g.Admit(ctx, TaskDescriptor{
		TaskID:               "waiter",
		EngineClass:          EngineClassPi,
		Priority:             PriorityNormal,
		EstimatedMemoryMB:    200,
		EstimatedCPUPriority: 1,
	})
	if err == nil {
		t.Fatal("expected error from timeout")
	}
	if !errors.Is(err, ErrCapacityExceeded) {
		t.Errorf("error = %v, want ErrCapacityExceeded", err)
	}

	resHeld.Release()
}

func TestReservationLeakFreeOnCancellation(t *testing.T) {
	g := NewGovernor(WithBudgets(map[EngineClass]ResourceBudget{
		EngineClassPi: {MaxWorkers: 2, MaxMemoryMB: 800, MaxCPUPriority: 4},
	}))

	// Fill the pool
	res1, err := g.Admit(context.Background(), TaskDescriptor{
		TaskID:               "r1",
		EngineClass:          EngineClassPi,
		Priority:             PriorityNormal,
		EstimatedMemoryMB:    200,
		EstimatedCPUPriority: 1,
	})
	if err != nil {
		t.Fatalf("r1: %v", err)
	}
	res2, err := g.Admit(context.Background(), TaskDescriptor{
		TaskID:               "r2",
		EngineClass:          EngineClassPi,
		Priority:             PriorityNormal,
		EstimatedMemoryMB:    200,
		EstimatedCPUPriority: 1,
	})
	if err != nil {
		t.Fatalf("r2: %v", err)
	}

	// Start a waiter that will be canceled
	ctx, cancel := context.WithCancel(context.Background())
	var waiterRes *Reservation
	done := make(chan error, 1)
	go func() {
		res, err := g.Admit(ctx, TaskDescriptor{
			TaskID:               "waiter",
			EngineClass:          EngineClassPi,
			Priority:             PriorityNormal,
			EstimatedMemoryMB:    200,
			EstimatedCPUPriority: 1,
		})
		waiterRes = res
		done <- err
	}()

	time.Sleep(50 * time.Millisecond)
	cancel()
	err = <-done
	if !errors.Is(err, context.Canceled) {
		t.Errorf("waiter error = %v, want Canceled", err)
	}
	if waiterRes != nil {
		t.Errorf("waiterRes = %v, want nil on cancel", waiterRes)
	}

	// Release one and verify the other can still get capacity
	res1.Release()
	time.Sleep(50 * time.Millisecond)

	// No leakage: the canceled waiter consumed no capacity
	telem := g.GetTelemetry()
	if telem.ClassTelemetry[EngineClassPi].CurrentActive != 1 {
		t.Errorf("active workers after cancel+release = %d, want 1", telem.ClassTelemetry[EngineClassPi].CurrentActive)
	}
	res2.Release()
}

func TestReservationLeakFreeOnTimeout(t *testing.T) {
	g := NewGovernor(WithBudgets(map[EngineClass]ResourceBudget{
		EngineClassPi: {MaxWorkers: 1, MaxMemoryMB: 500, MaxCPUPriority: 2},
	}))

	res1, err := g.Admit(context.Background(), TaskDescriptor{
		TaskID:               "r1",
		EngineClass:          EngineClassPi,
		Priority:             PriorityNormal,
		EstimatedMemoryMB:    200,
		EstimatedCPUPriority: 1,
	})
	if err != nil {
		t.Fatalf("r1: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	_, err = g.Admit(ctx, TaskDescriptor{
		TaskID:               "waiter",
		EngineClass:          EngineClassPi,
		Priority:             PriorityNormal,
		EstimatedMemoryMB:    200,
		EstimatedCPUPriority: 1,
	})
	if !errors.Is(err, ErrCapacityExceeded) {
		t.Errorf("timeout error = %v, want ErrCapacityExceeded", err)
	}

	// Release the holder
	res1.Release()

	// No leakage: the timed-out waiter consumed no capacity
	telem := g.GetTelemetry()
	if telem.ClassTelemetry[EngineClassPi].CurrentActive != 0 {
		t.Errorf("active workers after timeout+release = %d, want 0", telem.ClassTelemetry[EngineClassPi].CurrentActive)
	}
}

func TestEngineClassIsolation(t *testing.T) {
	g := NewGovernor(WithBudgets(map[EngineClass]ResourceBudget{
		EngineClassNative: {MaxWorkers: 1, MaxMemoryMB: 200, MaxCPUPriority: 2},
		EngineClassPi:     {MaxWorkers: 1, MaxMemoryMB: 500, MaxCPUPriority: 2},
		EngineClassOMP:    {MaxWorkers: 1, MaxMemoryMB: 500, MaxCPUPriority: 2},
	}))

	// Saturate Pi
	resPi, err := g.Admit(context.Background(), TaskDescriptor{
		TaskID:               "pi1",
		EngineClass:          EngineClassPi,
		Priority:             PriorityNormal,
		EstimatedMemoryMB:    300,
		EstimatedCPUPriority: 1,
	})
	if err != nil {
		t.Fatalf("pi1: %v", err)
	}

	// OMP must still admit
	resOmp, err := g.Admit(context.Background(), TaskDescriptor{
		TaskID:               "omp1",
		EngineClass:          EngineClassOMP,
		Priority:             PriorityNormal,
		EstimatedMemoryMB:    300,
		EstimatedCPUPriority: 1,
	})
	if err != nil {
		t.Fatalf("omp1: %v", err)
	}

	// Native must still admit
	resNative, err := g.Admit(context.Background(), TaskDescriptor{
		TaskID:               "native1",
		EngineClass:          EngineClassNative,
		Priority:             PriorityNormal,
		EstimatedMemoryMB:    100,
		EstimatedCPUPriority: 1,
	})
	if err != nil {
		t.Fatalf("native1: %v", err)
	}

	resPi.Release()
	resOmp.Release()
	resNative.Release()

	// Verify telemetry per class
	telem := g.GetTelemetry()
	for class, ct := range telem.ClassTelemetry {
		if ct.CurrentActive != 0 {
			t.Errorf("class %s active = %d after all releases", class, ct.CurrentActive)
		}
		if ct.ReleasedCount != ct.AdmittedCount {
			t.Errorf("class %s released=%d != admitted=%d", class, ct.ReleasedCount, ct.AdmittedCount)
		}
	}
}

func TestConcurrentAdmitRelease(t *testing.T) {
	g := NewGovernor(WithBudgets(map[EngineClass]ResourceBudget{
		EngineClassPi: {MaxWorkers: 4, MaxMemoryMB: 1000, MaxCPUPriority: 4},
	}))

	const workers = 20
	const iterations = 50
	var wg sync.WaitGroup
	admits := atomic.Int64{}
	releases := atomic.Int64{}
	errs := atomic.Int64{}

	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				res, err := g.Admit(context.Background(), TaskDescriptor{
					TaskID:               "t",
					EngineClass:          EngineClassPi,
					Priority:             PriorityNormal,
					EstimatedMemoryMB:    100,
					EstimatedCPUPriority: 1,
				})
				if err != nil {
					errs.Add(1)
					continue
				}
				admits.Add(1)
				// Simulate work
				time.Sleep(time.Microsecond * 10)
				res.Release()
				releases.Add(1)
			}
		}(i)
	}
	wg.Wait()

	if errs.Load() > 0 {
		t.Errorf("concurrent errors: %d", errs.Load())
	}
	if admits.Load() != releases.Load() {
		t.Errorf("admits (%d) != releases (%d)", admits.Load(), releases.Load())
	}
	telem := g.GetTelemetry()
	if telem.ClassTelemetry[EngineClassPi].CurrentActive != 0 {
		t.Errorf("leaked reservations: active = %d", telem.ClassTelemetry[EngineClassPi].CurrentActive)
	}
}

func TestUpdateUsageRecordsPeak(t *testing.T) {
	g := NewGovernor(WithBudgets(map[EngineClass]ResourceBudget{
		EngineClassPi: {MaxWorkers: 2, MaxMemoryMB: 1000, MaxCPUPriority: 4},
	}))
	res, err := g.Admit(context.Background(), TaskDescriptor{
		TaskID:               "t1",
		EngineClass:          EngineClassPi,
		Priority:             PriorityNormal,
		EstimatedMemoryMB:    200,
		EstimatedCPUPriority: 1,
	})
	if err != nil {
		t.Fatalf("admit: %v", err)
	}

	if err := g.UpdateUsage(res, 350, 2); err != nil {
		t.Fatalf("UpdateUsage: %v", err)
	}
	if err := g.UpdateUsage(res, 250, 1); err != nil {
		t.Fatalf("UpdateUsage: %v", err)
	}

	telem := g.GetTelemetry()
	if telem.ClassTelemetry[EngineClassPi].PeakMemoryMB != 350 {
		t.Errorf("peak memory = %d, want 350", telem.ClassTelemetry[EngineClassPi].PeakMemoryMB)
	}
	if telem.ClassTelemetry[EngineClassPi].PeakCPUPriority != 2 {
		t.Errorf("peak cpu = %d, want 2", telem.ClassTelemetry[EngineClassPi].PeakCPUPriority)
	}
	res.Release()
}

func TestSetClassBudgetAtRuntime(t *testing.T) {
	g := NewGovernor(WithBudgets(map[EngineClass]ResourceBudget{
		EngineClassPi: {MaxWorkers: 1, MaxMemoryMB: 500, MaxCPUPriority: 2},
	}))

	res, err := g.Admit(context.Background(), TaskDescriptor{
		TaskID:               "t1",
		EngineClass:          EngineClassPi,
		Priority:             PriorityNormal,
		EstimatedMemoryMB:    200,
		EstimatedCPUPriority: 1,
	})
	if err != nil {
		t.Fatalf("admit before: %v", err)
	}
	res.Release()

	// Increase budget
	if err := g.SetClassBudget(EngineClassPi, ResourceBudget{
		MaxWorkers: 2, MaxMemoryMB: 1000, MaxCPUPriority: 4,
	}); err != nil {
		t.Fatalf("SetClassBudget: %v", err)
	}

	res2, err := g.Admit(context.Background(), TaskDescriptor{
		TaskID:               "t2",
		EngineClass:          EngineClassPi,
		Priority:             PriorityNormal,
		EstimatedMemoryMB:    200,
		EstimatedCPUPriority: 1,
	})
	if err != nil {
		t.Fatalf("admit after: %v", err)
	}
	res2.Release()

	// Verify new budget
	budget := g.GetClassBudget(EngineClassPi)
	if budget.MaxWorkers != 2 {
		t.Errorf("budget.MaxWorkers = %d, want 2", budget.MaxWorkers)
	}
}

func TestRequirementsFieldUsedWhenEstimatesZero(t *testing.T) {
	g := NewGovernor(WithBudgets(map[EngineClass]ResourceBudget{
		EngineClassPi: {MaxWorkers: 2, MaxMemoryMB: 1000, MaxCPUPriority: 4},
	}))
	res, err := g.Admit(context.Background(), TaskDescriptor{
		TaskID:       "t1",
		EngineClass:  EngineClassPi,
		Priority:     PriorityNormal,
		Requirements: ResourceRequirement{PeakMemoryMB: 300, CPUPriority: 2},
	})
	if err != nil {
		t.Fatalf("admit with requirements: %v", err)
	}
	if res.MemoryMB != 300 {
		t.Errorf("res.MemoryMB = %d, want 300", res.MemoryMB)
	}
	if res.CPUPriority != 2 {
		t.Errorf("res.CPUPriority = %d, want 2", res.CPUPriority)
	}
	res.Release()
}

func TestPriorityNormalization(t *testing.T) {
	g := NewGovernor()
	res, err := g.Admit(context.Background(), TaskDescriptor{
		TaskID:               "t1",
		EngineClass:          EngineClassNative,
		Priority:             0,
		EstimatedMemoryMB:    50,
		EstimatedCPUPriority: 1,
	})
	if err != nil {
		t.Fatalf("admit: %v", err)
	}
	if res.Priority != PriorityNormal {
		t.Errorf("priority = %d, want %d", res.Priority, PriorityNormal)
	}
	res.Release()
}

func TestTaskIDRequired(t *testing.T) {
	g := NewGovernor()
	_, err := g.Admit(context.Background(), TaskDescriptor{
		TaskID:               "",
		EngineClass:          EngineClassNative,
		Priority:             PriorityNormal,
		EstimatedMemoryMB:    50,
		EstimatedCPUPriority: 1,
	})
	if err == nil {
		t.Fatal("expected error for empty task ID")
	}
	if !errors.Is(err, ErrInvalidContract) {
		t.Errorf("error = %v, want InvalidContract", err)
	}
}

func TestUnknownClassIsIsolated(t *testing.T) {
	g := NewGovernor()
	// EngineClass("custom") not configured, should get its own isolated accounting
	res, err := g.Admit(context.Background(), TaskDescriptor{
		TaskID:               "t1",
		EngineClass:          EngineClass("custom"),
		Priority:             PriorityNormal,
		EstimatedMemoryMB:    100,
		EstimatedCPUPriority: 1,
	})
	if err != nil {
		t.Fatalf("admit unknown class: %v", err)
	}
	res.Release()
	telem := g.GetTelemetry()
	if _, ok := telem.ClassTelemetry[EngineClass("custom")]; !ok {
		t.Error("unknown class missing from telemetry")
	}
}

func TestTelemetrySnapshotImmutable(t *testing.T) {
	now := time.Now()
	clockCalls := 0
	g := NewGovernor(
		WithBudgets(map[EngineClass]ResourceBudget{
			EngineClassPi: {MaxWorkers: 1, MaxMemoryMB: 500, MaxCPUPriority: 2},
		}),
		WithClock(func() time.Time {
			clockCalls++
			return now.Add(time.Duration(clockCalls) * time.Second)
		}),
	)
	res, err := g.Admit(context.Background(), TaskDescriptor{
		TaskID:               "t1",
		EngineClass:          EngineClassPi,
		Priority:             PriorityNormal,
		EstimatedMemoryMB:    200,
		EstimatedCPUPriority: 1,
	})
	if err != nil {
		t.Fatalf("admit: %v", err)
	}
	snap1 := g.GetTelemetry()
	res.Release()
	snap2 := g.GetTelemetry()
	if snap1.Timestamp.Equal(snap2.Timestamp) {
		t.Error("snapshot timestamps equal")
	}
	if snap1.ClassTelemetry[EngineClassPi].CurrentActive != 1 {
		t.Errorf("snap1 active = %d, want 1", snap1.ClassTelemetry[EngineClassPi].CurrentActive)
	}
	if snap2.ClassTelemetry[EngineClassPi].CurrentActive != 0 {
		t.Errorf("snap2 active = %d, want 0", snap2.ClassTelemetry[EngineClassPi].CurrentActive)
	}
}

func TestNewResourceGovernorConfig(t *testing.T) {
	cfg := GovernorConfig{
		Budgets: map[EngineClass]ResourceBudget{
			EngineClassPi: {MaxWorkers: 3, MaxMemoryMB: 1500, MaxCPUPriority: 3},
		},
	}
	gov, err := NewResourceGovernor(cfg)
	if err != nil {
		t.Fatalf("NewResourceGovernor: %v", err)
	}
	budget := gov.GetClassBudget(EngineClassPi)
	if budget.MaxWorkers != 3 {
		t.Errorf("MaxWorkers = %d, want 3", budget.MaxWorkers)
	}

	// Invalid budget
	invalidCfg := GovernorConfig{
		Budgets: map[EngineClass]ResourceBudget{
			EngineClassPi: {MaxWorkers: -1},
		},
	}
	_, err = NewResourceGovernor(invalidCfg)
	if err == nil {
		t.Fatal("expected error for negative budget")
	}
}
