package workspace

import (
	"context"
	"testing"
)

func TestMemoryWorkspaceProjectRegistry(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	memory := NewMemoryWorkspace()
	if err := memory.Open(ctx); err != nil {
		t.Fatal(err)
	}

	first, err := memory.RegisterProject(ctx, Project{ID: "project-b", Name: "B"})
	if err != nil {
		t.Fatal(err)
	}
	if first.SchemaVersion != ContractVersion || first.Status != ProjectStatusActive {
		t.Fatalf("project defaults = %#v", first)
	}
	if _, err := memory.RegisterProject(ctx, first); ErrorCodeOf(err) != ErrConflict {
		t.Fatalf("duplicate project error = %q, want %q", ErrorCodeOf(err), ErrConflict)
	}
	if _, err := memory.GetProject(ctx, "missing"); ErrorCodeOf(err) != ErrNotFound {
		t.Fatalf("missing project error = %q, want %q", ErrorCodeOf(err), ErrNotFound)
	}

	if _, err := memory.RegisterProject(ctx, Project{ID: "project-a", Name: "A"}); err != nil {
		t.Fatal(err)
	}
	projects, err := memory.ListProjects(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(projects) != 2 || projects[0].ID != "project-a" || projects[1].ID != "project-b" {
		t.Fatalf("projects are not deterministic: %#v", projects)
	}
	if err := memory.ArchiveProject(ctx, "project-a"); err != nil {
		t.Fatal(err)
	}
	archived, err := memory.GetProject(ctx, "project-a")
	if err != nil {
		t.Fatal(err)
	}
	if archived.Status != ProjectStatusArchived {
		t.Fatalf("status = %q, want %q", archived.Status, ProjectStatusArchived)
	}
}

func TestMemoryWorkspaceRejectsCanceledContext(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	memory := NewMemoryWorkspace()
	if _, err := memory.ListProjects(ctx); ErrorCodeOf(err) != ErrCanceled {
		t.Fatalf("error code = %q, want %q", ErrorCodeOf(err), ErrCanceled)
	}
}
