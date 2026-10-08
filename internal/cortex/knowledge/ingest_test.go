package knowledge

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/RahmatHadinata23758051/CortexOS/internal/cortex/workspace"
)

func TestIngestPipelineEndToEndWithProvenanceAndRedaction(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	mem := workspace.NewMemoryWorkspace()
	if err := mem.Open(ctx); err != nil {
		t.Fatal(err)
	}
	projectID := workspace.ProjectID("proj-e2e")
	if _, err := mem.RegisterProject(ctx, workspace.Project{ID: projectID, Name: "Test"}); err != nil {
		t.Fatal(err)
	}

	created := time.Now().UTC()
	// Note containing secrets that should be redacted
	body := `# System Architecture

## Authentication
Use OAuth2.1 and JWT tokens.
Contact admin@example.com for credentials.
API key: api_key=supersecretkey123456789
Bearer token: Bearer mybearertokenvalue123

## Database
Connection string: postgres://admin:secretpass@db.local:5432/cortex
Store snapshots in SQLite.`

	note, err := mem.CreateNote(ctx, workspace.VaultNote{
		ID:            "note-arch",
		ProjectID:     projectID,
		RelativePath:  "docs/architecture.md",
		Title:         "System Architecture",
		Body:          body,
		FormatVersion: "cortexos.vault.v1",
		Source:        "vault-editor",
		Author:        "security-lead",
		CreatedAt:     created,
		UpdatedAt:     created,
		ContentHash:   "hash-arch-1",
		Status:        workspace.NoteStatusActive,
	})
	if err != nil {
		t.Fatal(err)
	}

	store := NewMemoryStore()
	knowledgeSvc, err := NewService(store)
	if err != nil {
		t.Fatal(err)
	}

	pipeline, err := NewPipeline(mem, knowledgeSvc, DefaultIngestionConfig())
	if err != nil {
		t.Fatal(err)
	}

	workspaceID := WorkspaceID("ws-e2e")
	res, err := pipeline.IngestProject(ctx, projectID, workspaceID)
	if err != nil {
		t.Fatalf("IngestProject failed: %v", err)
	}

	if res.Discovered != 1 {
		t.Fatalf("expected 1 discovered note, got %d", res.Discovered)
	}
	if res.Ingested != 1 {
		t.Fatalf("expected 1 ingested note, got %d", res.Ingested)
	}
	if len(res.Errors) != 0 {
		t.Fatalf("unexpected errors: %+v", res.Errors)
	}

	// Verify knowledge items created
	items, err := knowledgeSvc.List(ctx, Filter{
		ProjectID:  projectIDPtr(string(projectID)),
		ActiveOnly: true,
	})
	if err != nil {
		t.Fatal(err)
	}

	if len(items) == 0 {
		t.Fatal("expected knowledge items to be created")
	}

	for _, item := range items {
		// ADR-0005: Verified and Active
		if item.Lifecycle != LifecycleActive {
			t.Errorf("expected active lifecycle, got %s", item.Lifecycle)
		}
		if item.ValidationStatus != StatusVerified {
			t.Errorf("expected verified status, got %s", item.ValidationStatus)
		}

		// Provenance verification
		if item.Provenance.SourceKind != "vault_note" {
			t.Errorf("unexpected source kind: %s", item.Provenance.SourceKind)
		}
		if item.Provenance.SourceURI != note.RelativePath {
			t.Errorf("unexpected source uri: %s", item.Provenance.SourceURI)
		}
		if item.Provenance.SourceHash != note.ContentHash {
			t.Errorf("unexpected source hash: %s", item.Provenance.SourceHash)
		}
		if item.Provenance.Author != note.Author {
			t.Errorf("unexpected author: %s", item.Provenance.Author)
		}
		if item.Provenance.Attribution != "vault-editor/security-lead" {
			t.Errorf("unexpected attribution: %s", item.Provenance.Attribution)
		}

		// Secrets must be redacted
		if strings.Contains(item.Content, "supersecretkey123456789") {
			t.Errorf("api key leaked in chunk content: %s", item.Content)
		}
		if strings.Contains(item.Content, "admin@example.com") {
			t.Errorf("email leaked in chunk content: %s", item.Content)
		}
		if strings.Contains(item.Content, "mybearertokenvalue123") {
			t.Errorf("bearer token leaked in chunk content: %s", item.Content)
		}
		if strings.Contains(item.Content, "secretpass") {
			t.Errorf("db password leaked in chunk content: %s", item.Content)
		}
	}

	// Second ingest without changes should skip (idempotency)
	res2, err := pipeline.IngestProject(ctx, projectID, workspaceID)
	if err != nil {
		t.Fatalf("second IngestProject failed: %v", err)
	}
	if res2.Ingested != 0 {
		t.Errorf("expected 0 ingested on unchanged notes, got %d", res2.Ingested)
	}
	if res2.Skipped != 1 {
		t.Errorf("expected 1 skipped on unchanged notes, got %d", res2.Skipped)
	}
}

func TestIngestPipelineCancellation(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	mem := workspace.NewMemoryWorkspace()
	if err := mem.Open(ctx); err != nil {
		t.Fatal(err)
	}
	projectID := workspace.ProjectID("proj-cancel")
	if _, err := mem.RegisterProject(ctx, workspace.Project{ID: projectID, Name: "Test"}); err != nil {
		t.Fatal(err)
	}

	store := NewMemoryStore()
	knowledgeSvc, err := NewService(store)
	if err != nil {
		t.Fatal(err)
	}

	pipeline, err := NewPipeline(mem, knowledgeSvc, DefaultIngestionConfig())
	if err != nil {
		t.Fatal(err)
	}

	cancCtx, cancel := context.WithCancel(ctx)
	cancel()

	_, err = pipeline.IngestProject(cancCtx, projectID, "ws-test")
	if err == nil {
		t.Fatal("expected cancellation error")
	}
}
