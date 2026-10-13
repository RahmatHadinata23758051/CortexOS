package application

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/RahmatHadinata23758051/CortexOS/internal/cortex/orchestra"
	"github.com/RahmatHadinata23758051/CortexOS/internal/cortex/staff"
)

const BrainOrchestraBridgeSchemaVersion = "cortexos.brain-orchestra.bridge.v1"

var ErrBrainOrchestraInvalidRequest = errors.New("invalid brain-orchestra routing request")

// BrainOrchestraPort is the application boundary for converting a Brain-produced
// multi-step intent into Orchestra-owned tasks. The Brain output remains advisory;
// Orchestra remains authoritative for task graph state, assignment, execution,
// evidence, and completion. This port exposes no provider keys, process handles,
// raw provider output, absolute filesystem paths, or Harness broker details.
type BrainOrchestraPort interface {
	// RouteBrainToOrchestra validates a bounded routing request, asks the Brain
	// for a structured plan, validates the plan, and submits Orchestra task creation
	// through the OrchestraPort. All authoritative transitions occur inside Orchestra.
	// The returned result contains only correlation IDs and sanitized status.
	RouteBrainToOrchestra(ctx context.Context, req BrainOrchestraRouteRequest) (BrainOrchestraRouteResult, error)
}

// BrainOrchestraRouteRequest carries a bounded routing intent from the UI/bridge
// into the application layer. All fields are safe domain values; no secrets,
// process handles, absolute paths, or raw provider payloads.
type BrainOrchestraRouteRequest struct {
	// CorrelationID is a caller-supplied idempotency key to prevent duplicate
	// authoritative task creation. Required; must be non-empty.
	CorrelationID string `json:"correlationId"`

	// UserPrompt is the raw user instruction that initiated the routing.
	UserPrompt string `json:"userPrompt"`

	// StaffID hints which Staff role should own the top-level task. Optional.
	StaffID string `json:"staffId,omitempty"`

	// Title is a short human-readable title for the top-level task. Required.
	Title string `json:"title"`

	// AcceptanceCriteria are the pass/fail checks for the top-level task.
	// At least one is required. Max 20 items.
	AcceptanceCriteria []string `json:"acceptanceCriteria"`

	// Dependencies are Orchestra TaskIDs that must succeed before this routing
	// creates the new task. Optional; max 10 items.
	Dependencies []orchestra.TaskID `json:"dependencies,omitempty"`

	// Priority for scheduling. Optional.
	Priority staff.AssignmentPriority `json:"priority,omitempty"`

	// MaxPlanSteps bounds the number of steps the Brain may propose in the plan.
	// Optional; default 8; max 20.
	MaxPlanSteps int `json:"maxPlanSteps,omitempty"`

	// MaxPlanTokens bounds the total token budget for the Brain planning call.
	// Optional; default 4096; max 32768.
	MaxPlanTokens int `json:"maxPlanTokens,omitempty"`

	// KnowledgeBudget optionally bounds retrieval grounding for the routing prompt.
	// Omitted budgets use safe application defaults.
	KnowledgeBudget *KnowledgeContextBudget `json:"knowledgeBudget,omitempty"`

	// SchemaVersion identifies the bridge contract version.
	SchemaVersion string `json:"schemaVersion"`
}

// Validate performs strict validation and returns a sanitized error code
// suitable for the bridge layer. It never returns raw provider errors,
// stack traces, or filesystem paths.
func (req BrainOrchestraRouteRequest) Validate() error {
	if req.SchemaVersion != BrainOrchestraBridgeSchemaVersion {
		return fmt.Errorf("%w: unsupported schema version %q", ErrBrainOrchestraInvalidRequest, req.SchemaVersion)
	}
	if strings.TrimSpace(req.CorrelationID) == "" {
		return fmt.Errorf("%w: correlationId is required", ErrBrainOrchestraInvalidRequest)
	}
	if strings.TrimSpace(req.UserPrompt) == "" {
		return fmt.Errorf("%w: userPrompt is required", ErrBrainOrchestraInvalidRequest)
	}
	if strings.TrimSpace(req.Title) == "" {
		return fmt.Errorf("%w: title is required", ErrBrainOrchestraInvalidRequest)
	}
	if len(req.AcceptanceCriteria) == 0 {
		return fmt.Errorf("%w: at least one acceptance criterion is required", ErrBrainOrchestraInvalidRequest)
	}
	if len(req.AcceptanceCriteria) > 20 {
		return fmt.Errorf("%w: acceptanceCriteria exceeds maximum of 20", ErrBrainOrchestraInvalidRequest)
	}
	for i, ac := range req.AcceptanceCriteria {
		if strings.TrimSpace(ac) == "" {
			return fmt.Errorf("%w: acceptanceCriteria[%d] must not be empty", ErrBrainOrchestraInvalidRequest, i)
		}
	}
	if len(req.Dependencies) > 10 {
		return fmt.Errorf("%w: dependencies exceeds maximum of 10", ErrBrainOrchestraInvalidRequest)
	}
	if req.MaxPlanSteps > 20 {
		return fmt.Errorf("%w: maxPlanSteps exceeds maximum of 20", ErrBrainOrchestraInvalidRequest)
	}
	if req.MaxPlanTokens > 32768 {
		return fmt.Errorf("%w: maxPlanTokens exceeds maximum of 32768", ErrBrainOrchestraInvalidRequest)
	}
	if req.KnowledgeBudget != nil {
		if err := req.KnowledgeBudget.Validate(); err != nil {
			return fmt.Errorf("%w: %v", ErrBrainOrchestraInvalidRequest, err)
		}
	}
	return nil
}

// WithDefaults returns a request with safe defaults applied. It does not mutate the input.
func (req BrainOrchestraRouteRequest) WithDefaults() BrainOrchestraRouteRequest {
	if req.MaxPlanSteps <= 0 {
		req.MaxPlanSteps = 8
	}
	if req.MaxPlanTokens <= 0 {
		req.MaxPlanTokens = 4096
	}
	if req.KnowledgeBudget != nil {
		budget := req.KnowledgeBudget.WithDefaults()
		req.KnowledgeBudget = &budget
	}
	return req
}

// BrainOrchestraRouteResult reports the outcome of the routing operation.
// It contains only correlation IDs and stable status codes; no provider output,
// process details, raw errors, or filesystem paths.
type BrainOrchestraRouteResult struct {
	CorrelationID   string               `json:"correlationId"`
	OrchestraTaskID string               `json:"orchestraTaskId,omitempty"`
	Status          string               `json:"status"` // accepted | rejected | queued | canceled
	Message         string               `json:"message,omitempty"`
	GroundedContext []GroundedContextRef `json:"groundedContext,omitempty"`
	SchemaVersion   string               `json:"schemaVersion"`
	SubmittedAt     time.Time            `json:"submittedAt"`
}

// BrainOrchestraPlan is the bounded structured plan returned by the Brain planning call.
// It is strictly validated before any Orchestra submission. No provider-specific fields.
type BrainOrchestraPlan struct {
	// Steps is the ordered list of intended steps. Each step maps to a proposed Orchestra task.
	Steps []BrainOrchestraPlanStep `json:"steps"`

	// TotalEstimatedTokens is the Brain's estimate for the entire plan.
	TotalEstimatedTokens int `json:"totalEstimatedTokens"`

	// SchemaVersion identifies the plan schema for forward compatibility.
	SchemaVersion string `json:"schemaVersion"`
}

const BrainOrchestraPlanSchemaVersion = "cortexos.brain-orchestra.plan.v1"

// BrainOrchestraPlanStep represents a single proposed step/task in the plan.
// All fields are safe domain values.
type BrainOrchestraPlanStep struct {
	// Index is the step order within the plan (0-based).
	Index int `json:"index"`

	// Title is a short human-readable description of the step.
	Title string `json:"title"`

	// Action is the high-level action verb (e.g., "code", "review", "test", "doc").
	Action string `json:"action"`

	// Resource is the target resource identifier (e.g., a file path or module name).
	Resource string `json:"resource"`

	// AcceptanceCriteria for this step. Optional; max 5 per step.
	AcceptanceCriteria []string `json:"acceptanceCriteria,omitempty"`

	// AssignedStaffID hints which Staff should execute this step. Optional.
	AssignedStaffID string `json:"assignedStaffId,omitempty"`

	// Dependencies references other step indices that must complete first.
	// Uses step indices, not Orchestra TaskIDs, since tasks don't exist yet.
	Dependencies []int `json:"dependencies,omitempty"`
}

// Validate validates the plan structure and bounds.
// Returns a sanitized error with a stable code.
func (plan BrainOrchestraPlan) Validate() error {
	if plan.SchemaVersion != BrainOrchestraPlanSchemaVersion {
		return fmt.Errorf("%w: unsupported plan schema version %q", ErrBrainOrchestraInvalidRequest, plan.SchemaVersion)
	}
	if len(plan.Steps) == 0 {
		return fmt.Errorf("%w: plan must contain at least one step", ErrBrainOrchestraInvalidRequest)
	}
	if len(plan.Steps) > 20 {
		return fmt.Errorf("%w: plan exceeds maximum of 20 steps", ErrBrainOrchestraInvalidRequest)
	}
	if plan.TotalEstimatedTokens > 32768 {
		return fmt.Errorf("%w: totalEstimatedTokens exceeds maximum of 32768", ErrBrainOrchestraInvalidRequest)
	}

	seenIndex := make(map[int]bool, len(plan.Steps))
	for i, step := range plan.Steps {
		if step.Index != i {
			return fmt.Errorf("%w: step index mismatch at position %d: expected %d, got %d", ErrBrainOrchestraInvalidRequest, i, i, step.Index)
		}
		if strings.TrimSpace(step.Title) == "" {
			return fmt.Errorf("%w: step %d title is required", ErrBrainOrchestraInvalidRequest, i)
		}
		if strings.TrimSpace(step.Action) == "" {
			return fmt.Errorf("%w: step %d action is required", ErrBrainOrchestraInvalidRequest, i)
		}
		if strings.TrimSpace(step.Resource) == "" {
			return fmt.Errorf("%w: step %d resource is required", ErrBrainOrchestraInvalidRequest, i)
		}
		if len(step.AcceptanceCriteria) > 5 {
			return fmt.Errorf("%w: step %d acceptanceCriteria exceeds maximum of 5", ErrBrainOrchestraInvalidRequest, i)
		}
		for j, ac := range step.AcceptanceCriteria {
			if strings.TrimSpace(ac) == "" {
				return fmt.Errorf("%w: step %d acceptanceCriteria[%d] must not be empty", ErrBrainOrchestraInvalidRequest, i, j)
			}
		}
		if len(step.Dependencies) > 5 {
			return fmt.Errorf("%w: step %d dependencies exceeds maximum of 5", ErrBrainOrchestraInvalidRequest, i)
		}
		for _, dep := range step.Dependencies {
			if dep < 0 || dep >= len(plan.Steps) {
				return fmt.Errorf("%w: step %d dependency index %d out of range", ErrBrainOrchestraInvalidRequest, i, dep)
			}
			if dep >= i {
				return fmt.Errorf("%w: step %d dependency %d must reference an earlier step", ErrBrainOrchestraInvalidRequest, i, dep)
			}
		}
		if seenIndex[step.Index] {
			return fmt.Errorf("%w: duplicate step index %d", ErrBrainOrchestraInvalidRequest, step.Index)
		}
		seenIndex[step.Index] = true
	}
	return nil
}
