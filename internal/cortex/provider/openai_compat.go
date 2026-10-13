package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
)

// OpenAICompatible implements Provider against any OpenAI-compatible endpoint.
// It uses a standard http.Client and performs request/response translation.
type OpenAICompatible struct {
	client   *http.Client
	baseURL  string
	model    string
	apiKey   string
	extra    map[string]string
	closed   bool
	closedMu sync.RWMutex
}

// NewOpenAICompatible creates a provider for an OpenAI-compatible endpoint.
// baseURL must include the /v1 path (e.g., "http://localhost:20128/v1").
// model is the default model to use (e.g., "OMP").
// apiKey may be empty for local endpoints that don't require auth.
func NewOpenAICompatible(baseURL, model, apiKey string, timeoutSec int, extra map[string]string) (*OpenAICompatible, error) {
	if baseURL == "" {
		return nil, fmt.Errorf("baseURL is required")
	}
	baseURL = strings.TrimRight(baseURL, "/")
	if !strings.HasSuffix(baseURL, "/v1") {
		baseURL = baseURL + "/v1"
	}
	if model == "" {
		return nil, fmt.Errorf("model is required")
	}
	if timeoutSec <= 0 {
		timeoutSec = 60
	}

	return &OpenAICompatible{
		baseURL: baseURL,
		model:   model,
		apiKey:  apiKey,
		extra:   extra,
		client: &http.Client{
			Timeout: time.Duration(timeoutSec) * time.Second,
		},
	}, nil
}

func (p *OpenAICompatible) Name() string {
	return "openai-compatible"
}

func (p *OpenAICompatible) Complete(ctx context.Context, req CompleteRequest) (io.ReadCloser, error) {
	if err := p.checkClosed(); err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	// Use default model if not specified
	if req.Model == "" {
		req.Model = p.model
	}

	// Stream must be false for Complete
	req.Stream = false

	return p.doRequest(ctx, "/chat/completions", req)
}

func (p *OpenAICompatible) Stream(ctx context.Context, req CompleteRequest) (io.ReadCloser, error) {
	if err := p.checkClosed(); err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	if req.Model == "" {
		req.Model = p.model
	}
	req.Stream = true

	return p.doRequest(ctx, "/chat/completions", req)
}

func (p *OpenAICompatible) Models(ctx context.Context) (io.ReadCloser, error) {
	if err := p.checkClosed(); err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	return p.doRequest(ctx, "/models", nil)
}

func (p *OpenAICompatible) Close() error {
	p.closedMu.Lock()
	defer p.closedMu.Unlock()
	if p.closed {
		return nil
	}
	p.closed = true
	// http.Client has no explicit Close; just let it be GC'd
	return nil
}

func (p *OpenAICompatible) checkClosed() error {
	p.closedMu.RLock()
	defer p.closedMu.RUnlock()
	if p.closed {
		return &ErrProviderUnavailable{Provider: p.Name(), Reason: "closed"}
	}
	return nil
}

func (p *OpenAICompatible) doRequest(ctx context.Context, path string, body any) (io.ReadCloser, error) {
	method := http.MethodGet
	var bodyReader io.Reader
	if body != nil {
		buf := new(bytes.Buffer)
		if err := json.NewEncoder(buf).Encode(body); err != nil {
			return nil, fmt.Errorf("encode request: %w", err)
		}
		bodyReader = buf
		method = http.MethodPost
	}

	req, err := http.NewRequestWithContext(ctx, method, p.baseURL+path, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	if p.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+p.apiKey)
	}
	for k, v := range p.extra {
		req.Header.Set(k, v)
	}

	resp, err := p.client.Do(req)
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return nil, &ErrProviderTimeout{Provider: p.Name(), Op: path}
		}
		if strings.Contains(err.Error(), "timeout") || strings.Contains(err.Error(), "deadline") {
			return nil, &ErrProviderTimeout{Provider: p.Name(), Op: path}
		}
		return nil, &ErrProviderUnavailable{Provider: p.Name(), Reason: err.Error()}
	}

	if resp.StatusCode >= 400 {
		defer resp.Body.Close()
		bodyBytes, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, &ErrProviderUnavailable{
			Provider: p.Name(),
			Reason:   fmt.Sprintf("HTTP %d: %s", resp.StatusCode, string(bodyBytes)),
		}
	}

	return resp.Body, nil
}

var _ Provider = (*OpenAICompatible)(nil)
