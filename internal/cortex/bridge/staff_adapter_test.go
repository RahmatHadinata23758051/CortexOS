package bridge

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/RahmatHadinata23758051/CortexOS/internal/cortex/application"
	"github.com/RahmatHadinata23758051/CortexOS/internal/cortex/staff"
	staffsqlite "github.com/RahmatHadinata23758051/CortexOS/internal/cortex/staff/sqlite"
)

func openTestStaffStore(t *testing.T) (*staffsqlite.Store, func()) {
	t.Helper()
	dir := t.TempDir()
	ctx := context.Background()
	store, err := staffsqlite.Open(ctx, filepath.Join(dir, "staff.db"))
	if err != nil {
		t.Fatalf("open test staff store: %v", err)
	}
	return store, func() { _ = store.Close() }
}

func TestStaffAdapterConstruction(t *testing.T) {
	_, err := NewStaffAdapter(StaffAdapterConfig{})
	if err == nil {
		t.Fatal("expected error when store is nil, got nil")
	}

	store, cleanup := openTestStaffStore(t)
	defer cleanup()

	adapter, err := NewStaffAdapter(StaffAdapterConfig{Store: store})
	if err != nil {
		t.Fatalf("unexpected error constructing adapter: %v", err)
	}
	if adapter == nil {
		t.Fatal("expected non-nil adapter")
	}
}

func TestStaffAdapterSeedAndQueries(t *testing.T) {
	store, cleanup := openTestStaffStore(t)
	defer cleanup()

	ctx := context.Background()

	// 1. Seed
	if err := SeedDefaultStaff(ctx, store); err != nil {
		t.Fatalf("seed default staff: %v", err)
	}

	// Idempotent seed
	if err := SeedDefaultStaff(ctx, store); err != nil {
		t.Fatalf("idempotent seed should succeed: %v", err)
	}

	adapter, err := NewStaffAdapter(StaffAdapterConfig{Store: store})
	if err != nil {
		t.Fatalf("new staff adapter: %v", err)
	}

	// 2. List all
	all, err := adapter.ListStaff(ctx, application.StaffListFilter{})
	if err != nil {
		t.Fatalf("list all staff: %v", err)
	}
	if len(all) != 4 {
		t.Fatalf("expected 4 seeded staff members, got %d", len(all))
	}

	// 3. Get single
	coord, err := adapter.GetStaff(ctx, "staff-coordinator")
	if err != nil {
		t.Fatalf("get staff-coordinator: %v", err)
	}
	if coord.Name != "Lead Coordinator" || coord.Role != staff.RoleCoordinator {
		t.Fatalf("unexpected coordinator summary: %+v", coord)
	}

	// 4. Filter by role
	planners, err := adapter.ListStaff(ctx, application.StaffListFilter{Role: ptrTo(staff.RolePlanner)})
	if err != nil {
		t.Fatalf("list staff by role: %v", err)
	}
	if len(planners) != 1 || planners[0].ID != "staff-planner" {
		t.Fatalf("expected 1 planner, got %v", planners)
	}

	// 5. List by workspace
	byWs, err := adapter.ListStaffByWorkspace(ctx, "default")
	if err != nil {
		t.Fatalf("list staff by workspace: %v", err)
	}
	if len(byWs) != 4 {
		t.Fatalf("expected 4 staff members in default workspace, got %d", len(byWs))
	}

	// 6. Get assignment
	assign, err := adapter.GetStaffAssignment(ctx, "staff-implementer")
	if err != nil {
		t.Fatalf("get staff assignment: %v", err)
	}
	if assign.WorkspaceID != "default" || assign.StaffID != "staff-implementer" {
		t.Fatalf("unexpected assignment: %+v", assign)
	}

	// 7. Get capabilities
	caps, err := adapter.GetStaffCapabilities(ctx)
	if err != nil {
		t.Fatalf("get staff capabilities: %v", err)
	}
	if len(caps) == 0 {
		t.Fatal("expected registered staff capabilities, got 0")
	}
}

func ptrTo[T any](v T) *T {
	return &v
}
