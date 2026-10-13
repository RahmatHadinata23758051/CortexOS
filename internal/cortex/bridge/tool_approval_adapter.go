// Tool approval gate adapter.
//
// This adapter implements application.ToolApprovalPort. It stores bounded,
// redacted tool proposals and applies operator approval decisions with
// compare-and-set semantics. Approved proposals are dispatched only through an
// application.ToolExecutionPort (the governed Harness path); the adapter never
// executes tools itself and never receives raw provider output, process
// handles, or absolute filesystem paths.
package bridge

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/RahmatHadinata23758051/CortexOS/internal/cortex/application"
)

// ToolApprovalAdapterConfig configures the approval gate.
type ToolApprovalAdapterConfig struct {
	// Allowlist maps a tool name to the single Harness capability it may invoke.
	// A tool not present in the allowlist is rejected at proposal time. Required.
	Allowlist map[string]string
	// Executor is the governed Harness execution path. Required.
	Executor application.ToolExecutionPort
	// Now optionally injects a clock; defaults to time.Now.
	Now func() time.Time
}

// ToolApprovalAdapter is a thread-safe, in-process approval gate.
type ToolApprovalAdapter struct {
	allowlist map[string]string
	executor  application.ToolExecutionPort
	now       func() time.Time

	mu        sync.Mutex
	proposals map[string]*application.ToolProposalRecord
	decisions map[string]application.ToolApprovalResult
}

// NewToolApprovalAdapter constructs the approval gate and validates its config.
func NewToolApprovalAdapter(cfg ToolApprovalAdapterConfig) (application.ToolApprovalPort, error) {
	if len(cfg.Allowlist) == 0 {
		return nil, errors.New("tool allowlist must not be empty")
	}
	if cfg.Executor == nil {
		return nil, errors.New("tool executor cannot be nil")
	}
	allow := make(map[string]string, len(cfg.Allowlist))
	for tool, cap := range cfg.Allowlist {
		tool = strings.TrimSpace(tool)
		cap = strings.TrimSpace(cap)
		if tool == "" || cap == "" {
			return nil, errors.New("tool allowlist entries must have non-empty tool and capability")
		}
		allow[tool] = cap
	}
	now := cfg.Now
	if now == nil {
		now = time.Now
	}
	return &ToolApprovalAdapter{
		allowlist: allow,
		executor:  cfg.Executor,
		now:       now,
		proposals: make(map[string]*application.ToolProposalRecord),
		decisions: make(map[string]application.ToolApprovalResult),
	}, nil
}

// SubmitProposal validates and stores a pending proposal. Unknown tools,
// capability mismatches, and duplicate proposal IDs fail closed (the latter
// returns the existing record without mutating it).
func (a *ToolApprovalAdapter) SubmitProposal(ctx context.Context, proposal application.ToolProposal) (application.ToolProposalResult, error) {
	if err := ctx.Err(); err != nil {
		return application.ToolProposalResult{}, err
	}
	if err := proposal.Validate(); err != nil {
		return application.ToolProposalResult{}, err
	}

	expectedCap, ok := a.allowlist[proposal.ToolName]
	if !ok {
		return application.ToolProposalResult{}, fmt.Errorf("%w: tool %q is not allowlisted", application.ErrToolApprovalInvalidRequest, proposal.ToolName)
	}
	if proposal.Capability != expectedCap {
		return application.ToolProposalResult{}, fmt.Errorf("%w: capability %q does not match allowlisted capability for tool %q", application.ErrToolApprovalInvalidRequest, proposal.Capability, proposal.ToolName)
	}

	a.mu.Lock()
	defer a.mu.Unlock()
	if existing, exists := a.proposals[proposal.ProposalID]; exists {
		return application.ToolProposalResult{
			ProposalID:    existing.ProposalID,
			Status:        existing.Status,
			SubmittedAt:   existing.ProposedAt,
			SchemaVersion: application.ToolProposalSchemaVersion,
		}, nil
	}
	proposedAt := proposal.ProposedAt
	if proposedAt.IsZero() {
		proposedAt = a.now().UTC()
	}
	a.proposals[proposal.ProposalID] = &application.ToolProposalRecord{
		ProposalID:          proposal.ProposalID,
		CorrelationID:       proposal.CorrelationID,
		TaskID:              proposal.TaskID,
		WorktreeID:          proposal.WorktreeID,
		ToolName:            proposal.ToolName,
		Capability:          proposal.Capability,
		RedactedArgsSummary: proposal.RedactedArgsSummary,
		ArgByteSize:         proposal.ArgByteSize,
		Status:              application.ToolApprovalPending,
		ProposedAt:          proposedAt,
		SchemaVersion:       application.ToolProposalSchemaVersion,
	}
	return application.ToolProposalResult{
		ProposalID:    proposal.ProposalID,
		Status:        application.ToolApprovalPending,
		SubmittedAt:   proposedAt,
		SchemaVersion: application.ToolProposalSchemaVersion,
	}, nil
}

// DecideProposal applies an operator decision. Decisions are idempotent by
// idempotency key. Approval triggers governed execution through the executor;
// denial and all failure paths fail closed without executing anything.
func (a *ToolApprovalAdapter) DecideProposal(ctx context.Context, decision application.ToolApprovalDecision) (application.ToolApprovalResult, error) {
	if err := decision.Validate(); err != nil {
		return application.ToolApprovalResult{}, err
	}

	idemKey := decision.ProposalID + ":" + decision.IdempotencyKey
	a.mu.Lock()
	if cached, ok := a.decisions[idemKey]; ok {
		a.mu.Unlock()
		return cached, nil
	}
	record, exists := a.proposals[decision.ProposalID]
	if !exists {
		a.mu.Unlock()
		return application.ToolApprovalResult{}, fmt.Errorf("%w: %s", application.ErrProposalNotFound, decision.ProposalID)
	}
	if record.Status != application.ToolApprovalPending {
		a.mu.Unlock()
		return application.ToolApprovalResult{}, fmt.Errorf("%w: %s", application.ErrProposalAlreadyDecided, decision.ProposalID)
	}
	decidedAt := decision.DecidedAt
	if decidedAt.IsZero() {
		decidedAt = a.now().UTC()
	}
	// Reserve the proposal with a terminal decision before releasing the lock so
	// concurrent decisions cannot both proceed.
	if decision.Decision == "deny" {
		record.Status = application.ToolApprovalDenied
		record.Actor = decision.Actor
		record.Reason = decision.Reason
		record.DecidedAt = &decidedAt
		result := application.ToolApprovalResult{
			ProposalID:    record.ProposalID,
			Status:        application.ToolApprovalDenied,
			Actor:         decision.Actor,
			DecidedAt:     decidedAt,
			SchemaVersion: application.ToolDecisionSchemaVersion,
		}
		a.decisions[idemKey] = result
		a.mu.Unlock()
		return result, nil
	}
	record.Status = application.ToolApprovalApproved
	record.Actor = decision.Actor
	record.Reason = decision.Reason
	record.DecidedAt = &decidedAt
	execReq := application.ToolExecutionRequest{
		ProposalID:    record.ProposalID,
		CorrelationID: record.CorrelationID,
		TaskID:        record.TaskID,
		WorktreeID:    record.WorktreeID,
		ToolName:      record.ToolName,
		Capability:    record.Capability,
		Approver:      decision.Actor,
		SchemaVersion: application.ToolProposalSchemaVersion,
	}
	a.mu.Unlock()

	execResult, err := a.executor.ExecuteApprovedTool(ctx, execReq)
	status := application.ToolApprovalExecuted
	if err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			status = application.ToolApprovalCanceled
		} else {
			status = application.ToolApprovalFailed
		}
	}

	a.mu.Lock()
	record.Status = status
	result := application.ToolApprovalResult{
		ProposalID:    record.ProposalID,
		Status:        status,
		Actor:         decision.Actor,
		DecidedAt:     decidedAt,
		SchemaVersion: application.ToolDecisionSchemaVersion,
	}
	a.decisions[idemKey] = result
	a.mu.Unlock()

	if err != nil {
		return result, sanitizeExecutionError(err)
	}
	_ = execResult
	return result, nil
}

// GetProposal returns a copy of a stored proposal record.
func (a *ToolApprovalAdapter) GetProposal(ctx context.Context, proposalID string) (application.ToolProposalRecord, error) {
	if err := ctx.Err(); err != nil {
		return application.ToolProposalRecord{}, err
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	record, ok := a.proposals[proposalID]
	if !ok {
		return application.ToolProposalRecord{}, fmt.Errorf("%w: %s", application.ErrProposalNotFound, proposalID)
	}
	return *record, nil
}

// ListPendingProposals returns pending proposals sorted by proposal ID.
func (a *ToolApprovalAdapter) ListPendingProposals(ctx context.Context) ([]application.ToolProposalRecord, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	result := make([]application.ToolProposalRecord, 0, len(a.proposals))
	for _, record := range a.proposals {
		if record.Status == application.ToolApprovalPending {
			result = append(result, *record)
		}
	}
	sort.Slice(result, func(i, j int) bool { return result[i].ProposalID < result[j].ProposalID })
	return result, nil
}

// sanitizeExecutionError maps arbitrary executor errors to a stable, safe error
// that never leaks provider output, process handles, or filesystem paths.
func sanitizeExecutionError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return err
	}
	return fmt.Errorf("%w: governed tool execution failed", application.ErrToolApprovalInvalidRequest)
}
