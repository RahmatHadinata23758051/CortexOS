package staff

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/RahmatHadinata23758051/CortexOS/internal/cortex/harness"
)

func newTestScheduler(t *testing.T) (*Scheduler, Store, *harness.BasicRouter) {
	t.Helper()
	router, store, hRouter := setupTestRouter(t)
	sched, err := NewScheduler(store, router)
	if err != nil {
		t.Fatalf("NewScheduler failed: %v", err)
	}
	return sched, store, hRouter
}

func createStaffDef(t *testing.T, store Store, id string, role Role, caps []Capability, ws string, avail AvailabilityState) Definition {
	t.Helper()
	now := time.Now().UTC()
	def, err := NewDefinition(Definition{
		Identity:      Identity{ID: StaffID(id), Name: "Staff " + id},
		Role:          role,
		Capabilities:  caps,
		Permissions:   PermissionSet{{ID: "p1", Action: "read", Resource: "src/*", Effect: EffectAllow}},
		Workspace:     WorkspaceAssignment{WorkspaceID: WorkspaceID(ws)},
		Lifecycle:     LifecycleActive,
		Availability:  avail,
		CreatedAt:     now,
		UpdatedAt:     now,
		SchemaVersion: ContractVersion,
	})
	if err != nil {
		t.Fatalf("invalid staff definition: %v", err)
	}
	saved, err := store.SaveDefinition(context.Background(), def)
	if err != nil {
		t.Fatalf("failed to save staff definition: %v", err)
	}
	return saved
}

func TestSchedulerSubmitAndAssign(t *testing.T) {
	sched, store, _ := newTestScheduler(t)
	ctx := context.Background()

	createStaffDef(t, store, "staff-available", RoleImplementer, []Capability{"coding"}, "ws-1", AvailabilityAvailable)
	createStaffDef(t, store, "staff-busy", RoleImplementer, []Capability{"coding"}, "ws-1", AvailabilityAvailable)

	// First task assigned immediately
	a1, err := sched.Submit(ctx, AssignmentRequest{
		TaskID:               "task-1",
		WorkspaceID:          "ws-1",
		RequiredCapabilities: []Capability{"coding"},
		Priority:             PriorityNormal,
		ResourceAvailable:    true,
	})
	if err != nil {
		t.Fatalf("Submit task-1: %v", err)
	}
	if a1.Status != AssignmentAssigned || a1.StaffID != "staff-available" {
		t.Errorf("expected assigned to staff-available, got %s (%s)", a1.StaffID, a1.Status)
	}

	// Second task queued because only one available (staff-busy is available but fair scheduling picks least-assigned which is staff-busy after first assignment)
	// Wait, staff-busy is also Available... fair scheduling should pick least assigned = staff-busy
	// Actually both are available initially. First assignment picks staff-available (lower ID in tie-break). Second should pick staff-busy.
	a2, err := sched.Submit(ctx, AssignmentRequest{
		TaskID:               "task-2",
		WorkspaceID:          "ws-1",
		RequiredCapabilities: []Capability{"coding"},
		Priority:             PriorityNormal,
		ResourceAvailable:    true,
	})
	if err != nil {
		t.Fatalf("Submit task-2: %v", err)
	}
	if a2.Status != AssignmentAssigned || a2.StaffID != "staff-busy" {
		t.Errorf("expected assigned to staff-busy (least assigned), got %s (%s)", a2.StaffID, a2.Status)
	}

	// Verify assignment counts
	if _, err := sched.Get("task-1"); err != nil {
		t.Errorf("Get task-1: %v", err)
	}
	if _, err := sched.Get("task-2"); err != nil {
		t.Errorf("Get task-2: %v", err)
	}
}

func TestSchedulerBusyQueuing(t *testing.T) {
	sched, store, _ := newTestScheduler(t)
	ctx := context.Background()

	createStaffDef(t, store, "staff-1", RoleImplementer, []Capability{"coding"}, "ws-1", AvailabilityAvailable)

	// First assignment
	a1, err := sched.Submit(ctx, AssignmentRequest{TaskID: "task-1", WorkspaceID: "ws-1", RequiredCapabilities: []Capability{"coding"}, Priority: PriorityNormal, ResourceAvailable: true})
	if err != nil {
		t.Fatalf("task-1: %v", err)
	}
	if a1.Status != AssignmentAssigned || a1.StaffID != "staff-1" {
		t.Errorf("expected assigned to staff-1, got %s", a1.StaffID)
	}

	// Second task queued (staff-1 is busy)
	a2, err := sched.Submit(ctx, AssignmentRequest{TaskID: "task-2", WorkspaceID: "ws-1", RequiredCapabilities: []Capability{"coding"}, Priority: PriorityNormal, ResourceAvailable: true})
	if err != nil {
		t.Fatalf("task-2: %v", err)
	}
	if a2.Status != AssignmentQueued {
		t.Errorf("expected queued, got %s", a2.Status)
	}

	// Release first and verify scheduling picks up queued
	if err := sched.Release(ctx, "task-1"); err != nil {
		t.Fatalf("Release task-1: %v", err)
	}

	assigned, err := sched.SchedulePending(ctx)
	if err != nil {
		t.Fatalf("SchedulePending: %v", err)
	}
	if len(assigned) != 1 || assigned[0].Request.TaskID != "task-2" || assigned[0].Status != AssignmentAssigned {
		t.Errorf("expected task-2 assigned, got %v", assigned)
	}
}

func TestSchedulerFairnessTieBreak(t *testing.T) {
	sched, store, _ := newTestScheduler(t)
	ctx := context.Background()

	// Two identical staff members - should alternate fairly
	createStaffDef(t, store, "staff-a", RoleImplementer, []Capability{"coding"}, "ws-1", AvailabilityAvailable)
	createStaffDef(t, store, "staff-b", RoleImplementer, []Capability{"coding"}, "ws-1", AvailabilityAvailable)

	// First two assignments
	for i := 1; i <= 2; i++ {
		a, err := sched.Submit(ctx, AssignmentRequest{TaskID: "task-" + string(rune('0'+i)), WorkspaceID: "ws-1", RequiredCapabilities: []Capability{"coding"}, Priority: PriorityNormal, ResourceAvailable: true})
		if err != nil {
			t.Fatalf("Submit %d: %v", i, err)
		}
		if a.Status != AssignmentAssigned {
			t.Errorf("task %d: expected assigned, got %s", i, a.Status)
		}
	}

	// Both assigned, one each
	assign1, _ := sched.Get("task-1")
	assign2, _ := sched.Get("task-2")
	if (assign1.StaffID == "staff-a" && assign2.StaffID != "staff-b") || (assign1.StaffID == "staff-b" && assign2.StaffID != "staff-a") {
		t.Errorf("expected one each, got %s and %s", assign1.StaffID, assign2.StaffID)
	}

	// Release both and submit 2 more - should alternate again
	sched.Release(ctx, "task-1")
	sched.Release(ctx, "task-2")
	for i := 3; i <= 4; i++ {
		a, err := sched.Submit(ctx, AssignmentRequest{TaskID: "task-" + string(rune('0'+i)), WorkspaceID: "ws-1", RequiredCapabilities: []Capability{"coding"}, Priority: PriorityNormal, ResourceAvailable: true})
		if err != nil {
			t.Fatalf("Submit %d: %v", i, err)
		}
		if a.Status != AssignmentAssigned {
			t.Errorf("task %d: expected assigned, got %s", i, a.Status)
		}
	}
}

func TestSchedulerPriorityOrdering(t *testing.T) {
	sched, store, _ := newTestScheduler(t)
	ctx := context.Background()

	createStaffDef(t, store, "staff-1", RoleImplementer, []Capability{"coding"}, "ws-1", AvailabilityAvailable)

	// First assignment occupies the only staff member; following requests queue.
	sched.Submit(ctx, AssignmentRequest{TaskID: "task-active", WorkspaceID: "ws-1", RequiredCapabilities: []Capability{"coding"}, Priority: PriorityNormal, ResourceAvailable: true})
	// Submit low priority first
	sched.Submit(ctx, AssignmentRequest{TaskID: "task-low", WorkspaceID: "ws-1", RequiredCapabilities: []Capability{"coding"}, Priority: PriorityLow, ResourceAvailable: true})
	// Submit high priority
	sched.Submit(ctx, AssignmentRequest{TaskID: "task-high", WorkspaceID: "ws-1", RequiredCapabilities: []Capability{"coding"}, Priority: PriorityHigh, ResourceAvailable: true})

	// Release staff and schedule pending - high priority should go first
	sched.Release(ctx, "task-active")

	assigned, err := sched.SchedulePending(ctx)
	if err != nil {
		t.Fatalf("SchedulePending: %v", err)
	}
	if len(assigned) != 1 {
		t.Fatalf("expected 1 assigned due to one slot, got %d", len(assigned))
	}
	if assigned[0].Request.TaskID != "task-high" {
		t.Errorf("expected high priority first, got %s", assigned[0].Request.TaskID)
	}
}

func TestSchedulerRejectionReasons(t *testing.T) {
	sched, store, _ := newTestScheduler(t)
	ctx := context.Background()

	createStaffDef(t, store, "staff-1", RoleImplementer, []Capability{"coding"}, "ws-1", AvailabilityAvailable)
	createStaffDef(t, store, "staff-2", RoleReviewer, []Capability{"review"}, "ws-1", AvailabilityAvailable)

	// Test capability mismatch
	a, err := sched.Submit(ctx, AssignmentRequest{TaskID: "task-cap", WorkspaceID: "ws-1", RequiredCapabilities: []Capability{"security_audit"}, Priority: PriorityNormal, ResourceAvailable: true})
	if err == nil || ErrorCodeOf(err) != ErrNotFound {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
	if len(a.Rejections) == 0 {
		t.Error("expected rejection reasons")
	}
	found := false
	for _, r := range a.Rejections {
		if r.Code == RejectCapability {
			found = true
		}
	}
	if !found {
		t.Errorf("expected RejectCapability in rejections: %v", a.Rejections)
	}

	// Test permission denial
	store2 := newMemStore()
	router2, _ := NewStaffEngineRouter(store2, harness.NewBasicRouter(harness.DefaultRoutingPolicy()), DefaultCapabilityRegistry())
	sched2, _ := NewScheduler(store2, router2)
	def, _ := NewDefinition(Definition{Identity: Identity{ID: "staff-deny", Name: "Staff Deny"}, Role: RoleImplementer, Capabilities: []Capability{"coding"}, Permissions: PermissionSet{{ID: "p1", Action: "read", Resource: "src/*", Effect: EffectAllow}, {ID: "p2", Action: "write", Resource: "secrets/*", Effect: EffectDeny}}, Workspace: WorkspaceAssignment{WorkspaceID: "ws-1"}, Lifecycle: LifecycleActive, Availability: AvailabilityAvailable, CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC(), SchemaVersion: ContractVersion})
	store2.SaveDefinition(ctx, def)
	_, err = sched2.Submit(ctx, AssignmentRequest{TaskID: "task-perm", WorkspaceID: "ws-1", RequiredCapabilities: []Capability{"coding"}, Action: "write", Resource: "secrets/key.pem", Priority: PriorityNormal, ResourceAvailable: true})
	if err == nil || !IsPermissionDenied(err) {
		t.Errorf("expected permission denied, got %v", err)
	}

	// Test workspace mismatch
	aWS, err := sched.Submit(ctx, AssignmentRequest{TaskID: "task-ws", WorkspaceID: "ws-2", RequiredCapabilities: []Capability{"coding"}, Priority: PriorityNormal, ResourceAvailable: true})
	if err == nil || ErrorCodeOf(err) != ErrNotFound {
		t.Fatalf("expected ErrNotFound for workspace mismatch, got %v", err)
	}
	found = false
	for _, r := range aWS.Rejections {
		if r.Code == RejectWorkspace {
			found = true
		}
	}
	if !found {
		t.Errorf("expected RejectWorkspace in rejections")
	}

	// Test inactive staff
	inactiveDef, _ := NewDefinition(Definition{Identity: Identity{ID: "staff-inactive", Name: "Staff Inactive"}, Role: RoleImplementer, Capabilities: []Capability{"coding"}, Permissions: PermissionSet{{ID: "p1", Action: "read", Resource: "src/*", Effect: EffectAllow}}, Workspace: WorkspaceAssignment{WorkspaceID: "ws-1"}, Lifecycle: LifecycleInactive, Availability: AvailabilityAvailable, CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC(), SchemaVersion: ContractVersion})
	store.SaveDefinition(ctx, inactiveDef)
	_, err = sched.Submit(ctx, AssignmentRequest{TaskID: "task-inactive", StaffID: "staff-inactive", WorkspaceID: "ws-1", RequiredCapabilities: []Capability{"coding"}, Priority: PriorityNormal, ResourceAvailable: true})
	if err == nil || ErrorCodeOf(err) != ErrNotFound {
		t.Fatalf("expected ErrNotFound for inactive staff, got %v", err)
	}
}

func TestSchedulerCancelAndReassign(t *testing.T) {
	sched, store, _ := newTestScheduler(t)
	ctx := context.Background()

	createStaffDef(t, store, "staff-1", RoleImplementer, []Capability{"coding"}, "ws-1", AvailabilityAvailable)
	createStaffDef(t, store, "staff-2", RoleImplementer, []Capability{"coding"}, "ws-1", AvailabilityAvailable)

	// Assign to staff-1
	a1, err := sched.Submit(ctx, AssignmentRequest{TaskID: "task-1", WorkspaceID: "ws-1", RequiredCapabilities: []Capability{"coding"}, Priority: PriorityNormal, ResourceAvailable: true})
	if err != nil {
		t.Fatalf("Submit: %v", err)
	}
	if a1.StaffID != "staff-1" {
		t.Errorf("expected staff-1, got %s", a1.StaffID)
	}

	// Cancel and verify staff-1 is free
	if err := sched.Cancel(ctx, "task-1"); err != nil {
		t.Fatalf("Cancel: %v", err)
	}
	a1, _ = sched.Get("task-1")
	if a1.Status != AssignmentCanceled {
		t.Errorf("expected canceled, got %s", a1.Status)
	}

	// Reassign - should be able to get staff-1 again
	a2, err := sched.Reassign(ctx, "task-1", AssignmentRequest{TaskID: "task-1", WorkspaceID: "ws-1", RequiredCapabilities: []Capability{"coding"}, Priority: PriorityNormal, ResourceAvailable: true})
	if err != nil {
		t.Fatalf("Reassign: %v", err)
	}
	if a2.Status != AssignmentAssigned {
		t.Errorf("expected assigned, got %s", a2.Status)
	}

	// Reassignment event should be emitted
	// (We can't easily test events without exposing sink, but we can verify the final state)
}

func TestSchedulerAvailabilityStates(t *testing.T) {
	sched, store, _ := newTestScheduler(t)
	ctx := context.Background()

	// Test unavailable staff
	createStaffDef(t, store, "staff-unavail", RoleImplementer, []Capability{"coding"}, "ws-1", AvailabilityUnavailable)
	_, err := sched.Submit(ctx, AssignmentRequest{TaskID: "task-u", WorkspaceID: "ws-1", RequiredCapabilities: []Capability{"coding"}, Priority: PriorityNormal, ResourceAvailable: true})
	if err == nil || ErrorCodeOf(err) != ErrNotFound {
		t.Fatalf("expected ErrNotFound for unavailable, got %v", err)
	}

	// Test offline staff
	store2 := newMemStore()
	router2, _ := NewStaffEngineRouter(store2, harness.NewBasicRouter(harness.DefaultRoutingPolicy()), DefaultCapabilityRegistry())
	sched2, _ := NewScheduler(store2, router2)
	defOffline, _ := NewDefinition(Definition{Identity: Identity{ID: "staff-offline", Name: "Staff Offline"}, Role: RoleImplementer, Capabilities: []Capability{"coding"}, Permissions: PermissionSet{{ID: "p1", Action: "read", Resource: "src/*", Effect: EffectAllow}}, Workspace: WorkspaceAssignment{WorkspaceID: "ws-1"}, Lifecycle: LifecycleActive, Availability: AvailabilityOffline, CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC(), SchemaVersion: ContractVersion})
	store2.SaveDefinition(ctx, defOffline)
	_, err = sched2.Submit(ctx, AssignmentRequest{TaskID: "task-o", WorkspaceID: "ws-1", RequiredCapabilities: []Capability{"coding"}, Priority: PriorityNormal, ResourceAvailable: true})
	if err == nil || ErrorCodeOf(err) != ErrNotFound {
		t.Fatalf("expected ErrNotFound for offline, got %v", err)
	}

	// Test busy staff (explicit busy state)
	store3 := newMemStore()
	router3, _ := NewStaffEngineRouter(store3, harness.NewBasicRouter(harness.DefaultRoutingPolicy()), DefaultCapabilityRegistry())
	sched3, _ := NewScheduler(store3, router3)
	defBusy, _ := NewDefinition(Definition{Identity: Identity{ID: "staff-busy-state", Name: "Staff Busy"}, Role: RoleImplementer, Capabilities: []Capability{"coding"}, Permissions: PermissionSet{{ID: "p1", Action: "read", Resource: "src/*", Effect: EffectAllow}}, Workspace: WorkspaceAssignment{WorkspaceID: "ws-1"}, Lifecycle: LifecycleActive, Availability: AvailabilityBusy, CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC(), SchemaVersion: ContractVersion})
	store3.SaveDefinition(ctx, defBusy)
	a, err := sched3.Submit(ctx, AssignmentRequest{TaskID: "task-b", WorkspaceID: "ws-1", RequiredCapabilities: []Capability{"coding"}, Priority: PriorityNormal, ResourceAvailable: true})
	if err != nil {
		t.Fatalf("unexpected error for busy state: %v", err)
	}
	if a.Status != AssignmentQueued {
		t.Errorf("expected queued for busy state, got %s", a.Status)
	}
}

func TestSchedulerConcurrency(t *testing.T) {
	sched, store, _ := newTestScheduler(t)
	ctx := context.Background()

	createStaffDef(t, store, "staff-1", RoleImplementer, []Capability{"coding"}, "ws-1", AvailabilityAvailable)
	createStaffDef(t, store, "staff-2", RoleImplementer, []Capability{"coding"}, "ws-1", AvailabilityAvailable)
	createStaffDef(t, store, "staff-3", RoleImplementer, []Capability{"coding"}, "ws-1", AvailabilityAvailable)

	var wg sync.WaitGroup
	results := make(chan Assignment, 100)

	for i := 1; i <= 30; i++ {
		wg.Add(1)
		go func(taskNum int) {
			defer wg.Done()
			taskID := "task-conc-" + string(rune('0'+taskNum/10)) + string(rune('0'+taskNum%10))
			a, err := sched.Submit(ctx, AssignmentRequest{TaskID: taskID, WorkspaceID: "ws-1", RequiredCapabilities: []Capability{"coding"}, Priority: PriorityNormal, ResourceAvailable: true})
			if err != nil {
				results <- Assignment{Status: "error"}
				return
			}
			results <- a
		}(i)
	}
	wg.Wait()
	close(results)

	assigned := 0
	queued := 0
	errors := 0
	for a := range results {
		if a.Status == AssignmentAssigned {
			assigned++
		}
		if a.Status == AssignmentQueued {
			queued++
		}
		if a.Status == "error" {
			errors++
		}
	}
	if assigned+queued+errors != 30 {
		t.Errorf("expected 30 results, got %d assigned + %d queued + %d errors", assigned, queued, errors)
	}
	if assigned > 3 {
		t.Errorf("expected at most 3 concurrent, got %d", assigned)
	}
}
