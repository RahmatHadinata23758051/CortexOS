package application

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/RahmatHadinata23758051/CortexOS/internal/cortex/workspace"
)

type mockWorkspaceStores struct {
	projects  map[workspace.ProjectID]workspace.Project
	worktrees map[workspace.WorktreeID]workspace.Worktree
	notes     map[workspace.NoteID]workspace.VaultNote
}

func newMockWorkspaceService(t *testing.T) (*workspace.Service, *mockWorkspaceStores, string) {
	t.Helper()
	tempDir := t.TempDir()

	repoRoot := filepath.Join(tempDir, "repo")
	vaultRoot := filepath.Join(tempDir, "vault")
	if err := os.MkdirAll(repoRoot, 0755); err != nil {
		t.Fatalf("failed to create repo root: %v", err)
	}
	if err := os.MkdirAll(vaultRoot, 0755); err != nil {
		t.Fatalf("failed to create vault root: %v", err)
	}

	// Create dummy repo files
	_ = os.WriteFile(filepath.Join(repoRoot, "main.go"), []byte("package main\n\nfunc main() {}\n"), 0644)
	_ = os.MkdirAll(filepath.Join(repoRoot, "pkg"), 0755)
	_ = os.WriteFile(filepath.Join(repoRoot, "pkg", "lib.go"), []byte("package pkg\n"), 0644)

	// Create dummy support files
	supportDir := filepath.Join(repoRoot, "support")
	_ = os.MkdirAll(supportDir, 0755)
	_ = os.WriteFile(filepath.Join(supportDir, "guide.txt"), []byte("User guide reference\n"), 0644)

	stores := &mockWorkspaceStores{
		projects: map[workspace.ProjectID]workspace.Project{
			"proj-1": {
				ID:             "proj-1",
				Name:           "Project One",
				RepositoryRoot: repoRoot,
				VaultRoot:      vaultRoot,
				Status:         "active",
				SchemaVersion:  workspace.ContractVersion,
			},
		},
		worktrees: make(map[workspace.WorktreeID]workspace.Worktree),
		notes: map[workspace.NoteID]workspace.VaultNote{
			"note-1": {
				ID:           "note-1",
				ProjectID:    "proj-1",
				RelativePath: "decisions/001-arch.md",
				Title:        "001 Architecture Decision",
				Body:         "# Arch Decision\n\nUse Go + React.",
				ContentHash:  "hash1",
			},
		},
	}

	memory := workspace.NewMemoryWorkspace()
	_ = memory.Open(context.Background())
	for _, p := range stores.projects {
		_, _ = memory.RegisterProject(context.Background(), p)
	}
	for _, n := range stores.notes {
		_, _ = memory.CreateNote(context.Background(), n)
	}

	ws, err := workspace.NewService(workspace.Dependencies{
		State:     memory,
		Projects:  memory,
		Worktrees: memory,
		Vault:     memory,
	})
	if err != nil {
		t.Fatalf("failed to create workspace service: %v", err)
	}

	return ws, stores, tempDir
}

func TestFilesAdapter_RepositoryListAndRead(t *testing.T) {
	ws, _, _ := newMockWorkspaceService(t)
	adapter := NewFilesAdapter(ws)
	ctx := context.Background()

	// List repository files
	listRes, err := adapter.ListFiles(ctx, FilesListRequest{
		SchemaVersion: FilesBridgeSchemaVersion,
		ProjectID:     "proj-1",
		Area:          FilesAreaRepository,
	})
	if err != nil {
		t.Fatalf("unexpected list error: %v", err)
	}
	if len(listRes.Entries) == 0 {
		t.Fatalf("expected non-empty entries")
	}

	// Read repository file
	readRes, err := adapter.ReadFile(ctx, FilesReadRequest{
		SchemaVersion: FilesBridgeSchemaVersion,
		ProjectID:     "proj-1",
		Area:          FilesAreaRepository,
		Path:          "main.go",
	})
	if err != nil {
		t.Fatalf("unexpected read error: %v", err)
	}
	if readRes.Content != "package main\n\nfunc main() {}\n" {
		t.Errorf("unexpected content: %q", readRes.Content)
	}
}

func TestFilesAdapter_SupportListAndRead(t *testing.T) {
	ws, _, _ := newMockWorkspaceService(t)
	adapter := NewFilesAdapter(ws)
	ctx := context.Background()

	// List support files
	listRes, err := adapter.ListFiles(ctx, FilesListRequest{
		SchemaVersion: FilesBridgeSchemaVersion,
		ProjectID:     "proj-1",
		Area:          FilesAreaSupport,
	})
	if err != nil {
		t.Fatalf("unexpected list error: %v", err)
	}
	if len(listRes.Entries) != 1 || listRes.Entries[0].Name != "guide.txt" {
		t.Fatalf("expected guide.txt in support entries, got: %+v", listRes.Entries)
	}

	// Read support file
	readRes, err := adapter.ReadFile(ctx, FilesReadRequest{
		SchemaVersion: FilesBridgeSchemaVersion,
		ProjectID:     "proj-1",
		Area:          FilesAreaSupport,
		Path:          "guide.txt",
	})
	if err != nil {
		t.Fatalf("unexpected read error: %v", err)
	}
	if readRes.Content != "User guide reference\n" {
		t.Errorf("unexpected content: %q", readRes.Content)
	}
}

func TestFilesAdapter_VaultListAndRead(t *testing.T) {
	ws, _, _ := newMockWorkspaceService(t)
	adapter := NewFilesAdapter(ws)
	ctx := context.Background()

	listRes, err := adapter.ListFiles(ctx, FilesListRequest{
		SchemaVersion: FilesBridgeSchemaVersion,
		ProjectID:     "proj-1",
		Area:          FilesAreaVault,
	})
	if err != nil {
		t.Fatalf("unexpected list error: %v", err)
	}
	if len(listRes.Entries) != 1 || listRes.Entries[0].Path != "decisions/001-arch.md" {
		t.Fatalf("expected vault note entry, got: %+v", listRes.Entries)
	}

	readRes, err := adapter.ReadFile(ctx, FilesReadRequest{
		SchemaVersion: FilesBridgeSchemaVersion,
		ProjectID:     "proj-1",
		Area:          FilesAreaVault,
		Path:          "decisions/001-arch.md",
	})
	if err != nil {
		t.Fatalf("unexpected read error: %v", err)
	}
	if readRes.Content != "# Arch Decision\n\nUse Go + React." {
		t.Errorf("unexpected vault content: %q", readRes.Content)
	}
}

func TestFilesAdapter_SecurityPathTraversalRejected(t *testing.T) {
	ws, _, _ := newMockWorkspaceService(t)
	adapter := NewFilesAdapter(ws)
	ctx := context.Background()

	// Escaping path must fail validation
	_, err := adapter.ReadFile(ctx, FilesReadRequest{
		SchemaVersion: FilesBridgeSchemaVersion,
		ProjectID:     "proj-1",
		Area:          FilesAreaRepository,
		Path:          "../../etc/passwd",
	})
	if err == nil {
		t.Fatalf("expected error for escaping path traversal")
	}
}

func TestFilesAdapter_SizeLimitRejected(t *testing.T) {
	ws, _, tempDir := newMockWorkspaceService(t)
	repoRoot := filepath.Join(tempDir, "repo")

	// Create file exceeding 2MB
	bigFilePath := filepath.Join(repoRoot, "huge.bin")
	hugeData := make([]byte, 3<<20) // 3MB
	_ = os.WriteFile(bigFilePath, hugeData, 0644)

	adapter := NewFilesAdapter(ws)
	ctx := context.Background()

	_, err := adapter.ReadFile(ctx, FilesReadRequest{
		SchemaVersion: FilesBridgeSchemaVersion,
		ProjectID:     "proj-1",
		Area:          FilesAreaRepository,
		Path:          "huge.bin",
	})
	if err == nil {
		t.Fatalf("expected error for file exceeding size limit")
	}
}
