package application

import (
	"context"
	"sort"

	"github.com/RahmatHadinata23758051/CortexOS/internal/cortex/staff"
)

func (s *Service) GetStaffSummary(ctx context.Context, request StaffRequest) (StaffSummary, error) {
	if err := validateStaffRequest(ctx, request.SchemaVersion); err != nil {
		return StaffSummary{}, staffBridgeError(err)
	}
	if s == nil || s.staff == nil {
		return StaffSummary{}, staffBridgeError(staff.WrapError(staff.ErrInternal, "staff service unavailable", nil))
	}
	summary, err := s.staff.GetStaff(ctx, request.StaffID)
	if err != nil {
		return StaffSummary{}, staffBridgeError(err)
	}
	return sanitizeStaffSummary(summary), nil
}

func (s *Service) ListStaffSummaries(ctx context.Context, request StaffListRequest) ([]StaffSummary, error) {
	if err := validateStaffRequest(ctx, request.SchemaVersion); err != nil {
		return nil, staffBridgeError(err)
	}
	if s == nil || s.staff == nil {
		return nil, staffBridgeError(staff.WrapError(staff.ErrInternal, "staff service unavailable", nil))
	}
	items, err := s.staff.ListStaff(ctx, request.Filter)
	if err != nil {
		return nil, staffBridgeError(err)
	}
	result := make([]StaffSummary, 0, len(items))
	for _, item := range items {
		result = append(result, sanitizeStaffSummary(item))
	}
	sort.Slice(result, func(i, j int) bool { return string(result[i].ID) < string(result[j].ID) })
	return result, nil
}

func (s *Service) ListStaffByWorkspace(ctx context.Context, request StaffByWorkspaceRequest) ([]StaffSummary, error) {
	if err := validateStaffRequest(ctx, request.SchemaVersion); err != nil {
		return nil, staffBridgeError(err)
	}
	if s == nil || s.staff == nil {
		return nil, staffBridgeError(staff.WrapError(staff.ErrInternal, "staff service unavailable", nil))
	}
	items, err := s.staff.ListStaffByWorkspace(ctx, request.WorkspaceID)
	if err != nil {
		return nil, staffBridgeError(err)
	}
	result := make([]StaffSummary, 0, len(items))
	for _, item := range items {
		result = append(result, sanitizeStaffSummary(item))
	}
	sort.Slice(result, func(i, j int) bool { return string(result[i].ID) < string(result[j].ID) })
	return result, nil
}

func (s *Service) ListStaffCapabilities(ctx context.Context, request StaffCapabilitiesRequest) ([]StaffCapabilitySummary, error) {
	if err := validateStaffRequest(ctx, request.SchemaVersion); err != nil {
		return nil, staffBridgeError(err)
	}
	if s == nil || s.staff == nil {
		return nil, staffBridgeError(staff.WrapError(staff.ErrInternal, "staff service unavailable", nil))
	}
	items, err := s.staff.GetStaffCapabilities(ctx)
	if err != nil {
		return nil, staffBridgeError(err)
	}
	result := make([]StaffCapabilitySummary, len(items))
	for i, item := range items {
		result[i] = sanitizeStaffCapability(item)
	}
	sort.Slice(result, func(i, j int) bool { return string(result[i].Capability) < string(result[j].Capability) })
	return result, nil
}

func (s *Service) GetStaffAssignment(ctx context.Context, request StaffAssignmentRequest) (StaffAssignmentSummary, error) {
	if err := validateStaffRequest(ctx, request.SchemaVersion); err != nil {
		return StaffAssignmentSummary{}, staffBridgeError(err)
	}
	if s == nil || s.staff == nil {
		return StaffAssignmentSummary{}, staffBridgeError(staff.WrapError(staff.ErrInternal, "staff service unavailable", nil))
	}
	assignment, err := s.staff.GetStaffAssignment(ctx, request.StaffID)
	if err != nil {
		return StaffAssignmentSummary{}, staffBridgeError(err)
	}
	return sanitizeStaffAssignment(assignment), nil
}

func sanitizeStaffSummary(summary StaffSummary) StaffSummary {
	return StaffSummary{
		ID:            summary.ID,
		Name:          summary.Name,
		Role:          summary.Role,
		Capabilities:  append([]staff.Capability(nil), summary.Capabilities...),
		WorkspaceID:   summary.WorkspaceID,
		ProjectID:     summary.ProjectID,
		WorktreeID:    summary.WorktreeID,
		Lifecycle:     summary.Lifecycle,
		Availability:  summary.Availability,
		SchemaVersion: StaffBridgeSchemaVersion,
		CreatedAt:     summary.CreatedAt,
		UpdatedAt:     summary.UpdatedAt,
	}
}

func sanitizeStaffCapability(item StaffCapabilitySummary) StaffCapabilitySummary {
	item.RequiredTools = append([]string(nil), item.RequiredTools...)
	item.PreferredEngines = append([]string(nil), item.PreferredEngines...)
	item.AllowedEngines = append([]string(nil), item.AllowedEngines...)
	item.SchemaVersion = StaffBridgeSchemaVersion
	return item
}

func sanitizeStaffAssignment(item StaffAssignmentSummary) StaffAssignmentSummary {
	item.SchemaVersion = StaffBridgeSchemaVersion
	return item
}
