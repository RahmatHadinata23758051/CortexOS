package bridge

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/RahmatHadinata23758051/CortexOS/internal/cortex/application"
)

type groundingKnowledgeStub struct {
	results []application.KnowledgeQueryResult
	err     error
	delay   time.Duration
}

func (s groundingKnowledgeStub) ListKnowledgeSources(context.Context, string) ([]application.KnowledgeSourceSummary, error) {
	return nil, nil
}

func (s groundingKnowledgeStub) QueryKnowledge(ctx context.Context, _ string, _ string, _ int) ([]application.KnowledgeQueryResult, error) {
	if s.delay > 0 {
		select {
		case <-time.After(s.delay):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	return s.results, s.err
}

func groundingRequest() application.KnowledgeContextRequest {
	return application.KnowledgeContextRequest{
		ProjectID:     "proj-1",
		Query:         "architecture",
		Budget:        application.KnowledgeContextBudget{MaxItems: 2, MaxSnippetChars: 80, MaxTotalChars: 150, TimeoutMs: 20},
		SchemaVersion: application.KnowledgeContextSchemaVersion,
	}
}

func TestKnowledgeGroundingAdapter_BoundsAndSanitizes(t *testing.T) {
	adapter, err := NewKnowledgeGroundingAdapter(groundingKnowledgeStub{results: []application.KnowledgeQueryResult{
		{DocumentID: "doc-1", Title: "Architecture", RelativePath: `C:\Users\secret\repo\docs\arch.md`, Snippet: "Use /home/user and apiKey=hidden", Score: 0.9},
		{DocumentID: "doc-2", Title: "Rules", RelativePath: "docs/rules.md", Snippet: "Short safe snippet", Score: 0.8},
		{DocumentID: "doc-3", Title: "Ignored", RelativePath: "docs/ignored.md", Snippet: "ignored", Score: 0.1},
	}})
	if err != nil {
		t.Fatal(err)
	}

	bundle, err := adapter.GroundRoutingContext(context.Background(), groundingRequest())
	if err != nil {
		t.Fatalf("GroundRoutingContext failed: %v", err)
	}
	if len(bundle.Items) != 2 || !bundle.Truncated {
		t.Fatalf("unexpected bounded bundle: %#v", bundle)
	}
	if bundle.Items[0].SourceLabel == `C:\Users\secret\repo\docs\arch.md` {
		t.Fatalf("absolute source path leaked: %#v", bundle.Items[0])
	}
	if bundle.Items[0].Snippet == "" || bundle.Items[0].Snippet == "Use /home/user and apiKey=hidden" {
		t.Fatalf("snippet was not sanitized: %#v", bundle.Items[0])
	}
}

func TestKnowledgeGroundingAdapter_TimeoutDegrades(t *testing.T) {
	adapter, err := NewKnowledgeGroundingAdapter(groundingKnowledgeStub{delay: 100 * time.Millisecond})
	if err != nil {
		t.Fatal(err)
	}

	bundle, err := adapter.GroundRoutingContext(context.Background(), groundingRequest())
	if err != nil {
		t.Fatalf("timeout should degrade without error: %v", err)
	}
	if !bundle.Degraded || bundle.DegradedReason != "retrieval_timeout" {
		t.Fatalf("unexpected degraded result: %#v", bundle)
	}
}

func TestKnowledgeGroundingAdapter_EmptyAndUnavailableDegrade(t *testing.T) {
	for _, tc := range []struct {
		name   string
		stub   groundingKnowledgeStub
		reason string
	}{
		{name: "empty", stub: groundingKnowledgeStub{}, reason: "no_matches"},
		{name: "unavailable", stub: groundingKnowledgeStub{err: errors.New("storage unavailable")}, reason: "retrieval_unavailable"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			adapter, err := NewKnowledgeGroundingAdapter(tc.stub)
			if err != nil {
				t.Fatal(err)
			}
			bundle, err := adapter.GroundRoutingContext(context.Background(), groundingRequest())
			if err != nil {
				t.Fatalf("expected safe degradation, got %v", err)
			}
			if !bundle.Degraded || bundle.DegradedReason != tc.reason {
				t.Fatalf("unexpected degraded bundle: %#v", bundle)
			}
		})
	}
}

func TestKnowledgeGroundingAdapter_CancellationPropagates(t *testing.T) {
	adapter, err := NewKnowledgeGroundingAdapter(groundingKnowledgeStub{})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = adapter.GroundRoutingContext(ctx, groundingRequest())
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected cancellation, got %v", err)
	}
}
