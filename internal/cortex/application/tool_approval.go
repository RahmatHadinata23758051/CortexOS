package application

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"
)

const (
	ToolProposalSchemaVersion = "cortexos.tool-approval.proposal.v1"
	ToolDecisionSchemaVersion = "cortexos.tool-approval.decision.v1"
)

var ErrToolApprovalInvalidRequest = errors.New("invalid tool approval request")
var ErrProposalNotFound = errors.New("tool proposal not found")
var ErrProposalAlreadyDecided = errors.New("tool proposal already decided")

// ToolApprovalStatus represents the lifecycle state of a tool proposal.
type ToolApprovalStatus string

const (
	ToolApprovalPending  ToolApprovalStatus = "pending"
	ToolApprovalApproved ToolApprovalStatus = "approved"
	ToolApprovalDenied   ToolApprovalStatus = "denied"
	ToolApprovalExecuted ToolApprovalStatus = "executed"
	ToolApprovalFailed   ToolApprovalStatus = "failed"
	ToolApprovalCanceled ToolApprovalStatus = "canceled"
)

// ToolApprovalPort is the application boundary for governed tool proposals
// and operator approval decisions. The Brain can propose tools, but execution
// authority remains strictly behind the approval gate and Harness policy.
// This port exposes no provider keys, process handles, raw provider output,
// absolute paths, or unredacted tool arguments.
type ToolApprovalPort interface {
	SubmitProposal(ctx context.Context, proposal ToolProposal) (ToolProposalResult, error)
	DecideProposal(ctx context.Context, decision ToolApprovalDecision) (ToolApprovalResult, error)
	GetProposal(ctx context.Context, proposalID string) (ToolProposalRecord, error)
	ListPendingProposals(ctx context.Context) ([]ToolProposalRecord, error)
}

// ToolProposal is the bounded, redacted proposal submitted when Brain
// requests a tool invocation requiring operator review.
type ToolProposal struct {
	// ProposalID is a unique opaque identifier for this proposal. Required.
	ProposalID string `json:"proposalId"`

	// CorrelationID links the proposal to the originating Brain request. Required.
	CorrelationID string `json:"correlationId"`

	// TaskID is the associated Orchestra Task ID. Required.
	TaskID string `json:"taskId"`

	// WorktreeID is the isolated worktree identifier. Required.
	WorktreeID string `json:"worktreeId"`

	// ToolName identifies the requested tool (e.g. "file_read", "file_write", "git"). Required.
	ToolName string `json:"toolName"`

	// Capability identifies the Harness capability being invoked. Required.
	Capability string `json:"capability"`

	// RedactedArgsSummary is a human-readable, sanitized explanation of the arguments.
	// Must not leak absolute paths, tokens, keys, or passwords. Max 1000 chars.
	RedactedArgsSummary string `json:"redactedArgsSummary"`

	// ArgByteSize is the total byte size of the raw input. Max 65536 (64 KB).
	ArgByteSize int `json:"argByteSize"`

	// ProposedAt records when the proposal was generated.
	ProposedAt time.Time `json:"proposedAt"`

	// SchemaVersion identifies the proposal contract version.
	SchemaVersion string `json:"schemaVersion"`
}

var (
	rxValidID = regexp.MustCompile(`^[a-zA-Z0-9_-]{1,64}$`)
	rxAbsWin  = regexp.MustCompile(`(?i)[a-zA-Z]:[\\/]`)
	rxAbsUnix = regexp.MustCompile(`(?:^|[\s"'])(\/(?:etc|var|home|Users|usr|bin|tmp|opt|private)[\/\w.-]*)`)
)

// Validate enforces bounds, required fields, and sanitization on ToolProposal.
func (p ToolProposal) Validate() error {
	if p.SchemaVersion != ToolProposalSchemaVersion {
		return fmt.Errorf("%w: unsupported proposal schema version %q", ErrToolApprovalInvalidRequest, p.SchemaVersion)
	}
	if !rxValidID.MatchString(p.ProposalID) {
		return fmt.Errorf("%w: invalid proposalId %q", ErrToolApprovalInvalidRequest, p.ProposalID)
	}
	if !rxValidID.MatchString(p.CorrelationID) {
		return fmt.Errorf("%w: invalid correlationId %q", ErrToolApprovalInvalidRequest, p.CorrelationID)
	}
	if strings.TrimSpace(p.TaskID) == "" || len(p.TaskID) > 64 {
		return fmt.Errorf("%w: invalid taskId", ErrToolApprovalInvalidRequest)
	}
	if strings.TrimSpace(p.WorktreeID) == "" || len(p.WorktreeID) > 64 {
		return fmt.Errorf("%w: invalid worktreeId", ErrToolApprovalInvalidRequest)
	}
	if !rxValidID.MatchString(p.ToolName) {
		return fmt.Errorf("%w: invalid toolName %q", ErrToolApprovalInvalidRequest, p.ToolName)
	}
	if strings.TrimSpace(p.Capability) == "" || len(p.Capability) > 64 {
		return fmt.Errorf("%w: invalid capability", ErrToolApprovalInvalidRequest)
	}
	if len(p.RedactedArgsSummary) > 1000 {
		return fmt.Errorf("%w: redactedArgsSummary exceeds maximum length of 1000 chars", ErrToolApprovalInvalidRequest)
	}
	if p.ArgByteSize < 0 || p.ArgByteSize > 65536 {
		return fmt.Errorf("%w: argByteSize must be between 0 and 65536 bytes", ErrToolApprovalInvalidRequest)
	}
	if err := checkSafeText(p.RedactedArgsSummary); err != nil {
		return fmt.Errorf("%w in redactedArgsSummary: %v", ErrToolApprovalInvalidRequest, err)
	}
	return nil
}

// ToolProposalResult is the outcome of submitting a proposal.
type ToolProposalResult struct {
	ProposalID    string             `json:"proposalId"`
	Status        ToolApprovalStatus `json:"status"`
	SubmittedAt   time.Time          `json:"submittedAt"`
	SchemaVersion string             `json:"schemaVersion"`
}

// ToolExecutionPort is the application boundary for executing an approved tool
// through the governed Harness path. Implementations must only dispatch
// allowlisted tools, enforce capability checks, and return sanitized evidence
// identifiers. Brain and the approval adapter never receive raw provider output,
// process handles, or filesystem paths through this port.
type ToolExecutionPort interface {
	// ExecuteApprovedTool runs a previously approved tool proposal. The caller
	// must have already applied the approval decision. Implementations must fail
	// closed on unknown tools, capability mismatch, timeout, and cancellation.
	ExecuteApprovedTool(ctx context.Context, req ToolExecutionRequest) (ToolExecutionResult, error)
}

// ToolExecutionRequest is a bounded, path-free request to execute an approved tool.
type ToolExecutionRequest struct {
	ProposalID    string `json:"proposalId"`
	CorrelationID string `json:"correlationId"`
	TaskID        string `json:"taskId"`
	WorktreeID    string `json:"worktreeId"`
	ToolName      string `json:"toolName"`
	Capability    string `json:"capability"`
	Approver      string `json:"approver"`
	SchemaVersion string `json:"schemaVersion"`
}

// Validate checks required fields for a governed execution request.
func (r ToolExecutionRequest) Validate() error {
	if r.SchemaVersion != ToolProposalSchemaVersion {
		return fmt.Errorf("%w: unsupported execution schema version %q", ErrToolApprovalInvalidRequest, r.SchemaVersion)
	}
	if !rxValidID.MatchString(r.ProposalID) {
		return fmt.Errorf("%w: invalid proposalId %q", ErrToolApprovalInvalidRequest, r.ProposalID)
	}
	if strings.TrimSpace(r.TaskID) == "" || len(r.TaskID) > 64 {
		return fmt.Errorf("%w: invalid taskId", ErrToolApprovalInvalidRequest)
	}
	if strings.TrimSpace(r.WorktreeID) == "" || len(r.WorktreeID) > 64 {
		return fmt.Errorf("%w: invalid worktreeId", ErrToolApprovalInvalidRequest)
	}
	if !rxValidID.MatchString(r.ToolName) {
		return fmt.Errorf("%w: invalid toolName %q", ErrToolApprovalInvalidRequest, r.ToolName)
	}
	if strings.TrimSpace(r.Capability) == "" || len(r.Capability) > 64 {
		return fmt.Errorf("%w: invalid capability", ErrToolApprovalInvalidRequest)
	}
	return nil
}

// ToolExecutionResult is the sanitized outcome of a governed tool execution.
// Harness output remains untrusted evidence; only stable IDs and statuses cross
// this boundary.
type ToolExecutionResult struct {
	ExecutionID   string             `json:"executionId"`
	Status        ToolApprovalStatus `json:"status"`
	EvidenceIDs   []string           `json:"evidenceIds,omitempty"`
	Message       string             `json:"message,omitempty"`
	SchemaVersion string             `json:"schemaVersion"`
}

// ToolApprovalDecision conveys the operator's decision for a pending proposal.
type ToolApprovalDecision struct {
	// ProposalID identifies the proposal being decided. Required.
	ProposalID string `json:"proposalId"`

	// Decision must be either "approve" or "deny".
	Decision string `json:"decision"`

	// Actor identifies the operator who made the decision. Required, max 128 chars.
	Actor string `json:"actor"`

	// Reason explains the decision. Optional, max 500 chars.
	Reason string `json:"reason,omitempty"`

	// IdempotencyKey prevents duplicate processing of the same decision. Required.
	IdempotencyKey string `json:"idempotencyKey"`

	// DecidedAt records when the decision was made.
	DecidedAt time.Time `json:"decidedAt"`

	// SchemaVersion identifies the decision contract version.
	SchemaVersion string `json:"schemaVersion"`
}

// Validate checks bounds and values for ToolApprovalDecision.
func (d ToolApprovalDecision) Validate() error {
	if d.SchemaVersion != ToolDecisionSchemaVersion {
		return fmt.Errorf("%w: unsupported decision schema version %q", ErrToolApprovalInvalidRequest, d.SchemaVersion)
	}
	if !rxValidID.MatchString(d.ProposalID) {
		return fmt.Errorf("%w: invalid proposalId %q", ErrToolApprovalInvalidRequest, d.ProposalID)
	}
	if d.Decision != "approve" && d.Decision != "deny" {
		return fmt.Errorf("%w: decision must be 'approve' or 'deny', got %q", ErrToolApprovalInvalidRequest, d.Decision)
	}
	if strings.TrimSpace(d.Actor) == "" || len(d.Actor) > 128 {
		return fmt.Errorf("%w: actor is required and must not exceed 128 chars", ErrToolApprovalInvalidRequest)
	}
	if len(d.Reason) > 500 {
		return fmt.Errorf("%w: reason exceeds maximum length of 500 chars", ErrToolApprovalInvalidRequest)
	}
	if strings.TrimSpace(d.IdempotencyKey) == "" || len(d.IdempotencyKey) > 64 {
		return fmt.Errorf("%w: idempotencyKey is required and must not exceed 64 chars", ErrToolApprovalInvalidRequest)
	}
	if err := checkSafeText(d.Reason); err != nil {
		return fmt.Errorf("%w in reason: %v", ErrToolApprovalInvalidRequest, err)
	}
	return nil
}

// ToolApprovalResult conveys the result of applying an approval decision.
type ToolApprovalResult struct {
	ProposalID    string             `json:"proposalId"`
	Status        ToolApprovalStatus `json:"status"`
	Actor         string             `json:"actor"`
	DecidedAt     time.Time          `json:"decidedAt"`
	SchemaVersion string             `json:"schemaVersion"`
}

// ToolProposalRecord holds the persisted state of a proposal for query inspection.
type ToolProposalRecord struct {
	ProposalID          string             `json:"proposalId"`
	CorrelationID       string             `json:"correlationId"`
	TaskID              string             `json:"taskId"`
	WorktreeID          string             `json:"worktreeId"`
	ToolName            string             `json:"toolName"`
	Capability          string             `json:"capability"`
	RedactedArgsSummary string             `json:"redactedArgsSummary"`
	ArgByteSize         int                `json:"argByteSize"`
	Status              ToolApprovalStatus `json:"status"`
	Actor               string             `json:"actor,omitempty"`
	Reason              string             `json:"reason,omitempty"`
	ProposedAt          time.Time          `json:"proposedAt"`
	DecidedAt           *time.Time         `json:"decidedAt,omitempty"`
	SchemaVersion       string             `json:"schemaVersion"`
}

// checkSafeText rejects control characters, secret patterns, and absolute paths.
func checkSafeText(text string) error {
	if text == "" {
		return nil
	}
	if strings.ContainsAny(text, "\x00\r") {
		return errors.New("contains forbidden control characters")
	}
	lower := strings.ToLower(text)
	forbiddenTokens := []string{
		"sk-",
		"bearer ",
		"authorization:",
		"-----begin private key-----",
		"-----begin rsa private key-----",
		"ghp_",
		"cortex_key_",
		"password=",
		"secret=",
	}
	for _, tok := range forbiddenTokens {
		if strings.Contains(lower, tok) {
			return errors.New("contains sensitive token or secret marker")
		}
	}
	if rxAbsWin.MatchString(text) || rxAbsUnix.MatchString(text) {
		return errors.New("contains absolute filesystem path")
	}
	return nil
}
