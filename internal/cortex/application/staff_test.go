package application

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/RahmatHadinata23758051/CortexOS/internal/cortex/staff"
)

type mockStaffPort struct {
	staffDefs []StaffSummary
	caps      []StaffCapabilitySummary
}

func (m *mockStaffPort) GetStaff(ctx context.Context, id staff.StaffID) (StaffSummary, error) {
	if err := ctx.Err(); err != nil {
		return StaffSummary{}, err
	}
	for _, s := range m.staffDefs {
		if s.ID == id {
			return s, nil
		}
	}
	return StaffSummary{}, staff.WrapError(staff.ErrNotFound, "staff not found", nil)
}

func (m *mockStaffPort) ListStaff(ctx context.Context, filter StaffListFilter) ([]StaffSummary, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	var res []StaffSummary
	for _, s := range m.staffDefs {
		if filter.WorkspaceID != nil && s.WorkspaceID != *filter.WorkspaceID {
			continue
		}
		if filter.Role != nil && s.Role != *filter.Role {
			continue
		}
		if filter.ActiveOnly && s.Lifecycle != staff.LifecycleActive {
			continue
		}
		res = append(res, s)
	}
	return res, nil
}

func (m *mockStaffPort) ListStaffByWorkspace(ctx context.Context, workspaceID staff.WorkspaceID) ([]StaffSummary, error) {
	return m.ListStaff(ctx, StaffListFilter{WorkspaceID: &workspaceID})
}

func (m *mockStaffPort) GetStaffCapabilities(ctx context.Context) ([]StaffCapabilitySummary, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return m.caps, nil
}

func (m *mockStaffPort) GetStaffAssignment(ctx context.Context, id staff.StaffID) (StaffAssignmentSummary, error) {
	if err := ctx.Err(); err != nil {
		return StaffAssignmentSummary{}, err
	}
	for _, s := range m.staffDefs {
		if s.ID == id {
			return StaffAssignmentSummary{
				StaffID:       s.ID,
				WorkspaceID:   s.WorkspaceID,
				ProjectID:     s.ProjectID,
				WorktreeID:    s.WorktreeID,
				SchemaVersion: StaffBridgeSchemaVersion,
			}, nil
		}
	}
	return StaffAssignmentSummary{}, staff.WrapError(staff.ErrNotFound, "staff assignment not found", nil)
}

func TestStaffBridgeSafeSerialization(t *testing.T) {
	now := time.Now().UTC()
	summary := StaffSummary{
		ID:            staff.StaffID("staff-dev-1"),
		Name:          "Lead Implementer",
		Role:          staff.RoleImplementer,
		Capabilities:  []staff.Capability{"coding", "test_run"},
		WorkspaceID:   staff.WorkspaceID("ws-main"),
		ProjectID:     staff.ProjectID("proj-core"),
		WorktreeID:    staff.WorktreeID("wt-1"),
		Lifecycle:     staff.LifecycleActive,
		Availability:  staff.AvailabilityAvailable,
		SchemaVersion: StaffBridgeSchemaVersion,
		CreatedAt:     now,
		UpdatedAt:     now,
	}

	data, err := json.Marshal(summary)
	if err != nil {
		t.Fatalf("failed to marshal staff summary: %v", err)
	}

	str := string(data)
	forbiddenWords := []string{
		"workerId", "processId", "pid", "cliIdentity", "processHandle",
		"secret", "token", "password", "credential", "permission",
		"providerOutput", "selfApprove",
	}
	for _, word := range forbiddenWords {
		if strings.Contains(strings.ToLower(str), strings.ToLower(word)) {
			t.Errorf("serialized staff summary contains forbidden token %q: %s", word, str)
		}
	}
}

func TestStaffBridgeOperations(t *testing.T) {
	now := time.Now().UTC()
	mock := &mockStaffPort{
		staffDefs: []StaffSummary{
			{
				ID:            "staff-1",
				Name:          "Staff One",
				Role:          staff.RoleImplementer,
				Capabilities:  []staff.Capability{"coding"},
				WorkspaceID:   "ws-alpha",
				Lifecycle:     staff.LifecycleActive,
				Availability:  staff.AvailabilityAvailable,
				SchemaVersion: StaffBridgeSchemaVersion,
				CreatedAt:     now,
				UpdatedAt:     now,
			},
			{
				ID:            "staff-2",
				Name:          "Staff Two",
				Role:          staff.RoleReviewer,
				Capabilities:  []staff.Capability{"review"},
				WorkspaceID:   "ws-beta",
				Lifecycle:     staff.LifecycleActive,
				Availability:  staff.AvailabilityBusy,
				SchemaVersion: StaffBridgeSchemaVersion,
				CreatedAt:     now,
				UpdatedAt:     now,
			},
		},
		caps: []StaffCapabilitySummary{
			{
				Capability:    "coding",
				Description:   "General coding",
				RequiredTools: []string{"coding"},
				SchemaVersion: StaffBridgeSchemaVersion,
			},
		},
	}

	svc := NewServiceWithStaff(mock)
	ctx := context.Background()

	// 1. GetStaffSummary
	res, err := svc.GetStaffSummary(ctx, StaffRequest{
		SchemaVersion: StaffBridgeSchemaVersion,
		StaffID:       "staff-1",
	})
	if err != nil {
		t.Fatalf("GetStaffSummary failed: %v", err)
	}
	if res.ID != "staff-1" || res.Name != "Staff One" {
		t.Errorf("unexpected staff summary: %+v", res)
	}

	// 2. ListStaffSummaries
	list, err := svc.ListStaffSummaries(ctx, StaffListRequest{
		SchemaVersion: StaffBridgeSchemaVersion,
	})
	if err != nil {
		t.Fatalf("ListStaffSummaries failed: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("expected 2 staff members, got %d", len(list))
	}

	// 3. ListStaffByWorkspace
	wsList, err := svc.ListStaffByWorkspace(ctx, StaffByWorkspaceRequest{
		SchemaVersion: StaffBridgeSchemaVersion,
		WorkspaceID:   "ws-alpha",
	})
	if err != nil {
		t.Fatalf("ListStaffByWorkspace failed: %v", err)
	}
	if len(wsList) != 1 || wsList[0].ID != "staff-1" {
		t.Fatalf("expected staff-1 in ws-alpha, got %+v", wsList)
	}

	// 4. ListStaffCapabilities
	capList, err := svc.ListStaffCapabilities(ctx, StaffCapabilitiesRequest{
		SchemaVersion: StaffBridgeSchemaVersion,
	})
	if err != nil {
		t.Fatalf("ListStaffCapabilities failed: %v", err)
	}
	if len(capList) != 1 || capList[0].Capability != "coding" {
		t.Fatalf("expected coding capability, got %+v", capList)
	}

	// 5. GetStaffAssignment
	assign, err := svc.GetStaffAssignment(ctx, StaffAssignmentRequest{
		SchemaVersion: StaffBridgeSchemaVersion,
		StaffID:       "staff-1",
	})
	if err != nil {
		t.Fatalf("GetStaffAssignment failed: %v", err)
	}
	if assign.WorkspaceID != "ws-alpha" {
		t.Errorf("expected ws-alpha workspace, got %s", assign.WorkspaceID)
	}

	// 6. Schema version failure
	_, err = svc.GetStaffSummary(ctx, StaffRequest{
		SchemaVersion: "cortexos.staff.v999",
		StaffID:       "staff-1",
	})
	if err == nil {
		t.Fatal("expected error for unsupported schema version")
	}

	// 7. Context cancellation
	cancCtx, cancel := context.WithCancel(ctx)
	cancel()
	_, err = svc.GetStaffSummary(cancCtx, StaffRequest{
		SchemaVersion: StaffBridgeSchemaVersion,
		StaffID:       "staff-1",
	})
	if err == nil {
		t.Fatal("expected error for canceled context")
	}
	if !errors.Is(err, context.Canceled) {
		var bridgeErr *StaffBridgeError
		if !errors.As(err, &bridgeErr) || bridgeErr.Code != "staff.canceled" {
			t.Errorf("expected canceled error code, got %v", err)
		}
	}
}
