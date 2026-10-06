package sqlite

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/RahmatHadinata23758051/CortexOS/internal/cortex/workspace"
)

func TestProjectRegistryNormalizesAndReopens(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	databasePath := filepath.Join(root, "state", "workspace.db")
	if err := ensureParent(databasePath); err != nil {
		t.Fatal(err)
	}
	store, err := Open(context.Background(), databasePath)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	project := workspace.Project{
		ID:             "project-1",
		Name:           "Example",
		RepositoryRoot: filepath.Join(root, "repo", "."),
		VaultRoot:      filepath.Join(root, "vault", "notes", ".."),
	}
	created, err := store.RegisterProject(ctx, project)
	if err != nil {
		t.Fatal(err)
	}
	if created.RepositoryRoot != filepath.Join(root, "repo") {
		t.Fatalf("repository root = %q", created.RepositoryRoot)
	}
	if created.VaultRoot != filepath.Join(root, "vault") {
		t.Fatalf("vault root = %q", created.VaultRoot)
	}
	if created.SchemaVersion != workspace.ContractVersion || created.Status != workspace.ProjectStatusActive {
		t.Fatalf("defaults = %#v", created)
	}
	if _, err := store.RegisterProject(ctx, project); workspace.ErrorCodeOf(err) != workspace.ErrConflict {
		t.Fatalf("duplicate code = %q, want %q", workspace.ErrorCodeOf(err), workspace.ErrConflict)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}

	reopened, err := Open(ctx, databasePath)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	loaded, err := reopened.GetProject(ctx, "project-1")
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Name != project.Name || loaded.RepositoryRoot != created.RepositoryRoot {
		t.Fatalf("loaded project = %#v", loaded)
	}
	if err := reopened.ArchiveProject(ctx, loaded.ID); err != nil {
		t.Fatal(err)
	}
	archived, err := reopened.GetProject(ctx, loaded.ID)
	if err != nil {
		t.Fatal(err)
	}
	if archived.Status != workspace.ProjectStatusArchived {
		t.Fatalf("status = %q, want %q", archived.Status, workspace.ProjectStatusArchived)
	}
}

func TestProjectRegistryRejectsUnsafeRootsAndMissingProjects(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	store, err := Open(context.Background(), filepath.Join(root, "workspace.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	_, err = store.RegisterProject(context.Background(), workspace.Project{
		ID:             "unsafe",
		Name:           "Unsafe",
		RepositoryRoot: "relative/repo",
		VaultRoot:      filepath.Join(root, "vault"),
	})
	if workspace.ErrorCodeOf(err) != workspace.ErrPathDenied {
		t.Fatalf("unsafe root code = %q, want %q", workspace.ErrorCodeOf(err), workspace.ErrPathDenied)
	}
	if _, err := store.GetProject(context.Background(), "missing"); workspace.ErrorCodeOf(err) != workspace.ErrNotFound {
		t.Fatalf("missing code = %q, want %q", workspace.ErrorCodeOf(err), workspace.ErrNotFound)
	}
}

func ensureParent(path string) error {
	return os.MkdirAll(filepath.Dir(path), 0o755)
}
