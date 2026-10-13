package brain

import (
	"context"
	"sync"
	"testing"
	"time"
)

func TestNewSession_Valid(t *testing.T) {
	s, err := NewSession(SessionConfig{Timeout: time.Second, StepLimit: 3, MaxTokens: 100})
	if err != nil {
		t.Fatalf("NewSession returned error: %v", err)
	}
	if got := s.State(); got != StateIdle {
		t.Fatalf("State = %q, want idle", got)
	}
}

func TestNewSession_RejectsNegativeConfig(t *testing.T) {
	for _, cfg := range []SessionConfig{{Timeout: -1}, {StepLimit: -1}, {MaxTokens: -1}} {
		if _, err := NewSession(cfg); err == nil {
			t.Fatalf("expected error for config %+v", cfg)
		}
	}
}

func TestTransition_HappyPath(t *testing.T) {
	s, _ := NewSession(SessionConfig{})
	path := []struct {
		to     State
		reason string
	}{
		{StatePlanning, "begin"},
		{StateExecuting, "call provider"},
		{StateStreaming, "stream"},
		{StateCompleted, "done"},
	}
	for _, step := range path {
		if err := s.Transition(step.to, step.reason); err != nil {
			t.Fatalf("Transition(%s) returned error: %v", step.to, err)
		}
		if got := s.State(); got != step.to {
			t.Fatalf("State = %q, want %q", got, step.to)
		}
	}
}

func TestTransition_LoopBackThroughToolCall(t *testing.T) {
	s, _ := NewSession(SessionConfig{})
	for _, next := range []State{StatePlanning, StateExecuting, StateToolCalling, StateExecuting, StateStreaming, StateCompleted} {
		if err := s.Transition(next, "step"); err != nil {
			t.Fatalf("Transition(%s) returned error: %v", next, err)
		}
	}
}

func TestTransition_RejectsInvalid(t *testing.T) {
	s, _ := NewSession(SessionConfig{})
	if err := s.Transition(StateToolCalling, "skip"); err == nil {
		t.Fatal("expected error for idle -> tool_calling")
	}
	if err := s.Transition(State("unknown"), "bad"); err == nil {
		t.Fatal("expected error for unknown target state")
	}

	s2, _ := NewSession(SessionConfig{})
	for _, next := range []State{StatePlanning, StateExecuting, StateCompleted} {
		if err := s2.Transition(next, "step"); err != nil {
			t.Fatal(err)
		}
	}
	if err := s2.Transition(StateExecuting, "reuse"); err == nil {
		t.Fatal("expected terminal-state guard on completed -> executing")
	}
	if err := s2.Reset(); err != nil {
		t.Fatalf("Reset returned error: %v", err)
	}
	if got := s2.State(); got != StateIdle {
		t.Fatalf("State after Reset = %q, want idle", got)
	}
}

func TestCompareAndSwap(t *testing.T) {
	s, _ := NewSession(SessionConfig{})
	if !s.CompareAndSwap(StateIdle, StatePlanning, "cas") {
		t.Fatal("expected CAS success for idle -> planning")
	}
	if s.CompareAndSwap(StateIdle, StateExecuting, "stale") {
		t.Fatal("expected CAS failure for stale expected state")
	}
	if s.CompareAndSwap(StatePlanning, StateCompleted, "invalid-edge") {
		t.Fatal("expected CAS failure for invalid planning -> completed")
	}
}

func TestBudgets_StepLimit(t *testing.T) {
	s, _ := NewSession(SessionConfig{StepLimit: 1})
	_ = s.Transition(StatePlanning, "begin")
	_ = s.Transition(StateExecuting, "run")
	if err := s.BeginStep(); err != nil {
		t.Fatalf("first step returned error: %v", err)
	}
	if err := s.BeginStep(); err == nil {
		t.Fatal("expected step-limit error on second step")
	}
	if got := s.State(); got != StateFailed {
		t.Fatalf("State = %q, want failed after step limit", got)
	}
}

func TestBudgets_TokenLimit(t *testing.T) {
	s, _ := NewSession(SessionConfig{MaxTokens: 10})
	_ = s.Transition(StatePlanning, "begin")
	_ = s.Transition(StateExecuting, "run")
	if err := s.AddTokens(6); err != nil {
		t.Fatalf("AddTokens returned error: %v", err)
	}
	if err := s.AddTokens(5); err == nil {
		t.Fatal("expected token-limit error")
	}
	if got := s.State(); got != StateFailed {
		t.Fatalf("State = %q, want failed after token limit", got)
	}
}

func TestObserve_TimeoutAndCancel(t *testing.T) {
	s, _ := NewSession(SessionConfig{Timeout: time.Nanosecond})
	_ = s.Transition(StatePlanning, "begin")
	time.Sleep(time.Millisecond)
	if err := s.Observe(context.Background()); err == nil {
		t.Fatal("expected timeout error")
	}
	if got := s.State(); got != StateFailed {
		t.Fatalf("State = %q, want failed after timeout", got)
	}

	s2, _ := NewSession(SessionConfig{})
	_ = s2.Transition(StatePlanning, "begin")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := s2.Observe(ctx); err == nil {
		t.Fatal("expected cancellation error")
	}
	if got := s2.State(); got != StateCanceled {
		t.Fatalf("State = %q, want canceled", got)
	}
}

func TestSubscribe_DropsWithoutBlocking(t *testing.T) {
	s, _ := NewSession(SessionConfig{})
	ch, cancel, err := s.Subscribe(1)
	if err != nil {
		t.Fatalf("Subscribe returned error: %v", err)
	}
	defer cancel()

	_ = s.Transition(StatePlanning, "one")
	_ = s.Transition(StateExecuting, "two")
	_ = s.Transition(StateCompleted, "three")

	var last StateChangeEvent
	drained := 0
	for drained <= 3 {
		select {
		case last = <-ch:
			drained++
		default:
			if drained == 0 {
				t.Fatal("expected at least one event")
			}
			return
		}
	}
	_ = last
}

func TestSession_ConcurrentTransitionsAreSafe(t *testing.T) {
	s, _ := NewSession(SessionConfig{})
	_ = s.Transition(StatePlanning, "begin")

	var wg sync.WaitGroup
	for i := 0; i < 32; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = s.CompareAndSwap(StatePlanning, StateExecuting, "race")
		}()
	}
	wg.Wait()
	if got := s.State(); got != StateExecuting {
		t.Fatalf("State = %q, want executing after concurrent CAS", got)
	}
}
