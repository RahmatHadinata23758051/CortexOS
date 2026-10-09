package application

import (
	"context"
	"time"
)

func (s *Service) GetCockpitRuntime(ctx context.Context) (CockpitRuntimeSnapshot, error) {
	if s == nil {
		return CockpitRuntimeSnapshot{}, &CockpitBridgeError{Code: "cockpit.internal", Message: "service unavailable"}
	}
	if s.cockpit != nil {
		return s.cockpit.GetCockpitRuntime(ctx)
	}
	runtime, err := s.GetRuntimeSnapshot(ctx, SnapshotRequest{SchemaVersion: runtimeSchemaVersion})
	if err != nil {
		return CockpitRuntimeSnapshot{}, mapCockpitError(err)
	}
	return CockpitRuntimeSnapshot{
		SchemaVersion: CockpitBridgeSchemaVersion,
		Status:        runtime.Status,
		Environment:   runtime.Environment,
		Provider:      runtime.Provider,
	}, nil
}

func (s *Service) GetCockpitWorkspace(ctx context.Context, request CockpitWorkspaceRequest) (CockpitWorkspaceSnapshot, error) {
	if err := validateCockpitRequest(ctx, request.SchemaVersion); err != nil {
		return CockpitWorkspaceSnapshot{}, err
	}
	if s == nil {
		return CockpitWorkspaceSnapshot{}, &CockpitBridgeError{Code: "cockpit.internal", Message: "service unavailable"}
	}
	if s.cockpit != nil {
		return s.cockpit.GetCockpitWorkspace(ctx, request)
	}
	ws, err := s.GetWorkspaceSnapshot(ctx, WorkspaceSnapshotRequest{SchemaVersion: WorkspaceSchemaVersion})
	if err != nil {
		return CockpitWorkspaceSnapshot{}, mapCockpitError(err)
	}
	projects := make([]CockpitProject, 0, len(ws.Projects))
	for _, p := range ws.Projects {
		if request.ProjectID != "" && p.ID != request.ProjectID {
			continue
		}
		projects = append(projects, CockpitProject{
			ID:            p.ID,
			Name:          p.Name,
			DefaultBranch: p.DefaultBranch,
			Status:        p.Status,
			CreatedAt:     p.CreatedAt,
			UpdatedAt:     p.UpdatedAt,
			SchemaVersion: CockpitBridgeSchemaVersion,
		})
	}
	worktrees := make([]CockpitWorktree, 0, len(ws.Worktrees))
	for _, w := range ws.Worktrees {
		if request.ProjectID != "" && w.ProjectID != request.ProjectID {
			continue
		}
		worktrees = append(worktrees, CockpitWorktree{
			ID:              w.ID,
			ProjectID:       w.ProjectID,
			Branch:          w.Branch,
			Revision:        w.Revision,
			Status:          w.Status,
			Dirty:           w.Dirty,
			ActiveReference: w.ActiveReference,
			CreatedAt:       w.CreatedAt,
			UpdatedAt:       w.UpdatedAt,
		})
	}
	return CockpitWorkspaceSnapshot{
		SchemaVersion:    CockpitBridgeSchemaVersion,
		Projects:         projects,
		Worktrees:        worktrees,
		VaultNoteCount:   ws.VaultNoteCount,
		WatcherState:     ws.WatcherState,
		WatcherErrorCode: ws.WatcherErrorCode,
		RetrievalVersion: ws.RetrievalVersion,
		RetrievalState:   ws.RetrievalState,
	}, nil
}

func (s *Service) RegisterCockpitProject(ctx context.Context, request CockpitProjectRequest) (CockpitProject, error) {
	if err := validateCockpitRequest(ctx, request.SchemaVersion); err != nil {
		return CockpitProject{}, err
	}
	if s == nil {
		return CockpitProject{}, &CockpitBridgeError{Code: "cockpit.internal", Message: "service unavailable"}
	}
	if s.cockpit != nil {
		return s.cockpit.RegisterCockpitProject(ctx, request)
	}
	project, err := s.RegisterWorkspaceProject(ctx, WorkspaceProjectRequest{
		SchemaVersion:  WorkspaceSchemaVersion,
		ID:             request.ID,
		Name:           request.Name,
		RepositoryRoot: request.RepositoryRoot,
		VaultRoot:      request.VaultRoot,
		DefaultBranch:  request.DefaultBranch,
	})
	if err != nil {
		return CockpitProject{}, mapCockpitError(err)
	}
	return CockpitProject{
		ID:            project.ID,
		Name:          project.Name,
		DefaultBranch: project.DefaultBranch,
		Status:        project.Status,
		CreatedAt:     project.CreatedAt,
		UpdatedAt:     project.UpdatedAt,
		SchemaVersion: CockpitBridgeSchemaVersion,
	}, nil
}

func (s *Service) QueryCockpitWorkspace(ctx context.Context, request CockpitQueryRequest) ([]CockpitQueryResult, error) {
	if err := validateCockpitRequest(ctx, request.SchemaVersion); err != nil {
		return nil, err
	}
	if s == nil {
		return nil, &CockpitBridgeError{Code: "cockpit.internal", Message: "service unavailable"}
	}
	if s.cockpit != nil {
		return s.cockpit.QueryCockpitWorkspace(ctx, request)
	}
	limit := request.Limit
	if limit <= 0 {
		limit = 10
	}
	results, err := s.QueryWorkspace(ctx, WorkspaceNoteQueryRequest{
		SchemaVersion: WorkspaceSchemaVersion,
		ProjectID:     request.ProjectID,
		Query:         request.Query,
		Limit:         limit,
	})
	if err != nil {
		return nil, mapCockpitError(err)
	}
	items := make([]CockpitQueryResult, 0, len(results))
	for _, r := range results {
		items = append(items, CockpitQueryResult{
			ID:            r.ID,
			NoteID:        r.NoteID,
			ProjectID:     r.ProjectID,
			RelativePath:  r.RelativePath,
			SourceHash:    r.SourceHash,
			Attribution:   r.Attribution,
			IndexedAt:     r.IndexedAt.Format(time.RFC3339),
			Excerpt:       r.Excerpt,
			SchemaVersion: CockpitBridgeSchemaVersion,
		})
	}
	return items, nil
}

func (s *Service) RebuildCockpitRetrieval(ctx context.Context, request CockpitMutationRequest) error {
	if err := validateCockpitRequest(ctx, request.SchemaVersion); err != nil {
		return err
	}
	if s == nil {
		return &CockpitBridgeError{Code: "cockpit.internal", Message: "service unavailable"}
	}
	if s.cockpit != nil {
		return s.cockpit.RebuildCockpitRetrieval(ctx, request)
	}
	return mapCockpitError(s.RebuildWorkspaceRetrieval(ctx, WorkspaceMutationRequest{
		SchemaVersion: WorkspaceSchemaVersion,
		ProjectID:     request.ProjectID,
	}))
}
