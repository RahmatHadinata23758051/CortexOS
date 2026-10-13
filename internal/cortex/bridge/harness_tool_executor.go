// Harness-governed tool executor.
//
// This adapter implements application.ToolExecutionPort by dispatching approved
// tool calls through the Harness ToolBroker. It is the only path from an
// approved approval-gate proposal to actual tool execution. The adapter passes
// no raw provider output, process handles, or filesystem paths across the
// application boundary: it forwards opaque IDs and returns sanitized evidence
// identifiers only.
package bridge

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/RahmatHadinata23758051/CortexOS/internal/cortex/application"
	"github.com/RahmatHadinata23758051/CortexOS/internal/cortex/harness"
)

// HarnessToolExecutorConfig configures the governed executor.
type HarnessToolExecutorConfig struct {
	// Broker is the mandatory Harness execution gate. Required.
	Broker harness.ToolBroker
	// DefaultTimeout bounds a single tool execution. Optional; default 60s.
	DefaultTimeout time.Duration
	// Now optionally injects a clock; defaults to time.Now.
	Now func() time.Time
}

// HarnessToolExecutor dispatches approved tool calls through the Harness broker.
type HarnessToolExecutor struct {
	broker         harness.ToolBroker
	defaultTimeout time.Duration
	now            func() time.Time
}

// NewHarnessToolExecutor constructs the executor and validates its config.
func NewHarnessToolExecutor(cfg HarnessToolExecutorConfig) (application.ToolExecutionPort, error) {
	if cfg.Broker == nil {
		return nil, errors.New("harness broker cannot be nil")
	}
	timeout := cfg.DefaultTimeout
	if timeout <= 0 {
		timeout = 60 * time.Second
	}
	now := cfg.Now
	if now == nil {
		now = time.Now
	}
	return &HarnessToolExecutor{broker: cfg.Broker, defaultTimeout: timeout, now: now}, nil
}

// ExecuteApprovedTool runs a single approved tool through the Harness broker.
// It fails closed on validation errors, unknown tools, policy denial, timeout,
// and cancellation, and never returns raw provider output or filesystem paths.
func (e *HarnessToolExecutor) ExecuteApprovedTool(ctx context.Context, req application.ToolExecutionRequest) (application.ToolExecutionResult, error) {
	if err := req.Validate(); err != nil {
		return application.ToolExecutionResult{}, err
	}
	if err := ctx.Err(); err != nil {
		return application.ToolExecutionResult{}, err
	}

	execID := executionIDFor(req.ProposalID)
	toolReq := harness.ToolRequest{
		ContractVersion: harness.HarnessContractVersion,
		ExecutionID:     execID,
		TaskID:          req.TaskID,
		WorktreeID:      req.WorktreeID,
		ToolName:        req.ToolName,
		Action:          req.Capability,
		Timeout:         e.defaultTimeout,
		Audit: harness.AuditMetadata{
			Actor:         req.Approver,
			CorrelationID: req.CorrelationID,
		},
	}
	if req.Approver != "" {
		toolReq.ExecutionID = execID
	}

	approval := harness.Approval{
		ExecutionID: execID,
		ToolName:    req.ToolName,
		Action:      req.Capability,
		Approver:    req.Approver,
		GrantedAt:   e.now().UTC(),
	}
	ctx = harness.WithApproval(ctx, approval)

	result, err := e.broker.Execute(harness.NewExecutionContext(ctx, req.CorrelationID), toolReq)
	if err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return application.ToolExecutionResult{
				ExecutionID:   execID,
				Status:        application.ToolApprovalCanceled,
				SchemaVersion: application.ToolProposalSchemaVersion,
			}, err
		}
		return application.ToolExecutionResult{
			ExecutionID:   execID,
			Status:        application.ToolApprovalFailed,
			Message:       "governed tool execution failed",
			SchemaVersion: application.ToolProposalSchemaVersion,
		}, fmt.Errorf("%w: governed tool execution failed", application.ErrToolApprovalInvalidRequest)
	}

	status := application.ToolApprovalExecuted
	switch result.Status {
	case harness.ToolStatusSuccess:
		status = application.ToolApprovalExecuted
	case harness.ToolStatusCanceled, harness.ToolStatusTimeout:
		status = application.ToolApprovalCanceled
	default:
		status = application.ToolApprovalFailed
	}

	return application.ToolExecutionResult{
		ExecutionID:   execID,
		Status:        status,
		EvidenceIDs:   append([]string(nil), result.EvidenceIDs...),
		SchemaVersion: application.ToolProposalSchemaVersion,
	}, nil
}

// executionIDFor derives a deterministic Harness execution ID from a proposal ID.
func executionIDFor(proposalID string) string {
	return "exec-" + proposalID
}
