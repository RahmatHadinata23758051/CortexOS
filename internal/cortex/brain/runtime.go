// Package brain owns the lifecycle of a single CortexOS reasoning session.
//
// The package is deliberately independent of providers, persistence, Wails, and
// process handles. It owns only session state, transition invariants, and
// resource budgets. Application/runtime adapters decide when to call it.
package brain

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"
)

// State is the lifecycle state of a reasoning session.
type State string

const (
	StateIdle        State = "idle"
	StatePlanning    State = "planning"
	StateExecuting   State = "executing"
	StateStreaming   State = "streaming"
	StateToolCalling State = "tool_calling"
	StateCompleted   State = "completed"
	StateFailed      State = "failed"
	StateCanceled    State = "canceled"
)

var (
	ErrInvalidTransition = errors.New("invalid brain state transition")
	ErrTerminalState     = errors.New("brain session is in a terminal state")
	ErrInvalidConfig     = errors.New("invalid brain session config")
	ErrStepLimit         = errors.New("brain session step limit exceeded")
	ErrTokenLimit        = errors.New("brain session token limit exceeded")
	ErrTimeout           = errors.New("brain session timeout exceeded")
)

// SessionConfig defines bounded resources for a session. A zero value means
// that the corresponding budget is disabled. Negative values are invalid.
type SessionConfig struct {
	Timeout   time.Duration
	StepLimit int
	MaxTokens int
}

func (c SessionConfig) Validate() error {
	if c.Timeout < 0 {
		return fmt.Errorf("%w: timeout must not be negative", ErrInvalidConfig)
	}
	if c.StepLimit < 0 {
		return fmt.Errorf("%w: step limit must not be negative", ErrInvalidConfig)
	}
	if c.MaxTokens < 0 {
		return fmt.Errorf("%w: max tokens must not be negative", ErrInvalidConfig)
	}
	return nil
}

// StateChangeEvent is an informational event. State itself remains authoritative
// in Session; subscribers must tolerate dropped events when their channel buffer
// is full and should re-read Session.State when they need certainty.
type StateChangeEvent struct {
	Sequence uint64
	At       time.Time
	From     State
	To       State
	Reason   string
	Steps    int
	Tokens   int
}

// Session is a concurrency-safe state machine for one reasoning execution.
// A Session is not reusable after a terminal result unless Reset is called.
type Session struct {
	mu          sync.Mutex
	state       State
	config      SessionConfig
	startedAt   time.Time
	steps       int
	tokens      int
	sequence    uint64
	nextSubID   uint64
	subscribers map[uint64]chan StateChangeEvent
}

// NewSession validates config and creates an idle session.
func NewSession(config SessionConfig) (*Session, error) {
	if err := config.Validate(); err != nil {
		return nil, err
	}
	return &Session{
		state:       StateIdle,
		config:      config,
		subscribers: make(map[uint64]chan StateChangeEvent),
	}, nil
}

// State returns the current state.
func (s *Session) State() State {
	if s == nil {
		return StateFailed
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.state
}

// Config returns the immutable session budget configuration.
func (s *Session) Config() SessionConfig {
	if s == nil {
		return SessionConfig{}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.config
}

// Stats returns current budget usage and the session start time.
func (s *Session) Stats() (steps, tokens int, startedAt time.Time) {
	if s == nil {
		return 0, 0, time.Time{}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.steps, s.tokens, s.startedAt
}

// Transition moves the session to a valid next state and publishes an event.
func (s *Session) Transition(to State, reason string) error {
	if s == nil {
		return ErrInvalidTransition
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.transitionLocked(to, reason)
}

// CompareAndSwap transitions only when the current state equals expected. It
// returns false for a stale expected state or an invalid transition.
func (s *Session) CompareAndSwap(expected, target State, reason string) bool {
	if s == nil {
		return false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.state != expected {
		return false
	}
	return s.transitionLocked(target, reason) == nil
}

// TransitionContext observes cancellation and timeout before attempting a
// transition. Cancellation is represented by StateCanceled; timeout by
// StateFailed because it is a runtime budget failure rather than user intent.
func (s *Session) TransitionContext(ctx context.Context, to State, reason string) error {
	if ctx == nil {
		return fmt.Errorf("%w: context is nil", ErrInvalidConfig)
	}
	if err := s.Observe(ctx); err != nil {
		return err
	}
	return s.Transition(to, reason)
}

// Observe applies context cancellation or the configured wall-clock timeout.
// It should be called by the owner at request/step boundaries; it does not
// spawn a hidden goroutine.
func (s *Session) Observe(ctx context.Context) error {
	if s == nil {
		return ErrInvalidTransition
	}
	if ctx == nil {
		return fmt.Errorf("%w: context is nil", ErrInvalidConfig)
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if s.isTerminalLocked() {
		return nil
	}
	if err := ctx.Err(); err != nil {
		if s.state != StateIdle {
			_ = s.transitionLocked(StateCanceled, "context canceled")
		}
		return err
	}
	if s.config.Timeout > 0 && !s.startedAt.IsZero() && time.Since(s.startedAt) >= s.config.Timeout {
		_ = s.transitionLocked(StateFailed, "session timeout")
		return ErrTimeout
	}
	return nil
}

// BeginStep accounts for one planning/execution/tool step and enforces the
// configured step budget. The budget failure transitions the session to failed.
func (s *Session) BeginStep() error {
	if s == nil {
		return ErrInvalidTransition
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.isTerminalLocked() || s.state == StateIdle {
		return ErrTerminalState
	}
	if s.config.StepLimit > 0 && s.steps >= s.config.StepLimit {
		_ = s.transitionLocked(StateFailed, "step limit exceeded")
		return ErrStepLimit
	}
	s.steps++
	return nil
}

// AddTokens accounts for provider usage and enforces the token budget.
func (s *Session) AddTokens(count int) error {
	if s == nil || count < 0 {
		return fmt.Errorf("%w: token count must not be negative", ErrInvalidConfig)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.isTerminalLocked() {
		return ErrTerminalState
	}
	if s.config.MaxTokens > 0 && count > s.config.MaxTokens-s.tokens {
		_ = s.transitionLocked(StateFailed, "token limit exceeded")
		return ErrTokenLimit
	}
	s.tokens += count
	return nil
}

// Reset explicitly returns a terminal session to idle and clears usage. Reset
// is the only way to leave a terminal state.
func (s *Session) Reset() error {
	if s == nil {
		return ErrInvalidTransition
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.isTerminalLocked() {
		return fmt.Errorf("%w: reset requires terminal state", ErrInvalidTransition)
	}
	return s.transitionLocked(StateIdle, "reset")
}

// Subscribe registers a best-effort buffered event stream. Cancel removes the
// subscription. Events are never allowed to block the state owner.
func (s *Session) Subscribe(buffer int) (<-chan StateChangeEvent, func(), error) {
	if s == nil {
		return nil, nil, ErrInvalidTransition
	}
	if buffer < 1 {
		return nil, nil, fmt.Errorf("%w: subscriber buffer must be positive", ErrInvalidConfig)
	}
	ch := make(chan StateChangeEvent, buffer)
	s.mu.Lock()
	s.nextSubID++
	id := s.nextSubID
	s.subscribers[id] = ch
	s.mu.Unlock()
	cancel := func() {
		s.mu.Lock()
		delete(s.subscribers, id)
		s.mu.Unlock()
	}
	return ch, cancel, nil
}

func (s *Session) transitionLocked(to State, reason string) error {
	if s.isTerminalLocked() && to != StateIdle {
		return fmt.Errorf("%w: %s cannot transition to %s", ErrTerminalState, s.state, to)
	}
	if !validTransition(s.state, to) {
		return fmt.Errorf("%w: %s -> %s", ErrInvalidTransition, s.state, to)
	}
	from := s.state
	s.state = to
	if from == StateIdle && to == StatePlanning {
		s.startedAt = time.Now()
	}
	s.sequence++
	event := StateChangeEvent{
		Sequence: s.sequence,
		At:       time.Now(),
		From:     from,
		To:       to,
		Reason:   reason,
		Steps:    s.steps,
		Tokens:   s.tokens,
	}
	for _, ch := range s.subscribers {
		select {
		case ch <- event:
		default:
		}
	}
	if to == StateIdle {
		s.startedAt = time.Time{}
		s.steps = 0
		s.tokens = 0
	}
	return nil
}

func (s *Session) isTerminalLocked() bool {
	return s.state == StateCompleted || s.state == StateFailed || s.state == StateCanceled
}

func validTransition(from, to State) bool {
	if to == StateIdle {
		return from == StateCompleted || from == StateFailed || from == StateCanceled
	}
	switch from {
	case StateIdle:
		return to == StatePlanning
	case StatePlanning:
		return to == StateExecuting || to == StateFailed || to == StateCanceled
	case StateExecuting:
		return to == StateStreaming || to == StateToolCalling || to == StateCompleted || to == StateFailed || to == StateCanceled
	case StateStreaming:
		return to == StateToolCalling || to == StateCompleted || to == StateFailed || to == StateCanceled
	case StateToolCalling:
		return to == StateExecuting || to == StateCompleted || to == StateFailed || to == StateCanceled
	default:
		return false
	}
}
