package platform

import (
	"context"

	"github.com/RahmatHadinata23758051/CortexOS/internal/cortex/application"
)

// Bridge is the thin Wails-facing adapter for application services.
type Bridge struct {
	service *application.Service
	ctx     context.Context
	emitter *EventEmitter
}

// NewBridge creates a binding adapter around an application service.
func NewBridge(service *application.Service) *Bridge {
	return &Bridge{
		service: service,
		ctx:     context.Background(),
		emitter: NewEventEmitter(),
	}
}

// WithContext returns a shallow copy of Bridge bound to the given context.
func (b *Bridge) WithContext(ctx context.Context) *Bridge {
	if ctx == nil {
		ctx = context.Background()
	}
	return &Bridge{service: b.service, ctx: ctx, emitter: b.emitter}
}

func (b *Bridge) getContext() context.Context {
	if b != nil && b.ctx != nil {
		return b.ctx
	}
	return context.Background()
}

// SetRuntimeContext installs the Wails runtime context for event emission.
func (b *Bridge) SetRuntimeContext(ctx context.Context) {
	if b != nil && b.emitter != nil {
		b.emitter.SetContext(ctx)
	}
}

// RegisterEventHook installs a runtime event hook.
func (b *Bridge) RegisterEventHook(hook RuntimeEventHook) func() {
	if b == nil || b.emitter == nil {
		return func() {}
	}
	return b.emitter.RegisterHook(hook)
}

// EmitEvent allows manual emission of bridge runtime events.
func (b *Bridge) EmitEvent(eventName string, payload any) {
	if b != nil && b.emitter != nil {
		b.emitter.Emit(eventName, payload)
	}
}

// GetRuntimeSnapshot exposes the deterministic application response to Wails.
func (b *Bridge) GetRuntimeSnapshot(request application.SnapshotRequest) (application.RuntimeSnapshot, error) {
	return b.service.GetRuntimeSnapshot(b.getContext(), request)
}

// DispatchBrainTask executes a Brain task through the typed application boundary.
func (b *Bridge) DispatchBrainTask(request application.BrainDispatchRequest) (application.BrainDispatchResult, error) {
	result, err := b.service.BrainDispatch(b.getContext(), request)
	if err == nil && b.emitter != nil {
		b.emitter.Emit(EventBrainStateChanged, result)
	}
	return result, err
}

// DispatchBrainTaskStream executes a Brain task and emits each stream chunk
// through the bridge event emitter. The callback remains bounded and synchronous
// with the brain stream reader; consumers must not block it.
func (b *Bridge) DispatchBrainTaskStream(request application.BrainDispatchRequest) (application.BrainDispatchResult, error) {
	result, err := b.service.BrainDispatchStream(b.getContext(), request, func(chunk application.StreamChunk) {
		if b.emitter != nil {
			b.emitter.Emit(EventBrainStreamChunk, chunk)
		}
	})
	if b.emitter != nil {
		b.emitter.Emit(EventBrainStateChanged, result)
	}
	return result, err
}

// RouteBrainTaskToOrchestra submits a bounded routing intent from Brain to Orchestra
// through the application service boundary and emits corresponding lifecycle events.
func (b *Bridge) RouteBrainTaskToOrchestra(request application.BrainOrchestraRouteRequest) (application.BrainOrchestraRouteResult, error) {
	result, err := b.service.RouteBrainTaskToOrchestra(b.getContext(), request)
	if b.emitter != nil {
		switch result.Status {
		case "accepted":
			b.emitter.Emit(EventBrainRouteAccepted, result)
		case "rejected":
			b.emitter.Emit(EventBrainRouteRejected, result)
		case "queued":
			b.emitter.Emit(EventBrainRouteQueued, result)
		case "canceled":
			b.emitter.Emit(EventBrainRouteCanceled, result)
		}
	}
	return result, err
}

// ProposeToolCall submits a bounded, redacted tool proposal for operator review
// and emits EventToolProposed when accepted.
func (b *Bridge) ProposeToolCall(proposal application.ToolProposal) (application.ToolProposalResult, error) {
	result, err := b.service.ProposeToolCall(b.getContext(), proposal)
	if err == nil && b.emitter != nil {
		b.emitter.Emit(EventToolProposed, result)
	}
	return result, err
}

// DecideToolCall applies an operator approval decision, executes the governed
// tool through Harness if approved, and emits lifecycle events.
func (b *Bridge) DecideToolCall(decision application.ToolApprovalDecision) (application.ToolApprovalResult, error) {
	result, err := b.service.DecideToolCall(b.getContext(), decision)
	if b.emitter != nil {
		if err == nil && decision.Decision == "approve" {
			b.emitter.Emit(EventToolApproved, result)
		}
		switch result.Status {
		case application.ToolApprovalDenied:
			b.emitter.Emit(EventToolDenied, result)
		case application.ToolApprovalExecuted:
			b.emitter.Emit(EventToolExecuted, result)
		case application.ToolApprovalFailed, application.ToolApprovalCanceled:
			b.emitter.Emit(EventToolFailed, result)
		}
	}
	return result, err
}

// GetToolApproval returns a single tool proposal record by ID.
func (b *Bridge) GetToolApproval(proposalID string) (application.ToolProposalRecord, error) {
	return b.service.GetToolApproval(b.getContext(), proposalID)
}

// ListPendingToolApprovals returns all pending proposals requiring operator decision.
func (b *Bridge) ListPendingToolApprovals() ([]application.ToolProposalRecord, error) {
	return b.service.ListPendingToolApprovals(b.getContext())
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

// MergeOrchestraTask exposes the Inspector-gated merge command to Wails.
// Business rules remain in the application and Orchestra layers.
func (b *Bridge) MergeOrchestraTask(request application.OrchestraMergeRequest) (application.OrchestraMergeResult, error) {
	return b.service.MergeOrchestraTask(b.getContext(), request)
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

// GetCockpitRuntime exposes runtime status through a typed DTO.
func (b *Bridge) GetCockpitRuntime() (application.CockpitRuntimeSnapshot, error) {
	runtime, err := b.service.GetCockpitRuntime(b.getContext())
	if err == nil && b.emitter != nil {
		b.emitter.Emit(EventCockpitRuntimeChanged, runtime)
	}
	return runtime, err
}

// GetCockpitWorkspace exposes workspace facts through a typed DTO.
func (b *Bridge) GetCockpitWorkspace(request application.CockpitWorkspaceRequest) (application.CockpitWorkspaceSnapshot, error) {
	ws, err := b.service.GetCockpitWorkspace(b.getContext(), request)
	if err == nil && b.emitter != nil {
		b.emitter.Emit(EventCockpitWorkspaceChanged, ws)
	}
	return ws, err
}

// RegisterCockpitProject exposes the controlled project-registration operation.
func (b *Bridge) RegisterCockpitProject(request application.CockpitProjectRequest) (application.CockpitProject, error) {
	project, err := b.service.RegisterCockpitProject(b.getContext(), request)
	if err == nil && b.emitter != nil {
		b.emitter.Emit(EventCockpitProjectRegistered, project)
	}
	return project, err
}

// QueryCockpitWorkspace exposes bounded workspace retrieval.
func (b *Bridge) QueryCockpitWorkspace(request application.CockpitQueryRequest) ([]application.CockpitQueryResult, error) {
	return b.service.QueryCockpitWorkspace(b.getContext(), request)
}

// RebuildCockpitRetrieval exposes a controlled retrieval mutation.
func (b *Bridge) RebuildCockpitRetrieval(request application.CockpitMutationRequest) error {
	err := b.service.RebuildCockpitRetrieval(b.getContext(), request)
	if err == nil && b.emitter != nil {
		b.emitter.Emit(EventCockpitRetrievalRebuilt, map[string]string{"projectId": request.ProjectID})
	}
	return err
}
