package brain

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/RahmatHadinata23758051/CortexOS/internal/cortex/application"
	"github.com/RahmatHadinata23758051/CortexOS/internal/cortex/provider"
)

func TestRetryPolicy_Validate(t *testing.T) {
	if err := DefaultRetryPolicy().Validate(); err != nil {
		t.Fatalf("default policy invalid: %v", err)
	}
	if err := NoRetryPolicy().Validate(); err != nil {
		t.Fatalf("no-retry policy invalid: %v", err)
	}
	if err := (RetryPolicy{MaxRetries: -1}).Validate(); err == nil {
		t.Fatal("expected error for negative retries")
	}
	if err := (RetryPolicy{BaseDelay: time.Second, MaxDelay: time.Millisecond}).Validate(); err == nil {
		t.Fatal("expected error for max < base")
	}
}

func TestRetryPolicy_BackoffGrowth(t *testing.T) {
	p := RetryPolicy{BaseDelay: 100 * time.Millisecond, MaxDelay: time.Second}
	prev := time.Duration(0)
	for i := 1; i <= 5; i++ {
		got := p.backoff(i)
		if got < prev {
			t.Fatalf("backoff should grow: attempt %d = %v, prev %v", i, got, prev)
		}
		if got > p.MaxDelay {
			t.Fatalf("backoff exceeded max: %v", got)
		}
		prev = got
	}
}

// flakyProvider fails the first n calls then succeeds.
type flakyProvider struct {
	failures int
	calls    int
}

func (f *flakyProvider) Name() string { return "flaky" }
func (f *flakyProvider) Close() error { return nil }
func (f *flakyProvider) Models(context.Context) (io.ReadCloser, error) {
	return io.NopCloser(strings.NewReader(`{"object":"list","data":[]}`)), nil
}
func (f *flakyProvider) Complete(context.Context, provider.CompleteRequest) (io.ReadCloser, error) {
	f.calls++
	if f.calls <= f.failures {
		return nil, &provider.ErrProviderUnavailable{Provider: "flaky", Reason: "unavailable: burst"}
	}
	return io.NopCloser(strings.NewReader(`{"choices":[{"message":{"content":"recovered"}}],"usage":{"total_tokens":3}}`)), nil
}
func (f *flakyProvider) Stream(context.Context, provider.CompleteRequest) (io.ReadCloser, error) {
	return f.Complete(context.Background(), provider.CompleteRequest{})
}

// flakyPort adapts flakyProvider to the application boundary.
type flakyPort struct{ p *flakyProvider }

func (f flakyPort) Name() string { return f.p.Name() }
func (f flakyPort) Complete(ctx context.Context, r application.ProviderCompleteRequest) (io.ReadCloser, error) {
	return f.p.Complete(ctx, provider.CompleteRequest{})
}
func (f flakyPort) Stream(ctx context.Context, r application.ProviderCompleteRequest) (io.ReadCloser, error) {
	return f.p.Stream(ctx, provider.CompleteRequest{})
}
func (f flakyPort) Models(ctx context.Context) (io.ReadCloser, error) { return f.p.Models(ctx) }

func TestResilientDispatcher_RetriesAndSucceeds(t *testing.T) {
	flaky := &flakyProvider{failures: 2}
	rd, err := NewResilientDispatcher(flakyPort{p: flaky}, RetryPolicy{
		MaxRetries: 3, BaseDelay: time.Millisecond, MaxDelay: 10 * time.Millisecond,
		RetryableCodes: []string{"unavailable"},
	})
	if err != nil {
		t.Fatal(err)
	}
	result, err := rd.DispatchWithRetry(context.Background(), TaskDispatchRequest{
		TaskID:     "retry-ok",
		UserPrompt: "hello",
	})
	if err != nil {
		t.Fatalf("expected success after retries, got %v", err)
	}
	if result.Output != "recovered" {
		t.Fatalf("unexpected output: %q", result.Output)
	}
	if rd.Attempts() != 2 {
		t.Fatalf("attempts = %d, want 2 retries", rd.Attempts())
	}
}

func TestResilientDispatcher_ExhaustsRetries(t *testing.T) {
	flaky := &flakyProvider{failures: 10}
	rd, err := NewResilientDispatcher(flakyPort{p: flaky}, RetryPolicy{
		MaxRetries: 2, BaseDelay: time.Millisecond, MaxDelay: 5 * time.Millisecond,
		RetryableCodes: []string{"unavailable"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := rd.DispatchWithRetry(context.Background(), TaskDispatchRequest{TaskID: "t", UserPrompt: "x"}); !errors.Is(err, ErrRetryExhausted) {
		t.Fatalf("expected retry exhaustion, got %v", err)
	}
}

func TestRecoveryManager_SaveLoadDelete(t *testing.T) {
	rm := NewRecoveryManager(time.Hour)
	cp := RecoveryCheckpoint{TaskID: "task-9", State: StateStreaming, Output: "partial", Steps: 2, Tokens: 5, At: time.Now()}
	rm.SaveCheckpoint(cp)

	got, ok := rm.LoadCheckpoint("task-9")
	if !ok {
		t.Fatal("expected checkpoint")
	}
	if got.Output != "partial" || got.State != StateStreaming {
		t.Fatalf("unexpected checkpoint: %#v", got)
	}

	rm.DeleteCheckpoint("task-9")
	if _, ok := rm.LoadCheckpoint("task-9"); ok {
		t.Fatal("expected checkpoint deleted")
	}
}
