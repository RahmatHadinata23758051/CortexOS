package application

import (
	"context"
	"encoding/json"
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
	ErrOrchestraPlanInvalidRequest   = errors.New("orchestra plan: invalid request")
	ErrOrchestraPlanNotFound         = errors.New("orchestra plan: not found")
	ErrOrchestraPlanAlreadyDecided   = errors.New("orchestra plan: already confirmed or rejected")
	ErrOrchestraPlanModelUnavailable = errors.New("orchestra plan: coordinator model unavailable")
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
	// An empty task list is valid: Orchestra classifies the prompt itself and
	// either answers it (inquiry) or derives the work (action). Title is only
	// required when the caller explicitly supplies tasks.
	if len(req.Tasks) > 0 && strings.TrimSpace(req.Title) == "" {
		return fmt.Errorf("%w: title is required", ErrOrchestraPlanInvalidRequest)
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
	Status               string                      `json:"status"`               // "proposed" | "confirmed" | "rejected" | "auto_dispatched" | "answered"
	Intent               string                      `json:"intent,omitempty"`     // "inquiry" | "action"
	Reply                string                      `json:"reply,omitempty"`      // conversational answer from Orchestra
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
	brain     BrainPort
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

	if len(req.Tasks) == 0 {
		decision, err := m.coordinate(ctx, req.UserPrompt)
		if err != nil {
			return OrchestraPlanProposal{}, err
		}
		if decision.Intent == "inquiry" {
			reply := strings.TrimSpace(decision.Reply)
			if reply == "" {
				return OrchestraPlanProposal{}, fmt.Errorf("%w: inquiry reply is empty", ErrOrchestraPlanModelUnavailable)
			}
			return m.storeAnswer(req, reply)
		}
		req.Title = firstNonEmpty(decision.Title, deriveOrchestraTitle(req.UserPrompt))
		task := decision.Task
		task.Index = 0
		task.Title = firstNonEmpty(task.Title, req.Title)
		task.Role = firstNonEmpty(task.Role, "implementer")
		task.Action = firstNonEmpty(task.Action, req.UserPrompt)
		if len(task.AcceptanceCriteria) == 0 {
			task.AcceptanceCriteria = []string{"Requested outcome is complete", "Evidence is ready for review"}
		}
		if task.EstimatedComplexity == "" {
			task.EstimatedComplexity = "standard"
		}
		task.Dependencies = []int{}
		req.Tasks = []OrchestraPlanProposedTask{task}
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
		Intent:               "action",
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

// SetBrain connects the coordinator model used to answer inquiries and plan
// actions. When unset, plan turns fail closed instead of inventing a reply.
func (m *OrchestraPlanManager) SetBrain(brain BrainPort) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.brain = brain
}

// orchestraCoordinatorRole instructs the model to act as Orchestra and return a
// single machine-readable decision. Kept verbatim so the contract is stable.
const OrchestraCoordinatorSystemPrompt = `You are Orchestra, the top-level project coordinator in CortexOS.
Read the user's message and decide what to do.
Return ONLY one raw JSON object, no markdown fence and no extra text, with this shape:
{"intent":"inquiry"|"action","reply":"<answer when intent is inquiry, else empty>","title":"<short task title when intent is action, else empty>","role":"<implementer|reviewer|planner when action, else empty>","action":"<short work description when action, else empty>","acceptanceCriteria":["<criterion>"],"estimatedComplexity":"trivial"|"standard"|"complex"}
Use "inquiry" for greetings, questions, and conversation. Use "action" only when the user asks for project work.`

// orchestraDecision is the model's structured coordinator turn.
type orchestraDecision struct {
	Intent string                    `json:"intent"`
	Reply  string                    `json:"reply"`
	Title  string                    `json:"title"`
	Task   OrchestraPlanProposedTask `json:"-"`
}

// coordinate asks the model (the CLI AI) to decide the intent and either answer
// the user or describe the work. The model is the authority; there is no
// keyword fallback. Fails closed when the model is unavailable.
func (m *OrchestraPlanManager) coordinate(ctx context.Context, prompt string) (orchestraDecision, error) {
	m.mu.RLock()
	brain := m.brain
	m.mu.RUnlock()
	if brain == nil {
		return orchestraDecision{}, fmt.Errorf("%w: no coordinator model is configured", ErrOrchestraPlanModelUnavailable)
	}
	res, err := brain.Dispatch(ctx, BrainDispatchRequest{
		TaskID:        fmt.Sprintf("orchestra-coordinate-%d", m.now().UnixNano()),
		UserPrompt:    fmt.Sprintf("%s\n\nUser message: %s", OrchestraCoordinatorSystemPrompt, prompt),
		StaffID:       "staff-coordinator",
		SchemaVersion: BrainBridgeContract,
	})
	if err != nil {
		return orchestraDecision{}, fmt.Errorf("%w: %v", ErrOrchestraPlanModelUnavailable, err)
	}
	return parseOrchestraDecision(res.Output)
}

// parseOrchestraDecision extracts the coordinator JSON from a model reply,
// tolerating prose or a fenced block around the object.
func parseOrchestraDecision(output string) (orchestraDecision, error) {
	raw := extractJSONObject(output)
	if raw == "" {
		return orchestraDecision{}, fmt.Errorf("%w: model returned no decision", ErrOrchestraPlanModelUnavailable)
	}
	var wire struct {
		Intent              string   `json:"intent"`
		Reply               string   `json:"reply"`
		Title               string   `json:"title"`
		Role                string   `json:"role"`
		Action              string   `json:"action"`
		AcceptanceCriteria  []string `json:"acceptanceCriteria"`
		EstimatedComplexity string   `json:"estimatedComplexity"`
	}
	if err := json.Unmarshal([]byte(raw), &wire); err != nil {
		return orchestraDecision{}, fmt.Errorf("%w: malformed decision: %v", ErrOrchestraPlanModelUnavailable, err)
	}
	decision := orchestraDecision{
		Intent: strings.ToLower(strings.TrimSpace(wire.Intent)),
		Reply:  strings.TrimSpace(wire.Reply),
		Title:  strings.TrimSpace(wire.Title),
		Task: OrchestraPlanProposedTask{
			Role:                strings.TrimSpace(wire.Role),
			Action:              strings.TrimSpace(wire.Action),
			AcceptanceCriteria:  wire.AcceptanceCriteria,
			EstimatedComplexity: strings.TrimSpace(wire.EstimatedComplexity),
		},
	}
	decision.Task.Title = decision.Title
	if decision.Intent == "" {
		decision.Intent = "inquiry"
	}
	if decision.Intent != "action" {
		decision.Intent = "inquiry"
		if decision.Reply == "" {
			decision.Reply = strings.TrimSpace(output)
		}
	}
	return decision, nil
}

// extractJSONObject returns the first balanced {...} block, handling fenced code.
func extractJSONObject(text string) string {
	start := strings.Index(text, "{")
	if start < 0 {
		return ""
	}
	depth := 0
	inString := false
	escaped := false
	for i := start; i < len(text); i++ {
		c := text[i]
		if inString {
			switch {
			case escaped:
				escaped = false
			case c == '\\':
				escaped = true
			case c == '"':
				inString = false
			}
			continue
		}
		switch c {
		case '"':
			inString = true
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return text[start : i+1]
			}
		}
	}
	return ""
}

// storeAnswer records an inquiry answer as a terminal proposal with no tasks.
func (m *OrchestraPlanManager) storeAnswer(req OrchestraPlanProposalRequest, reply string) (OrchestraPlanProposal, error) {
	now := m.now().UTC()
	proposal := OrchestraPlanProposal{
		PlanID:            fmt.Sprintf("answer-%s", req.CorrelationID),
		CorrelationID:     req.CorrelationID,
		ProjectID:         req.ProjectID,
		UserPrompt:        req.UserPrompt,
		Title:             "Orchestra reply",
		Summary:           reply,
		Status:            "answered",
		Intent:            "inquiry",
		Reply:             reply,
		Tasks:             []OrchestraPlanProposedTask{},
		CreatedAt:         now,
		DispatchedTaskIDs: []string{},
		SchemaVersion:     OrchestraPlanSchemaVersion,
	}
	m.mu.Lock()
	m.plans[proposal.PlanID] = proposal
	m.mu.Unlock()
	return proposal, nil
}

func deriveOrchestraTitle(prompt string) string {
	clean := strings.TrimSpace(prompt)
	if len(clean) > 80 {
		return clean[:77] + "..."
	}
	return clean
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
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
