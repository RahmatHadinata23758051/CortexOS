package orchestra

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"
)

var (
	ErrRetryExhausted      = errors.New("orchestra: retry budget exhausted")
	ErrNonRetryableFailure = errors.New("orchestra: failure is not retryable")
	ErrCircuitOpen         = errors.New("orchestra: circuit breaker is open")
)

type FailureClass string

const (
	FailureRetryable    FailureClass = "retryable"
	FailureNonRetryable FailureClass = "nonRetryable"
)

type RetryPolicy struct {
	MaxAttempts      int           `json:"maxAttempts"`
	Backoff          time.Duration `json:"backoff"`
	MaxBackoff       time.Duration `json:"maxBackoff"`
	CircuitThreshold int           `json:"circuitThreshold"`
	CircuitCooldown  time.Duration `json:"circuitCooldown"`
}

type CircuitState struct {
	mu        sync.Mutex
	failures  int
	openUntil time.Time
	threshold int
	cooldown  time.Duration
}

func NewCircuitState(policy RetryPolicy) *CircuitState {
	return &CircuitState{threshold: policy.CircuitThreshold, cooldown: policy.CircuitCooldown}
}

func (c *CircuitState) RecordFailure(now time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.failures++
	if c.threshold > 0 && c.failures >= c.threshold {
		c.openUntil = now.Add(c.cooldown)
		c.failures = 0
	}
}

func (c *CircuitState) Allow(now time.Time) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.openUntil.After(now) {
		return false
	}
	return true
}

func (c *CircuitState) Reset() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.failures = 0
	c.openUntil = time.Time{}
}

func Retryable(task Task, class FailureClass) error {
	if class == FailureNonRetryable {
		return fmt.Errorf("%w: task %q", ErrNonRetryableFailure, task.ID)
	}
	if task.AttemptCount >= task.MaxAttempts {
		return fmt.Errorf("%w: task %q", ErrRetryExhausted, task.ID)
	}
	return nil
}

func BackoffFor(policy RetryPolicy, attempt int) time.Duration {
	if policy.Backoff <= 0 {
		return 0
	}
	backoff := policy.Backoff * time.Duration(1<<uint(attempt-1))
	if backoff > policy.MaxBackoff {
		return policy.MaxBackoff
	}
	return backoff
}

func ShouldRetry(ctx context.Context, task Task, policy RetryPolicy, class FailureClass, circuit *CircuitState) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := Retryable(task, class); err != nil {
		return err
	}
	if circuit != nil && !circuit.Allow(time.Now()) {
		return ErrCircuitOpen
	}
	return nil
}
