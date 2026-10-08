package knowledge

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/RahmatHadinata23758051/CortexOS/internal/cortex/workspace"
)

// SourceRef identifies a single source document in the Vault.
type SourceRef struct {
	ProjectID    workspace.ProjectID
	NoteID       workspace.NoteID
	RelativePath string
	ContentHash  string
	UpdatedAt    time.Time
}

// Chunk represents a single indexable unit of content.
type Chunk struct {
	Index       int
	Content     string
	HeadingPath []string // e.g., ["Architecture", "Overview"]
	StartOffset int
	EndOffset   int
}

// ChunkID returns a deterministic ID for this chunk.
func (c Chunk) ChunkID(sourceID string) string {
	hash := sha256.Sum256([]byte(fmt.Sprintf("%s\x00%d\x00%s", sourceID, c.Index, c.Content)))
	return "sha256:" + hex.EncodeToString(hash[:])
}

// IngestionConfig controls the ingestion pipeline behavior.
type IngestionConfig struct {
	MaxChunkSize    int      // Maximum chunk size in characters (default: 2000)
	ChunkOverlap    int      // Overlap between chunks in characters (default: 200)
	SecretPatterns  []string // Additional regex patterns for secret detection
	AllowedPatterns []string // Patterns that should not be redacted (allowlist)
	MinChunkSize    int      // Minimum chunk size (default: 100)
}

// DefaultIngestionConfig returns a sensible default configuration.
func DefaultIngestionConfig() IngestionConfig {
	return IngestionConfig{
		MaxChunkSize: 2000,
		ChunkOverlap: 200,
		MinChunkSize: 100,
	}
}

// IngestionResult captures the outcome of an ingestion run.
type IngestionResult struct {
	ProjectID  workspace.ProjectID
	Discovered int
	Ingested   int
	Skipped    int
	Errors     []IngestionError
	Duration   time.Duration
}

// IngestionError represents a non-fatal error during ingestion.
type IngestionError struct {
	SourceRef SourceRef
	Operation string
	Err       error
}

func (e IngestionError) Error() string {
	return fmt.Sprintf("ingestion error [%s]: %v", e.Operation, e.Err)
}

// IndexedDocument represents a document ready for the retrieval index.
type IndexedDocument struct {
	ID           string
	NoteID       workspace.NoteID
	ProjectID    workspace.ProjectID
	WorktreeID   workspace.WorktreeID
	RelativePath string
	ChunkIndex   int
	SourceHash   string
	Content      string
	Attribution  string
	IndexedAt    time.Time
}
