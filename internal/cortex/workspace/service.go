package workspace

import (
	"context"
	"errors"
)

const (
	WatcherStateUnavailable = "unavailable"
	WatcherStateReady       = "ready"
	RetrievalStateUnknown   = "unknown"
)

// Dependencies are the only collaborators accepted by the Workspace service.
// Adapters remain replaceable and the service does not know about Wails,
// frontend DTOs, shell commands, or a concrete persistence implementation.
type Dependencies struct {
	State     StateStore
	Projects  ProjectRegistry
	Worktrees WorktreeManager
	Vault     VaultStore
	Watcher   FileWatcher
	Retrieval RetrievalIndex
}

type Service struct {
	dependencies Dependencies
}

func NewService(dependencies Dependencies) (*Service, error) {
	if dependencies.State == nil {
		return nil, NewError(ErrInvalidRequest, "workspace state store is required")
	}
	if dependencies.Projects == nil {
		return nil, NewError(ErrInvalidRequest, "workspace project registry is required")
	}
	if dependencies.Worktrees == nil {
		return nil, NewError(ErrInvalidRequest, "workspace worktree manager is required")
	}
	return &Service{dependencies: dependencies}, nil
}

func (s *Service) Open(ctx context.Context) error {
	if err := requireServiceContext(ctx); err != nil {
		return err
	}
	if err := s.dependencies.State.Open(ctx); err != nil {
		return CanceledError(err)
	}
	version, err := s.dependencies.State.SchemaVersion(ctx)
	if err != nil {
		return CanceledError(err)
	}
	if version != ContractVersion && version != "workspace.sqlite.v1" {
		return NewError(ErrUnsupportedVersion, "workspace state schema is unsupported")
	}
	return nil
}

func (s *Service) Close() error {
	if s == nil || s.dependencies.State == nil {
		return nil
	}
	return s.dependencies.State.Close()
}

func (s *Service) RegisterProject(ctx context.Context, project Project) (Project, error) {
	if err := requireServiceContext(ctx); err != nil {
		return Project{}, err
	}
	return s.dependencies.Projects.RegisterProject(ctx, project)
}

func (s *Service) UpdateProject(ctx context.Context, project Project) (Project, error) {
	if err := requireServiceContext(ctx); err != nil {
		return Project{}, err
	}
	return s.dependencies.Projects.UpdateProject(ctx, project)
}

func (s *Service) ArchiveProject(ctx context.Context, id ProjectID) error {
	if err := requireServiceContext(ctx); err != nil {
		return err
	}
	return s.dependencies.Projects.ArchiveProject(ctx, id)
}

func (s *Service) CreateWorktree(ctx context.Context, worktree Worktree) (Worktree, error) {
	if err := requireServiceContext(ctx); err != nil {
		return Worktree{}, err
	}
	return s.dependencies.Worktrees.CreateWorktree(ctx, worktree)
}

func (s *Service) InspectWorktree(ctx context.Context, id WorktreeID) (Worktree, error) {
	if err := requireServiceContext(ctx); err != nil {
		return Worktree{}, err
	}
	return s.dependencies.Worktrees.InspectWorktree(ctx, id)
}

func (s *Service) RemoveWorktree(ctx context.Context, id WorktreeID) error {
	if err := requireServiceContext(ctx); err != nil {
		return err
	}
	return s.dependencies.Worktrees.RemoveWorktree(ctx, id)
}

func (s *Service) Snapshot(ctx context.Context) (WorkspaceSnapshot, error) {
	if err := requireServiceContext(ctx); err != nil {
		return WorkspaceSnapshot{}, err
	}
	projects, err := s.dependencies.Projects.ListProjects(ctx)
	if err != nil {
		return WorkspaceSnapshot{}, CanceledError(err)
	}
	worktrees := make([]Worktree, 0)
	for _, project := range projects {
		items, err := s.dependencies.Worktrees.ListWorktrees(ctx, project.ID)
		if err != nil {
			return WorkspaceSnapshot{}, CanceledError(err)
		}
		worktrees = append(worktrees, items...)
	}
	var noteCount int
	if s.dependencies.Vault != nil {
		for _, project := range projects {
			notes, err := s.dependencies.Vault.ListNotes(ctx, project.ID)
			if err != nil {
				return WorkspaceSnapshot{}, CanceledError(err)
			}
			noteCount += len(notes)
		}
	}
	watcherState := WatcherStateUnavailable
	if s.dependencies.Watcher != nil {
		watcherState = WatcherStateReady
	}
	retrievalState := RetrievalStateUnknown
	if s.dependencies.Retrieval != nil {
		retrievalState = "ready"
	}
	return WorkspaceSnapshot{
		SchemaVersion:    ContractVersion,
		Projects:         projects,
		Worktrees:        worktrees,
		VaultNoteCount:   noteCount,
		WatcherState:     watcherState,
		RetrievalVersion: "workspace.retrieval.v1",
		RetrievalState:   retrievalState,
	}, nil
}

func requireServiceContext(ctx context.Context) error {
	if ctx == nil {
		return NewError(ErrInvalidRequest, "context is required")
	}
	if err := ctx.Err(); err != nil {
		return CanceledError(err)
	}
	return nil
}

func IsServiceDependencyError(err error) bool {
	return errors.Is(err, context.Canceled) || ErrorCodeOf(err) != ""
}
