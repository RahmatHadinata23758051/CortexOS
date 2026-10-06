package application

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/RahmatHadinata23758051/CortexOS/internal/cortex/workspace"
)

const WorkspaceSchemaVersion = workspace.ContractVersion

type WorkspaceSnapshotRequest struct {
	SchemaVersion string `json:"schemaVersion"`
}

type WorkspaceSnapshot struct {
	SchemaVersion    string              `json:"schemaVersion"`
	Projects         []WorkspaceProject  `json:"projects"`
	Worktrees        []WorkspaceWorktree `json:"worktrees"`
	VaultNoteCount   int                 `json:"vaultNoteCount"`
	WatcherState     string              `json:"watcherState"`
	WatcherErrorCode string              `json:"watcherErrorCode,omitempty"`
	RetrievalVersion string              `json:"retrievalVersion"`
	RetrievalState   string              `json:"retrievalState"`
}

type WorkspaceProject struct {
	ID            string    `json:"id"`
	Name          string    `json:"name"`
	DefaultBranch string    `json:"defaultBranch,omitempty"`
	Status        string    `json:"status"`
	CreatedAt     time.Time `json:"createdAt"`
	UpdatedAt     time.Time `json:"updatedAt"`
	SchemaVersion string    `json:"schemaVersion"`
}

type WorkspaceWorktree struct {
	ID              string    `json:"id"`
	ProjectID       string    `json:"projectId"`
	Branch          string    `json:"branch,omitempty"`
	Revision        string    `json:"revision,omitempty"`
	Status          string    `json:"status"`
	Dirty           bool      `json:"dirty"`
	ActiveReference string    `json:"activeReference,omitempty"`
	CreatedAt       time.Time `json:"createdAt"`
	UpdatedAt       time.Time `json:"updatedAt"`
}

type WorkspaceNoteQueryRequest struct {
	SchemaVersion string `json:"schemaVersion"`
	ProjectID     string `json:"projectId"`
	Query         string `json:"query"`
	Limit         int    `json:"limit"`
}

type WorkspaceNoteResult struct {
	ID           string    `json:"id"`
	NoteID       string    `json:"noteId"`
	ProjectID    string    `json:"projectId"`
	RelativePath string    `json:"relativePath"`
	SourceHash   string    `json:"sourceHash"`
	Attribution  string    `json:"attribution"`
	IndexedAt    time.Time `json:"indexedAt"`
	Excerpt      string    `json:"excerpt"`
}

type WorkspaceMutationRequest struct {
	SchemaVersion string `json:"schemaVersion"`
	ProjectID     string `json:"projectId"`
}

type WorkspaceProjectRequest struct {
	SchemaVersion  string `json:"schemaVersion"`
	ID             string `json:"id"`
	Name           string `json:"name"`
	RepositoryRoot string `json:"repositoryRoot"`
	VaultRoot      string `json:"vaultRoot"`
	DefaultBranch  string `json:"defaultBranch,omitempty"`
}

type WorkspaceBridgeError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func (e *WorkspaceBridgeError) Error() string {
	if e == nil {
		return "<nil>"
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

func mapWorkspaceError(err error) error {
	if err == nil {
		return nil
	}
	code := workspace.ErrorCodeOf(err)
	if code == "" {
		code = workspace.ErrInternal
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		code = workspace.ErrCanceled
	}
	return &WorkspaceBridgeError{Code: string(code), Message: safeWorkspaceMessage(code)}
}

func safeWorkspaceMessage(code workspace.ErrorCode) string {
	switch code {
	case workspace.ErrCanceled:
		return "workspace request canceled"
	case workspace.ErrNotFound:
		return "workspace resource was not found"
	case workspace.ErrConflict:
		return "workspace state changed; retry or refresh"
	case workspace.ErrPathDenied:
		return "workspace path was denied"
	case workspace.ErrUnsupportedVersion:
		return "workspace contract version is unsupported"
	case workspace.ErrRetrievalCorrupt:
		return "workspace retrieval index requires rebuild"
	case workspace.ErrRetrievalStale:
		return "workspace retrieval index is stale"
	default:
		return "workspace request failed"
	}
}

func (s *Service) GetWorkspaceSnapshot(ctx context.Context, request WorkspaceSnapshotRequest) (WorkspaceSnapshot, error) {
	if err := validateWorkspaceSchema(request.SchemaVersion); err != nil {
		return WorkspaceSnapshot{}, err
	}
	if s == nil || s.workspace == nil {
		return WorkspaceSnapshot{}, &WorkspaceBridgeError{Code: string(workspace.ErrStorageUnavailable), Message: "workspace service is unavailable"}
	}
	snapshot, err := s.workspace.Snapshot(ctx)
	if err != nil {
		return WorkspaceSnapshot{}, mapWorkspaceError(err)
	}
	return redactWorkspaceSnapshot(snapshot), nil
}

func (s *Service) QueryWorkspace(ctx context.Context, request WorkspaceNoteQueryRequest) ([]WorkspaceNoteResult, error) {
	if err := validateWorkspaceSchema(request.SchemaVersion); err != nil {
		return nil, err
	}
	if request.ProjectID == "" || request.Limit < 1 || request.Limit > 100 {
		return nil, &WorkspaceBridgeError{Code: string(workspace.ErrInvalidRequest), Message: "project and query limit are invalid"}
	}
	if s == nil || s.workspace == nil {
		return nil, &WorkspaceBridgeError{Code: string(workspace.ErrStorageUnavailable), Message: "workspace service is unavailable"}
	}
	documents, err := s.workspace.QueryRetrieval(ctx, workspace.ProjectID(request.ProjectID), request.Query, request.Limit)
	if err != nil {
		return nil, mapWorkspaceError(err)
	}
	result := make([]WorkspaceNoteResult, 0, len(documents))
	for _, document := range documents {
		result = append(result, WorkspaceNoteResult{
			ID: document.ID, NoteID: string(document.NoteID), ProjectID: string(document.ProjectID),
			RelativePath: document.RelativePath, SourceHash: document.SourceHash,
			Attribution: document.Attribution, IndexedAt: document.IndexedAt,
			Excerpt: boundedExcerpt(document.Content),
		})
	}
	return result, nil
}

func (s *Service) RebuildWorkspaceRetrieval(ctx context.Context, request WorkspaceMutationRequest) error {
	if err := validateWorkspaceSchema(request.SchemaVersion); err != nil {
		return err
	}
	if s == nil || s.workspace == nil {
		return &WorkspaceBridgeError{Code: string(workspace.ErrStorageUnavailable), Message: "workspace service is unavailable"}
	}
	if err := s.workspace.RebuildRetrieval(ctx, workspace.ProjectID(request.ProjectID)); err != nil {
		return mapWorkspaceError(err)
	}
	return nil
}

func (s *Service) RegisterWorkspaceProject(ctx context.Context, request WorkspaceProjectRequest) (WorkspaceProject, error) {
	if err := validateWorkspaceSchema(request.SchemaVersion); err != nil {
		return WorkspaceProject{}, err
	}
	if s == nil || s.workspace == nil {
		return WorkspaceProject{}, &WorkspaceBridgeError{Code: string(workspace.ErrStorageUnavailable), Message: "workspace service is unavailable"}
	}
	if err := validateWorkspaceProjectRequest(request); err != nil {
		return WorkspaceProject{}, &WorkspaceBridgeError{Code: string(workspace.ErrInvalidRequest), Message: err.Error()}
	}
	project, err := s.workspace.RegisterProject(ctx, workspace.Project{
		ID: workspace.ProjectID(request.ID), Name: request.Name, RepositoryRoot: request.RepositoryRoot,
		VaultRoot: request.VaultRoot, DefaultBranch: request.DefaultBranch,
	})
	if err != nil {
		return WorkspaceProject{}, mapWorkspaceError(err)
	}
	return redactWorkspaceProject(project), nil
}

func validateWorkspaceSchema(version string) error {
	if version != WorkspaceSchemaVersion {
		return &WorkspaceBridgeError{Code: string(workspace.ErrUnsupportedVersion), Message: "workspace contract version is unsupported"}
	}
	return nil
}

func validateWorkspaceProjectRequest(request WorkspaceProjectRequest) error {
	if request.ID == "" || request.Name == "" || request.RepositoryRoot == "" || request.VaultRoot == "" {
		return fmt.Errorf("project id, name, repository root, and Vault root are required")
	}
	if _, err := workspace.CanonicalRoot(request.RepositoryRoot); err != nil {
		return fmt.Errorf("repository root is invalid")
	}
	if _, err := workspace.CanonicalRoot(request.VaultRoot); err != nil {
		return fmt.Errorf("Vault root is invalid")
	}
	return nil
}

func redactWorkspaceSnapshot(snapshot workspace.WorkspaceSnapshot) WorkspaceSnapshot {
	projects := make([]WorkspaceProject, 0, len(snapshot.Projects))
	for _, project := range snapshot.Projects {
		projects = append(projects, redactWorkspaceProject(project))
	}
	worktrees := make([]WorkspaceWorktree, 0, len(snapshot.Worktrees))
	for _, worktree := range snapshot.Worktrees {
		worktrees = append(worktrees, WorkspaceWorktree{
			ID: string(worktree.ID), ProjectID: string(worktree.ProjectID), Branch: worktree.Branch,
			Revision: worktree.Revision, Status: worktree.Status, Dirty: worktree.Dirty,
			ActiveReference: worktree.ActiveReference, CreatedAt: worktree.CreatedAt, UpdatedAt: worktree.UpdatedAt,
		})
	}
	return WorkspaceSnapshot{
		SchemaVersion: snapshot.SchemaVersion, Projects: projects, Worktrees: worktrees,
		VaultNoteCount: snapshot.VaultNoteCount, WatcherState: snapshot.WatcherState,
		WatcherErrorCode: snapshot.WatcherErrorCode, RetrievalVersion: snapshot.RetrievalVersion,
		RetrievalState: snapshot.RetrievalState,
	}
}

func redactWorkspaceProject(project workspace.Project) WorkspaceProject {
	return WorkspaceProject{
		ID: string(project.ID), Name: project.Name, DefaultBranch: project.DefaultBranch,
		Status: project.Status, CreatedAt: project.CreatedAt, UpdatedAt: project.UpdatedAt,
		SchemaVersion: project.SchemaVersion,
	}
}

func boundedExcerpt(content string) string {
	const maxExcerpt = 240
	if len(content) <= maxExcerpt {
		return content
	}
	return content[:maxExcerpt]
}
