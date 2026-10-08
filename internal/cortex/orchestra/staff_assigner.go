package orchestra

import (
	"context"
	"fmt"
	"time"

	"github.com/RahmatHadinata23758051/CortexOS/internal/cortex/harness"
	"github.com/RahmatHadinata23758051/CortexOS/internal/cortex/staff"
)

// StaffTaskAssigner enriches execution envelopes with governed logical Staff,
// skill injection, advisory memory, and assignment provenance.
// It does not grant permissions; Harness policy remains the sole tool
// authorization boundary, and Inspector/Orchestra is the sole success authority.
type StaffTaskAssigner struct {
	staffStore     staff.Store
	staffRouter    staff.Router
	staffScheduler *staff.Scheduler
	taskStore      TaskStore
	skillStore     staff.SkillPort
	memoryStore    staff.MemoryPort
	capabilityReg  *staff.CapabilityRegistry
	harnessRouter  harness.AdapterRouter
	clock          func() time.Time
}

// SetTaskStore attaches a TaskStore to record assignment and audit events.
func (a *StaffTaskAssigner) SetTaskStore(store TaskStore) {
	a.taskStore = store
}

// Release releases any logical reservation held by the scheduler for the task.
func (a *StaffTaskAssigner) Release(ctx context.Context, taskID TaskID) error {
	if a.staffScheduler != nil {
		return a.staffScheduler.Release(ctx, string(taskID))
	}
	return nil
}

// Cancel cancels any logical reservation held by the scheduler for the task.
func (a *StaffTaskAssigner) Cancel(ctx context.Context, taskID TaskID) error {
	if a.staffScheduler != nil {
		return a.staffScheduler.Cancel(ctx, string(taskID))
	}
	return nil
}

// NewStaffTaskAssigner constructs a validated assigner with required dependencies.
// The scheduler is optional; if provided, it handles queueing, fairness, and
// availability scheduling. If nil, the router is used directly.
func NewStaffTaskAssigner(
	staffStore staff.Store,
	staffRouter staff.Router,
	skillStore staff.SkillPort,
	memoryStore staff.MemoryPort,
	capabilityReg *staff.CapabilityRegistry,
	harnessRouter harness.AdapterRouter,
	sched ...*staff.Scheduler,
) (*StaffTaskAssigner, error) {
	if staffStore == nil {
		return nil, fmt.Errorf("staff store is required")
	}
	if staffRouter == nil {
		return nil, fmt.Errorf("staff router is required")
	}
	if capabilityReg == nil {
		capabilityReg = staff.DefaultCapabilityRegistry()
	}
	if err := capabilityReg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid capability registry: %w", err)
	}
	var scheduler *staff.Scheduler
	if len(sched) > 0 {
		scheduler = sched[0]
	}
	return &StaffTaskAssigner{
		staffStore:     staffStore,
		staffRouter:    staffRouter,
		staffScheduler: scheduler,
		skillStore:     skillStore,
		memoryStore:    memoryStore,
		capabilityReg:  capabilityReg,
		harnessRouter:  harnessRouter,
		clock:          time.Now,
	}, nil
}

// Prepare implements EnvelopeProvider by resolving a Staff member,
// injecting applicable skills, selecting relevant advisory memory,
// and recording assignment provenance.
func (a *StaffTaskAssigner) Prepare(ctx context.Context, task Task, execution Execution) (ExecutionEnvelope, error) {
	if err := ctx.Err(); err != nil {
		return ExecutionEnvelope{}, err
	}
	if err := ValidateTask(task); err != nil {
		return ExecutionEnvelope{}, err
	}

	// Determine required capabilities from task acceptance criteria and role hints
	requiredCaps := a.inferCapabilities(task)

	// Resolve the logical Staff member (either pre-assigned or via router)
	var selectedStaff *staff.Definition
	var assignmentID, assignmentSource string
	if task.AssignedStaffID != "" {
		if a.staffScheduler != nil {
			priority := task.Priority
			if priority == 0 {
				priority = staff.PriorityNormal
			}
			caps := task.RequiredCapabilities
			if len(caps) == 0 {
				caps = requiredCaps
			}
			assignment, err := a.staffScheduler.Submit(ctx, staff.AssignmentRequest{
				TaskID: string(task.ID), StaffID: task.AssignedStaffID,
				WorkspaceID: staff.WorkspaceID(task.WorkspaceID), ProjectID: staff.ProjectID(task.ProjectID),
				WorktreeID: staff.WorktreeID(task.WorktreeID), RequiredCapabilities: caps,
				Action: task.Action, Resource: task.Resource, Priority: priority, ResourceAvailable: true,
			})
			if err != nil {
				return ExecutionEnvelope{}, fmt.Errorf("preassigned Staff rejected: %w", err)
			}
			if assignment.Status != staff.AssignmentAssigned {
				return ExecutionEnvelope{}, fmt.Errorf("%w: preassigned Staff unavailable for task %s", ErrAssignmentQueued, task.ID)
			}
			staffDef, err := a.staffStore.GetDefinition(ctx, assignment.StaffID)
			if err != nil {
				return ExecutionEnvelope{}, fmt.Errorf("assigned staff lookup failed: %w", err)
			}
			selectedStaff = &staffDef
			assignmentID = assignment.AssignmentID
			assignmentSource = "scheduler_preassignment"
		} else {
			staffDef, err := a.staffStore.GetDefinition(ctx, task.AssignedStaffID)
			if err != nil {
				return ExecutionEnvelope{}, fmt.Errorf("failed to get assigned staff: %w", err)
			}
			selectedStaff = &staffDef
			assignmentID = fmt.Sprintf("preassigned-%s", task.AssignedStaffID)
			assignmentSource = "task_preassignment"
		}
	} else {
		// Route through the race-safe scheduler when configured. This reserves a
		// logical Staff slot only; Harness still authorizes tools and Orchestra
		// still owns task outcomes.
		if a.staffScheduler != nil {
			priority := task.Priority
			if priority == 0 {
				priority = staff.PriorityNormal
			}
			caps := task.RequiredCapabilities
			if len(caps) == 0 {
				caps = requiredCaps
			}
			assignment, err := a.staffScheduler.Submit(ctx, staff.AssignmentRequest{
				TaskID: string(task.ID), WorkspaceID: staff.WorkspaceID(task.WorkspaceID),
				ProjectID: staff.ProjectID(task.ProjectID), WorktreeID: staff.WorktreeID(task.WorktreeID),
				RequiredCapabilities: caps, Action: task.Action, Resource: task.Resource,
				Priority: priority, ResourceAvailable: true,
			})
			if err != nil {
				return ExecutionEnvelope{}, fmt.Errorf("staff assignment rejected: %w", err)
			}
			if assignment.Status != staff.AssignmentAssigned {
				return ExecutionEnvelope{}, fmt.Errorf("%w: staff assignment queued for task %s", ErrAssignmentQueued, task.ID)
			}
			staffDef, err := a.staffStore.GetDefinition(ctx, assignment.StaffID)
			if err != nil {
				return ExecutionEnvelope{}, fmt.Errorf("assigned staff lookup failed: %w", err)
			}
			selectedStaff = &staffDef
			assignmentID = assignment.AssignmentID
			assignmentSource = "scheduler"
		} else {
			// Route to an eligible Staff member
			candidates, err := a.staffRouter.SelectCandidates(ctx, staff.RouterRequest{
				WorkspaceID:          staff.WorkspaceID(task.WorkspaceID),
				ProjectID:            staff.ProjectID(task.ProjectID),
				RequiredCapabilities: requiredCaps,
			})
			if err != nil {
				return ExecutionEnvelope{}, fmt.Errorf("staff candidate selection failed: %w", err)
			}
			if len(candidates) == 0 {
				return ExecutionEnvelope{}, fmt.Errorf("no eligible staff found for task %s", task.ID)
			}
			selectedStaff = &candidates[0]
			assignmentID = fmt.Sprintf("routed-%s", selectedStaff.ID)
			assignmentSource = "router_selection"
		}
	}

	// Prepare skill injection
	var skillInjection *harness.SkillInjection
	if a.skillStore != nil && len(selectedStaff.Skills) > 0 {
		contextData := fmt.Sprintf("Task: %s\nAcceptance: %v", task.Title, task.AcceptanceCriteria)
		envelope := &harness.ExecutionEnvelope{
			ContractVersion: harness.HarnessContractVersion,
			ExecutionID:     string(execution.ID),
			TaskID:          string(task.ID),
			WorktreeID:      task.WorktreeID,
			ProjectID:       task.ProjectID,
			ToolName:        "coding",
			PolicyDecision:  harness.PolicyDecision{Allowed: true, Effect: harness.EffectAllow, Reason: "staff assignment context"},
		}
		// InjectSkillIntoEnvelope uses staff store; we call it via staff package
		if err := staff.InjectSkillIntoEnvelope(envelope, *selectedStaff, a.skillStore, contextData); err != nil {
			return ExecutionEnvelope{}, fmt.Errorf("skill injection failed: %w", err)
		}
		skillInjection = envelope.SkillInjection
	}

	// Prepare advisory memory context (bounded, redacted)
	var memoryContext []AdvisoryMemory
	if a.memoryStore != nil && len(selectedStaff.Memory) > 0 {
		for _, ref := range selectedStaff.Memory {
			meta, err := a.memoryStore.ResolveReference(ctx, selectedStaff.ID, ref)
			if err != nil {
				continue // Skip unresolvable references
			}
			// Content retrieval is advisory; we include metadata and a bounded excerpt
			memoryContext = append(memoryContext, AdvisoryMemory{
				ID:      meta.ID,
				Kind:    meta.Kind,
				Content: fmt.Sprintf("[Memory: %s v%s]", meta.Kind, meta.Version),
				Source:  "memory_adapter",
				Provenance: AssignmentProvenance{
					TaskID:      string(task.ID),
					ExecutionID: string(execution.ID),
					TraceID:     string(execution.ID),
				},
			})
		}
	}

	// Select Harness engine adapter via capability-based router
	var selectedAdapter string
	if a.harnessRouter != nil && len(requiredCaps) > 0 {
		tools, _, _, _, err := a.resolveHarnessCapabilities(requiredCaps)
		if err == nil {
			selection, routeErr := a.harnessRouter.FindWithConstraints(ctx, tools, harness.RoutingConstraints{})
			if routeErr == nil {
				selectedAdapter = selection.Descriptor.Identity.Name
			}
		}
	}

	envelope := ExecutionEnvelope{
		TaskID:      task.ID,
		ProjectID:   task.ProjectID,
		WorktreeID:  task.WorktreeID,
		WorkspaceID: task.WorkspaceID,
		Attempt:     execution.Attempt,
		Title:       task.Title,
		TraceID:     string(execution.ID),
		Staff:       selectedStaff,
		Skill:       skillInjection,
		Memory:      memoryContext,
		Assignment: AssignmentProvenance{
			AssignmentID: assignmentID,
			Source:       assignmentSource,
			AssignedAt:   a.clock(),
			TaskID:       string(task.ID),
			ExecutionID:  string(execution.ID),
			TraceID:      string(execution.ID),
		},
		SelectedAdapter: selectedAdapter,
	}
	return envelope, nil
}

// inferCapabilities derives a capability set from the task. This is a heuristic;
// production callers should attach explicit capability requirements to the task.
func (a *StaffTaskAssigner) inferCapabilities(task Task) []staff.Capability {
	// Start with role defaults if task has role hint
	capSet := make(map[staff.Capability]struct{})
	for _, criterion := range task.AcceptanceCriteria {
		// Simple keyword matching for common capabilities
		lc := criterion
		if contains(lc, "test") {
			capSet[staff.Capability("test_run")] = struct{}{}
		}
		if contains(lc, "build") {
			capSet[staff.Capability("build")] = struct{}{}
		}
		if contains(lc, "lint") {
			capSet[staff.Capability("lint")] = struct{}{}
		}
		if contains(lc, "code") || contains(lc, "implement") || contains(lc, "feature") {
			capSet[staff.Capability("coding")] = struct{}{}
			capSet[staff.Capability("file_read")] = struct{}{}
			capSet[staff.Capability("file_write")] = struct{}{}
			capSet[staff.Capability("file_edit")] = struct{}{}
		}
		if contains(lc, "review") {
			capSet[staff.Capability("review")] = struct{}{}
		}
		if contains(lc, "debug") {
			capSet[staff.Capability("debug")] = struct{}{}
		}
		if contains(lc, "refactor") {
			capSet[staff.Capability("refactor")] = struct{}{}
		}
		if contains(lc, "analy") {
			capSet[staff.Capability("analysis")] = struct{}{}
		}
	}
	// Default: implementer-style capabilities if nothing matched
	if len(capSet) == 0 {
		for _, cap := range a.capabilityReg.RoleDefaults[staff.RoleImplementer] {
			capSet[cap] = struct{}{}
		}
	}
	var caps []staff.Capability
	for cap := range capSet {
		caps = append(caps, cap)
	}
	return caps
}

func (a *StaffTaskAssigner) resolveHarnessCapabilities(staffCaps []staff.Capability) ([]harness.ToolCapability, []harness.EngineClass, []harness.EngineClass, int, error) {
	toolMap := make(map[harness.ToolCapability]struct{})
	var tools []harness.ToolCapability
	var allowed, preferred []harness.EngineClass
	maxMinMemory := 0

	for _, cap := range staffCaps {
		mapping, ok := a.capabilityReg.GetMapping(cap)
		if !ok {
			continue
		}
		for _, tool := range mapping.RequiredTools {
			if _, exists := toolMap[tool]; !exists {
				toolMap[tool] = struct{}{}
				tools = append(tools, tool)
			}
		}
		for _, eng := range mapping.AllowedEngines {
			if !containsEngineClass(allowed, eng) {
				allowed = append(allowed, eng)
			}
		}
		for _, eng := range mapping.PreferredEngines {
			if !containsEngineClass(preferred, eng) {
				preferred = append(preferred, eng)
			}
		}
		if mapping.MinMemoryMB > maxMinMemory {
			maxMinMemory = mapping.MinMemoryMB
		}
	}
	return tools, preferred, allowed, maxMinMemory, nil
}

func containsEngineClass(slice []harness.EngineClass, item harness.EngineClass) bool {
	for _, v := range slice {
		if v == item {
			return true
		}
	}
	return false
}
