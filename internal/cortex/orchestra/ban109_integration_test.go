package orchestra

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/RahmatHadinata23758051/CortexOS/internal/cortex/staff"
)

func newBAN109Scheduler(t *testing.T, defs ...staff.Definition) (*staff.Scheduler, *testStaffStore) {
	t.Helper()
	store := newTestStaffStore(t)
	for _, def := range defs {
		store.defs[def.ID] = def
	}
	scheduler, err := staff.NewScheduler(store, store)
	if err != nil {
		t.Fatalf("create scheduler: %v", err)
	}
	return scheduler, store
}

func ban109Definition(id string, availability staff.AvailabilityState, caps []staff.Capability, perms staff.PermissionSet) staff.Definition {
	now := time.Now().UTC()
	return staff.Definition{
		Identity:      staff.Identity{ID: staff.StaffID(id), Name: "Staff " + id},
		Role:          staff.RoleImplementer,
		Capabilities:  caps,
		Permissions:   perms,
		Workspace:     staff.WorkspaceAssignment{WorkspaceID: "ws-ban109", ProjectID: "project-ban109"},
		Lifecycle:     staff.LifecycleActive,
		Availability:  availability,
		CreatedAt:     now,
		UpdatedAt:     now,
		SchemaVersion: staff.ContractVersion,
	}
}

func ban109Request(taskID string) staff.AssignmentRequest {
	return staff.AssignmentRequest{
		TaskID:               taskID,
		WorkspaceID:          "ws-ban109",
		ProjectID:            "project-ban109",
		RequiredCapabilities: []staff.Capability{"coding"},
		Priority:             staff.PriorityNormal,
		ResourceAvailable:    true,
	}
}

func TestBAN109_Integration_MatchesScopeCapabilitiesAndAuditsAssignment(t *testing.T) {
	allowCoding := staff.PermissionSet{{ID: "coding", Action: "coding", Resource: "repo/*", Effect: staff.EffectAllow}}
	scheduler, store := newBAN109Scheduler(t,
		ban109Definition("staff-b", staff.AvailabilityAvailable, []staff.Capability{"coding"}, allowCoding),
		ban109Definition("staff-a", staff.AvailabilityAvailable, []staff.Capability{"coding"}, allowCoding),
	)

	taskStore := NewMemoryTaskStore()
	dispatcher := NewDispatcher(taskStore, nil)
	dispatcher.SetStaffScheduler(scheduler)

	assignment, err := scheduler.Submit(context.Background(), ban109Request("task-match"))
	if err != nil {
		t.Fatalf("submit matching task: %v", err)
	}
	if assignment.Status != staff.AssignmentAssigned {
		t.Fatalf("expected assigned status, got %s", assignment.Status)
	}
	if assignment.StaffID != "staff-a" {
		t.Fatalf("expected deterministic ID tie-break to choose staff-a, got %s", assignment.StaffID)
	}
	if assignment.Reason == "" || assignment.AssignmentID == "" {
		t.Fatalf("expected assignment reason and ID: %+v", assignment)
	}

	events, err := taskStore.ListEvents(context.Background(), "task-match")
	if err != nil {
		t.Fatalf("list assignment audit events: %v", err)
	}
	if len(events) != 1 || events[0].Type != EventTaskAssigned {
		t.Fatalf("expected one task.assigned audit event, got %+v", events)
	}
	_ = store
}

func TestBAN109_Integration_NoMatchHasExplicitRejectionReasons(t *testing.T) {
	allowCoding := staff.PermissionSet{{ID: "coding", Action: "coding", Resource: "repo/*", Effect: staff.EffectAllow}}
	scheduler, store := newBAN109Scheduler(t,
		ban109Definition("staff-capability", staff.AvailabilityAvailable, []staff.Capability{"review"}, allowCoding),
		ban109Definition("staff-workspace", staff.AvailabilityAvailable, []staff.Capability{"coding"}, allowCoding),
	)
	workspaceDef := store.defs["staff-workspace"]
	workspaceDef.Workspace.WorkspaceID = "other-workspace"
	store.defs["staff-workspace"] = workspaceDef

	request := ban109Request("task-no-match")
	assignment, err := scheduler.Submit(context.Background(), request)
	if err == nil {
		t.Fatal("expected no-match error")
	}
	if staff.ErrorCodeOf(err) != staff.ErrNotFound {
		t.Fatalf("expected staff.not_found, got %s (%v)", staff.ErrorCodeOf(err), err)
	}
	if assignment.Status != staff.AssignmentRejected {
		t.Fatalf("expected rejected assignment, got %s", assignment.Status)
	}
	if len(assignment.Rejections) == 0 {
		t.Fatal("expected explicit rejection reasons")
	}
	foundCapability := false
	for _, reason := range assignment.Rejections {
		if reason.Code == staff.RejectCapability {
			foundCapability = true
		}
	}
	if !foundCapability {
		t.Fatalf("expected capability rejection, got %+v", assignment.Rejections)
	}

	workspaceAssignment, workspaceErr := scheduler.Submit(context.Background(), staff.AssignmentRequest{
		TaskID:               "task-workspace",
		WorkspaceID:          "unknown-workspace",
		RequiredCapabilities: []staff.Capability{"coding"},
		Priority:             staff.PriorityNormal,
		ResourceAvailable:    true,
	})
	if workspaceErr == nil || workspaceAssignment.Status != staff.AssignmentRejected {
		t.Fatalf("expected workspace rejection, assignment=%+v err=%v", workspaceAssignment, workspaceErr)
	}
	foundWorkspace := false
	for _, reason := range workspaceAssignment.Rejections {
		if reason.Code == staff.RejectWorkspace {
			foundWorkspace = true
		}
	}
	if !foundWorkspace {
		t.Fatalf("expected workspace rejection, got %+v", workspaceAssignment.Rejections)
	}
}

func TestBAN109_Integration_BusyQueuesOfflineRejectsAndPendingWorkSchedules(t *testing.T) {
	allowCoding := staff.PermissionSet{{ID: "coding", Action: "coding", Resource: "repo/*", Effect: staff.EffectAllow}}
	scheduler, store := newBAN109Scheduler(t,
		ban109Definition("staff-busy", staff.AvailabilityBusy, []staff.Capability{"coding"}, allowCoding),
		ban109Definition("staff-offline", staff.AvailabilityOffline, []staff.Capability{"coding"}, allowCoding),
	)
	ctx := context.Background()

	queued, err := scheduler.Submit(ctx, ban109Request("task-queued"))
	if err != nil {
		t.Fatalf("busy staff should queue: %v", err)
	}
	if queued.Status != staff.AssignmentQueued {
		t.Fatalf("expected queued assignment, got %s", queued.Status)
	}
	if !hasBAN109Rejection(queued.Rejections, staff.RejectBusy) {
		t.Fatalf("expected busy reason, got %+v", queued.Rejections)
	}

	offline, err := scheduler.Submit(ctx, staff.AssignmentRequest{
		TaskID:               "task-offline",
		WorkspaceID:          "ws-ban109",
		ProjectID:            "project-ban109",
		StaffID:              "staff-offline",
		RequiredCapabilities: []staff.Capability{"coding"},
		Priority:             staff.PriorityNormal,
		ResourceAvailable:    true,
	})
	if err == nil || offline.Status != staff.AssignmentRejected {
		t.Fatalf("expected offline rejection, assignment=%+v err=%v", offline, err)
	}
	if !hasBAN109Rejection(offline.Rejections, staff.RejectUnavailable) {
		t.Fatalf("expected unavailable reason, got %+v", offline.Rejections)
	}

	// A queued request can be promoted after the Staff definition is made available.
	busyDef := store.defs["staff-busy"]
	busyDef.Availability = staff.AvailabilityAvailable
	store.defs["staff-busy"] = busyDef

	scheduled, err := scheduler.SchedulePending(ctx)
	if err != nil {
		t.Fatalf("schedule pending: %v", err)
	}
	if len(scheduled) != 1 || scheduled[0].Status != staff.AssignmentAssigned || scheduled[0].StaffID != "staff-busy" {
		t.Fatalf("expected queued task to be assigned to staff-busy, got %+v", scheduled)
	}
	retrieved, err := scheduler.Get("task-queued")
	if err != nil || retrieved.Status != staff.AssignmentAssigned {
		t.Fatalf("expected task-queued to be assigned, retrieved=%+v err=%v", retrieved, err)
	}
}

func hasBAN109Rejection(reasons []staff.RejectionReason, code string) bool {
	for _, reason := range reasons {
		if reason.Code == code {
			return true
		}
	}
	return false
}

func TestBAN109_Integration_FairnessCancellationAndReassignment(t *testing.T) {
	allowCoding := staff.PermissionSet{{ID: "coding", Action: "coding", Resource: "repo/*", Effect: staff.EffectAllow}}
	scheduler, _ := newBAN109Scheduler(t,
		ban109Definition("staff-a", staff.AvailabilityAvailable, []staff.Capability{"coding"}, allowCoding),
		ban109Definition("staff-b", staff.AvailabilityAvailable, []staff.Capability{"coding"}, allowCoding),
	)
	ctx := context.Background()

	first, err := scheduler.Submit(ctx, ban109Request("task-1"))
	if err != nil {
		t.Fatal(err)
	}
	second, err := scheduler.Submit(ctx, ban109Request("task-2"))
	if err != nil {
		t.Fatal(err)
	}
	if first.StaffID == second.StaffID {
		t.Fatalf("expected fair distribution, got %s for both tasks", first.StaffID)
	}

	if err := scheduler.Cancel(ctx, "task-1"); err != nil {
		t.Fatalf("cancel assignment: %v", err)
	}
	canceled, err := scheduler.Get("task-1")
	if err != nil || canceled.Status != staff.AssignmentCanceled {
		t.Fatalf("expected canceled assignment, assignment=%+v err=%v", canceled, err)
	}

	reassignedRequest := ban109Request("task-1")
	reassignedRequest.StaffID = "staff-a"
	reassigned, err := scheduler.Reassign(ctx, "task-1", reassignedRequest)
	if err != nil {
		t.Fatalf("reassign task: %v", err)
	}
	if reassigned.Status != staff.AssignmentAssigned || reassigned.StaffID != "staff-a" {
		t.Fatalf("expected reassignment to staff-a, got %+v", reassigned)
	}
	if reassigned.AssignmentID == first.AssignmentID {
		t.Fatal("expected reassignment to create a fresh assignment ID")
	}
}

func TestBAN109_Integration_ConcurrentTransitionsRemainConsistent(t *testing.T) {
	allowCoding := staff.PermissionSet{{ID: "coding", Action: "coding", Resource: "repo/*", Effect: staff.EffectAllow}}
	scheduler, _ := newBAN109Scheduler(t,
		ban109Definition("staff-a", staff.AvailabilityAvailable, []staff.Capability{"coding"}, allowCoding),
		ban109Definition("staff-b", staff.AvailabilityAvailable, []staff.Capability{"coding"}, allowCoding),
		ban109Definition("staff-c", staff.AvailabilityAvailable, []staff.Capability{"coding"}, allowCoding),
	)
	ctx := context.Background()

	const taskCount = 30
	results := make(chan staff.Assignment, taskCount)
	errorsCh := make(chan error, taskCount)
	var submitWG sync.WaitGroup
	for i := 0; i < taskCount; i++ {
		submitWG.Add(1)
		go func(index int) {
			defer submitWG.Done()
			assignment, err := scheduler.Submit(ctx, ban109Request(fmt.Sprintf("task-concurrent-%02d", index)))
			if err != nil {
				errorsCh <- err
				return
			}
			results <- assignment
		}(i)
	}
	submitWG.Wait()
	close(results)
	close(errorsCh)

	assigned, queued := 0, 0
	for assignment := range results {
		switch assignment.Status {
		case staff.AssignmentAssigned:
			assigned++
		case staff.AssignmentQueued:
			queued++
		default:
			t.Fatalf("unexpected concurrent assignment state: %s", assignment.Status)
		}
	}
	for err := range errorsCh {
		t.Fatalf("unexpected concurrent submit error: %v", err)
	}
	if assigned != 3 || queued != taskCount-assigned {
		t.Fatalf("expected three reservations and remaining queued, assigned=%d queued=%d", assigned, queued)
	}

	// Concurrent cancellation of one assignment must have exactly one winner.
	var cancelWG sync.WaitGroup
	cancelResults := make(chan error, 10)
	for i := 0; i < cap(cancelResults); i++ {
		cancelWG.Add(1)
		go func() {
			defer cancelWG.Done()
			cancelResults <- scheduler.Cancel(ctx, "task-concurrent-00")
		}()
	}
	cancelWG.Wait()
	close(cancelResults)
	canceledCount := 0
	conflictCount := 0
	for err := range cancelResults {
		if err == nil {
			canceledCount++
		} else if staff.ErrorCodeOf(err) == staff.ErrConflict {
			conflictCount++
		} else {
			t.Fatalf("unexpected concurrent cancel error: %v", err)
		}
	}
	if canceledCount != 1 || conflictCount != cap(cancelResults)-1 {
		t.Fatalf("expected one successful cancel and conflicts, canceled=%d conflicts=%d", canceledCount, conflictCount)
	}
	final, err := scheduler.Get("task-concurrent-00")
	if err != nil || final.Status != staff.AssignmentCanceled {
		t.Fatalf("expected stable canceled state, assignment=%+v err=%v", final, err)
	}
}

func TestBAN109_Integration_DispatcherPreservesInspectorAuthority(t *testing.T) {
	allowCoding := staff.PermissionSet{{ID: "coding", Action: "coding", Resource: "repo/*", Effect: staff.EffectAllow}}
	scheduler, store := newBAN109Scheduler(t, ban109Definition("staff-a", staff.AvailabilityAvailable, []staff.Capability{"coding"}, allowCoding))
	taskStore := NewMemoryTaskStore()
	dispatcher := NewDispatcher(taskStore, nil)
	dispatcher.SetStaffScheduler(scheduler)

	task := Task{
		ID: "task-authority", WorkspaceID: "ws-ban109", ProjectID: "project-ban109", WorktreeID: "wt-ban109",
		Title: "authority boundary", AcceptanceCriteria: []string{"code compiles"}, Status: TaskReady,
		MaxAttempts: 1, RequiredCapabilities: []staff.Capability{"coding"}, SchemaVersion: ContractVersion,
	}
	if err := taskStore.SaveTask(context.Background(), task); err != nil {
		t.Fatal(err)
	}
	_ = store

	// No executor means dispatch stops before worker execution; the task remains
	// running and can only be completed through Orchestra/Inspector APIs.
	execution, err := dispatcher.Dispatch(context.Background(), task.ID)
	if err != nil {
		t.Fatalf("dispatch: %v", err)
	}
	if execution.Status != TaskRunning {
		t.Fatalf("expected running execution, got %s", execution.Status)
	}
	saved, err := taskStore.GetTask(context.Background(), task.ID)
	if err != nil || saved.Status != TaskRunning {
		t.Fatalf("expected task to remain running, task=%+v err=%v", saved, err)
	}
	if saved.Status == TaskSuccess {
		t.Fatal("worker/assignment path must not mark task successful")
	}
}

func TestBAN109_Integration_InvalidContextIsRejected(t *testing.T) {
	scheduler, _ := newBAN109Scheduler(t, ban109Definition("staff-a", staff.AvailabilityAvailable, []staff.Capability{"coding"}, nil))
	var nilContext context.Context
	_, err := scheduler.Submit(nilContext, ban109Request("task-invalid-context"))
	if err == nil || staff.ErrorCodeOf(err) != staff.ErrInvalidRequest {
		t.Fatalf("expected invalid context request error, got %v", err)
	}
	if !errors.Is(err, context.Canceled) && err.Error() == "" {
		t.Fatal("expected a descriptive context validation error")
	}
}
