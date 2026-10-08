package knowledge

import (
	"context"
	"testing"
	"time"

	"github.com/RahmatHadinata23758051/CortexOS/internal/cortex/workspace"
	"github.com/RahmatHadinata23758051/CortexOS/internal/cortex/workspace/retrieval"
	"github.com/RahmatHadinata23758051/CortexOS/internal/cortex/workspace/vault"
)

func TestIncrementalIngestUpdatesAndDeletions(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	mem := workspace.NewMemoryWorkspace()
	if err := mem.Open(ctx); err != nil {
		t.Fatal(err)
	}
	projectID := workspace.ProjectID("proj-incremental")
	if _, err := mem.RegisterProject(ctx, workspace.Project{ID: projectID, Name: "Test"}); err != nil {
		t.Fatal(err)
	}

	created := time.Now().UTC()

	// 1. Create two notes
	note1, err := mem.CreateNote(ctx, workspace.VaultNote{
		ID:            "note-1",
		ProjectID:     projectID,
		RelativePath:  "docs/first.md",
		Title:         "First Note",
		Body:          "# First\n\nOriginal first content.",
		FormatVersion: vault.FormatVersion,
		Source:        "manual",
		Author:        "author1",
		CreatedAt:     created,
		UpdatedAt:     created,
		ContentHash:   vault.HashBody("# First\n\nOriginal first content."),
		Status:        workspace.NoteStatusActive,
	})
	if err != nil {
		t.Fatal(err)
	}

	note2, err := mem.CreateNote(ctx, workspace.VaultNote{
		ID:            "note-2",
		ProjectID:     projectID,
		RelativePath:  "docs/second.md",
		Title:         "Second Note",
		Body:          "# Second\n\nOriginal second content.",
		FormatVersion: vault.FormatVersion,
		Source:        "manual",
		Author:        "author2",
		CreatedAt:     created,
		UpdatedAt:     created,
		ContentHash:   vault.HashBody("# Second\n\nOriginal second content."),
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

	workspaceID := WorkspaceID("ws-inc")
	indexer := NewIndexer(retrievalIndex, workspaceID)

	// Initial incremental ingest (discovers and ingests both)
	res1, err := pipeline.IncrementalIngest(ctx, projectID, workspaceID, indexer)
	if err != nil {
		t.Fatalf("initial IncrementalIngest failed: %v", err)
	}
	if res1.Discovered != 2 || res1.Ingested != 2 {
		t.Fatalf("expected 2 discovered and 2 ingested, got discovered=%d ingested=%d", res1.Discovered, res1.Ingested)
	}

	// Verify both in retrieval index
	results1, err := retrievalIndex.Query(ctx, projectID, "content", 10)
	if err != nil || len(results1) != 2 {
		t.Fatalf("expected 2 query results, got %d, err=%v", len(results1), err)
	}

	// 2. Modify note 1
	updatedNote1, err := mem.UpdateNote(ctx, workspace.VaultNote{
		ID:            note1.ID,
		ProjectID:     projectID,
		RelativePath:  note1.RelativePath,
		Title:         note1.Title,
		Body:          "# First\n\nModified unique keyword first content.",
		FormatVersion: vault.FormatVersion,
		Source:        note1.Source,
		Author:        note1.Author,
		CreatedAt:     note1.CreatedAt,
		UpdatedAt:     created.Add(time.Hour),
		ContentHash:   vault.HashBody("# First\n\nModified unique keyword first content."),
		Status:        workspace.NoteStatusActive,
	}, note1.ContentHash)
	if err != nil {
		t.Fatal(err)
	}

	// Run incremental ingest again
	res2, err := pipeline.IncrementalIngest(ctx, projectID, workspaceID, indexer)
	if err != nil {
		t.Fatalf("second IncrementalIngest failed: %v", err)
	}
	// Note 1 was modified, Note 2 was skipped
	if res2.Ingested != 1 {
		t.Errorf("expected 1 ingested (note1), got %d", res2.Ingested)
	}
	if res2.Skipped != 1 {
		t.Errorf("expected 1 skipped (note2), got %d", res2.Skipped)
	}

	// Verify query matches modified content
	results2, err := retrievalIndex.Query(ctx, projectID, "unique keyword", 10)
	if err != nil || len(results2) != 1 {
		t.Fatalf("expected 1 result for unique keyword, got %d, err=%v", len(results2), err)
	}
	if results2[0].SourceHash != updatedNote1.ContentHash {
		t.Errorf("expected source hash %s, got %s", updatedNote1.ContentHash, results2[0].SourceHash)
	}

	// 3. Delete note 2 from Vault
	if err := mem.DeleteNote(ctx, note2.ID); err != nil {
		t.Fatal(err)
	}

	// Run incremental ingest after delete
	res3, err := pipeline.IncrementalIngest(ctx, projectID, workspaceID, indexer)
	if err != nil {
		t.Fatalf("third IncrementalIngest failed: %v", err)
	}
	if res3.Discovered != 1 {
		t.Errorf("expected 1 discovered after note2 deletion, got %d", res3.Discovered)
	}

	// Verify note 2 is no longer in retrieval index
	results3, err := retrievalIndex.Query(ctx, projectID, "second", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(results3) != 0 {
		t.Errorf("expected 0 results for deleted note, got %d", len(results3))
	}
}
