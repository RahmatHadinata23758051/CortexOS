package provider

import (
	"context"
	"io"
)

// Provider is the contract for LLM completion providers.
// Implementations must be safe for concurrent use and respect context cancellation.
type Provider interface {
	// Complete performs a non-streaming chat completion.
	// The returned body is a JSON payload matching OpenAI Chat Completion response format.
	Complete(ctx context.Context, request CompleteRequest) (io.ReadCloser, error)

	// Stream performs a streaming chat completion.
	// The returned reader emits SSE-formatted data lines (data: {...}\n\n).
	// The caller must close the reader. Cancellation via ctx stops the stream.
	Stream(ctx context.Context, request CompleteRequest) (io.ReadCloser, error)

	// Models returns the list of available models.
	// The returned body is a JSON payload matching OpenAI Models list response format.
	Models(ctx context.Context) (io.ReadCloser, error)

	// Name returns a short identifier for this provider (e.g., "9router", "ollama", "openai").
	Name() string

	// Close releases any resources held by the provider.
	Close() error
}

// CompleteRequest is the normalized request sent to any provider.
// It maps 1:1 with OpenAI Chat Completions request fields.
type CompleteRequest struct {
	Model       string    `json:"model"`
	Messages    []Message `json:"messages"`
	Temperature *float64  `json:"temperature,omitempty"`
	MaxTokens   *int      `json:"max_tokens,omitempty"`
	Stream      bool      `json:"stream,omitempty"`
	Tools       []Tool    `json:"tools,omitempty"`
	ToolChoice  any       `json:"tool_choice,omitempty"`
	// Extra holds provider-specific fields that are passed through.
	Extra map[string]any `json:"-"`
}

// Message represents a chat message.
type Message struct {
	Role       string     `json:"role"`                   // "system", "user", "assistant", "tool"
	Content    string     `json:"content,omitempty"`      // text content
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`   // assistant tool calls
	ToolCallID string     `json:"tool_call_id,omitempty"` // tool response
	Name       string     `json:"name,omitempty"`         // function name for tool calls
}

// ToolCall represents a function call from the model.
type ToolCall struct {
	ID       string `json:"id"`
	Type     string `json:"type"` // "function"
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	} `json:"function"`
}

// Tool represents a function definition.
type Tool struct {
	Type     string `json:"type"` // "function"
	Function struct {
		Name        string `json:"name"`
		Description string `json:"description,omitempty"`
		Parameters  any    `json:"parameters,omitempty"`
	} `json:"function"`
}

// ProviderConfig holds configuration for a provider implementation.
type ProviderConfig struct {
	BaseURL    string
	Model      string
	APIKey     string
	TimeoutSec int
	Extra      map[string]string
}

// ErrProviderUnavailable is returned when the provider is not configured or unreachable.
type ErrProviderUnavailable struct {
	Provider string
	Reason   string
}

func (e *ErrProviderUnavailable) Error() string {
	return "provider " + e.Provider + " unavailable: " + e.Reason
}

// IsProviderUnavailable checks if an error is ErrProviderUnavailable.
func IsProviderUnavailable(err error) bool {
	_, ok := err.(*ErrProviderUnavailable)
	return ok
}

// ErrProviderTimeout is returned when a provider request times out.
type ErrProviderTimeout struct {
	Provider string
	Op       string
}

func (e *ErrProviderTimeout) Error() string {
	return "provider " + e.Provider + " timeout during " + e.Op
}

// IsProviderTimeout checks if an error is ErrProviderTimeout.
func IsProviderTimeout(err error) bool {
	_, ok := err.(*ErrProviderTimeout)
	return ok
}
