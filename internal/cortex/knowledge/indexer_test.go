package knowledge

import (
	"context"
	"testing"
	"time"

	"github.com/RahmatHadinata23758051/CortexOS/internal/cortex/workspace"
	"github.com/RahmatHadinata23758051/CortexOS/internal/cortex/workspace/retrieval"
)

func TestIndexerRoundTrip(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	mem := workspace.NewMemoryWorkspace()
	if err := mem.Open(ctx); err != nil {
		t.Fatal(err)
	}
	projectID := workspace.ProjectID("proj-index")
	if _, err := mem.RegisterProject(ctx, workspace.Project{ID: projectID, Name: "Test"}); err != nil {
		t.Fatal(err)
	}

	created := time.Now().UTC()
	_, err := mem.CreateNote(ctx, workspace.VaultNote{
		ID:            "note-index",
		ProjectID:     projectID,
		RelativePath:  "docs/indexing.md",
		Title:         "Indexing Test",
		Body:          "# Indexing\n\nThis is a test for indexing.",
		FormatVersion: "cortexos.vault.v1",
		Source:        "vault",
		Author:        "dave",
		CreatedAt:     created,
		UpdatedAt:     created,
		ContentHash:   "hash-index",
		Status:        workspace.NoteStatusActive,
	})
	if err != nil {
		t.Fatal(err)
	}

	// Setup knowledge service
	store := NewMemoryStore()
	svc, err := NewService(store)
	if err != nil {
		t.Fatal(err)
	}

	// Setup retrieval index
	indexRoot := t.TempDir()
	index, err := retrieval.New(indexRoot)
	if err != nil {
		t.Fatal(err)
	}

	// Create pipeline and indexer
	pipeline, err := NewPipeline(mem, svc, DefaultIngestionConfig())
	if err != nil {
		t.Fatal(err)
	}
	indexer := NewIndexer(index, "ws-test")

	workspaceID := WorkspaceID("ws-test")

	// Ingest the note
	res, err := pipeline.IngestProject(ctx, projectID, workspaceID)
	if err != nil {
		t.Fatalf("IngestProject failed: %v", err)
	}
	if res.Ingested == 0 {
		t.Fatal("expected at least 1 item ingested")
	}

	items, err := svc.List(ctx, Filter{ProjectID: projectIDPtr(string(projectID)), ActiveOnly: true})
	if err != nil {
		t.Fatal(err)
	}
	if err := indexer.IndexItems(ctx, items); err != nil {
		t.Fatalf("IndexItems failed: %v", err)
	}

	// Query the retrieval index
	results, err := index.Query(ctx, projectID, "indexing", 10)
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("expected results from retrieval index")
	}
	for _, r := range results {
		if r.Content == "" {
			t.Errorf("empty content in result: %+v", r)
		}
	}
}

func TestIndexerRemoveItem(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	mem := workspace.NewMemoryWorkspace()
	if err := mem.Open(ctx); err != nil {
		t.Fatal(err)
	}
	projectID := workspace.ProjectID("proj-remove")
	if _, err := mem.RegisterProject(ctx, workspace.Project{ID: projectID, Name: "Test"}); err != nil {
		t.Fatal(err)
	}

	created := time.Now().UTC()
	_, err := mem.CreateNote(ctx, workspace.VaultNote{
		ID:            "note-remove",
		ProjectID:     projectID,
		RelativePath:  "docs/remove.md",
		Title:         "Remove Test",
		Body:          "# Remove\n\nContent to remove.",
		FormatVersion: "cortexos.vault.v1",
		Source:        "vault",
		Author:        "eve",
		CreatedAt:     created,
		UpdatedAt:     created,
		ContentHash:   "hash-remove",
		Status:        workspace.NoteStatusActive,
	})
	if err != nil {
		t.Fatal(err)
	}

	store := NewMemoryStore()
	svc, err := NewService(store)
	if err != nil {
		t.Fatal(err)
	}

	indexRoot := t.TempDir()
	index, err := retrieval.New(indexRoot)
	if err != nil {
		t.Fatal(err)
	}

	pipeline, err := NewPipeline(mem, svc, DefaultIngestionConfig())
	if err != nil {
		t.Fatal(err)
	}
	indexer := NewIndexer(index, "ws-test")

	workspaceID := WorkspaceID("ws-test")

	// Ingest
	_, err = pipeline.IngestProject(ctx, projectID, workspaceID)
	if err != nil {
		t.Fatalf("IngestProject failed: %v", err)
	}

	items, err := svc.List(ctx, Filter{ProjectID: projectIDPtr(string(projectID)), ActiveOnly: true})
	if err != nil {
		t.Fatal(err)
	}
	if err := indexer.IndexItems(ctx, items); err != nil {
		t.Fatalf("IndexItems failed: %v", err)
	}

	// Verify indexed
	results, err := index.Query(ctx, projectID, "remove", 10)
	if err != nil || len(results) == 0 {
		t.Fatal("expected indexed results")
	}

	// Get the knowledge items and remove them
	items, err = svc.List(ctx, Filter{ProjectID: projectIDPtr(string(projectID)), ActiveOnly: true})
	if err != nil {
		t.Fatal(err)
	}

	for _, item := range items {
		if err := indexer.RemoveItem(ctx, item); err != nil {
			t.Fatalf("RemoveItem failed: %v", err)
		}
	}

	// Verify removed
	results, err = index.Query(ctx, projectID, "remove", 10)
	if err != nil {
		t.Fatalf("Query after remove failed: %v", err)
	}
	if len(results) != 0 {
		t.Fatalf("expected no results after remove, got %d", len(results))
	}
}

func TestIndexerRebuildProject(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	mem := workspace.NewMemoryWorkspace()
	if err := mem.Open(ctx); err != nil {
		t.Fatal(err)
	}
	projectID := workspace.ProjectID("proj-rebuild")
	if _, err := mem.RegisterProject(ctx, workspace.Project{ID: projectID, Name: "Test"}); err != nil {
		t.Fatal(err)
	}

	created := time.Now().UTC()
	if _, err := mem.CreateNote(ctx, workspace.VaultNote{
		ID:            "note-rebuild",
		ProjectID:     projectID,
		RelativePath:  "docs/rebuild.md",
		Title:         "Rebuild Test",
		Body:          "# Rebuild\n\nContent for rebuild test.",
		FormatVersion: "cortexos.vault.v1",
		Source:        "vault",
		Author:        "frank",
		CreatedAt:     created,
		UpdatedAt:     created,
		ContentHash:   "hash-rebuild",
		Status:        workspace.NoteStatusActive,
	}); err != nil {
		t.Fatal(err)
	}

	store := NewMemoryStore()
	svc, err := NewService(store)
	if err != nil {
		t.Fatal(err)
	}

	indexRoot := t.TempDir()
	index, err := retrieval.New(indexRoot)
	if err != nil {
		t.Fatal(err)
	}

	pipeline, err := NewPipeline(mem, svc, DefaultIngestionConfig())
	if err != nil {
		t.Fatal(err)
	}
	indexer := NewIndexer(index, "ws-test")

	workspaceID := WorkspaceID("ws-test")

	// Ingest
	_, err = pipeline.IngestProject(ctx, projectID, workspaceID)
	if err != nil {
		t.Fatalf("IngestProject failed: %v", err)
	}

	// Rebuild
	items, err := svc.List(ctx, Filter{ProjectID: projectIDPtr(string(projectID)), ActiveOnly: true})
	if err != nil {
		t.Fatal(err)
	}
	if err := indexer.RebuildProject(ctx, ProjectID(projectID), items); err != nil {
		t.Fatalf("RebuildProject failed: %v", err)
	}

	// Verify still indexed
	results, err := index.Query(ctx, projectID, "rebuild", 10)
	if err != nil || len(results) == 0 {
		t.Fatal("expected results after rebuild")
	}
}
