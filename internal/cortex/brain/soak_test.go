package brain

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/RahmatHadinata23758051/CortexOS/internal/cortex/application"
	"github.com/RahmatHadinata23758051/CortexOS/internal/cortex/provider"
)

// TestBrain_SoakConcurrentDispatch runs many dispatches concurrently against the
// fake provider to prove session isolation, bounded resources, and no data race.
// Run with: go test -race -run TestBrain_Soak ./internal/cortex/brain
func TestBrain_SoakConcurrentDispatch(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping soak in short mode")
	}
	fake := provider.NewFakeProvider("soak-fake")
	d, err := NewDispatcher(application.NewProviderAdapter(fake))
	if err != nil {
		t.Fatal(err)
	}

	const workers = 8
	const perWorker = 25

	var wg sync.WaitGroup
	errCh := make(chan error, workers*perWorker)

	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func(worker int) {
			defer wg.Done()
			for i := 0; i < perWorker; i++ {
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				result, err := d.Dispatch(ctx, TaskDispatchRequest{
					TaskID:     fmt.Sprintf("soak-%d-%d", worker, i),
					UserPrompt: "soak ping",
					Config:     SessionConfig{StepLimit: 4, MaxTokens: 1000, Timeout: 5 * time.Second},
				})
				cancel()
				if err != nil {
					errCh <- err
					return
				}
				if result.State != StateCompleted {
					errCh <- fmt.Errorf("task %s state = %s, want completed", result.TaskID, result.State)
					return
				}
			}
		}(w)
	}
	wg.Wait()
	close(errCh)
	for err := range errCh {
		t.Fatal(err)
	}

	calls := len(fake.Calls())
	want := workers * perWorker
	if calls != want {
		t.Fatalf("fake provider calls = %d, want %d", calls, want)
	}
}

// TestBrain_SoakSessionTransitions hammers a single session with concurrent CAS
// attempts to prove only one winner transitions and terminal guards hold.
func TestBrain_SoakSessionTransitions(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping soak in short mode")
	}
	s, err := NewSession(SessionConfig{StepLimit: 1000})
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Transition(StatePlanning, "soak start"); err != nil {
		t.Fatal(err)
	}

	const racers = 64
	var wg sync.WaitGroup
	wins := make(chan struct{}, racers)
	for i := 0; i < racers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if s.CompareAndSwap(StatePlanning, StateExecuting, "soak race") {
				wins <- struct{}{}
			}
		}()
	}
	wg.Wait()
	close(wins)

	count := 0
	for range wins {
		count++
	}
	if count != 1 {
		t.Fatalf("expected exactly 1 CAS winner, got %d", count)
	}
	if got := s.State(); got != StateExecuting {
		t.Fatalf("state = %s, want executing", got)
	}
}

// TestBrain_SoakRecoveryCheckpointChurn proves the recovery manager stays bounded
// under checkpoint save/load/delete churn.
func TestBrain_SoakRecoveryCheckpointChurn(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping soak in short mode")
	}
	rm := NewRecoveryManager(time.Hour)
	const n = 500
	for i := 0; i < n; i++ {
		id := fmt.Sprintf("churn-%d", i)
		rm.SaveCheckpoint(RecoveryCheckpoint{
			TaskID: id, State: StateStreaming, Output: "partial", Steps: i, At: time.Now(),
		})
		if _, ok := rm.LoadCheckpoint(id); !ok {
			t.Fatalf("missing checkpoint %s", id)
		}
		rm.DeleteCheckpoint(id)
	}
	if _, ok := rm.LoadCheckpoint("churn-0"); ok {
		t.Fatal("expected churn-0 deleted")
	}
	// Expired cleanup path.
	rm.SaveCheckpoint(RecoveryCheckpoint{TaskID: "old", State: StateFailed, At: time.Now().Add(-2 * time.Hour)})
	if removed := rm.CleanupExpired(time.Now()); removed != 1 {
		t.Fatalf("cleanup removed = %d, want 1", removed)
	}
}
