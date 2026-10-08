package application

import (
	"context"
	"errors"
	"strings"

	"github.com/RahmatHadinata23758051/CortexOS/internal/cortex/knowledge"
)

// knowledgeAdapter implements KnowledgePort using the knowledge lifecycle service.
type knowledgeAdapter struct {
	svc *knowledge.Service
}

func NewKnowledgeAdapter(svc *knowledge.Service) *knowledgeAdapter {
	return &knowledgeAdapter{svc: svc}
}

func (a *knowledgeAdapter) ListKnowledgeSources(ctx context.Context, projectID string) ([]KnowledgeSourceSummary, error) {
	if projectID == "" {
		return nil, errors.New("project id is required")
	}
	items, err := a.svc.List(ctx, knowledge.Filter{
		ProjectID:  projectIDPtr(projectID),
		ActiveOnly: true,
	})
	if err != nil {
		return nil, err
	}
	result := make([]KnowledgeSourceSummary, 0, len(items))
	for _, item := range items {
		result = append(result, KnowledgeSourceSummary{
			ID:            string(item.ID),
			ProjectID:     string(item.ProjectID),
			Title:         item.Title,
			RelativePath:  item.Provenance.SourceURI,
			SourceKind:    item.Provenance.SourceKind,
			Status:        string(item.Lifecycle),
			ContentHash:   item.ContentHash,
			UpdatedAt:     item.UpdatedAt,
			SchemaVersion: KnowledgeBridgeSchemaVersion,
		})
	}
	return result, nil
}

func (a *knowledgeAdapter) QueryKnowledge(ctx context.Context, projectID, query string, limit int) ([]KnowledgeQueryResult, error) {
	if projectID == "" || strings.TrimSpace(query) == "" {
		return nil, errors.New("project id and query are required")
	}
	if limit <= 0 {
		limit = 10
	}
	if limit > 100 {
		limit = 100
	}

	// For now, filter by active items in the project and do simple text matching
	items, err := a.svc.List(ctx, knowledge.Filter{
		ProjectID:  projectIDPtr(projectID),
		Lifecycle:  lifecycleActivePtr(),
		ActiveOnly: true,
	})
	if err != nil {
		return nil, err
	}

	var matches []knowledge.Item
	queryLower := strings.ToLower(query)
	for _, item := range items {
		if strings.Contains(strings.ToLower(item.Title), queryLower) ||
			strings.Contains(strings.ToLower(item.Content), queryLower) {
			matches = append(matches, item)
		}
		if len(matches) >= limit {
			break
		}
	}

	result := make([]KnowledgeQueryResult, 0, len(matches))
	for _, item := range matches {
		// Extract a snippet around the match
		snippet := extractSnippet(item.Content, query, 200)
		result = append(result, KnowledgeQueryResult{
			DocumentID:    string(item.ID),
			Title:         item.Title,
			RelativePath:  item.Provenance.SourceURI,
			Snippet:       snippet,
			Score:         calculateScore(item.Content, query),
			SchemaVersion: KnowledgeBridgeSchemaVersion,
		})
	}
	return result, nil
}

func projectIDPtr(s string) *knowledge.ProjectID {
	id := knowledge.ProjectID(s)
	return &id
}

func lifecycleActivePtr() *knowledge.LifecycleState {
	s := knowledge.LifecycleActive
	return &s
}

func extractSnippet(content, query string, maxLen int) string {
	if len(content) <= maxLen {
		return content
	}
	idx := strings.Index(strings.ToLower(content), strings.ToLower(query))
	if idx < 0 {
		idx = 0
	}
	start := idx - maxLen/2
	if start < 0 {
		start = 0
	}
	end := start + maxLen
	if end > len(content) {
		end = len(content)
	}
	snippet := content[start:end]
	if start > 0 {
		snippet = "..." + snippet
	}
	if end < len(content) {
		snippet = snippet + "..."
	}
	return snippet
}

func calculateScore(content, query string) float64 {
	contentLower := strings.ToLower(content)
	queryLower := strings.ToLower(query)
	count := 0
	for i := 0; i < len(contentLower); {
		idx := strings.Index(contentLower[i:], queryLower)
		if idx < 0 {
			break
		}
		count++
		i += idx + len(queryLower)
	}
	// Simple score: occurrences normalized by content length
	return float64(count) * 10.0 / float64(len(contentLower)/100+1)
}
