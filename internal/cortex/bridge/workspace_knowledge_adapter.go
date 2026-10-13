package bridge

import (
	"context"
	"errors"
	"strings"

	"github.com/RahmatHadinata23758051/CortexOS/internal/cortex/application"
	"github.com/RahmatHadinata23758051/CortexOS/internal/cortex/workspace"
)

// WorkspaceKnowledgeAdapter exposes the workspace retrieval index through the
// application KnowledgePort without crossing raw workspace DTOs or paths.
type WorkspaceKnowledgeAdapter struct {
	workspace *workspace.Service
}

func NewWorkspaceKnowledgeAdapter(service *workspace.Service) (application.KnowledgePort, error) {
	if service == nil {
		return nil, errors.New("workspace service cannot be nil")
	}
	return &WorkspaceKnowledgeAdapter{workspace: service}, nil
}

func (a *WorkspaceKnowledgeAdapter) ListKnowledgeSources(ctx context.Context, projectID string) ([]application.KnowledgeSourceSummary, error) {
	if strings.TrimSpace(projectID) == "" {
		return nil, errors.New("project id is required")
	}
	notes, err := a.workspace.ListNotes(ctx, workspace.ProjectID(projectID))
	if err != nil {
		return nil, err
	}
	sources := make([]application.KnowledgeSourceSummary, 0, len(notes))
	for _, note := range notes {
		sources = append(sources, application.KnowledgeSourceSummary{
			ID:            string(note.ID),
			ProjectID:     string(note.ProjectID),
			Title:         note.Title,
			RelativePath:  note.RelativePath,
			SourceKind:    "workspace_note",
			Status:        note.Status,
			ContentHash:   note.ContentHash,
			UpdatedAt:     note.UpdatedAt,
			SchemaVersion: application.KnowledgeBridgeSchemaVersion,
		})
	}
	return sources, nil
}

func (a *WorkspaceKnowledgeAdapter) QueryKnowledge(ctx context.Context, projectID, query string, limit int) ([]application.KnowledgeQueryResult, error) {
	if strings.TrimSpace(projectID) == "" || strings.TrimSpace(query) == "" {
		return nil, errors.New("project id and query are required")
	}
	if limit <= 0 {
		limit = 10
	}
	if limit > 100 {
		limit = 100
	}
	documents, err := a.workspace.QueryRetrieval(ctx, workspace.ProjectID(projectID), query, limit)
	if err != nil {
		return nil, err
	}
	results := make([]application.KnowledgeQueryResult, 0, len(documents))
	for _, document := range documents {
		results = append(results, application.KnowledgeQueryResult{
			DocumentID:    document.ID,
			Title:         "Knowledge document",
			RelativePath:  document.RelativePath,
			Snippet:       boundedKnowledgeSnippet(document.Content, 400),
			Score:         knowledgeMatchScore(document.Content, query),
			SchemaVersion: application.KnowledgeBridgeSchemaVersion,
		})
	}
	return results, nil
}

func boundedKnowledgeSnippet(content string, max int) string {
	content = strings.TrimSpace(content)
	if len(content) <= max {
		return content
	}
	return content[:max-3] + "..."
}

func knowledgeMatchScore(content, query string) float64 {
	content = strings.ToLower(content)
	query = strings.ToLower(strings.TrimSpace(query))
	if query == "" || !strings.Contains(content, query) {
		return 0
	}
	return 1
}
