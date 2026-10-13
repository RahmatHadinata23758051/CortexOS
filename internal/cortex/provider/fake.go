package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"sync"
)

// FakeProvider is an in-memory Provider implementation for tests.
// It records calls and returns canned responses.
type FakeProvider struct {
	mu            sync.Mutex
	name          string
	calls         []CompleteRequest
	cannedResp    string
	cannedStream  []string
	cannedModels  string
	returnError   error
	streamDelayMs int
}

// NewFakeProvider returns a FakeProvider initialized with sensible defaults.
func NewFakeProvider(name string) *FakeProvider {
	if name == "" {
		name = "fake"
	}
	return &FakeProvider{
		name: name,
		cannedResp: `{
			"id": "chatcmpl-fake-1",
			"object": "chat.completion",
			"created": 1677652288,
			"model": "fake-model",
			"choices": [{
				"index": 0,
				"message": {
					"role": "assistant",
					"content": "Fake response from in-memory provider."
				},
				"finish_reason": "stop"
			}],
			"usage": {
				"prompt_tokens": 10,
				"completion_tokens": 8,
				"total_tokens": 18
			}
		}`,
		cannedModels: `{
			"object": "list",
			"data": [
				{"id": "fake-model", "object": "model", "owned_by": "test"}
			]
		}`,
	}
}

func (f *FakeProvider) Complete(ctx context.Context, req CompleteRequest) (io.ReadCloser, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.returnError != nil {
		return nil, f.returnError
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	f.calls = append(f.calls, req)
	return io.NopCloser(strings.NewReader(f.cannedResp)), nil
}

func (f *FakeProvider) Stream(ctx context.Context, req CompleteRequest) (io.ReadCloser, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.returnError != nil {
		return nil, f.returnError
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	f.calls = append(f.calls, req)

	// Build SSE lines
	var buf bytes.Buffer
	if len(f.cannedStream) > 0 {
		for _, chunk := range f.cannedStream {
			buf.WriteString("data: " + chunk + "\n\n")
		}
	} else {
		buf.WriteString("data: {\"choices\":[{\"delta\":{\"content\":\"Fake stream chunk\"}}]}\n\n")
		buf.WriteString("data: [DONE]\n\n")
	}

	return io.NopCloser(&buf), nil
}

func (f *FakeProvider) Models(ctx context.Context) (io.ReadCloser, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.returnError != nil {
		return nil, f.returnError
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	return io.NopCloser(strings.NewReader(f.cannedModels)), nil
}

func (f *FakeProvider) Name() string {
	return f.name
}

func (f *FakeProvider) Close() error {
	return nil
}

// Test helpers

func (f *FakeProvider) SetResponse(resp string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.cannedResp = resp
}

func (f *FakeProvider) SetStreamChunks(chunks []string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.cannedStream = chunks
}

func (f *FakeProvider) SetError(err error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.returnError = err
}

func (f *FakeProvider) Calls() []CompleteRequest {
	f.mu.Lock()
	defer f.mu.Unlock()
	copied := make([]CompleteRequest, len(f.calls))
	copy(copied, f.calls)
	return copied
}

func (f *FakeProvider) Reset() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = nil
	f.returnError = nil
}

var _ Provider = (*FakeProvider)(nil)

// Verify that a json message unmarshals safely
func UnmarshalResponse(r io.Reader, v any) error {
	if r == nil {
		return errors.New("reader is nil")
	}
	return json.NewDecoder(r).Decode(v)
}
