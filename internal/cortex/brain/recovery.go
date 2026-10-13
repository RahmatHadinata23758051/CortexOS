package brain

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/RahmatHadinata23758051/CortexOS/internal/cortex/application"
)

var (
	ErrRetryExhausted = errors.New("max retries exhausted")
	ErrRateLimited    = errors.New("rate limited by provider")
)

// RetryPolicy defines backoff and retry behavior for provider calls.
type RetryPolicy struct {
	MaxRetries     int
	BaseDelay      time.Duration
	MaxDelay       time.Duration
	Jitter         float64 // 0.0-1.0 fraction of delay to add as jitter
	RetryableCodes []string
}

func (p RetryPolicy) Validate() error {
	if p.MaxRetries < 0 {
		return fmt.Errorf("max retries must not be negative")
	}
	if p.BaseDelay < 0 {
		return fmt.Errorf("base delay must not be negative")
	}
	if p.MaxDelay < p.BaseDelay {
		return fmt.Errorf("max delay must be >= base delay")
	}
	if p.Jitter < 0 || p.Jitter > 1 {
		return fmt.Errorf("jitter must be in [0, 1]")
	}
	return nil
}

// DefaultRetryPolicy returns a conservative policy suitable for local providers.
func DefaultRetryPolicy() RetryPolicy {
	return RetryPolicy{
		MaxRetries: 3,
		BaseDelay:  250 * time.Millisecond,
		MaxDelay:   5 * time.Second,
		Jitter:     0.2,
		RetryableCodes: []string{
			"rate_limit_exceeded",
			"timeout",
			"unavailable",
			"internal_error",
			"service_unavailable",
		},
	}
}

// NoRetryPolicy returns a policy that performs no retries.
func NoRetryPolicy() RetryPolicy {
	return RetryPolicy{MaxRetries: 0}
}

// retryable determines if an error matches the retryable codes.
func (p RetryPolicy) retryable(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	for _, code := range p.RetryableCodes {
		if contains(msg, code) {
			return true
		}
	}
	return false
}

// backoff calculates the delay for a given attempt with jitter.
func (p RetryPolicy) backoff(attempt int) time.Duration {
	if attempt <= 0 {
		return 0
	}
	delay := p.BaseDelay
	for i := 1; i < attempt; i++ {
		delay *= 2
		if delay > p.MaxDelay {
			delay = p.MaxDelay
			break
		}
	}
	if p.Jitter > 0 {
		jitter := time.Duration(float64(delay) * p.Jitter)
		delay += time.Duration(float64(jitter) * (float64(attempt) / 10.0))
	}
	if delay > p.MaxDelay {
		delay = p.MaxDelay
	}
	return delay
}

func contains(s, substr string) bool {
	for i := 0; i+len(substr) <= len(s); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// ResilientDispatcher wraps a Dispatcher with retry and recovery logic.
type ResilientDispatcher struct {
	dispatcher *Dispatcher
	policy     RetryPolicy
	mu         sync.Mutex
	attempts   int
}

// NewResilientDispatcher creates a dispatcher with automatic retries.
func NewResilientDispatcher(p application.ProviderPort, policy RetryPolicy) (*ResilientDispatcher, error) {
	d, err := NewDispatcher(p)
	if err != nil {
		return nil, err
	}
	if err := policy.Validate(); err != nil {
		return nil, err
	}
	return &ResilientDispatcher{
		dispatcher: d,
		policy:     policy,
	}, nil
}

// DispatchWithRetry executes the task with retry logic according to the policy.
func (rd *ResilientDispatcher) DispatchWithRetry(ctx context.Context, req TaskDispatchRequest) (TaskDispatchResult, error) {
	if rd == nil {
		return TaskDispatchResult{}, errors.New("resilient dispatcher is nil")
	}

	rd.mu.Lock()
	rd.attempts = 0
	rd.mu.Unlock()

	var lastErr error
	for attempt := 0; attempt <= rd.policy.MaxRetries; attempt++ {
		rd.mu.Lock()
		rd.attempts = attempt
		rd.mu.Unlock()

		if attempt > 0 {
			delay := rd.policy.backoff(attempt)
			select {
			case <-ctx.Done():
				return TaskDispatchResult{}, ctx.Err()
			case <-time.After(delay):
			}
		}

		result, err := rd.dispatcher.Dispatch(ctx, req)
		if err == nil {
			return result, nil
		}

		lastErr = err

		// Check if error is retryable
		if !rd.policy.retryable(err) {
			break
		}
	}

	return TaskDispatchResult{}, fmt.Errorf("%w: %v", ErrRetryExhausted, lastErr)
}

// Attempts returns the number of attempts made in the last DispatchWithRetry call.
func (rd *ResilientDispatcher) Attempts() int {
	rd.mu.Lock()
	defer rd.mu.Unlock()
	return rd.attempts
}

// ProviderName returns the name of the underlying provider port.
func (rd *ResilientDispatcher) ProviderName() string {
	if rd == nil || rd.dispatcher == nil {
		return "disabled"
	}
	return rd.dispatcher.ProviderName()
}

// RecoveryManager orchestrates checkpoint/recovery for long-running brain sessions.
// It is designed for soak testing and production durability.
type RecoveryManager struct {
	mu          sync.Mutex
	checkpoints map[string]RecoveryCheckpoint
	maxAge      time.Duration
}

// RecoveryCheckpoint captures session state at a recovery point.
type RecoveryCheckpoint struct {
	TaskID string
	State  State
	Output string
	Steps  int
	Tokens int
	At     time.Time
	Config SessionConfig
}

// NewRecoveryManager creates a manager with automatic cleanup of stale checkpoints.
func NewRecoveryManager(maxAge time.Duration) *RecoveryManager {
	if maxAge <= 0 {
		maxAge = 24 * time.Hour
	}
	return &RecoveryManager{
		checkpoints: make(map[string]RecoveryCheckpoint),
		maxAge:      maxAge,
	}
}

// SaveCheckpoint records the current session state for potential recovery.
func (rm *RecoveryManager) SaveCheckpoint(cp RecoveryCheckpoint) {
	if rm == nil || cp.TaskID == "" {
		return
	}
	rm.mu.Lock()
	defer rm.mu.Unlock()
	rm.checkpoints[cp.TaskID] = cp
}

// LoadCheckpoint retrieves a saved checkpoint for recovery.
func (rm *RecoveryManager) LoadCheckpoint(taskID string) (RecoveryCheckpoint, bool) {
	if rm == nil {
		return RecoveryCheckpoint{}, false
	}
	rm.mu.Lock()
	defer rm.mu.Unlock()
	cp, ok := rm.checkpoints[taskID]
	return cp, ok
}

// DeleteCheckpoint removes a checkpoint after successful recovery or abandonment.
func (rm *RecoveryManager) DeleteCheckpoint(taskID string) {
	if rm == nil {
		return
	}
	rm.mu.Lock()
	defer rm.mu.Unlock()
	delete(rm.checkpoints, taskID)
}

// CleanupExpired removes checkpoints older than the configured retention age.
// It is explicit so the manager owns no hidden goroutine and can be used safely
// in short-lived desktop/test lifecycles.
func (rm *RecoveryManager) CleanupExpired(now time.Time) int {
	if rm == nil {
		return 0
	}
	if now.IsZero() {
		now = time.Now()
	}
	rm.mu.Lock()
	defer rm.mu.Unlock()
	removed := 0
	for id, cp := range rm.checkpoints {
		if !cp.At.IsZero() && now.Sub(cp.At) > rm.maxAge {
			delete(rm.checkpoints, id)
			removed++
		}
	}
	return removed
}
