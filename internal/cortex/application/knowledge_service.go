package application

import (
	"context"
	"errors"
)

func (s *Service) ListKnowledgeSources(ctx context.Context, request KnowledgeSourcesRequest) ([]KnowledgeSourceSummary, error) {
	if err := validateKnowledgeRequest(ctx, request.SchemaVersion); err != nil {
		return nil, knowledgeBridgeError(err)
	}
	if s == nil || s.knowledge == nil {
		return nil, knowledgeBridgeError(errors.New("knowledge service unavailable"))
	}
	items, err := s.knowledge.ListKnowledgeSources(ctx, request.ProjectID)
	if err != nil {
		return nil, knowledgeBridgeError(err)
	}
	result := make([]KnowledgeSourceSummary, 0, len(items))
	for _, item := range items {
		result = append(result, sanitizeKnowledgeSource(item))
	}
	return result, nil
}

func (s *Service) QueryKnowledge(ctx context.Context, request KnowledgeQueryRequest) ([]KnowledgeQueryResult, error) {
	if err := validateKnowledgeRequest(ctx, request.SchemaVersion); err != nil {
		return nil, knowledgeBridgeError(err)
	}
	if s == nil || s.knowledge == nil {
		return nil, knowledgeBridgeError(errors.New("knowledge service unavailable"))
	}
	limit := request.Limit
	if limit <= 0 {
		limit = 10
	}
	if limit > 100 {
		limit = 100
	}
	items, err := s.knowledge.QueryKnowledge(ctx, request.ProjectID, request.Query, limit)
	if err != nil {
		return nil, knowledgeBridgeError(err)
	}
	result := make([]KnowledgeQueryResult, 0, len(items))
	for _, item := range items {
		result = append(result, sanitizeKnowledgeQuery(item))
	}
	return result, nil
}

func sanitizeKnowledgeSource(item KnowledgeSourceSummary) KnowledgeSourceSummary {
	item.SchemaVersion = KnowledgeBridgeSchemaVersion
	return item
}

func sanitizeKnowledgeQuery(item KnowledgeQueryResult) KnowledgeQueryResult {
	item.SchemaVersion = KnowledgeBridgeSchemaVersion
	return item
}
