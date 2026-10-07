package platform

import (
	"context"

	"github.com/RahmatHadinata23758051/CortexOS/internal/cortex/application"
)

// Bridge is the thin Wails-facing adapter for application services.
type Bridge struct {
	service *application.Service
	ctx     context.Context
}

// NewBridge creates a binding adapter around an application service.
func NewBridge(service *application.Service) *Bridge {
	return &Bridge{service: service, ctx: context.Background()}
}

// WithContext returns a shallow copy of Bridge bound to the given context.
func (b *Bridge) WithContext(ctx context.Context) *Bridge {
	if ctx == nil {
		ctx = context.Background()
	}
	return &Bridge{service: b.service, ctx: ctx}
}

func (b *Bridge) getContext() context.Context {
	if b != nil && b.ctx != nil {
		return b.ctx
	}
	return context.Background()
}

// GetRuntimeSnapshot exposes the deterministic application response to Wails.
func (b *Bridge) GetRuntimeSnapshot(request application.SnapshotRequest) (application.RuntimeSnapshot, error) {
	return b.service.GetRuntimeSnapshot(b.getContext(), request)
}

func (b *Bridge) GetWorkspaceSnapshot(request application.WorkspaceSnapshotRequest) (application.WorkspaceSnapshot, error) {
	return b.service.GetWorkspaceSnapshot(b.getContext(), request)
}

func (b *Bridge) QueryWorkspace(request application.WorkspaceNoteQueryRequest) ([]application.WorkspaceNoteResult, error) {
	return b.service.QueryWorkspace(b.getContext(), request)
}

func (b *Bridge) RebuildWorkspaceRetrieval(request application.WorkspaceMutationRequest) error {
	return b.service.RebuildWorkspaceRetrieval(b.getContext(), request)
}

func (b *Bridge) RegisterWorkspaceProject(request application.WorkspaceProjectRequest) (application.WorkspaceProject, error) {
	return b.service.RegisterWorkspaceProject(b.getContext(), request)
}

func (b *Bridge) ListOrchestraTasks(request application.OrchestraTaskListRequest) ([]application.OrchestraTaskSummary, error) {
	return b.service.ListOrchestraTasks(b.getContext(), request)
}

func (b *Bridge) GetOrchestraTask(request application.OrchestraTaskRequest) (application.OrchestraTaskDetail, error) {
	return b.service.GetOrchestraTask(b.getContext(), request)
}

func (b *Bridge) CancelOrchestraTask(request application.OrchestraTaskRequest) error {
	return b.service.CancelOrchestraTask(b.getContext(), request)
}

func (b *Bridge) RetryOrchestraTask(request application.OrchestraTaskRequest) error {
	return b.service.RetryOrchestraTask(b.getContext(), request)
}
