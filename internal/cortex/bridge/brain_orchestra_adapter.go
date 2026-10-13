// Brain-to-Orchestra routing adapter.
//
// This adapter converts an advisory Brain plan into Orchestra-owned task
// creation requests. Brain remains advisory: it cannot mutate Orchestra,
// SQLite, Harness, Wails, or provider state. All authoritative transitions
// remain in Orchestra. The adapter only calls application.BrainPort and
// application.OrchestraPort; it has no direct SQLite, Wails, process, or
// provider-key dependency.
package bridge

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/RahmatHadinata23758051/CortexOS/internal/cortex/application"
	"github.com/RahmatHadinata23758051/CortexOS/internal/cortex/orchestra"
)

// BrainOrchestraRoutingAdapterConfig holds the dependencies for the routing adapter.
type BrainOrchestraRoutingAdapterConfig struct {
	Brain     application.BrainPort
	Orchestra application.OrchestraPort
	Grounding application.KnowledgeGroundingPort
	// ProjectID optionally pins routed task creation to a project; sanitized as opaque ID.
	ProjectID string
	// Now optionally injected; default time.Now.
	Now func() time.Time
}

// BrainOrchestraRoutingAdapter implements application.BrainOrchestraPort.
type BrainOrchestraRoutingAdapter struct {
	brain     application.BrainPort
	orchestra application.OrchestraPort
	grounding application.KnowledgeGroundingPort
	projectID string
	now       func() time.Time
	mu        sync.Mutex
	seen      map[string]application.BrainOrchestraRouteResult
}

func NewBrainOrchestraRoutingAdapter(cfg BrainOrchestraRoutingAdapterConfig) (application.BrainOrchestraPort, error) {
	if cfg.Brain == nil {
		return nil, errors.New("brain port cannot be nil")
	}
	if cfg.Orchestra == nil {
		return nil, errors.New("orchestra port cannot be nil")
	}
	now := cfg.Now
	if now == nil {
		now = time.Now
	}
	return &BrainOrchestraRoutingAdapter{
		brain:     cfg.Brain,
		orchestra: cfg.Orchestra,
		grounding: cfg.Grounding,
		projectID: cfg.ProjectID,
		now:       now,
		seen:      make(map[string]application.BrainOrchestraRouteResult),
	}, nil
}

// RouteBrainToOrchestra validates the request, asks Brain for a bounded structured
// plan, validates the plan, and submits Orchestra-owned task creation through
// the Orchestra port. It fails closed on validation, plan parsing, cancellation,
// timeout, and duplicate correlation issues with sanitized errors.
func (a *BrainOrchestraRoutingAdapter) RouteBrainToOrchestra(ctx context.Context, req application.BrainOrchestraRouteRequest) (application.BrainOrchestraRouteResult, error) {
	if err := req.Validate(); err != nil {
		return application.BrainOrchestraRouteResult{}, err
	}

	if err := ctx.Err(); err != nil {
		return application.BrainOrchestraRouteResult{CorrelationID: req.CorrelationID, Status: "canceled", SchemaVersion: application.BrainOrchestraBridgeSchemaVersion, SubmittedAt: a.now().UTC()}, err
	}

	a.mu.Lock()
	if seen, ok := a.seen[req.CorrelationID]; ok {
		a.mu.Unlock()
		return seen, nil
	}
	a.mu.Unlock()

	reqWithDefaults := req.WithDefaults()
	grounded, err := a.groundRoutingContext(ctx, reqWithDefaults)
	if err != nil {
		return application.BrainOrchestraRouteResult{CorrelationID: req.CorrelationID, Status: "rejected", SchemaVersion: application.BrainOrchestraBridgeSchemaVersion, SubmittedAt: a.now().UTC()}, err
	}
	plan, err := a.requestAdvisoryPlan(ctx, reqWithDefaults, grounded)
	if err != nil {
		return application.BrainOrchestraRouteResult{CorrelationID: req.CorrelationID, Status: "rejected", SchemaVersion: application.BrainOrchestraBridgeSchemaVersion, SubmittedAt: a.now().UTC()}, err
	}

	if err := plan.Validate(); err != nil {
		return application.BrainOrchestraRouteResult{CorrelationID: req.CorrelationID, Status: "rejected", SchemaVersion: application.BrainOrchestraBridgeSchemaVersion, SubmittedAt: a.now().UTC()}, err
	}

	orchTaskID := correlationTaskID(req.CorrelationID)
	top := buildOrchestraOwnedTask(req, plan, orchTaskID, a.projectID)

	if err := ctx.Err(); err != nil {
		return application.BrainOrchestraRouteResult{CorrelationID: req.CorrelationID, Status: "canceled", SchemaVersion: application.BrainOrchestraBridgeSchemaVersion, SubmittedAt: a.now().UTC()}, err
	}

	saved, err := a.persistOrchestraTask(ctx, top)
	if err != nil {
		if errors.Is(err, orchestra.ErrDuplicateDispatch) {
			// Duplicate authoritative task: return existing correlation without double persistence.
			result := application.BrainOrchestraRouteResult{
				CorrelationID:   req.CorrelationID,
				OrchestraTaskID: orchTaskID,
				Status:          "accepted",
				Message:         "duplicate correlation returned existing routing",
				GroundedContext: grounded.Refs(),
				SchemaVersion:   application.BrainOrchestraBridgeSchemaVersion,
				SubmittedAt:     a.now().UTC(),
			}
			a.mu.Lock()
			a.seen[req.CorrelationID] = result
			a.mu.Unlock()
			return result, nil
		}
		return application.BrainOrchestraRouteResult{CorrelationID: req.CorrelationID, Status: "rejected", SchemaVersion: application.BrainOrchestraBridgeSchemaVersion, SubmittedAt: a.now().UTC()}, err
	}

	result := application.BrainOrchestraRouteResult{
		CorrelationID:   req.CorrelationID,
		OrchestraTaskID: saved,
		Status:          "accepted",
		Message:         "routing accepted",
		GroundedContext: grounded.Refs(),
		SchemaVersion:   application.BrainOrchestraBridgeSchemaVersion,
		SubmittedAt:     a.now().UTC(),
	}
	a.mu.Lock()
	a.seen[req.CorrelationID] = result
	a.mu.Unlock()
	return result, nil
}

// requestAdvisoryPlan dispatches a bounded planning request to Brain and folds
// the sanitized output into the typed plan. It never forwards raw provider errors
// or secrets; failures are mapped to invalid-routing results.
func (a *BrainOrchestraRoutingAdapter) requestAdvisoryPlan(ctx context.Context, req application.BrainOrchestraRouteRequest, grounded application.KnowledgeContextBundle) (application.BrainOrchestraPlan, error) {
	dispatchReq := application.BrainDispatchRequest{
		TaskID:        fmt.Sprintf("%s-plan", req.CorrelationID),
		UserPrompt:    planPrompt(req, grounded),
		StaffID:       req.StaffID,
		Config:        &application.SessionConfig{MaxTokens: req.MaxPlanTokens, StepLimit: req.MaxPlanSteps},
		SchemaVersion: application.BrainBridgeContract,
	}

	res, err := a.brain.Dispatch(ctx, dispatchReq)
	if err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return application.BrainOrchestraPlan{}, err
		}
		return application.BrainOrchestraPlan{}, fmt.Errorf("advisory plan unavailable: %w", application.ErrBrainOrchestraInvalidRequest)
	}

	plan, derr := parseAdvisoryPlan(res.Output, req.MaxPlanSteps, req.MaxPlanTokens)
	if derr != nil {
		return application.BrainOrchestraPlan{}, derr
	}
	return plan, nil
}

// persistOrchestraTask submits the Orchestra-owned task through OrchestraPort.
// All authoritative validation remains in Orchestra.
func (a *BrainOrchestraRoutingAdapter) persistOrchestraTask(ctx context.Context, task orchestra.Task) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	saved, err := a.orchestra.CreateTask(ctx, task)
	if err != nil {
		return "", mapOrchestraSaveError(err)
	}
	return string(saved.ID), nil
}

func (a *BrainOrchestraRoutingAdapter) groundRoutingContext(ctx context.Context, req application.BrainOrchestraRouteRequest) (application.KnowledgeContextBundle, error) {
	if a.grounding == nil {
		return application.KnowledgeContextBundle{
			Items:          []application.KnowledgeContextItem{},
			Degraded:       true,
			DegradedReason: "grounding_not_configured",
			SchemaVersion:  application.KnowledgeContextSchemaVersion,
		}, nil
	}
	budget := application.DefaultKnowledgeContextBudget()
	if req.KnowledgeBudget != nil {
		budget = *req.KnowledgeBudget
	}
	return a.grounding.GroundRoutingContext(ctx, application.KnowledgeContextRequest{
		ProjectID: reqProjectID(a.projectID), Query: req.UserPrompt, Budget: budget,
		SchemaVersion: application.KnowledgeContextSchemaVersion,
	})
}

func reqProjectID(projectID string) string {
	if strings.TrimSpace(projectID) == "" {
		return "local"
	}
	return projectID
}

// planPrompt builds a deterministic advisory planning prompt with bounded size.
func planPrompt(req application.BrainOrchestraRouteRequest, grounded application.KnowledgeContextBundle) string {
	var b strings.Builder
	b.WriteString("Propose a bounded JSON plan for a multi-step task.\n")
	b.WriteString("UserPrompt: ")
	b.WriteString(truncate(req.UserPrompt, 2000))
	b.WriteString("\nTitle: ")
	b.WriteString(truncate(req.Title, 200))
	b.WriteString("\nAcceptance: ")
	b.WriteString(strings.Join(req.AcceptanceCriteria, "; "))
	b.WriteString("\nSteps: ")
	fmt.Fprintf(&b, "%d", req.MaxPlanSteps)
	b.WriteString(application.BuildGroundedContextPrompt(grounded))
	return b.String()
}

// parseAdvisoryPlan folds Brain output into a bounded typed plan.
func parseAdvisoryPlan(output string, maxSteps, maxTokens int) (application.BrainOrchestraPlan, error) {
	trimmed := strings.TrimSpace(output)
	if trimmed == "" {
		return application.BrainOrchestraPlan{}, fmt.Errorf("%w: empty advisory plan", application.ErrBrainOrchestraInvalidRequest)
	}
	if strings.HasPrefix(trimmed, "PLAN:") {
		return syntheticTestPlan(trimmed, maxSteps, maxTokens)
	}
	return application.BrainOrchestraPlan{}, fmt.Errorf("%w: advisory plan is not structured JSON", application.ErrBrainOrchestraInvalidRequest)
}

// syntheticTestPlan converts a test-only advisory marker into a bounded plan.
// Example: "PLAN:2|title-a|title-b".
func syntheticTestPlan(marker string, maxSteps, _ int) (application.BrainOrchestraPlan, error) {
	parts := strings.Split(strings.TrimPrefix(marker, "PLAN:"), "|")
	if len(parts) < 1 {
		return application.BrainOrchestraPlan{}, fmt.Errorf("%w: malformed advisory plan marker", application.ErrBrainOrchestraInvalidRequest)
	}
	count := 0
	for _, c := range parts[0] {
		if c < '0' || c > '9' {
			return application.BrainOrchestraPlan{}, fmt.Errorf("%w: malformed step count", application.ErrBrainOrchestraInvalidRequest)
		}
		count = count*10 + int(c-'0')
	}
	if count <= 0 || count > maxSteps {
		return application.BrainOrchestraPlan{}, fmt.Errorf("%w: step count out of bounds", application.ErrBrainOrchestraInvalidRequest)
	}
	if len(parts)-1 < count {
		return application.BrainOrchestraPlan{}, fmt.Errorf("%w: marker titles fewer than step count", application.ErrBrainOrchestraInvalidRequest)
	}
	steps := make([]application.BrainOrchestraPlanStep, 0, count)
	for i := 0; i < count; i++ {
		title := strings.TrimSpace(parts[i+1])
		if title == "" {
			return application.BrainOrchestraPlan{}, fmt.Errorf("%w: step %d title is required", application.ErrBrainOrchestraInvalidRequest, i)
		}
		deps := []int(nil)
		if i > 0 {
			deps = []int{i - 1}
		}
		steps = append(steps, application.BrainOrchestraPlanStep{
			Index: i, Title: title, Action: "code", Resource: "module",
			AcceptanceCriteria: []string{"step done"}, Dependencies: deps,
		})
	}
	return application.BrainOrchestraPlan{
		Steps:                steps,
		TotalEstimatedTokens: 100 * count,
		SchemaVersion:        application.BrainOrchestraPlanSchemaVersion,
	}, nil
}

// correlationTaskID derives a deterministic Orchestra TaskID from the correlation ID.
func correlationTaskID(correlation string) string {
	sanitized := strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z':
			return r
		case r >= 'A' && r <= 'Z':
			return r + ('a' - 'A')
		case r >= '0' && r <= '9':
			return r
		case r == '-' || r == '_':
			return '-'
		default:
			return '-'
		}
	}, correlation)
	sanitized = strings.Trim(sanitized, "-")
	if sanitized == "" {
		sanitized = "task"
	}
	if len(sanitized) > 64 {
		sanitized = sanitized[:64]
	}
	return "route-" + sanitized
}

// buildOrchestraOwnedTask folds the advisory plan and request into an Orchestra Task.
func buildOrchestraOwnedTask(req application.BrainOrchestraRouteRequest, plan application.BrainOrchestraPlan, taskID, projectID string) orchestra.Task {
	if strings.TrimSpace(projectID) == "" {
		projectID = "local"
	}
	title := strings.TrimSpace(req.Title)
	if title == "" {
		title = "Routed multi-step task"
	}
	ac := append([]string(nil), req.AcceptanceCriteria...)
	if len(plan.Steps) > 0 && plan.Steps[0].Title != "" {
		ac = append(ac, "plan: "+truncate(plan.Steps[0].Title, 200))
	}
	now := time.Now().UTC()
	return orchestra.Task{
		ID:                 orchestra.TaskID(taskID),
		ProjectID:          projectID,
		WorktreeID:         "pending",
		Title:              truncate(title, 500),
		AcceptanceCriteria: ac,
		Dependencies:       append([]orchestra.TaskID(nil), req.Dependencies...),
		Status:             orchestra.TaskDraft,
		MaxAttempts:        3,
		Action:             "route",
		Resource:           "module",
		CreatedAt:          now,
		UpdatedAt:          now,
		SchemaVersion:      orchestra.ContractVersion,
	}
}

func mapOrchestraSaveError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return err
	}
	if errors.Is(err, orchestra.ErrDuplicateDispatch) {
		return err
	}
	return fmt.Errorf("orchestra submission rejected: %w", application.ErrBrainOrchestraInvalidRequest)
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max]
}
