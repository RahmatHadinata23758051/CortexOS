package refresh

import (
	"context"

	"github.com/RahmatHadinata23758051/CortexOS/internal/cortex/knowledge"
	"github.com/RahmatHadinata23758051/CortexOS/internal/cortex/workspace"
)

// KnowledgeIngestHandler creates a refresh handler that performs incremental knowledge ingestion.
func KnowledgeIngestHandler(pipeline *knowledge.Pipeline, indexer *knowledge.Indexer, workspaceID knowledge.WorkspaceID) Handler {
	return func(ctx context.Context, request Request) error {
		if pipeline == nil || indexer == nil {
			return workspace.NewError(workspace.ErrInvalidRequest, "pipeline and indexer are required")
		}

		switch request.Operation {
		case workspace.FileRescanRequired, workspace.FileError:
			// Full rebuild on rescan or error
			_, err := pipeline.RebuildAll(ctx, request.ProjectID, workspaceID, indexer)
			return err
		case workspace.FileCreated, workspace.FileModified, workspace.FileRemoved, workspace.FileRenamed:
			// Incremental update for regular changes
			_, err := pipeline.IncrementalIngest(ctx, request.ProjectID, workspaceID, indexer)
			return err
		default:
			// Ignore unknown operations
			return nil
		}
	}
}
