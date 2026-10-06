package orchestra

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestRetryableClassifiesFailures(t *testing.T) {
	task := validTask()
	if err := Retryable(task, FailureRetryable); err != nil {
		t.Fatal(err)
	}
	if err := Retryable(task, FailureNonRetryable); !errors.Is(err, ErrNonRetryableFailure) {
		t.Fatalf("non-retryable error = %v", err)
	}
	task.AttemptCount = task.MaxAttempts
	if err := Retryable(task, FailureRetryable); !errors.Is(err, ErrRetryExhausted) {
		t.Fatalf("exhausted error = %v", err)
	}
}

func TestBackoffIsBoundedAndExponential(t *testing.T) {
	policy := RetryPolicy{Backoff: 100 * time.Millisecond, MaxBackoff: 500 * time.Millisecond}
	if got := BackoffFor(policy, 1); got != 100*time.Millisecond {
		t.Fatalf("attempt 1 backoff = %v", got)
	}
	if got := BackoffFor(policy, 3); got != 400*time.Millisecond {
		t.Fatalf("attempt 3 backoff = %v", got)
	}
	if got := BackoffFor(policy, 10); got != 500*time.Millisecond {
		t.Fatalf("capped backoff = %v", got)
	}
}

func TestCircuitOpensAfterThreshold(t *testing.T) {
	circuit := NewCircuitState(RetryPolicy{CircuitThreshold: 2, CircuitCooldown: 50 * time.Millisecond})
	now := time.Now()
	circuit.RecordFailure(now)
	if !circuit.Allow(now) {
		t.Fatal("circuit should allow before threshold")
	}
	circuit.RecordFailure(now)
	if circuit.Allow(now) {
		t.Fatal("circuit should be open after threshold")
	}
	if !circuit.Allow(now.Add(60 * time.Millisecond)) {
		t.Fatal("circuit should close after cooldown")
	}
}

func TestShouldRetryRespectsCancellationAndCircuit(t *testing.T) {
	task := validTask()
	policy := RetryPolicy{CircuitThreshold: 1, CircuitCooldown: 50 * time.Millisecond}
	circuit := NewCircuitState(policy)
	circuit.RecordFailure(time.Now())
	if err := ShouldRetry(context.Background(), task, policy, FailureRetryable, circuit); !errors.Is(err, ErrCircuitOpen) {
		t.Fatalf("circuit error = %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := ShouldRetry(ctx, task, policy, FailureRetryable, nil); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled error = %v", err)
	}
}
