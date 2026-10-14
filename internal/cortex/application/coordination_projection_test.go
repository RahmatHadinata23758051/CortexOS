package application

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/RahmatHadinata23758051/CortexOS/internal/cortex/orchestra"
	"github.com/RahmatHadinata23758051/CortexOS/internal/cortex/staff"
)

type fakeCoordinationSource struct {
	tasks []orchestra.Task
	staff []StaffSummary
	err   error
}

func (f *fakeCoordinationSource) ListTasks(context.Context) ([]orchestra.Task, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.tasks, nil
}

func (f *fakeCoordinationSource) ListStaff(context.Context) ([]StaffSummary, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.staff, nil
}

func sampleCoordinationSource() *fakeCoordinationSource {
	now := time.Unix(1_700_000_000, 0).UTC()
	return &fakeCoordinationSource{
		tasks: []orchestra.Task{
			{
				ID: "task-root", ProjectID: "proj-a", Title: "Plan sprint",
				AssignedStaffID: "staff-lead", Status: orchestra.TaskSuccess,
				AttemptCount: 1, MaxAttempts: 3, UpdatedAt: now,
			},
			{
				ID: "task-build", ProjectID: "proj-a", Title: "Build assets",
				AssignedStaffID: "staff-dev", Status: orchestra.TaskRunning,
				Dependencies: []orchestra.TaskID{"task-root"},
				AttemptCount: 1, MaxAttempts: 3, UpdatedAt: now,
			},
			{
				ID: "task-ship", ProjectID: "proj-a", Title: "Ship release",
				Status:       orchestra.TaskReady,
				Dependencies: []orchestra.TaskID{"task-build"},
				MaxAttempts:  3, UpdatedAt: now,
			},
		},
		staff: []StaffSummary{
			{ID: "staff-lead", Name: "Lead", Role: staff.RoleCoordinator, Availability: staff.AvailabilityAvailable},
			{ID: "staff-dev", Name: "Dev", Role: staff.RoleImplementer, Availability: staff.AvailabilityBusy},
		},
	}
}

func TestCoordinationProjectionRefresh(t *testing.T) {
	projection := NewCoordinationProjection(sampleCoordinationSource())
	snapshot, err := projection.Refresh(context.Background(), CoordinationSnapshotRequest{
		SchemaVersion: CoordinationSchemaVersion,
		ProjectID:     "proj-a",
	})
	if err != nil {
		t.Fatalf("Refresh failed: %v", err)
	}
	if snapshot.SchemaVersion != CoordinationSchemaVersion {
		t.Fatalf("unexpected schema version %q", snapshot.SchemaVersion)
	}
	if len(snapshot.Tasks) != 3 {
		t.Fatalf("expected 3 tasks, got %d", len(snapshot.Tasks))
	}
	if snapshot.TotalPhases != 3 {
		t.Fatalf("expected 3 phases, got %d", snapshot.TotalPhases)
	}
	if snapshot.Metrics.RunningTasks != 1 || snapshot.Metrics.CompletedTasks != 1 {
		t.Fatalf("unexpected metrics: %+v", snapshot.Metrics)
	}
	// task-ship depends on a running task and must be blocked.
	var ship CoordinationTask
	for _, task := range snapshot.Tasks {
		if task.ID == "task-ship" {
			ship = task
		}
	}
	if !ship.IsBlocked {
		t.Fatalf("expected task-ship to be blocked, got %+v", ship)
	}
	// Deterministic ordering by phase then ID.
	if snapshot.Tasks[0].ID != "task-root" {
		t.Fatalf("expected task-root first, got %s", snapshot.Tasks[0].ID)
	}
	if len(snapshot.Assignments) != 2 {
		t.Fatalf("expected 2 assignments, got %d", len(snapshot.Assignments))
	}
	if snapshot.Metrics.TotalStaffCount != 2 {
		t.Fatalf("expected 2 staff, got %d", snapshot.Metrics.TotalStaffCount)
	}
}

func TestCoordinationProjectionSnapshotRequiresRefresh(t *testing.T) {
	projection := NewCoordinationProjection(sampleCoordinationSource())
	_, err := projection.Snapshot(context.Background(), CoordinationSnapshotRequest{SchemaVersion: CoordinationSchemaVersion})
	if !errors.Is(err, ErrCoordinationInvalidRequest) {
		t.Fatalf("expected ErrCoordinationInvalidRequest before refresh, got %v", err)
	}
}

func TestCoordinationProjectionRejectsUnsupportedVersion(t *testing.T) {
	projection := NewCoordinationProjection(sampleCoordinationSource())
	_, err := projection.Refresh(context.Background(), CoordinationSnapshotRequest{SchemaVersion: "bad.version"})
	if !errors.Is(err, ErrCoordinationUnsupportedVersion) {
		t.Fatalf("expected ErrCoordinationUnsupportedVersion, got %v", err)
	}
}

func TestCoordinationProjectionFailsClosedOnSourceError(t *testing.T) {
	source := sampleCoordinationSource()
	source.err = errors.New("source unavailable")
	projection := NewCoordinationProjection(source)
	_, err := projection.Refresh(context.Background(), CoordinationSnapshotRequest{SchemaVersion: CoordinationSchemaVersion})
	if err == nil {
		t.Fatalf("expected error when source fails")
	}
}

func TestCoordinationProjectionBoundsExceeded(t *testing.T) {
	source := &fakeCoordinationSource{}
	for i := 0; i < MaxCoordinationTasks+1; i++ {
		source.tasks = append(source.tasks, orchestra.Task{
			ID: orchestra.TaskID("task-"), ProjectID: "proj-a", Title: "Task", Status: orchestra.TaskReady,
		})
	}
	projection := NewCoordinationProjection(source)
	_, err := projection.Refresh(context.Background(), CoordinationSnapshotRequest{SchemaVersion: CoordinationSchemaVersion})
	if !errors.Is(err, ErrCoordinationLimitExceeded) {
		t.Fatalf("expected ErrCoordinationLimitExceeded, got %v", err)
	}
}

func TestCoordinationProjectionCanceledContext(t *testing.T) {
	projection := NewCoordinationProjection(sampleCoordinationSource())
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := projection.Refresh(ctx, CoordinationSnapshotRequest{SchemaVersion: CoordinationSchemaVersion})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
}

func TestCloneCoordinationSnapshotPreservesJSONArrays(t *testing.T) {
	snapshot := cloneCoordinationSnapshot(CoordinationSnapshot{
		SchemaVersion: CoordinationSchemaVersion,
		Tasks:         []CoordinationTask{{ID: "task", Dependencies: []string{}}},
		Assignments:   []StaffAssignmentStatus{},
	})

	encoded, err := json.Marshal(snapshot)
	if err != nil {
		t.Fatalf("marshal snapshot: %v", err)
	}
	var payload struct {
		Tasks       json.RawMessage `json:"tasks"`
		Assignments json.RawMessage `json:"assignments"`
	}
	if err := json.Unmarshal(encoded, &payload); err != nil {
		t.Fatalf("unmarshal snapshot: %v", err)
	}
	if string(payload.Assignments) != "[]" {
		t.Fatalf("expected assignments to marshal as [], got %s", payload.Assignments)
	}
	var tasks []struct {
		Dependencies []string `json:"dependencies"`
	}
	if err := json.Unmarshal(payload.Tasks, &tasks); err != nil {
		t.Fatalf("unmarshal tasks: %v", err)
	}
	if len(tasks) != 1 || tasks[0].Dependencies == nil {
		t.Fatalf("expected one task with dependencies as [], got %s", payload.Tasks)
	}
}

func TestCloneCoordinationSnapshotEmptyArraysMarshalAsArrays(t *testing.T) {
	snapshot := cloneCoordinationSnapshot(CoordinationSnapshot{
		SchemaVersion: CoordinationSchemaVersion,
		Tasks:         []CoordinationTask{},
		Assignments:   []StaffAssignmentStatus{},
	})
	encoded, err := json.Marshal(snapshot)
	if err != nil {
		t.Fatalf("marshal snapshot: %v", err)
	}
	var payload struct {
		Tasks       json.RawMessage `json:"tasks"`
		Assignments json.RawMessage `json:"assignments"`
	}
	if err := json.Unmarshal(encoded, &payload); err != nil {
		t.Fatalf("unmarshal snapshot: %v", err)
	}
	if string(payload.Tasks) != "[]" || string(payload.Assignments) != "[]" {
		t.Fatalf("expected empty arrays, got tasks=%s assignments=%s", payload.Tasks, payload.Assignments)
	}
}
