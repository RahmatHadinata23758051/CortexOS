package brain

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/RahmatHadinata23758051/CortexOS/internal/cortex/application"
	"github.com/RahmatHadinata23758051/CortexOS/internal/cortex/provider"
)

func testProvider(t *testing.T) application.ProviderPort {
	t.Helper()
	return application.NewProviderAdapter(provider.NewFakeProvider("fake"))
}

func TestDispatcher_Complete(t *testing.T) {
	d, err := NewDispatcher(testProvider(t))
	if err != nil {
		t.Fatalf("NewDispatcher returned error: %v", err)
	}
	result, err := d.Dispatch(context.Background(), TaskDispatchRequest{
		TaskID:     "task-1",
		StaffID:    "staff-1",
		SystemRole: "You are a helpful assistant.",
		UserPrompt: "say hello",
		ContextDocs: []string{
			"synthetic context",
		},
		Config: SessionConfig{StepLimit: 2, MaxTokens: 100},
	})
	if err != nil {
		t.Fatalf("Dispatch returned error: %v", err)
	}
	if result.State != StateCompleted {
		t.Fatalf("State = %q, want completed", result.State)
	}
	if result.Output == "" || !strings.Contains(result.Output, "Fake response") {
		t.Fatalf("unexpected output: %q", result.Output)
	}
	if result.Steps != 1 {
		t.Fatalf("Steps = %d, want 1", result.Steps)
	}
}

func TestDispatcher_Stream(t *testing.T) {
	fake := provider.NewFakeProvider("fake")
	fake.SetStreamChunks([]string{
		`{"choices":[{"delta":{"content":"one "}}]}`,
		`{"choices":[{"delta":{"content":"two"}}]}`,
	})
	d, err := NewDispatcher(application.NewProviderAdapter(fake))
	if err != nil {
		t.Fatal(err)
	}
	var streamed []StreamChunk
	result, err := d.Dispatch(context.Background(), TaskDispatchRequest{
		TaskID:     "task-stream",
		UserPrompt: "stream",
		Stream:     true,
		StreamHandler: func(c StreamChunk) error {
			streamed = append(streamed, c)
			return nil
		},
		Config: SessionConfig{StepLimit: 2},
	})
	if err != nil {
		t.Fatalf("Dispatch returned error: %v", err)
	}
	if result.State != StateCompleted {
		t.Fatalf("State = %q, want completed", result.State)
	}
	if !strings.Contains(result.Output, "one two") {
		t.Fatalf("unexpected stream output: %q", result.Output)
	}
	if len(streamed) == 0 {
		t.Fatal("expected stream chunks captured")
	}
}

func TestDispatcher_RejectsInvalidRequest(t *testing.T) {
	d, _ := NewDispatcher(testProvider(t))
	if _, err := d.Dispatch(context.Background(), TaskDispatchRequest{UserPrompt: "missing task"}); !errors.Is(err, ErrDispatcherInvalidTask) {
		t.Fatalf("expected invalid task error, got %v", err)
	}
	if _, err := NewDispatcher(nil); !errors.Is(err, ErrDispatcherNilProvider) {
		t.Fatalf("expected nil provider error, got %v", err)
	}
}

func TestDispatcher_CanceledContext(t *testing.T) {
	d, _ := NewDispatcher(testProvider(t))
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	result, err := d.Dispatch(ctx, TaskDispatchRequest{TaskID: "cancel", UserPrompt: "nope"})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context canceled, got %v", err)
	}
	if result.State != StateCanceled {
		t.Fatalf("State = %q, want canceled", result.State)
	}
}

type errorProvider struct{ err error }

func (p errorProvider) Name() string { return "error" }
func (p errorProvider) Complete(context.Context, application.ProviderCompleteRequest) (io.ReadCloser, error) {
	return nil, p.err
}
func (p errorProvider) Stream(context.Context, application.ProviderCompleteRequest) (io.ReadCloser, error) {
	return nil, p.err
}
func (p errorProvider) Models(context.Context) (io.ReadCloser, error) { return nil, p.err }

func TestDispatcher_ProviderError(t *testing.T) {
	d, _ := NewDispatcher(errorProvider{err: errors.New("synthetic provider failure")})
	result, err := d.Dispatch(context.Background(), TaskDispatchRequest{TaskID: "failure", UserPrompt: "fail"})
	if err == nil {
		t.Fatal("expected provider error")
	}
	if result.State != StateFailed {
		t.Fatalf("State = %q, want failed", result.State)
	}
	if !strings.Contains(result.Error, "synthetic provider failure") {
		t.Fatalf("unexpected result error: %q", result.Error)
	}
}

func TestParseProviderResponse(t *testing.T) {
	content, tokens, err := parseProviderResponse(strings.NewReader(`{"choices":[{"message":{"content":"answer"}}],"usage":{"total_tokens":7}}`))
	if err != nil {
		t.Fatal(err)
	}
	if content != "answer" || tokens != 7 {
		t.Fatalf("got content=%q tokens=%d", content, tokens)
	}

	if _, _, err := parseProviderResponse(strings.NewReader("not json")); err == nil {
		t.Fatal("expected malformed response error")
	}
}
