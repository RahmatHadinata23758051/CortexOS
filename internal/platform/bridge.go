package platform

import (
	"context"

	"github.com/RahmatHadinata23758051/CortexOS/internal/cortex/application"
)

// Bridge is the thin Wails-facing adapter for application services.
type Bridge struct {
	service *application.Service
}

// NewBridge creates a binding adapter around an application service.
func NewBridge(service *application.Service) *Bridge {
	return &Bridge{service: service}
}

// GetRuntimeSnapshot exposes the deterministic application response to Wails.
func (b *Bridge) GetRuntimeSnapshot(request application.SnapshotRequest) (application.RuntimeSnapshot, error) {
	return b.service.GetRuntimeSnapshot(context.Background(), request)
}

func (b *Bridge) GetWorkspaceSnapshot(request application.WorkspaceSnapshotRequest) (application.WorkspaceSnapshot, error) {
	return b.service.GetWorkspaceSnapshot(context.Background(), request)
}

func (b *Bridge) QueryWorkspace(request application.WorkspaceNoteQueryRequest) ([]application.WorkspaceNoteResult, error) {
	return b.service.QueryWorkspace(context.Background(), request)
}

func (b *Bridge) RebuildWorkspaceRetrieval(request application.WorkspaceMutationRequest) error {
	return b.service.RebuildWorkspaceRetrieval(context.Background(), request)
}

func (b *Bridge) RegisterWorkspaceProject(request application.WorkspaceProjectRequest) (application.WorkspaceProject, error) {
	return b.service.RegisterWorkspaceProject(context.Background(), request)
}
