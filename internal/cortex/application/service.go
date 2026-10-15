package application

import (
	"context"
	"errors"
	"fmt"

	"github.com/RahmatHadinata23758051/CortexOS/internal/cortex/workspace"
)

const runtimeSchemaVersion = "cortexos.runtime.v1"

var ErrInvalidRequest = errors.New("invalid runtime snapshot request")

// SnapshotRequest identifies the version of the bridge contract requested by a client.
type SnapshotRequest struct {
	SchemaVersion string `json:"schemaVersion"`
}

// RuntimeSnapshot is the deterministic read-only response exposed by the first bridge slice.
type RuntimeSnapshot struct {
	SchemaVersion string `json:"schemaVersion"`
	Status        string `json:"status"`
	Environment   string `json:"environment"`
	Provider      string `json:"provider"`
}

// Service owns application behavior and does not depend on Wails or frontend packages.
type Service struct {
	workspace      *workspace.Service
	orchestra      OrchestraPort
	harness        HarnessPort
	staff          StaffPort
	knowledge      KnowledgePort
	cockpit        CockpitPort
	provider       ProviderPort
	brain          BrainPort
	brainOrchestra BrainOrchestraPort
	toolApproval   ToolApprovalPort
	coordination   CoordinationPort
	grounding      KnowledgeGroundingPort
	plan           OrchestraPlanPort
	files          FilesPort
	workers        WorkerLifecyclePort
}

// NewService creates the minimal Phase 1 application service.
func NewService() *Service {
	return &Service{}
}

// NewServiceWithWorkspace adds the optional Workspace application boundary
// without coupling the application package to Wails or frontend DTOs.
func NewServiceWithWorkspace(workspaceService *workspace.Service) *Service {
	return &Service{workspace: workspaceService}
}

// NewServiceWithOrchestra adds the optional Orchestra application boundary.
func NewServiceWithOrchestra(orchestraService OrchestraPort) *Service {
	return &Service{orchestra: orchestraService}
}

// NewServiceWithWorkspaceAndOrchestra adds both boundaries.
func NewServiceWithWorkspaceAndOrchestra(workspaceService *workspace.Service, orchestraService OrchestraPort) *Service {
	return &Service{workspace: workspaceService, orchestra: orchestraService}
}

// NewServiceWithHarness adds the optional Harness observability boundary.
func NewServiceWithHarness(harnessService HarnessPort) *Service {
	return &Service{harness: harnessService}
}

// NewServiceWithStaff adds the optional Staff observability boundary.
func NewServiceWithStaff(staffService StaffPort) *Service {
	return &Service{staff: staffService}
}

// NewServiceWithKnowledge adds the optional Knowledge observability boundary.
func NewServiceWithKnowledge(knowledgeService KnowledgePort) *Service {
	return &Service{knowledge: knowledgeService}
}

// NewServiceWithCockpit adds the optional Cockpit application boundary.
func NewServiceWithCockpit(cockpitService CockpitPort) *Service {
	return &Service{cockpit: cockpitService}
}

// NewServiceWithProvider adds the optional Provider application boundary.
func NewServiceWithProvider(providerService ProviderPort) *Service {
	return &Service{provider: providerService}
}

// NewServiceWithBrain adds the optional Brain application boundary.
func NewServiceWithBrain(brainService BrainPort) *Service {
	return &Service{brain: brainService}
}

// NewServiceWithBrainOrchestra adds the governed Brain-to-Orchestra routing port.
func NewServiceWithBrainOrchestra(brainOrchestraService BrainOrchestraPort) *Service {
	return &Service{brainOrchestra: brainOrchestraService}
}

// NewServiceWithBrainAndOrchestra adds Brain, Orchestra, and the governed routing port.
func NewServiceWithBrainAndOrchestra(brainService BrainPort, orchestraService OrchestraPort, brainOrchestraService BrainOrchestraPort) *Service {
	return &Service{brain: brainService, orchestra: orchestraService, brainOrchestra: brainOrchestraService}
}

// NewServiceWithProviderAndBrain adds Provider and Brain boundaries.
func NewServiceWithProviderAndBrain(providerService ProviderPort, brainService BrainPort) *Service {
	return &Service{provider: providerService, brain: brainService}
}

// NewServiceWithWorkspaceAndOrchestraAndProviderAndBrain adds Workspace, Orchestra, Provider, and Brain boundaries.
func NewServiceWithWorkspaceAndOrchestraAndProviderAndBrain(workspaceService *workspace.Service, orchestraService OrchestraPort, providerService ProviderPort, brainService BrainPort) *Service {
	return &Service{workspace: workspaceService, orchestra: orchestraService, provider: providerService, brain: brainService}
}

// NewServiceWithWorkspaceAndOrchestraAndProvider is deprecated; use the four-port
// constructor and pass an explicit nil brain when the provider is disabled.
// Removed to force fail-closed Brain wiring through OpenBrain.
func NewServiceWithWorkspaceAndOrchestraAndProvider(workspaceService *workspace.Service, orchestraService OrchestraPort, providerService ProviderPort) *Service {
	return NewServiceWithWorkspaceAndOrchestraAndProviderAndBrain(workspaceService, orchestraService, providerService, nil)
}

// NewServiceWithFull combines all available application boundaries.
func NewServiceWithFull(workspaceService *workspace.Service, orchestraService OrchestraPort, harnessService HarnessPort, staffService StaffPort, knowledgeService KnowledgePort) *Service {
	return &Service{workspace: workspaceService, orchestra: orchestraService, harness: harnessService, staff: staffService, knowledge: knowledgeService}
}

// NewServiceWithAll combines all available application boundaries including Brain.
func NewServiceWithAll(workspaceService *workspace.Service, orchestraService OrchestraPort, harnessService HarnessPort, staffService StaffPort, knowledgeService KnowledgePort, brainService BrainPort) *Service {
	return &Service{workspace: workspaceService, orchestra: orchestraService, harness: harnessService, staff: staffService, knowledge: knowledgeService, brain: brainService}
}

// GetRuntimeSnapshot returns a deterministic local runtime status.
func (s *Service) GetRuntimeSnapshot(ctx context.Context, request SnapshotRequest) (RuntimeSnapshot, error) {
	if err := ctx.Err(); err != nil {
		return RuntimeSnapshot{}, err
	}
	if request.SchemaVersion != runtimeSchemaVersion {
		return RuntimeSnapshot{}, fmt.Errorf("%w: schema version %q is not supported", ErrInvalidRequest, request.SchemaVersion)
	}

	providerStatus := "disabled"
	if s != nil && s.provider != nil {
		providerStatus = "configured"
	}

	return RuntimeSnapshot{
		SchemaVersion: runtimeSchemaVersion,
		Status:        "ready",
		Environment:   "local",
		Provider:      providerStatus,
	}, nil
}

// RuntimeSchemaVersion returns the supported bridge schema identifier.
func RuntimeSchemaVersion() string {
	return runtimeSchemaVersion
}

// BrainDispatch executes a brain task via the application boundary.
func (s *Service) BrainDispatch(ctx context.Context, req BrainDispatchRequest) (BrainDispatchResult, error) {
	if s == nil || s.brain == nil {
		return BrainDispatchResult{}, errors.New("brain not configured")
	}
	return s.brain.Dispatch(ctx, req)
}

// BrainDispatchStream executes a brain task with streaming callbacks.
func (s *Service) BrainDispatchStream(ctx context.Context, req BrainDispatchRequest, handler StreamChunkHandler) (BrainDispatchResult, error) {
	if s == nil || s.brain == nil {
		return BrainDispatchResult{}, errors.New("brain not configured")
	}
	return s.brain.DispatchStream(ctx, req, handler)
}

// SetBrainOrchestra attaches the BrainOrchestra application port.
func (s *Service) SetBrainOrchestra(port BrainOrchestraPort) {
	if s != nil {
		s.brainOrchestra = port
	}
}

// SetToolApproval attaches the governed tool approval application port.
func (s *Service) SetToolApproval(port ToolApprovalPort) {
	if s != nil {
		s.toolApproval = port
	}
}

// SetCoordination attaches the bounded multi-Staff coordination projection.
func (s *Service) SetCoordination(port CoordinationPort) {
	if s != nil {
		s.coordination = port
	}
}

// GetCoordinationSnapshot returns the latest bounded multi-Staff coordination view.
func (s *Service) GetCoordinationSnapshot(ctx context.Context, request CoordinationSnapshotRequest) (CoordinationSnapshot, error) {
	if s == nil || s.coordination == nil {
		return CoordinationSnapshot{}, errors.New("coordination not configured")
	}
	return s.coordination.GetCoordinationSnapshot(ctx, request)
}

// ProposeToolCall submits a bounded, redacted tool proposal for operator approval.
func (s *Service) ProposeToolCall(ctx context.Context, proposal ToolProposal) (ToolProposalResult, error) {
	if s == nil || s.toolApproval == nil {
		return ToolProposalResult{}, errors.New("tool approval not configured")
	}
	return s.toolApproval.SubmitProposal(ctx, proposal)
}

// DecideToolCall applies an idempotent operator decision to a pending tool proposal.
func (s *Service) DecideToolCall(ctx context.Context, decision ToolApprovalDecision) (ToolApprovalResult, error) {
	if s == nil || s.toolApproval == nil {
		return ToolApprovalResult{}, errors.New("tool approval not configured")
	}
	return s.toolApproval.DecideProposal(ctx, decision)
}

// GetToolApproval returns one sanitized proposal record by ID.
func (s *Service) GetToolApproval(ctx context.Context, proposalID string) (ToolProposalRecord, error) {
	if s == nil || s.toolApproval == nil {
		return ToolProposalRecord{}, errors.New("tool approval not configured")
	}
	return s.toolApproval.GetProposal(ctx, proposalID)
}

// ListPendingToolApprovals returns pending proposals in stable order.
func (s *Service) ListPendingToolApprovals(ctx context.Context) ([]ToolProposalRecord, error) {
	if s == nil || s.toolApproval == nil {
		return nil, errors.New("tool approval not configured")
	}
	return s.toolApproval.ListPendingProposals(ctx)
}

// RouteBrainTaskToOrchestra submits a bounded routing intent from Brain to Orchestra.
func (s *Service) RouteBrainTaskToOrchestra(ctx context.Context, req BrainOrchestraRouteRequest) (BrainOrchestraRouteResult, error) {
	if s == nil || s.brainOrchestra == nil {
		return BrainOrchestraRouteResult{}, errors.New("brain-orchestra routing not configured")
	}
	return s.brainOrchestra.RouteBrainToOrchestra(ctx, req)
}

// SetKnowledge attaches the safe Knowledge application port.
// SetStaff attaches the Staff observability port.
func (s *Service) SetStaff(port StaffPort) {
	if s != nil {
		s.staff = port
	}
}

func (s *Service) SetKnowledge(port KnowledgePort) {
	if s != nil {
		s.knowledge = port
	}
}

// SetKnowledgeGrounding attaches the Knowledge retrieval grounding port.
func (s *Service) SetKnowledgeGrounding(port KnowledgeGroundingPort) {
	if s != nil {
		s.grounding = port
	}
}

// SetFiles attaches the read-only project Files port.
func (s *Service) SetFiles(port FilesPort) {
	if s != nil {
		s.files = port
	}
}

// ListFiles lists a bounded project-scoped file tree.
func (s *Service) ListFiles(ctx context.Context, request FilesListRequest) (FilesListResult, error) {
	if s == nil || s.files == nil {
		return FilesListResult{}, ErrFilesUnavailable
	}
	return s.files.ListFiles(ctx, request)
}

// ReadFile reads a bounded project-scoped file or Vault Markdown document.
func (s *Service) ReadFile(ctx context.Context, request FilesReadRequest) (FilesReadResult, error) {
	if s == nil || s.files == nil {
		return FilesReadResult{}, ErrFilesUnavailable
	}
	return s.files.ReadFile(ctx, request)
}

// SetWorkerLifecycle attaches the ephemeral worker-card projection.
func (s *Service) SetWorkerLifecycle(port WorkerLifecyclePort) {
	if s != nil {
		s.workers = port
	}
}

// GetWorkerCards returns active worker cards and optional lifecycle history.
func (s *Service) GetWorkerCards(ctx context.Context, request WorkerCardsRequest) (WorkerCardsResult, error) {
	if s == nil || s.workers == nil {
		return WorkerCardsResult{}, ErrWorkerInvalidRequest
	}
	return s.workers.Sync(ctx, request)
}

// TransitionWorker moves an ephemeral worker card through its governed lifecycle.
func (s *Service) TransitionWorker(ctx context.Context, workerID string, target WorkerLifecycleState, reason string) (WorkerCard, WorkerLifecycleEvent, error) {
	if s == nil || s.workers == nil {
		return WorkerCard{}, WorkerLifecycleEvent{}, ErrWorkerInvalidRequest
	}
	return s.workers.Transition(ctx, workerID, target, reason)
}

// SetOrchestraPlan attaches the Orchestra plan proposal/confirmation port.
// When unset, plan operations fail closed with a stable error rather than
// silently auto-dispatching work.
func (s *Service) SetOrchestraPlan(port OrchestraPlanPort) {
	if s != nil {
		s.plan = port
	}
}

// ErrOrchestraPlanNotConfigured is returned when the plan port is absent.
var ErrOrchestraPlanNotConfigured = errors.New("orchestra plan not configured")

// ProposeOrchestraPlan evaluates a user intent and returns an Orchestra plan
// proposal. Low-risk single-step plans may be auto-dispatched; larger or
// higher-risk plans are returned as proposals awaiting confirmation.
func (s *Service) ProposeOrchestraPlan(ctx context.Context, req OrchestraPlanProposalRequest) (OrchestraPlanProposal, error) {
	if s == nil || s.plan == nil {
		return OrchestraPlanProposal{}, ErrOrchestraPlanNotConfigured
	}
	return s.plan.ProposePlan(ctx, req)
}

// ConfirmOrchestraPlan accepts a proposed plan and dispatches its tasks.
func (s *Service) ConfirmOrchestraPlan(ctx context.Context, req OrchestraPlanConfirmRequest) (OrchestraPlanConfirmResult, error) {
	if s == nil || s.plan == nil {
		return OrchestraPlanConfirmResult{}, ErrOrchestraPlanNotConfigured
	}
	return s.plan.ConfirmPlan(ctx, req)
}

// RejectOrchestraPlan declines a proposed plan without creating tasks.
func (s *Service) RejectOrchestraPlan(ctx context.Context, req OrchestraPlanRejectRequest) error {
	if s == nil || s.plan == nil {
		return ErrOrchestraPlanNotConfigured
	}
	return s.plan.RejectPlan(ctx, req)
}

// GetOrchestraPlan returns a stored plan proposal by ID.
func (s *Service) GetOrchestraPlan(ctx context.Context, planID string) (OrchestraPlanProposal, error) {
	if s == nil || s.plan == nil {
		return OrchestraPlanProposal{}, ErrOrchestraPlanNotConfigured
	}
	return s.plan.GetPlan(ctx, planID)
}

// GroundRoutingContext generates bounded, metadata-safe retrieval context for routing.
func (s *Service) GroundRoutingContext(ctx context.Context, req KnowledgeContextRequest) (KnowledgeContextBundle, error) {
	if s == nil || s.grounding == nil {
		return KnowledgeContextBundle{
			Items:          []KnowledgeContextItem{},
			Degraded:       true,
			DegradedReason: "grounding_not_configured",
			SchemaVersion:  KnowledgeContextSchemaVersion,
		}, nil
	}
	return s.grounding.GroundRoutingContext(ctx, req)
}
