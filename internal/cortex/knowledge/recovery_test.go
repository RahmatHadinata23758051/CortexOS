package knowledge

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/RahmatHadinata23758051/CortexOS/internal/cortex/workspace"
	"github.com/RahmatHadinata23758051/CortexOS/internal/cortex/workspace/retrieval"
	"github.com/RahmatHadinata23758051/CortexOS/internal/cortex/workspace/vault"
)

func TestRebuildAllRecoversFromCorruptIndex(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	mem := workspace.NewMemoryWorkspace()
	if err := mem.Open(ctx); err != nil {
		t.Fatal(err)
	}
	projectID := workspace.ProjectID("proj-recovery")
	if _, err := mem.RegisterProject(ctx, workspace.Project{ID: projectID, Name: "Test"}); err != nil {
		t.Fatal(err)
	}

	created := time.Now().UTC()
	_, err := mem.CreateNote(ctx, workspace.VaultNote{
		ID:            "note-recover",
		ProjectID:     projectID,
		RelativePath:  "docs/recover.md",
		Title:         "Recovery Document",
		Body:          "# Recovery\n\nResilient content that survives index corruption.",
		FormatVersion: vault.FormatVersion,
		Source:        "manual",
		Author:        "recovery-agent",
		CreatedAt:     created,
		UpdatedAt:     created,
		ContentHash:   vault.HashBody("# Recovery\n\nResilient content that survives index corruption."),
		Status:        workspace.NoteStatusActive,
	})
	if err != nil {
		t.Fatal(err)
	}

	knowledgeStore := NewMemoryStore()
	knowledgeSvc, err := NewService(knowledgeStore)
	if err != nil {
		t.Fatal(err)
	}

	indexRoot := t.TempDir()
	retrievalIndex, err := retrieval.New(indexRoot)
	if err != nil {
		t.Fatal(err)
	}

	pipeline, err := NewPipeline(mem, knowledgeSvc, DefaultIngestionConfig())
	if err != nil {
		t.Fatal(err)
	}

	workspaceID := WorkspaceID("ws-recovery")
	indexer := NewIndexer(retrievalIndex, workspaceID)

	// Ingest initially
	res, err := pipeline.RebuildAll(ctx, projectID, workspaceID, indexer)
	if err != nil {
		t.Fatalf("initial RebuildAll failed: %v", err)
	}
	if res.Discovered != 1 || res.Ingested != 1 {
		t.Fatalf("expected 1 discovered and ingested, got %+v", res)
	}

	// Verify query works
	results, err := retrievalIndex.Query(ctx, projectID, "resilient", 10)
	if err != nil || len(results) != 1 {
		t.Fatalf("initial query failed: len=%d, err=%v", len(results), err)
	}

	// Corrupt the index file on disk
	indexPath := retrievalIndex.Path()
	if err := os.WriteFile(indexPath, []byte(`{invalid-json`), 0o600); err != nil {
		t.Fatal(err)
	}

	// Query now fails with corruption error
	_, err = retrievalIndex.Query(ctx, projectID, "resilient", 10)
	if workspace.ErrorCodeOf(err) != workspace.ErrRetrievalCorrupt {
		t.Fatalf("expected ErrRetrievalCorrupt, got %v", err)
	}

	// RebuildAll recovers the index completely from the authoritative Vault
	res2, err := pipeline.RebuildAll(ctx, projectID, workspaceID, indexer)
	if err != nil {
		t.Fatalf("RebuildAll after corruption failed: %v", err)
	}
	if res2.Discovered != 1 || res2.Ingested != 1 {
		t.Fatalf("expected 1 discovered and ingested on recovery, got %+v", res2)
	}

	// Query succeeds again after rebuild
	results2, err := retrievalIndex.Query(ctx, projectID, "resilient", 10)
	if err != nil || len(results2) != 1 {
		t.Fatalf("query after recovery failed: len=%d, err=%v", len(results2), err)
	}
}

func TestRebuildAllCancellation(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	mem := workspace.NewMemoryWorkspace()
	if err := mem.Open(ctx); err != nil {
		t.Fatal(err)
	}
	projectID := workspace.ProjectID("proj-rebuild-cancel")
	if _, err := mem.RegisterProject(ctx, workspace.Project{ID: projectID, Name: "Test"}); err != nil {
		t.Fatal(err)
	}

	knowledgeStore := NewMemoryStore()
	knowledgeSvc, err := NewService(knowledgeStore)
	if err != nil {
		t.Fatal(err)
	}

	indexRoot := t.TempDir()
	retrievalIndex, err := retrieval.New(indexRoot)
	if err != nil {
		t.Fatal(err)
	}

	pipeline, err := NewPipeline(mem, knowledgeSvc, DefaultIngestionConfig())
	if err != nil {
		t.Fatal(err)
	}

	indexer := NewIndexer(retrievalIndex, "ws-cancel")

	cancCtx, cancel := context.WithCancel(ctx)
	cancel()

	_, err = pipeline.RebuildAll(cancCtx, projectID, "ws-cancel", indexer)
	if err == nil {
		t.Fatal("expected cancellation error")
	}
}
