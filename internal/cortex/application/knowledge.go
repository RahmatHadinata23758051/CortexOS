package application

import (
	"context"
	"errors"
	"time"
)

const KnowledgeBridgeSchemaVersion = "cortexos.knowledge.bridge.v1"

var ErrKnowledgeInvalidRequest = errors.New("invalid knowledge bridge request")

// KnowledgePort is the application boundary for knowledge and retrieval observability.
// Raw absolute filesystem paths, database handles, secrets, and provider credentials
// are strictly omitted from bridge records.
type KnowledgePort interface {
	ListKnowledgeSources(ctx context.Context, projectID string) ([]KnowledgeSourceSummary, error)
	QueryKnowledge(ctx context.Context, projectID, query string, limit int) ([]KnowledgeQueryResult, error)
}

// KnowledgeSourceSummary is the safe, serialized form of a knowledge/vault document.
// Absolute filesystem paths, full file contents, and raw database keys are omitted.
type KnowledgeSourceSummary struct {
	ID            string    `json:"id"`
	ProjectID     string    `json:"projectId"`
	Title         string    `json:"title"`
	RelativePath  string    `json:"relativePath"`
	SourceKind    string    `json:"sourceKind"`
	Status        string    `json:"status"`
	ContentHash   string    `json:"contentHash"`
	UpdatedAt     time.Time `json:"updatedAt"`
	SchemaVersion string    `json:"schemaVersion"`
}

// KnowledgeQueryResult is the safe, serialized form of a retrieval match.
// Absolute filesystem paths and raw embedding vectors are omitted.
type KnowledgeQueryResult struct {
	DocumentID    string  `json:"documentId"`
	Title         string  `json:"title"`
	RelativePath  string  `json:"relativePath"`
	Snippet       string  `json:"snippet"`
	Score         float64 `json:"score"`
	SchemaVersion string  `json:"schemaVersion"`
}

type KnowledgeSourcesRequest struct {
	SchemaVersion string `json:"schemaVersion"`
	ProjectID     string `json:"projectId"`
}

type KnowledgeQueryRequest struct {
	SchemaVersion string `json:"schemaVersion"`
	ProjectID     string `json:"projectId"`
	Query         string `json:"query"`
	Limit         int    `json:"limit,omitempty"`
}

// KnowledgeBridgeError is the stable, generic error returned across the Knowledge bridge.
type KnowledgeBridgeError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func (e *KnowledgeBridgeError) Error() string {
	if e == nil {
		return "<nil>"
	}
	return e.Code + ": " + e.Message
}

func validateKnowledgeRequest(ctx context.Context, schemaVersion string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if schemaVersion != KnowledgeBridgeSchemaVersion {
		return errors.New("knowledge: unsupported schema version " + schemaVersion)
	}
	return nil
}

func knowledgeBridgeError(err error) *KnowledgeBridgeError {
	if err == nil {
		return nil
	}
	return &KnowledgeBridgeError{Code: "knowledge.error", Message: err.Error()}
}
