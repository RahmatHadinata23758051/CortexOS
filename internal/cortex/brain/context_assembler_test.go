package brain

import (
	"context"
	"errors"
	"testing"

	"github.com/RahmatHadinata23758051/CortexOS/internal/cortex/application"
)

type fakeKnowledge struct {
	results []application.KnowledgeQueryResult
	err     error
}

func (f fakeKnowledge) ListKnowledgeSources(context.Context, string) ([]application.KnowledgeSourceSummary, error) {
	return nil, nil
}
func (f fakeKnowledge) QueryKnowledge(context.Context, string, string, int) ([]application.KnowledgeQueryResult, error) {
	return f.results, f.err
}

func TestContextAssembler_FormatsAndBounds(t *testing.T) {
	a := NewContextAssembler(fakeKnowledge{results: []application.KnowledgeQueryResult{
		{Title: "First", Snippet: "alpha"},
		{Title: "Second", Snippet: "bravo"},
	}}, 30)

	got, err := a.AssembleQuery(context.Background(), "project-1", "query", 5)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d snippets, want 2", len(got))
	}
	if got[0] != "[First] alpha" {
		t.Fatalf("unexpected first snippet: %q", got[0])
	}
}

func TestContextAssembler_Truncates(t *testing.T) {
	a := NewContextAssembler(fakeKnowledge{results: []application.KnowledgeQueryResult{
		{Title: "Long", Snippet: "abcdefghijklmnopqrstuvwxyz"},
	}}, 15)
	got, err := a.AssembleQuery(context.Background(), "project-1", "query", 5)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || len(got[0]) > 15 {
		t.Fatalf("expected bounded snippet, got %#v", got)
	}
}

func TestContextAssembler_OptionalAndInvalid(t *testing.T) {
	if got, err := NewContextAssembler(nil, 10).AssembleQuery(context.Background(), "", "query", 1); err != nil || got != nil {
		t.Fatalf("nil knowledge should be optional, got %#v %v", got, err)
	}
	a := NewContextAssembler(fakeKnowledge{results: nil}, 10)
	got, err := a.AssembleQuery(context.Background(), "", "", 1)
	if err != nil || got != nil {
		t.Fatalf("empty query should return no context, got %#v %v", got, err)
	}
}

func TestContextAssembler_PropagatesError(t *testing.T) {
	expected := errors.New("knowledge unavailable")
	a := NewContextAssembler(fakeKnowledge{err: expected}, 100)
	if _, err := a.AssembleQuery(context.Background(), "p", "q", 1); !errors.Is(err, expected) {
		t.Fatalf("expected wrapped error, got %v", err)
	}
}
