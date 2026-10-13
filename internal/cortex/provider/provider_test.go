package provider

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

func TestFakeProvider_Complete(t *testing.T) {
	fp := NewFakeProvider("test-fake")
	ctx := context.Background()

	req := CompleteRequest{
		Model: "test-model",
		Messages: []Message{
			{Role: "user", Content: "hello"},
		},
	}

	reader, err := fp.Complete(ctx, req)
	if err != nil {
		t.Fatalf("Complete failed: %v", err)
	}
	defer reader.Close()

	var resp struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.NewDecoder(reader).Decode(&resp); err != nil {
		t.Fatalf("decode failed: %v", err)
	}

	if len(resp.Choices) == 0 || resp.Choices[0].Message.Content == "" {
		t.Fatalf("expected non-empty message content, got %+v", resp)
	}

	calls := fp.Calls()
	if len(calls) != 1 {
		t.Fatalf("expected 1 call, got %d", len(calls))
	}
	if calls[0].Model != "test-model" {
		t.Errorf("expected model test-model, got %s", calls[0].Model)
	}
}

func TestFakeProvider_Stream(t *testing.T) {
	fp := NewFakeProvider("test-fake")
	ctx := context.Background()

	fp.SetStreamChunks([]string{
		`{"choices":[{"delta":{"content":"chunk1"}}]}`,
		`{"choices":[{"delta":{"content":"chunk2"}}]}`,
	})

	req := CompleteRequest{
		Model: "test-model",
		Messages: []Message{
			{Role: "user", Content: "stream test"},
		},
	}

	reader, err := fp.Stream(ctx, req)
	if err != nil {
		t.Fatalf("Stream failed: %v", err)
	}
	defer reader.Close()

	body, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("read stream failed: %v", err)
	}

	s := string(body)
	if !strings.Contains(s, "chunk1") || !strings.Contains(s, "chunk2") {
		t.Fatalf("expected stream to contain chunk1 and chunk2, got %q", s)
	}
}

func TestFakeProvider_Cancellation(t *testing.T) {
	fp := NewFakeProvider("test-fake")
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	req := CompleteRequest{Model: "test"}
	_, err := fp.Complete(ctx, req)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
}

func TestFakeProvider_Error(t *testing.T) {
	fp := NewFakeProvider("test-fake")
	expectedErr := errors.New("provider failure")
	fp.SetError(expectedErr)

	ctx := context.Background()
	_, err := fp.Complete(ctx, CompleteRequest{Model: "test"})
	if !errors.Is(err, expectedErr) {
		t.Fatalf("expected %v, got %v", expectedErr, err)
	}
}

func TestOpenAICompatible_Construct(t *testing.T) {
	// BaseURL missing
	_, err := NewOpenAICompatible("", "OMP", "", 60, nil)
	if err == nil {
		t.Error("expected error for empty baseURL")
	}

	// Model missing
	_, err = NewOpenAICompatible("http://localhost:20128/v1", "", "", 60, nil)
	if err == nil {
		t.Error("expected error for empty model")
	}

	// Normal construction automatically adds /v1
	p, err := NewOpenAICompatible("http://localhost:20128", "OMP", "test-key", 30, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.baseURL != "http://localhost:20128/v1" {
		t.Errorf("expected baseURL http://localhost:20128/v1, got %s", p.baseURL)
	}
}

func TestOpenAICompatible_MockServer_Complete(t *testing.T) {
	mockResp := `{"id":"test-1","choices":[{"message":{"role":"assistant","content":"mock reply"}}]}`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" {
			t.Errorf("expected path /v1/chat/completions, got %s", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer mock-api-key" {
			t.Errorf("expected bearer token mock-api-key, got %s", r.Header.Get("Authorization"))
		}

		var body CompleteRequest
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("decode request failed: %v", err)
		}
		if body.Stream {
			t.Errorf("expected stream false in Complete")
		}

		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockResp)
	}))
	defer server.Close()

	provider, err := NewOpenAICompatible(server.URL+"/v1", "OMP", "mock-api-key", 5, nil)
	if err != nil {
		t.Fatalf("create provider failed: %v", err)
	}
	defer provider.Close()

	reader, err := provider.Complete(context.Background(), CompleteRequest{
		Messages: []Message{{Role: "user", Content: "hi"}},
	})
	if err != nil {
		t.Fatalf("Complete failed: %v", err)
	}
	defer reader.Close()

	content, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("read failed: %v", err)
	}
	if string(content) != mockResp {
		t.Errorf("expected %q, got %q", mockResp, string(content))
	}
}

func TestOpenAICompatible_MockServer_Timeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
	}))
	defer server.Close()

	provider, err := NewOpenAICompatible(server.URL+"/v1", "OMP", "", 1, nil)
	if err != nil {
		t.Fatalf("create provider failed: %v", err)
	}
	defer provider.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()

	_, err = provider.Complete(ctx, CompleteRequest{
		Messages: []Message{{Role: "user", Content: "hi"}},
	})
	if err == nil {
		t.Fatal("expected timeout error, got nil")
	}
	if !IsProviderTimeout(err) {
		t.Fatalf("expected ErrProviderTimeout, got %T: %v", err, err)
	}
}

func TestOpenAICompatible_MockServer_HTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"error":{"message":"rate limited"}}`, http.StatusTooManyRequests)
	}))
	defer server.Close()

	provider, err := NewOpenAICompatible(server.URL+"/v1", "OMP", "", 5, nil)
	if err != nil {
		t.Fatalf("create provider failed: %v", err)
	}
	defer provider.Close()

	_, err = provider.Complete(context.Background(), CompleteRequest{
		Messages: []Message{{Role: "user", Content: "hi"}},
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !IsProviderUnavailable(err) {
		t.Fatalf("expected ErrProviderUnavailable, got %T: %v", err, err)
	}
	if !strings.Contains(err.Error(), "429") {
		t.Errorf("expected error message to contain 429, got %v", err)
	}
}

func TestOpenAICompatible_Close(t *testing.T) {
	provider, err := NewOpenAICompatible("http://localhost:20128/v1", "OMP", "", 5, nil)
	if err != nil {
		t.Fatalf("create provider failed: %v", err)
	}
	if err := provider.Close(); err != nil {
		t.Fatalf("close failed: %v", err)
	}

	_, err = provider.Complete(context.Background(), CompleteRequest{
		Messages: []Message{{Role: "user", Content: "hi"}},
	})
	if err == nil {
		t.Fatal("expected error after close, got nil")
	}
}

// TestOpenAICompatible_9router_Live_OptIn runs against the local 9router only
// when explicitly opted in via CORTEXOS_9ROUTER_LIVE_TEST=1.
func TestOpenAICompatible_9router_Live_OptIn(t *testing.T) {
	if os.Getenv("CORTEXOS_9ROUTER_LIVE_TEST") != "1" {
		t.Skip("skipping opt-in live 9router test (set CORTEXOS_9ROUTER_LIVE_TEST=1 to run)")
	}

	p, err := NewOpenAICompatible("http://localhost:20128/v1", "OMP", "", 15, nil)
	if err != nil {
		t.Fatalf("create provider failed: %v", err)
	}
	defer p.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	maxTokens := 10
	reader, err := p.Complete(ctx, CompleteRequest{
		Model: "OMP",
		Messages: []Message{
			{Role: "user", Content: "ping"},
		},
		MaxTokens: &maxTokens,
	})
	if err != nil {
		t.Fatalf("live 9router complete failed: %v", err)
	}
	defer reader.Close()

	var resp struct {
		ID      string `json:"id"`
		Model   string `json:"model"`
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.NewDecoder(reader).Decode(&resp); err != nil {
		t.Fatalf("decode live response failed: %v", err)
	}

	if resp.ID == "" {
		t.Errorf("expected non-empty ID from 9router")
	}
	t.Logf("Live 9router responded successfully: ID=%s, Model=%s", resp.ID, resp.Model)
}
