package staff

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/RahmatHadinata23758051/CortexOS/internal/cortex/harness"
)

// TaskRouteRequest specifies the requirements for routing a task to an eligible
// Staff member and corresponding Harness engine adapter.
type TaskRouteRequest struct {
	TaskID               string
	WorkspaceID          WorkspaceID
	ProjectID            ProjectID
	WorktreeID           WorktreeID
	StaffID              StaffID
	RoleHint             Role
	RequiredCapabilities []Capability
	Action               string
	Resource             string
	ResourceConstraints  harness.RoutingConstraints
	Timeout              time.Duration
}

// Validate checks that the route request contains required fields and valid tokens.
func (r TaskRouteRequest) Validate() error {
	if r.TaskID == "" {
		return newError(ErrInvalidRequest, "taskId is required")
	}
	if err := validateToken(r.TaskID, "taskId"); err != nil {
		return err
	}
	if r.WorkspaceID == "" {
		return newError(ErrInvalidRequest, "workspaceId is required")
	}
	if err := validateToken(string(r.WorkspaceID), "workspaceId"); err != nil {
		return err
	}
	if r.ProjectID != "" {
		if err := validateToken(string(r.ProjectID), "projectId"); err != nil {
			return err
		}
	}
	if r.WorktreeID != "" {
		if err := validateToken(string(r.WorktreeID), "worktreeId"); err != nil {
			return err
		}
	}
	if r.StaffID != "" {
		if err := validateToken(string(r.StaffID), "staffId"); err != nil {
			return err
		}
	}
	if r.RoleHint != "" {
		if !validRole(r.RoleHint) {
			return newError(ErrInvalidRequest, fmt.Sprintf("unsupported role hint %q", r.RoleHint))
		}
	}
	if len(r.RequiredCapabilities) == 0 {
		return newError(ErrInvalidRequest, "at least one required capability must be specified")
	}
	seenCaps := make(map[Capability]struct{}, len(r.RequiredCapabilities))
	for _, cap := range r.RequiredCapabilities {
		if err := validateToken(string(cap), "capability"); err != nil {
			return err
		}
		if _, exists := seenCaps[cap]; exists {
			return newError(ErrInvalidRequest, fmt.Sprintf("duplicate capability %q in request", cap))
		}
		seenCaps[cap] = struct{}{}
	}
	return nil
}

// TaskRouteDecision represents the deterministic assignment of a logical Staff member
// and an eligible Harness engine adapter. It intentionally carries NO OS process handle
// or worker PID, preserving ADR-0003 decoupling.
type TaskRouteDecision struct {
	ContractVersion       string                            `json:"contractVersion"`
	TaskID                string                            `json:"taskId"`
	StaffID               StaffID                           `json:"staffId"`
	StaffRole             Role                              `json:"staffRole"`
	StaffSummary          Summary                           `json:"staffSummary"`
	EngineDescriptor      harness.ExtendedAdapterDescriptor `json:"engineDescriptor"`
	EngineClass           harness.EngineClass               `json:"engineClass"`
	EngineInstanceID      string                            `json:"engineInstanceId"`
	RequiredTools         []harness.ToolCapability          `json:"requiredTools"`
	PolicyDecision        harness.PolicyDecision            `json:"policyDecision"`
	Reason                string                            `json:"reason"`
	DeterministicRouteKey string                            `json:"deterministicRouteKey"`
	RoutedAt              time.Time                         `json:"routedAt"`
}

// StaffEngineRouter orchestrates deterministic candidate Staff selection and
// adapter matching via the Phase 4 Harness router.
type StaffEngineRouter struct {
	store    Store
	harness  harness.AdapterRouter
	registry *CapabilityRegistry
}

// NewStaffEngineRouter constructs a validated router with standard registries.
func NewStaffEngineRouter(store Store, harnessRouter harness.AdapterRouter, registry *CapabilityRegistry) (*StaffEngineRouter, error) {
	if store == nil {
		return nil, newError(ErrInvalidRequest, "staff store is required")
	}
	if harnessRouter == nil {
		return nil, newError(ErrInvalidRequest, "harness adapter router is required")
	}
	if registry == nil {
		registry = DefaultCapabilityRegistry()
	}
	if err := registry.Validate(); err != nil {
		return nil, newError(ErrInvalidRequest, fmt.Sprintf("invalid capability registry: %v", err))
	}
	return &StaffEngineRouter{
		store:    store,
		harness:  harnessRouter,
		registry: registry,
	}, nil
}

// SelectCandidates implements staff.Router by querying the store and sorting matches
// deterministically according to role, availability, and StaffID.
func (r *StaffEngineRouter) SelectCandidates(ctx context.Context, req RouterRequest) ([]Definition, error) {
	if ctx == nil {
		return nil, newError(ErrInvalidRequest, "context is required")
	}
	if err := ctx.Err(); err != nil {
		return nil, CanceledError(err)
	}
	if err := req.Validate(); err != nil {
		return nil, err
	}

	filter := Filter{
		WorkspaceID: &req.WorkspaceID,
		ActiveOnly:  true,
	}
	if req.ProjectID != "" {
		filter.ProjectID = &req.ProjectID
	}
	if req.RoleHint != "" {
		filter.Role = &req.RoleHint
	}

	defs, err := r.store.ListDefinitions(ctx)
	if err != nil {
		return nil, err
	}

	excludeMap := make(map[StaffID]struct{}, len(req.ExcludeIDs))
	for _, id := range req.ExcludeIDs {
		excludeMap[id] = struct{}{}
	}

	candidates := make([]Definition, 0, len(defs))
	for _, def := range defs {
		if _, excluded := excludeMap[def.ID]; excluded {
			continue
		}
		if def.Lifecycle != LifecycleActive {
			continue
		}
		if def.Workspace.WorkspaceID != req.WorkspaceID {
			continue
		}
		if req.ProjectID != "" && def.Workspace.ProjectID != "" && def.Workspace.ProjectID != req.ProjectID {
			continue
		}
		if !hasAllCapabilities(def.Capabilities, req.RequiredCapabilities) {
			continue
		}
		candidates = append(candidates, def)
	}

	// Deterministic sorting
	sort.Slice(candidates, func(i, j int) bool {
		// 1. Role hint priority
		if req.RoleHint != "" {
			iRoleMatch := candidates[i].Role == req.RoleHint
			jRoleMatch := candidates[j].Role == req.RoleHint
			if iRoleMatch != jRoleMatch {
				return iRoleMatch
			}
		}
		// 2. Availability priority (available > busy > unavailable > offline)
		if candidates[i].Availability != candidates[j].Availability {
			return availabilityRank(candidates[i].Availability) < availabilityRank(candidates[j].Availability)
		}
		// 3. Stable tie-breaking by StaffID
		return candidates[i].ID < candidates[j].ID
	})

	return candidates, nil
}

// RouteTask evaluates role capabilities, staff availability, policy/permissions,
// and engine health/budgets to produce an immutable TaskRouteDecision.
func (r *StaffEngineRouter) RouteTask(ctx context.Context, req TaskRouteRequest) (*TaskRouteDecision, error) {
	if ctx == nil {
		return nil, newError(ErrInvalidRequest, "context is required")
	}
	if err := ctx.Err(); err != nil {
		return nil, CanceledError(err)
	}
	if err := req.Validate(); err != nil {
		return nil, err
	}

	var selectedStaff Definition

	if req.StaffID != "" {
		// Specific Staff requested
		staffDef, err := r.store.GetDefinition(ctx, req.StaffID)
		if err != nil {
			return nil, err
		}
		if staffDef.Lifecycle != LifecycleActive {
			return nil, newError(ErrInactiveStaff, fmt.Sprintf("staff %q is not active (%s)", staffDef.ID, staffDef.Lifecycle))
		}
		if staffDef.Availability != AvailabilityAvailable && staffDef.Availability != AvailabilityBusy {
			return nil, newError(ErrUnavailableStaff, fmt.Sprintf("staff %q is unavailable (%s)", staffDef.ID, staffDef.Availability))
		}
		if staffDef.Workspace.WorkspaceID != req.WorkspaceID {
			return nil, newError(ErrInvalidAssignment, fmt.Sprintf("staff %q workspace mismatch: expected %q, got %q", staffDef.ID, req.WorkspaceID, staffDef.Workspace.WorkspaceID))
		}
		if req.ProjectID != "" && staffDef.Workspace.ProjectID != "" && staffDef.Workspace.ProjectID != req.ProjectID {
			return nil, newError(ErrInvalidAssignment, fmt.Sprintf("staff %q project mismatch: expected %q, got %q", staffDef.ID, req.ProjectID, staffDef.Workspace.ProjectID))
		}
		if !hasAllCapabilities(staffDef.Capabilities, req.RequiredCapabilities) {
			return nil, newError(ErrInvalidRequest, fmt.Sprintf("staff %q lacks required capabilities", staffDef.ID))
		}
		selectedStaff = staffDef
	} else {
		// Discover candidate Staff
		candidates, err := r.SelectCandidates(ctx, RouterRequest{
			WorkspaceID:          req.WorkspaceID,
			ProjectID:            req.ProjectID,
			RoleHint:             req.RoleHint,
			RequiredCapabilities: req.RequiredCapabilities,
		})
		if err != nil {
			return nil, err
		}
		if len(candidates) == 0 {
			return nil, newError(ErrNotFound, "no eligible staff candidates found matching request")
		}
		selectedStaff = candidates[0]
	}

	// Permission / Policy evaluation
	policyDecision := harness.PolicyDecision{
		Allowed: true,
		Effect:  harness.EffectAllow,
		Reason:  "default allow for non-action request",
	}

	if req.Action != "" && req.Resource != "" {
		decision, err := selectedStaff.Permissions.Evaluate(req.Action, req.Resource)
		if err != nil {
			return nil, WrapError(ErrInvalidPermission, "permission evaluation failed", err)
		}
		if decision.Effect == EffectDeny {
			return nil, newError(ErrPermissionDenied, fmt.Sprintf("staff %q policy denied action %q on %q: %s", selectedStaff.ID, req.Action, req.Resource, decision.Reason))
		}

		var envelopeAdapter EnvelopeAdapter
		envDecision, envErr := envelopeAdapter.ToEnvelopePolicyDecision(decision, req.Action, req.Resource)
		if envErr != nil {
			return nil, WrapError(ErrPermissionDenied, "envelope translation rejected decision", envErr)
		}
		if !envDecision.Allowed {
			return nil, newError(ErrPermissionDenied, fmt.Sprintf("permission not authorized for execution: %s", envDecision.Reason))
		}
		policyDecision = envDecision
	}

	// Map capabilities to Harness ToolCapabilities and determine Engine constraints
	requiredTools, allowedEngines, _, minMemory, err := r.resolveCapabilities(req.RequiredCapabilities)
	if err != nil {
		return nil, err
	}

	// Build constraints for Harness AdapterRouter
	constraints := req.ResourceConstraints
	if minMemory > constraints.MinMemoryMB {
		constraints.MinMemoryMB = minMemory
	}

	// Check if allowedEngines restrict classes
	if len(allowedEngines) > 0 {
		var excluded []harness.EngineClass
		allClasses := []harness.EngineClass{harness.EngineClassNative, harness.EngineClassPi, harness.EngineClassOMP}
		for _, class := range allClasses {
			if !containsEngineClass(allowedEngines, class) {
				excluded = append(excluded, class)
			}
		}
		constraints.ExcludeClasses = append(constraints.ExcludeClasses, excluded...)
	}

	// Query Harness Router
	selection, err := r.harness.FindWithConstraints(ctx, requiredTools, constraints)
	if err != nil {
		return nil, WrapError(ErrInternal, fmt.Sprintf("engine adapter selection failed: %v", err), err)
	}

	routeKey := fmt.Sprintf("%s|%s|%s", selectedStaff.ID, selection.Descriptor.Identity.Name, selection.Descriptor.Identity.InstanceID)

	return &TaskRouteDecision{
		ContractVersion:       ContractVersion,
		TaskID:                req.TaskID,
		StaffID:               selectedStaff.ID,
		StaffRole:             selectedStaff.Role,
		StaffSummary:          selectedStaff.Summary(),
		EngineDescriptor:      selection.Descriptor,
		EngineClass:           selection.Descriptor.Identity.Class,
		EngineInstanceID:      selection.Descriptor.Identity.InstanceID,
		RequiredTools:         requiredTools,
		PolicyDecision:        policyDecision,
		Reason:                fmt.Sprintf("deterministic route: staff %q -> adapter %q (%s)", selectedStaff.ID, selection.Descriptor.Name, selection.Reason),
		DeterministicRouteKey: routeKey,
		RoutedAt:              time.Now().UTC(),
	}, nil
}

func (r *StaffEngineRouter) resolveCapabilities(caps []Capability) ([]harness.ToolCapability, []harness.EngineClass, []harness.EngineClass, int, error) {
	toolMap := make(map[harness.ToolCapability]struct{})
	var tools []harness.ToolCapability
	var allowed []harness.EngineClass
	var preferred []harness.EngineClass
	maxMinMemory := 0

	for _, cap := range caps {
		mapping, ok := r.registry.GetMapping(cap)
		if !ok {
			return nil, nil, nil, 0, newError(ErrInvalidRequest, fmt.Sprintf("unregistered capability %q", cap))
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

	sort.Slice(tools, func(i, j int) bool { return string(tools[i]) < string(tools[j]) })
	return tools, allowed, preferred, maxMinMemory, nil
}

func hasAllCapabilities(staffCaps []Capability, required []Capability) bool {
	staffMap := make(map[Capability]struct{}, len(staffCaps))
	for _, cap := range staffCaps {
		staffMap[cap] = struct{}{}
	}
	for _, req := range required {
		if _, ok := staffMap[req]; !ok {
			return false
		}
	}
	return true
}

func availabilityRank(state AvailabilityState) int {
	switch state {
	case AvailabilityAvailable:
		return 0
	case AvailabilityBusy:
		return 1
	case AvailabilityUnavailable:
		return 2
	case AvailabilityOffline:
		return 3
	default:
		return 4
	}
}

func containsEngineClass(classes []harness.EngineClass, class harness.EngineClass) bool {
	for _, c := range classes {
		if c == class {
			return true
		}
	}
	return false
}
