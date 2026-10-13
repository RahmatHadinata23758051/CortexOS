package application

import (
	"context"
	"errors"
	"io"

	"github.com/RahmatHadinata23758051/CortexOS/internal/cortex/provider"
)

// ProviderAdapter adapts a provider.Provider to the application.ProviderPort boundary.
type ProviderAdapter struct {
	provider provider.Provider
}

var _ ProviderPort = (*ProviderAdapter)(nil)

// NewProviderAdapter creates a new ProviderAdapter wrapping the given provider.
func NewProviderAdapter(p provider.Provider) *ProviderAdapter {
	return &ProviderAdapter{provider: p}
}

func (a *ProviderAdapter) Name() string {
	if a == nil || a.provider == nil {
		return "disabled"
	}
	return a.provider.Name()
}

func (a *ProviderAdapter) Complete(ctx context.Context, req ProviderCompleteRequest) (io.ReadCloser, error) {
	if a == nil || a.provider == nil {
		return nil, errors.New("provider not configured")
	}
	messages := make([]provider.Message, 0, len(req.Messages))
	for _, m := range req.Messages {
		messages = append(messages, provider.Message{
			Role:    m.Role,
			Content: m.Content,
		})
	}
	return a.provider.Complete(ctx, provider.CompleteRequest{
		Model:     req.Model,
		Messages:  messages,
		MaxTokens: req.MaxTokens,
	})
}

func (a *ProviderAdapter) Stream(ctx context.Context, req ProviderCompleteRequest) (io.ReadCloser, error) {
	if a == nil || a.provider == nil {
		return nil, errors.New("provider not configured")
	}
	messages := make([]provider.Message, 0, len(req.Messages))
	for _, m := range req.Messages {
		messages = append(messages, provider.Message{
			Role:    m.Role,
			Content: m.Content,
		})
	}
	return a.provider.Stream(ctx, provider.CompleteRequest{
		Model:     req.Model,
		Messages:  messages,
		MaxTokens: req.MaxTokens,
		Stream:    true,
	})
}

func (a *ProviderAdapter) Models(ctx context.Context) (io.ReadCloser, error) {
	if a == nil || a.provider == nil {
		return nil, errors.New("provider not configured")
	}
	return a.provider.Models(ctx)
}
