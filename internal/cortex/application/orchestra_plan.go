package application

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/RahmatHadinata23758051/CortexOS/internal/cortex/orchestra"
	"github.com/RahmatHadinata23758051/CortexOS/internal/cortex/staff"
)

// OrchestraPlanSchemaVersion is the stable schema identifier for Orchestra task planning.
const OrchestraPlanSchemaVersion = "cortexos.orchestra.plan.v1"

var (
	ErrOrchestraPlanInvalidRequest = errors.New("orchestra plan: invalid request")
	ErrOrchestraPlanNotFound       = errors.New("orchestra plan: not found")
	ErrOrchestraPlanAlreadyDecided = errors.New("orchestra plan: already confirmed or rejected")
)

// OrchestraPlanPort defines the application contract for proposing, reviewing,
// and dispatching Orchestra plans.
type OrchestraPlanPort interface {
	ProposePlan(ctx context.Context, req OrchestraPlanProposalRequest) (OrchestraPlanProposal, error)
	ConfirmPlan(ctx context.Context, req OrchestraPlanConfirmRequest) (OrchestraPlanConfirmResult, error)
	RejectPlan(ctx context.Context, req OrchestraPlanRejectRequest) error
	GetPlan(ctx context.Context, planID string) (OrchestraPlanProposal, error)
}

// OrchestraPlanProposalRequest is the inbound user prompt / intent request to Orchestra.
type OrchestraPlanProposalRequest struct {
	CorrelationID       string                      `json:"correlationId"`
	ProjectID           string                      `json:"projectId"`
	UserPrompt          string                      `json:"userPrompt"`
	Title               string                      `json:"title"`
	Summary             string                      `json:"summary,omitempty"`
	RequireConfirmation *bool                       `json:"requireConfirmation,omitempty"`
	Tasks               []OrchestraPlanProposedTask `json:"tasks"`
	SchemaVersion       string                      `json:"schemaVersion"`
}

// Validate checks bounds and schema version.
func (req OrchestraPlanProposalRequest) Validate() error {
	if req.SchemaVersion != OrchestraPlanSchemaVersion {
		return fmt.Errorf("%w: unsupported schema version %q", ErrOrchestraPlanInvalidRequest, req.SchemaVersion)
	}
	if strings.TrimSpace(req.CorrelationID) == "" {
		return fmt.Errorf("%w: correlationId is required", ErrOrchestraPlanInvalidRequest)
	}
	if strings.TrimSpace(req.UserPrompt) == "" {
		return fmt.Errorf("%w: userPrompt is required", ErrOrchestraPlanInvalidRequest)
	}
	if strings.TrimSpace(req.Title) == "" {
		return fmt.Errorf("%w: title is required", ErrOrchestraPlanInvalidRequest)
	}
	if len(req.Tasks) == 0 {
		return fmt.Errorf("%w: at least one proposed task is required", ErrOrchestraPlanInvalidRequest)
	}
	if len(req.Tasks) > 20 {
		return fmt.Errorf("%w: task count exceeds maximum of 20", ErrOrchestraPlanInvalidRequest)
	}
	for i, task := range req.Tasks {
		if err := task.Validate(i, len(req.Tasks)); err != nil {
			return err
		}
	}
	return nil
}

// OrchestraPlanProposedTask represents an individual task in the proposed plan.
type OrchestraPlanProposedTask struct {
	Index               int      `json:"index"`
	Title               string   `json:"title"`
	Role                string   `json:"role"`
	CallSign            string   `json:"callSign,omitempty"`
	Action              string   `json:"action"`
	Resource            string   `json:"resource,omitempty"`
	AcceptanceCriteria  []string `json:"acceptanceCriteria"`
	EstimatedComplexity string   `json:"estimatedComplexity"` // "trivial" | "standard" | "complex"
	Dependencies        []int    `json:"dependencies"`        // 0-based index of preceding steps
}

// Validate checks proposed task bounds.
func (t OrchestraPlanProposedTask) Validate(pos, total int) error {
	if t.Index != pos {
		return fmt.Errorf("%w: task index mismatch: expected %d, got %d", ErrOrchestraPlanInvalidRequest, pos, t.Index)
	}
	if strings.TrimSpace(t.Title) == "" {
		return fmt.Errorf("%w: task %d title is required", ErrOrchestraPlanInvalidRequest, pos)
	}
	if strings.TrimSpace(t.Role) == "" {
		return fmt.Errorf("%w: task %d role is required", ErrOrchestraPlanInvalidRequest, pos)
	}
	if strings.TrimSpace(t.Action) == "" {
		return fmt.Errorf("%w: task %d action is required", ErrOrchestraPlanInvalidRequest, pos)
	}
	if len(t.AcceptanceCriteria) == 0 {
		return fmt.Errorf("%w: task %d requires at least one acceptance criterion", ErrOrchestraPlanInvalidRequest, pos)
	}
	for _, dep := range t.Dependencies {
		if dep < 0 || dep >= total {
			return fmt.Errorf("%w: task %d dependency %d out of bounds", ErrOrchestraPlanInvalidRequest, pos, dep)
		}
		if dep >= pos {
			return fmt.Errorf("%w: task %d dependency %d must refer to an earlier task", ErrOrchestraPlanInvalidRequest, pos, dep)
		}
	}
	return nil
}

// OrchestraPlanProposal is the evaluated, immutable proposal presented to the user.
type OrchestraPlanProposal struct {
	PlanID               string                      `json:"planId"`
	CorrelationID        string                      `json:"correlationId"`
	ProjectID            string                      `json:"projectId"`
	UserPrompt           string                      `json:"userPrompt"`
	Title                string                      `json:"title"`
	Summary              string                      `json:"summary"`
	RiskLevel            string                      `json:"riskLevel"`            // "low" | "medium" | "high"
	RequiresConfirmation bool                        `json:"requiresConfirmation"` // true = wait for user, false = auto-dispatched
	Status               string                      `json:"status"`               // "proposed" | "confirmed" | "rejected" | "auto_dispatched"
	Tasks                []OrchestraPlanProposedTask `json:"tasks"`
	CreatedAt            time.Time                   `json:"createdAt"`
	ConfirmedAt          *time.Time                  `json:"confirmedAt,omitempty"`
	DispatchedTaskIDs    []string                    `json:"dispatchedTaskIds"`
	SchemaVersion        string                      `json:"schemaVersion"`
}

// OrchestraPlanConfirmRequest is sent when the user accepts a proposed plan.
type OrchestraPlanConfirmRequest struct {
	PlanID        string `json:"planId"`
	SchemaVersion string `json:"schemaVersion"`
}

// Validate checks confirmation request.
func (req OrchestraPlanConfirmRequest) Validate() error {
	if req.SchemaVersion != OrchestraPlanSchemaVersion {
		return fmt.Errorf("%w: unsupported schema version %q", ErrOrchestraPlanInvalidRequest, req.SchemaVersion)
	}
	if strings.TrimSpace(req.PlanID) == "" {
		return fmt.Errorf("%w: planId is required", ErrOrchestraPlanInvalidRequest)
	}
	return nil
}

// OrchestraPlanRejectRequest is sent when the user rejects a proposed plan.
type OrchestraPlanRejectRequest struct {
	PlanID        string `json:"planId"`
	Reason        string `json:"reason,omitempty"`
	SchemaVersion string `json:"schemaVersion"`
}

// Validate checks rejection request.
func (req OrchestraPlanRejectRequest) Validate() error {
	if req.SchemaVersion != OrchestraPlanSchemaVersion {
		return fmt.Errorf("%w: unsupported schema version %q", ErrOrchestraPlanInvalidRequest, req.SchemaVersion)
	}
	if strings.TrimSpace(req.PlanID) == "" {
		return fmt.Errorf("%w: planId is required", ErrOrchestraPlanInvalidRequest)
	}
	return nil
}

// OrchestraPlanConfirmResult reports the outcome of confirming and dispatching a plan.
type OrchestraPlanConfirmResult struct {
	PlanID            string    `json:"planId"`
	Status            string    `json:"status"`
	DispatchedTaskIDs []string  `json:"dispatchedTaskIds"`
	ConfirmedAt       time.Time `json:"confirmedAt"`
	SchemaVersion     string    `json:"schemaVersion"`
}

// OrchestraPlanGetRequest reads a stored plan proposal by ID.
type OrchestraPlanGetRequest struct {
	PlanID        string `json:"planId"`
	SchemaVersion string `json:"schemaVersion"`
}

// Validate checks the plan read request.
func (req OrchestraPlanGetRequest) Validate() error {
	if strings.TrimSpace(req.PlanID) == "" {
		return fmt.Errorf("%w: planId is required", ErrOrchestraPlanInvalidRequest)
	}
	return nil
}

// EvaluatePlanConfirmationPolicy determines whether a plan can be auto-dispatched
// or requires explicit user agreement.
//
// Rule:
//   - Small tasks (single step, trivial/standard complexity, non-destructive action)
//     do NOT require confirmation; they auto-dispatch to maintain high flow speed.
//   - Multi-step tasks (> 1 step), complex tasks, or destructive actions
//     REQUIRE confirmation before dispatching worker cards onto the canvas.
//   - An explicit override (if supplied) takes priority.
func EvaluatePlanConfirmationPolicy(tasks []OrchestraPlanProposedTask, override *bool) (bool, string) {
	if override != nil {
		risk := "standard"
		if *override {
			risk = "medium"
		} else {
			risk = "low"
		}
		return *override, risk
	}

	if len(tasks) == 0 {
		return false, "low"
	}

	// Check if any task is complex or destructive
	hasDestructive := false
	hasComplex := false
	for _, t := range tasks {
		action := strings.ToLower(strings.TrimSpace(t.Action))
		if action == "delete" || action == "destroy" || action == "drop" || action == "force" {
			hasDestructive = true
		}
		complexity := strings.ToLower(strings.TrimSpace(t.EstimatedComplexity))
		if complexity == "complex" || complexity == "high" {
			hasComplex = true
		}
	}

	if hasDestructive {
		return true, "high"
	}
	if len(tasks) > 1 || hasComplex {
		return true, "medium"
	}

	// Single task, standard/trivial complexity, non-destructive: auto-dispatch
	return false, "low"
}

// OrchestraPlanManager is a thread-safe, memory-bounded manager for Orchestra plans.
type OrchestraPlanManager struct {
	mu        sync.RWMutex
	orchestra OrchestraPort
	plans     map[string]OrchestraPlanProposal
	now       func() time.Time
}

// NewOrchestraPlanManager creates a new plan manager.
func NewOrchestraPlanManager(orchestra OrchestraPort) *OrchestraPlanManager {
	return &OrchestraPlanManager{
		orchestra: orchestra,
		plans:     make(map[string]OrchestraPlanProposal),
		now:       time.Now,
	}
}

// ProposePlan evaluates the incoming intent, determines confirmation policy,
// stores the proposal, and auto-dispatches if the policy allows it.
func (m *OrchestraPlanManager) ProposePlan(ctx context.Context, req OrchestraPlanProposalRequest) (OrchestraPlanProposal, error) {
	if err := req.Validate(); err != nil {
		return OrchestraPlanProposal{}, err
	}
	if err := ctx.Err(); err != nil {
		return OrchestraPlanProposal{}, err
	}

	planID := fmt.Sprintf("plan-%s", req.CorrelationID)
	reqConfirmation, riskLevel := EvaluatePlanConfirmationPolicy(req.Tasks, req.RequireConfirmation)

	now := m.now().UTC()
	status := "proposed"
	dispatchedIDs := make([]string, 0)

	// Clean clone of tasks ensuring non-nil slices
	tasksClone := make([]OrchestraPlanProposedTask, len(req.Tasks))
	for i, t := range req.Tasks {
		tasksClone[i] = t
		if tasksClone[i].AcceptanceCriteria == nil {
			tasksClone[i].AcceptanceCriteria = []string{}
		}
		if tasksClone[i].Dependencies == nil {
			tasksClone[i].Dependencies = []int{}
		}
		if tasksClone[i].CallSign == "" {
			tasksClone[i].CallSign = fmt.Sprintf("%s-%d", t.Role, i+1)
		}
	}

	proposal := OrchestraPlanProposal{
		PlanID:               planID,
		CorrelationID:        req.CorrelationID,
		ProjectID:            req.ProjectID,
		UserPrompt:           req.UserPrompt,
		Title:                req.Title,
		Summary:              req.Summary,
		RiskLevel:            riskLevel,
		RequiresConfirmation: reqConfirmation,
		Status:               status,
		Tasks:                tasksClone,
		CreatedAt:            now,
		DispatchedTaskIDs:    dispatchedIDs,
		SchemaVersion:        OrchestraPlanSchemaVersion,
	}

	// Auto-dispatch small tasks immediately
	if !reqConfirmation {
		createdIDs, err := m.dispatchTasks(ctx, proposal)
		if err != nil {
			return OrchestraPlanProposal{}, err
		}
		proposal.Status = "auto_dispatched"
		proposal.DispatchedTaskIDs = createdIDs
		proposal.ConfirmedAt = &now
	}

	m.mu.Lock()
	// ponytail: in-memory bounded cache holds max 100 plan proposals per session. Upgrade path: persist to state/orchestra_plans.db with schema migration when plans must survive process restart.
	if len(m.plans) >= 100 {
		var oldestKey string
		var oldestTime time.Time
		for k, p := range m.plans {
			if oldestKey == "" || p.CreatedAt.Before(oldestTime) {
				oldestKey = k
				oldestTime = p.CreatedAt
			}
		}
		delete(m.plans, oldestKey)
	}
	m.plans[planID] = proposal
	m.mu.Unlock()

	return proposal, nil
}

// ConfirmPlan transitions a proposed plan to confirmed and dispatches its tasks to Orchestra.
func (m *OrchestraPlanManager) ConfirmPlan(ctx context.Context, req OrchestraPlanConfirmRequest) (OrchestraPlanConfirmResult, error) {
	if err := req.Validate(); err != nil {
		return OrchestraPlanConfirmResult{}, err
	}
	if err := ctx.Err(); err != nil {
		return OrchestraPlanConfirmResult{}, err
	}

	m.mu.Lock()
	plan, exists := m.plans[req.PlanID]
	if !exists {
		m.mu.Unlock()
		return OrchestraPlanConfirmResult{}, ErrOrchestraPlanNotFound
	}
	if plan.Status != "proposed" {
		m.mu.Unlock()
		return OrchestraPlanConfirmResult{}, ErrOrchestraPlanAlreadyDecided
	}

	now := m.now().UTC()
	createdIDs, err := m.dispatchTasks(ctx, plan)
	if err != nil {
		m.mu.Unlock()
		return OrchestraPlanConfirmResult{}, err
	}

	plan.Status = "confirmed"
	plan.ConfirmedAt = &now
	plan.DispatchedTaskIDs = createdIDs
	m.plans[req.PlanID] = plan
	m.mu.Unlock()

	return OrchestraPlanConfirmResult{
		PlanID:            plan.PlanID,
		Status:            "confirmed",
		DispatchedTaskIDs: createdIDs,
		ConfirmedAt:       now,
		SchemaVersion:     OrchestraPlanSchemaVersion,
	}, nil
}

// RejectPlan marks a proposed plan as rejected without creating tasks.
func (m *OrchestraPlanManager) RejectPlan(ctx context.Context, req OrchestraPlanRejectRequest) error {
	if err := req.Validate(); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	plan, exists := m.plans[req.PlanID]
	if !exists {
		return ErrOrchestraPlanNotFound
	}
	if plan.Status != "proposed" {
		return ErrOrchestraPlanAlreadyDecided
	}

	plan.Status = "rejected"
	m.plans[req.PlanID] = plan
	return nil
}

// GetPlan returns a plan by ID.
func (m *OrchestraPlanManager) GetPlan(ctx context.Context, planID string) (OrchestraPlanProposal, error) {
	if strings.TrimSpace(planID) == "" {
		return OrchestraPlanProposal{}, fmt.Errorf("%w: planId is required", ErrOrchestraPlanInvalidRequest)
	}
	if err := ctx.Err(); err != nil {
		return OrchestraPlanProposal{}, err
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	plan, exists := m.plans[planID]
	if !exists {
		return OrchestraPlanProposal{}, ErrOrchestraPlanNotFound
	}
	return plan, nil
}

// dispatchTasks translates proposed plan tasks into Orchestra-owned tasks and saves them.
func (m *OrchestraPlanManager) dispatchTasks(ctx context.Context, plan OrchestraPlanProposal) ([]string, error) {
	if m.orchestra == nil {
		return make([]string, 0), nil
	}

	dispatchedIDs := make([]string, len(plan.Tasks))
	for i := range plan.Tasks {
		dispatchedIDs[i] = fmt.Sprintf("%s-task-%d", plan.PlanID, i)
	}

	now := m.now().UTC()
	for i, t := range plan.Tasks {
		taskID := dispatchedIDs[i]

		// Resolve dependency task IDs from 0-based indices
		depIDs := make([]orchestra.TaskID, 0, len(t.Dependencies))
		for _, depIdx := range t.Dependencies {
			if depIdx >= 0 && depIdx < len(dispatchedIDs) {
				depIDs = append(depIDs, orchestra.TaskID(dispatchedIDs[depIdx]))
			}
		}

		projID := plan.ProjectID
		if strings.TrimSpace(projID) == "" {
			projID = "local"
		}

		action := t.Action
		if action == "" {
			action = "code"
		}
		resource := t.Resource
		if resource == "" {
			resource = "workspace"
		}

		initialStatus := orchestra.TaskReady
		if len(depIDs) > 0 {
			initialStatus = orchestra.TaskDraft
		}

		task := orchestra.Task{
			ID:                 orchestra.TaskID(taskID),
			ProjectID:          projID,
			WorktreeID:         "active",
			Title:              t.Title,
			AcceptanceCriteria: append([]string(nil), t.AcceptanceCriteria...),
			Dependencies:       depIDs,
			Status:             initialStatus,
			AttemptCount:       0,
			MaxAttempts:        3,
			Action:             action,
			Resource:           resource,
			AssignedStaffID:    staff.StaffID(t.CallSign),
			CreatedAt:          now,
			UpdatedAt:          now,
			SchemaVersion:      orchestra.ContractVersion,
		}

		if _, err := m.orchestra.CreateTask(ctx, task); err != nil {
			return nil, fmt.Errorf("failed to create orchestra task %s: %w", taskID, err)
		}
	}

	return dispatchedIDs, nil
}
