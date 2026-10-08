package knowledge

import (
	"context"
	"testing"
	"time"

	"github.com/RahmatHadinata23758051/CortexOS/internal/cortex/workspace"
)

func TestDiscoveryFiltersOutNonMarkdownAndInactive(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	mem := workspace.NewMemoryWorkspace()
	if err := mem.Open(ctx); err != nil {
		t.Fatal(err)
	}
	projectID := workspace.ProjectID("proj-1")
	if _, err := mem.RegisterProject(ctx, workspace.Project{ID: projectID, Name: "Test"}); err != nil {
		t.Fatal(err)
	}

	created := time.Now().UTC()
	// Active valid markdown note
	if _, err := mem.CreateNote(ctx, workspace.VaultNote{
		ID:            "note-valid",
		ProjectID:     projectID,
		RelativePath:  "docs/architecture.md",
		Title:         "Architecture",
		Body:          "# Architecture\n\nContent here.",
		FormatVersion: "cortexos.vault.v1",
		Source:        "vault",
		Author:        "alice",
		CreatedAt:     created,
		UpdatedAt:     created,
		ContentHash:   "hash-1",
		Status:        workspace.NoteStatusActive,
	}); err != nil {
		t.Fatal(err)
	}

	// Another valid note
	if _, err := mem.CreateNote(ctx, workspace.VaultNote{
		ID:            "note-specs",
		ProjectID:     projectID,
		RelativePath:  "specs/api.md",
		Title:         "API Specs",
		Body:          "# API\n\nEndpoints here.",
		FormatVersion: "cortexos.vault.v1",
		Source:        "vault",
		Author:        "bob",
		CreatedAt:     created,
		UpdatedAt:     created,
		ContentHash:   "hash-2",
		Status:        workspace.NoteStatusActive,
	}); err != nil {
		t.Fatal(err)
	}

	discovery := NewDiscovery(mem)
	refs, err := discovery.Discover(ctx, projectID)
	if err != nil {
		t.Fatalf("Discover failed: %v", err)
	}

	if len(refs) != 2 {
		t.Fatalf("expected 2 discovered notes, got %d", len(refs))
	}
	if refs[0].RelativePath != "docs/architecture.md" || refs[1].RelativePath != "specs/api.md" {
		t.Fatalf("unexpected order or paths: %+v", refs)
	}
}

func TestDiscoveryCancellationAndInvalidInput(t *testing.T) {
	t.Parallel()
	discovery := NewDiscovery(nil)
	if _, err := discovery.Discover(context.Background(), "proj"); err == nil {
		t.Fatal("expected error with nil vault")
	}

	mem := workspace.NewMemoryWorkspace()
	discovery = NewDiscovery(mem)
	if _, err := discovery.Discover(context.Background(), ""); err == nil {
		t.Fatal("expected error with empty project ID")
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := discovery.Discover(ctx, "proj"); err == nil {
		t.Fatal("expected cancellation error")
	}
}

func TestDiscoverySingleNote(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	mem := workspace.NewMemoryWorkspace()
	if err := mem.Open(ctx); err != nil {
		t.Fatal(err)
	}
	projectID := workspace.ProjectID("proj-1")
	if _, err := mem.RegisterProject(ctx, workspace.Project{ID: projectID, Name: "Test"}); err != nil {
		t.Fatal(err)
	}

	created := time.Now().UTC()
	note, err := mem.CreateNote(ctx, workspace.VaultNote{
		ID:            "note-single",
		ProjectID:     projectID,
		RelativePath:  "guides/setup.md",
		Title:         "Setup Guide",
		Body:          "# Setup\n\nSteps here.",
		FormatVersion: "cortexos.vault.v1",
		Source:        "vault",
		Author:        "carol",
		CreatedAt:     created,
		UpdatedAt:     created,
		ContentHash:   "hash-setup",
		Status:        workspace.NoteStatusActive,
	})
	if err != nil {
		t.Fatal(err)
	}

	discovery := NewDiscovery(mem)
	ref, err := discovery.DiscoverNote(ctx, note.ID)
	if err != nil {
		t.Fatalf("DiscoverNote failed: %v", err)
	}
	if ref.NoteID != note.ID || ref.RelativePath != "guides/setup.md" || ref.ContentHash != "hash-setup" {
		t.Fatalf("unexpected ref: %+v", ref)
	}
}
