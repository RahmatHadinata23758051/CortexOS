package application

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"
)

type mockKnowledgePort struct {
	sources []KnowledgeSourceSummary
	results []KnowledgeQueryResult
}

func (m *mockKnowledgePort) ListKnowledgeSources(ctx context.Context, projectID string) ([]KnowledgeSourceSummary, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	var res []KnowledgeSourceSummary
	for _, s := range m.sources {
		if projectID == "" || s.ProjectID == projectID {
			res = append(res, s)
		}
	}
	return res, nil
}

func (m *mockKnowledgePort) QueryKnowledge(ctx context.Context, projectID, query string, limit int) ([]KnowledgeQueryResult, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return m.results, nil
}

func TestKnowledgeBridgeSafeSerialization(t *testing.T) {
	now := time.Now().UTC()
	source := KnowledgeSourceSummary{
		ID:            "source-1",
		ProjectID:     "proj-main",
		Title:         "Architecture Decision Record",
		RelativePath:  "docs/adr-001.md",
		SourceKind:    "markdown",
		Status:        "indexed",
		ContentHash:   "hash-123456",
		UpdatedAt:     now,
		SchemaVersion: KnowledgeBridgeSchemaVersion,
	}

	data, err := json.Marshal(source)
	if err != nil {
		t.Fatalf("failed to marshal knowledge source summary: %v", err)
	}

	str := string(data)
	forbiddenWords := []string{
		"rootPath", "vaultRoot", "repositoryRoot", "absolutePath", "token", "secret", "embedding",
	}
	for _, word := range forbiddenWords {
		if strings.Contains(strings.ToLower(str), strings.ToLower(word)) {
			t.Errorf("knowledge source summary contains forbidden token %q: %s", word, str)
		}
	}
}

func TestKnowledgeBridgeOperations(t *testing.T) {
	now := time.Now().UTC()
	mock := &mockKnowledgePort{
		sources: []KnowledgeSourceSummary{
			{
				ID:            "source-1",
				ProjectID:     "proj-1",
				Title:         "Design Specs",
				RelativePath:  "docs/specs.md",
				SourceKind:    "vault",
				Status:        "indexed",
				ContentHash:   "sha-111",
				UpdatedAt:     now,
				SchemaVersion: KnowledgeBridgeSchemaVersion,
			},
		},
		results: []KnowledgeQueryResult{
			{
				DocumentID:    "doc-1",
				Title:         "Design Specs",
				RelativePath:  "docs/specs.md",
				Snippet:       "Overview of the system architecture...",
				Score:         0.95,
				SchemaVersion: KnowledgeBridgeSchemaVersion,
			},
		},
	}

	svc := NewServiceWithKnowledge(mock)
	ctx := context.Background()

	// 1. ListKnowledgeSources
	sources, err := svc.ListKnowledgeSources(ctx, KnowledgeSourcesRequest{
		SchemaVersion: KnowledgeBridgeSchemaVersion,
		ProjectID:     "proj-1",
	})
	if err != nil {
		t.Fatalf("ListKnowledgeSources failed: %v", err)
	}
	if len(sources) != 1 || sources[0].ID != "source-1" {
		t.Errorf("unexpected sources: %+v", sources)
	}

	// 2. QueryKnowledge
	results, err := svc.QueryKnowledge(ctx, KnowledgeQueryRequest{
		SchemaVersion: KnowledgeBridgeSchemaVersion,
		ProjectID:     "proj-1",
		Query:         "architecture",
		Limit:         5,
	})
	if err != nil {
		t.Fatalf("QueryKnowledge failed: %v", err)
	}
	if len(results) != 1 || results[0].Score != 0.95 {
		t.Errorf("unexpected query results: %+v", results)
	}

	// 3. Schema validation error
	_, err = svc.ListKnowledgeSources(ctx, KnowledgeSourcesRequest{
		SchemaVersion: "unknown-v99",
		ProjectID:     "proj-1",
	})
	if err == nil {
		t.Fatal("expected error for unsupported schema version")
	}

	// 4. Context cancellation
	cancCtx, cancel := context.WithCancel(ctx)
	cancel()
	_, err = svc.QueryKnowledge(cancCtx, KnowledgeQueryRequest{
		SchemaVersion: KnowledgeBridgeSchemaVersion,
		ProjectID:     "proj-1",
		Query:         "architecture",
	})
	if err == nil || !errors.Is(err, context.Canceled) {
		var bridgeErr *KnowledgeBridgeError
		if !errors.As(err, &bridgeErr) {
			t.Errorf("expected context canceled error, got %v", err)
		}
	}
}
