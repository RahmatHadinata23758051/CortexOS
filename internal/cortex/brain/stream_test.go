package brain

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestConsumeStream_BasicDeltas(t *testing.T) {
	input := strings.NewReader(`data: {"choices":[{"delta":{"content":"hello "}}]}

data: {"choices":[{"delta":{"content":"world"}}]}

data: [DONE]
`)
	var chunks []StreamChunk
	output, err := ConsumeStream(context.Background(), "task-1", input, func(c StreamChunk) error {
		chunks = append(chunks, c)
		return nil
	})
	if err != nil {
		t.Fatalf("ConsumeStream returned error: %v", err)
	}
	if output != "hello world" {
		t.Fatalf("output = %q, want %q", output, "hello world")
	}
	if len(chunks) < 3 {
		t.Fatalf("expected at least 3 chunks (2 delta + done), got %d", len(chunks))
	}
	if !chunks[len(chunks)-1].Done {
		t.Fatal("expected final chunk to be done")
	}
}

func TestConsumeStream_ToolCalls(t *testing.T) {
	input := strings.NewReader(`data: {"choices":[{"delta":{"tool_calls":[{"index":0,"id":"call_1","function":{"name":"search","arguments":"{\"q\":"}}]}}]}

data: [DONE]
`)
	var got []StreamChunk
	_, err := ConsumeStream(context.Background(), "task-1", input, func(c StreamChunk) error {
		got = append(got, c)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) == 0 || got[0].ToolCall == nil {
		t.Fatalf("expected tool call chunk, got %#v", got)
	}
	if got[0].ToolCall.Name != "search" || got[0].ToolCall.ID != "call_1" {
		t.Fatalf("unexpected tool call: %#v", got[0].ToolCall)
	}
}

func TestConsumeStream_Canceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	input := strings.NewReader(`data: {"choices":[{"delta":{"content":"x"}}]}
`)
	if _, err := ConsumeStream(ctx, "t", input, nil); !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context canceled, got %v", err)
	}
}

func TestConsumeStream_HandlerAbort(t *testing.T) {
	expected := errors.New("ui closed")
	input := strings.NewReader(`data: {"choices":[{"delta":{"content":"x"}}]}
`)
	_, err := ConsumeStream(context.Background(), "t", input, func(StreamChunk) error {
		return expected
	})
	if !errors.Is(err, expected) {
		t.Fatalf("expected handler error, got %v", err)
	}
}

func TestConsumeStream_MalformedLinesSkipped(t *testing.T) {
	input := strings.NewReader(`: heartbeat

data: not-json
data: {"choices":[]}
data: {"choices":[{"delta":{"content":"ok"}}]}

data: [DONE]
`)
	out, err := ConsumeStream(context.Background(), "t", input, nil)
	if err != nil {
		t.Fatal(err)
	}
	if out != "ok" {
		t.Fatalf("output = %q, want ok", out)
	}
}
