package application

import (
	"errors"
	"strings"
	"testing"
)

func TestKnowledgeContextBudgetValidation(t *testing.T) {
	b := DefaultKnowledgeContextBudget()
	if err := b.Validate(); err != nil {
		t.Fatalf("default budget should be valid, got %v", err)
	}

	tests := []struct {
		name string
		edit func(*KnowledgeContextBudget)
	}{
		{"maxItems ceiling", func(b *KnowledgeContextBudget) { b.MaxItems = 21 }},
		{"maxSnippetChars ceiling", func(b *KnowledgeContextBudget) { b.MaxSnippetChars = 2001 }},
		{"maxTotalChars ceiling", func(b *KnowledgeContextBudget) { b.MaxTotalChars = 8001 }},
		{"timeoutMs ceiling", func(b *KnowledgeContextBudget) { b.TimeoutMs = 10001 }},
		{"negative value", func(b *KnowledgeContextBudget) { b.MaxItems = -1 }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cand := DefaultKnowledgeContextBudget()
			tt.edit(&cand)
			if err := cand.Validate(); err == nil || !errors.Is(err, ErrKnowledgeContextInvalidRequest) {
				t.Fatalf("expected ErrKnowledgeContextInvalidRequest, got %v", err)
			}
		})
	}
}

func TestKnowledgeContextRequestValidation(t *testing.T) {
	req := KnowledgeContextRequest{
		ProjectID:     "proj-1",
		Query:         "how to deploy",
		Budget:        DefaultKnowledgeContextBudget(),
		SchemaVersion: KnowledgeContextSchemaVersion,
	}
	if err := req.Validate(); err != nil {
		t.Fatalf("valid request failed: %v", err)
	}

	tests := []struct {
		name string
		edit func(*KnowledgeContextRequest)
	}{
		{"schema version", func(r *KnowledgeContextRequest) { r.SchemaVersion = "bad.v1" }},
		{"project ID empty", func(r *KnowledgeContextRequest) { r.ProjectID = "" }},
		{"query empty", func(r *KnowledgeContextRequest) { r.Query = "" }},
		{"query too long", func(r *KnowledgeContextRequest) { r.Query = strings.Repeat("x", 4001) }},
		{"budget invalid", func(r *KnowledgeContextRequest) { r.Budget.MaxItems = 99 }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cand := req
			tt.edit(&cand)
			if err := cand.Validate(); err == nil || !errors.Is(err, ErrKnowledgeContextInvalidRequest) {
				t.Fatalf("expected error, got %v", err)
			}
		})
	}
}

func TestSanitizeSourceLabel(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"", "unknown"},
		{"docs/arch.md", "docs/arch.md"},
		{`C:\Users\Secret\repo\docs\arch.md`, "repo/docs/arch.md"},
		{"/home/user/repo/specs/plan.md", "repo/specs/plan.md"},
		{"/etc/passwd", "source"},
	}

	for _, tt := range tests {
		got := SanitizeSourceLabel(tt.input)
		if strings.Contains(got, `C:\`) || strings.HasPrefix(got, "/home") || strings.HasPrefix(got, "/etc") {
			t.Errorf("SanitizeSourceLabel(%q) leaked absolute path: %q", tt.input, got)
		}
		if got != tt.expected {
			t.Errorf("SanitizeSourceLabel(%q) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}

func TestSanitizeSnippet(t *testing.T) {
	input := "Bearer secret_token_12345 should not leak. Also C:\\Windows\\System32 and /home/user/.ssh/id_rsa."
	clean, _ := SanitizeSnippet(input, 200)

	for _, forbidden := range []string{"secret_token_12345", `C:\Windows`, "/home/user"} {
		if strings.Contains(clean, forbidden) {
			t.Errorf("SanitizeSnippet leaked forbidden token %q: %q", forbidden, clean)
		}
	}

	// Truncation
	snip, trunc := SanitizeSnippet("Hello world reasoning", 10)
	if !trunc || len(snip) > 10 || !strings.HasSuffix(snip, "...") {
		t.Errorf("expected truncated snippet with ellipsis, got %q (trunc=%v)", snip, trunc)
	}
}

func TestFoldKnowledgeQueryResults_DeterministicAndBounded(t *testing.T) {
	results := []KnowledgeQueryResult{
		{DocumentID: "doc-2", Title: "Doc Two", RelativePath: "docs/two.md", Snippet: "Second doc content", Score: 0.8},
		{DocumentID: "doc-1", Title: "Doc One", RelativePath: "docs/one.md", Snippet: "First doc content", Score: 0.95},
		{DocumentID: "doc-3", Title: "Doc Three", RelativePath: "docs/three.md", Snippet: "Third doc content", Score: 0.5},
	}

	budget := KnowledgeContextBudget{
		MaxItems:        2,
		MaxSnippetChars: 50,
		MaxTotalChars:   200,
		TimeoutMs:       1000,
	}

	bundle := FoldKnowledgeQueryResults(results, budget)
	if len(bundle.Items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(bundle.Items))
	}
	// Highest score should be first
	if bundle.Items[0].ID != "doc-1" || bundle.Items[1].ID != "doc-2" {
		t.Fatalf("items not sorted by score: %#v", bundle.Items)
	}
	if !bundle.Truncated {
		t.Fatalf("expected truncated=true since 3 results existed for budget 2")
	}

	prompt := BuildGroundedContextPrompt(bundle)
	if !strings.Contains(prompt, "Doc One") || !strings.Contains(prompt, "Doc Two") {
		t.Fatalf("prompt missing expected titles: %s", prompt)
	}
	if strings.Contains(prompt, "Doc Three") {
		t.Fatalf("prompt includes item beyond MaxItems: %s", prompt)
	}
}
