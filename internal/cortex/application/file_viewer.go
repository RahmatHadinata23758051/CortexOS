package application

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/RahmatHadinata23758051/CortexOS/internal/cortex/workspace"
)

// FilesBridgeSchemaVersion is the stable contract for the project-scoped Files surface.
const FilesBridgeSchemaVersion = "cortexos.files.bridge.v1"

const (
	FilesAreaRepository = "repository"
	FilesAreaSupport    = "support"
	FilesAreaVault      = "vault"
	// ponytail: file listing capped at 2000 entries and reads capped at 2MB text. Upgrade path: add page cursor and range streaming when repositories exceed frontend memory budget.
	maxFileEntries = 2000
	maxFileBytes   = 2 << 20
)

var (
	ErrFilesInvalidRequest = errors.New("invalid files bridge request")
	ErrFilesUnavailable    = errors.New("files bridge unavailable")
)

type FilesArea string

type FilesListRequest struct {
	SchemaVersion string    `json:"schemaVersion"`
	ProjectID     string    `json:"projectId"`
	WorktreeID    string    `json:"worktreeId,omitempty"`
	Area          FilesArea `json:"area"`
	Path          string    `json:"path,omitempty"`
}

type FilesReadRequest struct {
	SchemaVersion string    `json:"schemaVersion"`
	ProjectID     string    `json:"projectId"`
	WorktreeID    string    `json:"worktreeId,omitempty"`
	Area          FilesArea `json:"area"`
	Path          string    `json:"path"`
}

type FilesEntry struct {
	Path        string    `json:"path"`
	Name        string    `json:"name"`
	Kind        string    `json:"kind"` // file | directory
	Size        int64     `json:"size,omitempty"`
	ModifiedAt  time.Time `json:"modifiedAt,omitempty"`
	Readable    bool      `json:"readable"`
	Source      string    `json:"source,omitempty"`
	ContentHash string    `json:"contentHash,omitempty"`
}

type FilesListResult struct {
	ProjectID     string       `json:"projectId"`
	WorktreeID    string       `json:"worktreeId,omitempty"`
	Area          FilesArea    `json:"area"`
	RootLabel     string       `json:"rootLabel"`
	Entries       []FilesEntry `json:"entries"`
	HasMore       bool         `json:"hasMore"`
	SchemaVersion string       `json:"schemaVersion"`
}

type FilesReadResult struct {
	ProjectID     string    `json:"projectId"`
	WorktreeID    string    `json:"worktreeId,omitempty"`
	Area          FilesArea `json:"area"`
	Path          string    `json:"path"`
	Content       string    `json:"content"`
	ContentHash   string    `json:"contentHash"`
	ModifiedAt    time.Time `json:"modifiedAt"`
	SchemaVersion string    `json:"schemaVersion"`
}

// FilesPort is the read-only application boundary used by the Files viewer.
// Mutations remain governed by Orchestra/Harness and are intentionally absent.
type FilesPort interface {
	ListFiles(context.Context, FilesListRequest) (FilesListResult, error)
	ReadFile(context.Context, FilesReadRequest) (FilesReadResult, error)
}

func (r FilesListRequest) validate() error {
	if r.SchemaVersion != FilesBridgeSchemaVersion {
		return fmt.Errorf("%w: unsupported schema version", ErrFilesInvalidRequest)
	}
	if strings.TrimSpace(r.ProjectID) == "" {
		return fmt.Errorf("%w: projectId is required", ErrFilesInvalidRequest)
	}
	if !validFilesArea(r.Area) {
		return fmt.Errorf("%w: unsupported area", ErrFilesInvalidRequest)
	}
	return validateFilesPath(r.Path)
}

func (r FilesReadRequest) validate() error {
	if r.SchemaVersion != FilesBridgeSchemaVersion {
		return fmt.Errorf("%w: unsupported schema version", ErrFilesInvalidRequest)
	}
	if strings.TrimSpace(r.ProjectID) == "" || strings.TrimSpace(r.Path) == "" {
		return fmt.Errorf("%w: projectId and path are required", ErrFilesInvalidRequest)
	}
	if !validFilesArea(r.Area) {
		return fmt.Errorf("%w: unsupported area", ErrFilesInvalidRequest)
	}
	return validateFilesPath(r.Path)
}

func validFilesArea(area FilesArea) bool {
	return area == FilesAreaRepository || area == FilesAreaSupport || area == FilesAreaVault
}

func validateFilesPath(path string) error {
	if path == "" {
		return nil
	}
	if len(path) > 512 {
		return fmt.Errorf("%w: path is too long", ErrFilesInvalidRequest)
	}
	if _, err := workspace.CanonicalRelative(path); err != nil {
		return fmt.Errorf("%w: path is invalid", ErrFilesInvalidRequest)
	}
	return nil
}

// FilesAdapter implements FilesPort on top of workspace.Service and local storage.
type FilesAdapter struct {
	workspace *workspace.Service
}

func NewFilesAdapter(ws *workspace.Service) *FilesAdapter {
	return &FilesAdapter{workspace: ws}
}

func (a *FilesAdapter) ListFiles(ctx context.Context, request FilesListRequest) (FilesListResult, error) {
	if err := request.validate(); err != nil {
		return FilesListResult{}, err
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return FilesListResult{}, err
	}
	if a == nil || a.workspace == nil {
		return FilesListResult{}, ErrFilesUnavailable
	}
	project, err := a.workspace.GetProject(ctx, workspace.ProjectID(request.ProjectID))
	if err != nil {
		return FilesListResult{}, mapWorkspaceError(err)
	}

	if request.Area == FilesAreaVault {
		return a.listVaultFiles(ctx, project, request)
	}
	root, worktreeID, err := a.fileRoot(ctx, project, request.WorktreeID, request.Area)
	if err != nil {
		return FilesListResult{}, err
	}
	return listDiskFiles(root, request.Path, request.Area, request.ProjectID, worktreeID)
}

func (a *FilesAdapter) ReadFile(ctx context.Context, request FilesReadRequest) (FilesReadResult, error) {
	if err := request.validate(); err != nil {
		return FilesReadResult{}, err
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return FilesReadResult{}, err
	}
	if a == nil || a.workspace == nil {
		return FilesReadResult{}, ErrFilesUnavailable
	}
	project, err := a.workspace.GetProject(ctx, workspace.ProjectID(request.ProjectID))
	if err != nil {
		return FilesReadResult{}, mapWorkspaceError(err)
	}
	if request.Area == FilesAreaVault {
		return a.readVaultFile(ctx, project, request)
	}
	root, worktreeID, err := a.fileRoot(ctx, project, request.WorktreeID, request.Area)
	if err != nil {
		return FilesReadResult{}, err
	}
	return readDiskFile(root, request.Path, request.Area, request.ProjectID, worktreeID)
}

func (a *FilesAdapter) fileRoot(ctx context.Context, project workspace.Project, worktreeID string, area FilesArea) (string, string, error) {
	root := project.RepositoryRoot
	resolvedWorktree := ""
	if worktreeID != "" {
		worktree, err := a.workspace.InspectWorktree(ctx, workspace.WorktreeID(worktreeID))
		if err != nil {
			return "", "", mapWorkspaceError(err)
		}
		if worktree.ProjectID != project.ID {
			return "", "", fmt.Errorf("%w: worktree does not belong to project", ErrFilesInvalidRequest)
		}
		root = worktree.Path
		resolvedWorktree = string(worktree.ID)
	}
	if area == FilesAreaSupport {
		// Support is deliberately a read-only convention inside the project boundary.
		supportDir := filepath.Join(project.RepositoryRoot, "support")
		if _, err := os.Stat(supportDir); err != nil {
			supportDir = filepath.Join(project.RepositoryRoot, ".cortexos", "support")
		}
		root = supportDir
		resolvedWorktree = ""
	}
	canonical, err := workspace.CanonicalRoot(root)
	if err != nil {
		return "", "", fmt.Errorf("%w: file root is unavailable", ErrFilesUnavailable)
	}
	return canonical, resolvedWorktree, nil
}

func listDiskFiles(root, relative string, area FilesArea, projectID, worktreeID string) (FilesListResult, error) {
	start := root
	var err error
	if relative != "" {
		start, err = workspace.ContainedPath(root, relative)
		if err != nil {
			return FilesListResult{}, fmt.Errorf("%w: path is outside file root", ErrFilesInvalidRequest)
		}
	}
	entries := make([]FilesEntry, 0, 64)
	walkErr := filepath.WalkDir(start, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			if errors.Is(walkErr, os.ErrNotExist) {
				return fmt.Errorf("%w: file root does not exist", ErrFilesUnavailable)
			}
			return walkErr
		}
		if len(entries) >= maxFileEntries {
			return fs.SkipAll
		}
		if path == start {
			return nil
		}
		if entry.IsDir() && entry.Name() == ".git" {
			return fs.SkipDir
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		entries = append(entries, FilesEntry{
			Path: filepath.ToSlash(rel), Name: entry.Name(), Kind: kindOf(entry),
			Size: info.Size(), ModifiedAt: info.ModTime().UTC(), Readable: entry.Type()&os.ModeSymlink == 0,
		})
		return nil
	})
	if walkErr != nil {
		return FilesListResult{}, walkErr
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Path < entries[j].Path })
	return FilesListResult{ProjectID: projectID, WorktreeID: worktreeID, Area: area, RootLabel: string(area), Entries: entries, HasMore: len(entries) >= maxFileEntries, SchemaVersion: FilesBridgeSchemaVersion}, nil
}

func readDiskFile(root, relative string, area FilesArea, projectID, worktreeID string) (FilesReadResult, error) {
	path, err := workspace.ContainedPath(root, relative)
	if err != nil {
		return FilesReadResult{}, fmt.Errorf("%w: path is outside file root", ErrFilesInvalidRequest)
	}
	if err := verifyLinkContainment(root, path); err != nil {
		return FilesReadResult{}, err
	}
	info, err := os.Lstat(path)
	if err != nil {
		return FilesReadResult{}, fmt.Errorf("%w: file does not exist", ErrFilesUnavailable)
	}
	if info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return FilesReadResult{}, fmt.Errorf("%w: only regular files can be read", ErrFilesInvalidRequest)
	}
	if info.Size() > maxFileBytes {
		return FilesReadResult{}, fmt.Errorf("%w: file exceeds read limit", ErrFilesInvalidRequest)
	}
	content, err := os.ReadFile(path)
	if err != nil {
		return FilesReadResult{}, fmt.Errorf("%w: file could not be read", ErrFilesUnavailable)
	}
	return FilesReadResult{ProjectID: projectID, WorktreeID: worktreeID, Area: area, Path: filepath.ToSlash(relative), Content: string(content), ContentHash: fmt.Sprintf("%x", sha256Sum(content)), ModifiedAt: info.ModTime().UTC(), SchemaVersion: FilesBridgeSchemaVersion}, nil
}

func (a *FilesAdapter) listVaultFiles(ctx context.Context, project workspace.Project, request FilesListRequest) (FilesListResult, error) {
	notes, err := a.workspace.ListNotes(ctx, project.ID)
	if err != nil {
		return FilesListResult{}, mapWorkspaceError(err)
	}
	entries := make([]FilesEntry, 0, len(notes))
	for _, note := range notes {
		if request.Path != "" && !filesPathWithin(note.RelativePath, request.Path) {
			continue
		}
		entries = append(entries, FilesEntry{Path: filepath.ToSlash(note.RelativePath), Name: note.Title, Kind: "file", ModifiedAt: note.UpdatedAt, Readable: true, Source: note.Source, ContentHash: note.ContentHash})
	}
	return FilesListResult{ProjectID: request.ProjectID, Area: FilesAreaVault, RootLabel: "vault", Entries: entries, SchemaVersion: FilesBridgeSchemaVersion}, nil
}

func (a *FilesAdapter) readVaultFile(ctx context.Context, project workspace.Project, request FilesReadRequest) (FilesReadResult, error) {
	notes, err := a.workspace.ListNotes(ctx, project.ID)
	if err != nil {
		return FilesReadResult{}, mapWorkspaceError(err)
	}
	for _, note := range notes {
		if filepath.ToSlash(note.RelativePath) == filepath.ToSlash(request.Path) {
			return FilesReadResult{ProjectID: request.ProjectID, Area: FilesAreaVault, Path: filepath.ToSlash(note.RelativePath), Content: note.Body, ContentHash: note.ContentHash, ModifiedAt: note.UpdatedAt, SchemaVersion: FilesBridgeSchemaVersion}, nil
		}
	}
	return FilesReadResult{}, fmt.Errorf("%w: Vault file does not exist", ErrFilesUnavailable)
}

func verifyLinkContainment(root, target string) error {
	resolvedRoot, err := filepath.EvalSymlinks(root)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("%w: file root cannot be resolved", ErrFilesUnavailable)
	}
	if err != nil {
		resolvedRoot = filepath.Clean(root)
	}
	ancestor := target
	for {
		resolved, resolveErr := filepath.EvalSymlinks(ancestor)
		if resolveErr == nil {
			relative, relativeErr := filepath.Rel(resolvedRoot, resolved)
			if relativeErr != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) || filepath.IsAbs(relative) {
				return fmt.Errorf("%w: file path resolves outside approved root", ErrFilesInvalidRequest)
			}
			return nil
		}
		if !errors.Is(resolveErr, os.ErrNotExist) {
			return fmt.Errorf("%w: file link cannot be resolved", ErrFilesInvalidRequest)
		}
		parent := filepath.Dir(ancestor)
		if parent == ancestor {
			return nil
		}
		ancestor = parent
	}
}

func filesPathWithin(candidate, prefix string) bool {
	candidate = strings.Trim(filepath.ToSlash(candidate), "/")
	prefix = strings.Trim(filepath.ToSlash(prefix), "/")
	return candidate == prefix || strings.HasPrefix(candidate, prefix+"/")
}

func kindOf(entry fs.DirEntry) string {
	if entry.IsDir() {
		return "directory"
	}
	return "file"
}

func sha256Sum(content []byte) [32]byte {
	// kept in a helper so the read boundary has one deterministic hashing path
	return sha256.Sum256(content)
}
