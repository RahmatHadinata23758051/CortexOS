package bridge

import (
	"context"
	"sort"
	"time"

	"github.com/RahmatHadinata23758051/CortexOS/internal/cortex/application"
	"github.com/RahmatHadinata23758051/CortexOS/internal/cortex/staff"
)

// StaffAdapterConfig configures the production Staff adapter.
type StaffAdapterConfig struct {
	Store    staff.Store
	Registry *staff.CapabilityRegistry
}

// StaffAdapter bridges persistent staff.Store to the application.StaffPort boundary.
type StaffAdapter struct {
	store    staff.Store
	registry *staff.CapabilityRegistry
}

var _ application.StaffPort = (*StaffAdapter)(nil)

// NewStaffAdapter constructs a validated StaffAdapter.
func NewStaffAdapter(cfg StaffAdapterConfig) (*StaffAdapter, error) {
	if cfg.Store == nil {
		return nil, staff.WrapError(staff.ErrInvalidRequest, "staff store is required", nil)
	}
	reg := cfg.Registry
	if reg == nil {
		reg = staff.DefaultCapabilityRegistry()
	}
	return &StaffAdapter{
		store:    cfg.Store,
		registry: reg,
	}, nil
}

// GetStaff returns a safe summary of a Staff definition by ID.
func (a *StaffAdapter) GetStaff(ctx context.Context, id staff.StaffID) (application.StaffSummary, error) {
	if a == nil || a.store == nil {
		return application.StaffSummary{}, staff.WrapError(staff.ErrInternal, "staff store unavailable", nil)
	}
	def, err := a.store.GetDefinition(ctx, id)
	if err != nil {
		return application.StaffSummary{}, err
	}
	return toAppStaffSummary(def), nil
}

// ListStaff returns all Staff summaries with optional workspace, role, or active filter.
func (a *StaffAdapter) ListStaff(ctx context.Context, filter application.StaffListFilter) ([]application.StaffSummary, error) {
	if a == nil || a.store == nil {
		return nil, staff.WrapError(staff.ErrInternal, "staff store unavailable", nil)
	}

	var defs []staff.Definition
	var err error
	if filter.WorkspaceID != nil && *filter.WorkspaceID != "" {
		defs, err = a.store.ListDefinitionsByWorkspace(ctx, *filter.WorkspaceID)
	} else if filter.Role != nil && *filter.Role != "" {
		defs, err = a.store.ListDefinitionsByRole(ctx, *filter.Role)
	} else {
		defs, err = a.store.ListDefinitions(ctx)
	}
	if err != nil {
		return nil, err
	}

	results := make([]application.StaffSummary, 0, len(defs))
	for _, def := range defs {
		if filter.Role != nil && *filter.Role != "" && def.Role != *filter.Role {
			continue
		}
		if filter.WorkspaceID != nil && *filter.WorkspaceID != "" && def.Workspace.WorkspaceID != *filter.WorkspaceID {
			continue
		}
		if filter.ActiveOnly && def.Lifecycle != staff.LifecycleActive {
			continue
		}
		results = append(results, toAppStaffSummary(def))
	}
	sort.Slice(results, func(i, j int) bool { return string(results[i].ID) < string(results[j].ID) })
	return results, nil
}

// ListStaffByWorkspace returns Staff summaries assigned to a workspace.
func (a *StaffAdapter) ListStaffByWorkspace(ctx context.Context, workspaceID staff.WorkspaceID) ([]application.StaffSummary, error) {
	return a.ListStaff(ctx, application.StaffListFilter{WorkspaceID: &workspaceID})
}

// GetStaffCapabilities returns capability metadata for all registered Staff capabilities.
func (a *StaffAdapter) GetStaffCapabilities(ctx context.Context) ([]application.StaffCapabilitySummary, error) {
	if a == nil || a.registry == nil {
		return nil, staff.WrapError(staff.ErrInternal, "capability registry unavailable", nil)
	}
	allCaps := a.registry.AllCapabilities()
	results := make([]application.StaffCapabilitySummary, 0, len(allCaps))
	for _, cap := range allCaps {
		mapping, ok := a.registry.GetMapping(cap)
		if !ok {
			continue
		}
		tools := make([]string, len(mapping.RequiredTools))
		for i, t := range mapping.RequiredTools {
			tools[i] = string(t)
		}
		preferred := make([]string, len(mapping.PreferredEngines))
		for i, e := range mapping.PreferredEngines {
			preferred[i] = string(e)
		}
		allowed := make([]string, len(mapping.AllowedEngines))
		for i, e := range mapping.AllowedEngines {
			allowed[i] = string(e)
		}
		results = append(results, application.StaffCapabilitySummary{
			Capability:       cap,
			Description:      mapping.Description,
			RequiredTools:    tools,
			PreferredEngines: preferred,
			AllowedEngines:   allowed,
			MinMemoryMB:      mapping.MinMemoryMB,
			RequiresAsk:      mapping.RequiresAsk,
			SchemaVersion:    application.StaffBridgeSchemaVersion,
		})
	}
	return results, nil
}

// GetStaffAssignment returns the current workspace assignment for a Staff member.
func (a *StaffAdapter) GetStaffAssignment(ctx context.Context, id staff.StaffID) (application.StaffAssignmentSummary, error) {
	if a == nil || a.store == nil {
		return application.StaffAssignmentSummary{}, staff.WrapError(staff.ErrInternal, "staff store unavailable", nil)
	}
	def, err := a.store.GetDefinition(ctx, id)
	if err != nil {
		return application.StaffAssignmentSummary{}, err
	}
	return application.StaffAssignmentSummary{
		StaffID:       def.ID,
		WorkspaceID:   def.Workspace.WorkspaceID,
		ProjectID:     def.Workspace.ProjectID,
		WorktreeID:    def.Workspace.WorktreeID,
		SchemaVersion: application.StaffBridgeSchemaVersion,
	}, nil
}

func staffIDPointer(value *string) *string {
	if value == nil {
		return nil
	}
	copy := *value
	return &copy
}

func toAppStaffSummary(def staff.Definition) application.StaffSummary {
	return application.StaffSummary{
		ID:            def.ID,
		Name:          def.Name,
		Role:          def.Role,
		Capabilities:  append([]staff.Capability(nil), def.Capabilities...),
		WorkspaceID:   def.Workspace.WorkspaceID,
		ProjectID:     def.Workspace.ProjectID,
		WorktreeID:    def.Workspace.WorktreeID,
		ParentID:      staffIDPointer(def.ParentID),
		Profile:       def.Profile,
		Lifecycle:     def.Lifecycle,
		Availability:  def.Availability,
		SchemaVersion: application.StaffBridgeSchemaVersion,
		CreatedAt:     def.CreatedAt,
		UpdatedAt:     def.UpdatedAt,
	}
}

// SeedDefaultStaff seeds initial canonical staff definitions into the store if empty.
func SeedDefaultStaff(ctx context.Context, store staff.Store) error {
	existing, err := store.ListDefinitions(ctx)
	if err != nil {
		return err
	}
	if len(existing) > 0 {
		return nil
	}

	now := time.Now().UTC()
	coordinatorID := "staff-coordinator"
	plannerID := "staff-planner"
	defaults := []staff.Definition{
		{
			Identity: staff.Identity{ID: "staff-coordinator", Name: "Lead Coordinator"},
			Role:     staff.RoleCoordinator,
			Capabilities: []staff.Capability{
				staff.Capability("task_plan"), staff.Capability("git"), staff.Capability("search"),
			},
			Workspace:     staff.WorkspaceAssignment{WorkspaceID: "default"},
			Profile:       "default",
			Lifecycle:     staff.LifecycleActive,
			Availability:  staff.AvailabilityAvailable,
			CreatedAt:     now,
			UpdatedAt:     now,
			SchemaVersion: staff.ContractVersion,
		},
		{
			Identity: staff.Identity{ID: "staff-planner", Name: "Staff Planner"},
			Role:     staff.RolePlanner,
			Capabilities: []staff.Capability{
				staff.Capability("task_plan"), staff.Capability("code_navigation"), staff.Capability("analysis"),
			},
			Workspace:     staff.WorkspaceAssignment{WorkspaceID: "default"},
			ParentID:      &coordinatorID,
			Profile:       "default",
			Lifecycle:     staff.LifecycleActive,
			Availability:  staff.AvailabilityAvailable,
			CreatedAt:     now,
			UpdatedAt:     now,
			SchemaVersion: staff.ContractVersion,
		},
		{
			Identity: staff.Identity{ID: "staff-implementer", Name: "Core Implementer"},
			Role:     staff.RoleImplementer,
			Capabilities: []staff.Capability{
				staff.Capability("coding"), staff.Capability("file_read"), staff.Capability("file_write"),
				staff.Capability("file_edit"), staff.Capability("git"), staff.Capability("test_run"), staff.Capability("build"),
			},
			Workspace:     staff.WorkspaceAssignment{WorkspaceID: "default"},
			ParentID:      &plannerID,
			Profile:       "default",
			Lifecycle:     staff.LifecycleActive,
			Availability:  staff.AvailabilityAvailable,
			CreatedAt:     now,
			UpdatedAt:     now,
			SchemaVersion: staff.ContractVersion,
		},
		{
			Identity: staff.Identity{ID: "staff-reviewer", Name: "Staff Reviewer"},
			Role:     staff.RoleReviewer,
			Capabilities: []staff.Capability{
				staff.Capability("review"), staff.Capability("analysis"), staff.Capability("code_navigation"),
				staff.Capability("lint"), staff.Capability("test_run"),
			},
			Workspace:     staff.WorkspaceAssignment{WorkspaceID: "default"},
			ParentID:      &coordinatorID,
			Profile:       "default",
			Lifecycle:     staff.LifecycleActive,
			Availability:  staff.AvailabilityAvailable,
			CreatedAt:     now,
			UpdatedAt:     now,
			SchemaVersion: staff.ContractVersion,
		},
	}

	for _, d := range defaults {
		if _, err := store.SaveDefinition(ctx, d); err != nil {
			return err
		}
	}
	return nil
}
