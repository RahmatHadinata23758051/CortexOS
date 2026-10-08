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

// ListHarnessCapabilities exposes safe Harness capability metadata.
func (b *Bridge) ListHarnessCapabilities(request application.HarnessCapabilitiesRequest) ([]application.HarnessCapabilitySummary, error) {
	return b.service.ListHarnessCapabilities(b.getContext(), request)
}

// ListHarnessWorkers exposes safe Harness worker health summaries.
func (b *Bridge) ListHarnessWorkers(request application.HarnessWorkersRequest) ([]application.HarnessWorkerSummary, error) {
	return b.service.ListHarnessWorkers(b.getContext(), request)
}

// ListActiveHarnessExecutions exposes safe active execution summaries.
func (b *Bridge) ListActiveHarnessExecutions(request application.HarnessActiveExecutionsRequest) ([]application.HarnessExecutionSummary, error) {
	return b.service.ListActiveHarnessExecutions(b.getContext(), request)
}

// GetHarnessEvidence exposes metadata-only, redacted evidence summaries.
func (b *Bridge) GetHarnessEvidence(request application.HarnessEvidenceRequest) ([]application.HarnessEvidenceSummary, error) {
	return b.service.GetHarnessEvidence(b.getContext(), request)
}

// GetStaffSummary exposes a safe logical Staff summary without process identity or secrets.
func (b *Bridge) GetStaffSummary(request application.StaffRequest) (application.StaffSummary, error) {
	return b.service.GetStaffSummary(b.getContext(), request)
}

// ListStaffSummaries exposes safe Staff observability summaries.
func (b *Bridge) ListStaffSummaries(request application.StaffListRequest) ([]application.StaffSummary, error) {
	return b.service.ListStaffSummaries(b.getContext(), request)
}

// ListStaffByWorkspace exposes Staff summaries scoped to an approved workspace.
func (b *Bridge) ListStaffByWorkspace(request application.StaffByWorkspaceRequest) ([]application.StaffSummary, error) {
	return b.service.ListStaffByWorkspace(b.getContext(), request)
}

// ListStaffCapabilities exposes safe Staff capability metadata.
func (b *Bridge) ListStaffCapabilities(request application.StaffCapabilitiesRequest) ([]application.StaffCapabilitySummary, error) {
	return b.service.ListStaffCapabilities(b.getContext(), request)
}

// GetStaffAssignment exposes only logical workspace assignment identifiers.
func (b *Bridge) GetStaffAssignment(request application.StaffAssignmentRequest) (application.StaffAssignmentSummary, error) {
	return b.service.GetStaffAssignment(b.getContext(), request)
}

// ListKnowledgeSources exposes safe knowledge source summaries.
func (b *Bridge) ListKnowledgeSources(request application.KnowledgeSourcesRequest) ([]application.KnowledgeSourceSummary, error) {
	return b.service.ListKnowledgeSources(b.getContext(), request)
}

// QueryKnowledge exposes bounded, metadata-safe retrieval results.
func (b *Bridge) QueryKnowledge(request application.KnowledgeQueryRequest) ([]application.KnowledgeQueryResult, error) {
	return b.service.QueryKnowledge(b.getContext(), request)
}
