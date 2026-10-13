package application

import (
	"context"
	"io"
)

// ProviderPort defines the application boundary for external reasoning/LLM providers.
// It keeps secrets, network details, and provider-specific schemas outside the application layer.
type ProviderPort interface {
	Name() string
	Complete(ctx context.Context, request ProviderCompleteRequest) (io.ReadCloser, error)
	Stream(ctx context.Context, request ProviderCompleteRequest) (io.ReadCloser, error)
	Models(ctx context.Context) (io.ReadCloser, error)
}

// ProviderCompleteRequest is the application request shape for provider completion.
type ProviderCompleteRequest struct {
	Model     string            `json:"model"`
	Messages  []ProviderMessage `json:"messages"`
	MaxTokens *int              `json:"max_tokens,omitempty"`
}

// ProviderMessage is a message in a conversation.
type ProviderMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}
