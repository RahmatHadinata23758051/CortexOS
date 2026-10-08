package refresh

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/RahmatHadinata23758051/CortexOS/internal/cortex/knowledge"
	"github.com/RahmatHadinata23758051/CortexOS/internal/cortex/workspace"
	"github.com/RahmatHadinata23758051/CortexOS/internal/cortex/workspace/retrieval"
	"github.com/RahmatHadinata23758051/CortexOS/internal/cortex/workspace/vault"
)

func TestKnowledgeIngestHandlerRefreshesOnVaultChange(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	root := t.TempDir()
	vaultRoot := filepath.Join(root, "vault")
	vaultStore, err := vault.New(vaultRoot)
	if err != nil {
		t.Fatal(err)
	}

	created := time.Now().UTC()
	note, err := vaultStore.CreateNote(ctx, workspace.VaultNote{
		ID:            "note-refresh-1",
		ProjectID:     "project-refresh",
		RelativePath:  "docs/architecture.md",
		Title:         "Architecture",
		Body:          "# Architecture\n\nInitial architecture notes.\n",
		FormatVersion: vault.FormatVersion,
		Source:        "vault",
		Author:        "lead-architect",
		CreatedAt:     created,
		UpdatedAt:     created,
	})
	if err != nil {
		t.Fatal(err)
	}

	knowledgeStore := knowledge.NewMemoryStore()
	knowledgeSvc, err := knowledge.NewService(knowledgeStore)
	if err != nil {
		t.Fatal(err)
	}

	indexRoot := filepath.Join(root, "index")
	retrievalIndex, err := retrieval.New(indexRoot)
	if err != nil {
		t.Fatal(err)
	}

	pipeline, err := knowledge.NewPipeline(vaultStore, knowledgeSvc, knowledge.DefaultIngestionConfig())
	if err != nil {
		t.Fatal(err)
	}

	workspaceID := knowledge.WorkspaceID("ws-refresh")
	indexer := knowledge.NewIndexer(retrievalIndex, workspaceID)

	handler := KnowledgeIngestHandler(pipeline, indexer, workspaceID)

	// Test FileCreated / FileModified operation
	req := Request{
		ProjectID:    note.ProjectID,
		RootID:       vaultStore.Root(),
		RelativePath: note.RelativePath,
		Operation:    workspace.FileModified,
		ContentHash:  note.ContentHash,
		ObservedAt:   time.Now().UTC(),
	}

	if err := handler(ctx, req); err != nil {
		t.Fatalf("handler failed on FileModified: %v", err)
	}

	// Verify indexed
	results, err := retrievalIndex.Query(ctx, note.ProjectID, "architecture", 10)
	if err != nil || len(results) == 0 {
		t.Fatalf("retrieval query failed after handler: len=%d, err=%v", len(results), err)
	}

	// Test FileRescanRequired triggers full rebuild
	rescanReq := Request{
		ProjectID: note.ProjectID,
		Operation: workspace.FileRescanRequired,
	}
	if err := handler(ctx, rescanReq); err != nil {
		t.Fatalf("handler failed on FileRescanRequired: %v", err)
	}

	// Verify still indexed after rebuild
	results2, err := retrievalIndex.Query(ctx, note.ProjectID, "architecture", 10)
	if err != nil || len(results2) == 0 {
		t.Fatalf("retrieval query failed after rescan rebuild: len=%d, err=%v", len(results2), err)
	}
}

func TestKnowledgeIngestHandlerHandlerDirectCall(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	root := t.TempDir()
	vaultRoot := filepath.Join(root, "vault")
	vaultStore, err := vault.New(vaultRoot)
	if err != nil {
		t.Fatal(err)
	}

	created := time.Now().UTC()
	note, err := vaultStore.CreateNote(ctx, workspace.VaultNote{
		ID:            "note-handler-direct",
		ProjectID:     "project-handler",
		RelativePath:  "notes/direct.md",
		Title:         "Direct Handler",
		Body:          "# Direct\n\nInitial body.\n",
		FormatVersion: vault.FormatVersion,
		Source:        "vault",
		Author:        "tester",
		CreatedAt:     created,
		UpdatedAt:     created,
	})
	if err != nil {
		t.Fatal(err)
	}

	knowledgeStore := knowledge.NewMemoryStore()
	knowledgeSvc, err := knowledge.NewService(knowledgeStore)
	if err != nil {
		t.Fatal(err)
	}

	indexRoot := filepath.Join(root, "index")
	retrievalIndex, err := retrieval.New(indexRoot)
	if err != nil {
		t.Fatal(err)
	}

	pipeline, err := knowledge.NewPipeline(vaultStore, knowledgeSvc, knowledge.DefaultIngestionConfig())
	if err != nil {
		t.Fatal(err)
	}

	workspaceID := knowledge.WorkspaceID("ws-handler")
	indexer := knowledge.NewIndexer(retrievalIndex, workspaceID)

	handler := KnowledgeIngestHandler(pipeline, indexer, workspaceID)

	// Direct handler call (bypassing coordinator debounce)
	req := Request{
		ProjectID:    note.ProjectID,
		RootID:       vaultStore.Root(),
		RelativePath: note.RelativePath,
		Operation:    workspace.FileModified,
		ContentHash:  note.ContentHash,
		ObservedAt:   time.Now().UTC(),
	}

	if err := handler(ctx, req); err != nil {
		t.Fatalf("handler failed: %v", err)
	}

	// Verify retrieval index contains the content
	results, err := retrievalIndex.Query(ctx, note.ProjectID, "Direct", 10)
	if err != nil || len(results) == 0 {
		t.Fatalf("retrieval query failed: len=%d, err=%v", len(results), err)
	}
}

func TestKnowledgeIngestHandlerValidation(t *testing.T) {
	t.Parallel()

	// Nil pipeline or indexer returns error
	handler := KnowledgeIngestHandler(nil, nil, "ws")
	err := handler(context.Background(), Request{ProjectID: "proj"})
	if workspace.ErrorCodeOf(err) != workspace.ErrInvalidRequest {
		t.Fatalf("expected ErrInvalidRequest, got %v", err)
	}
}
