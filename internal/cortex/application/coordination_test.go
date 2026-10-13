package application

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestCoordinationSnapshotRequestValidation(t *testing.T) {
	ctx := context.Background()

	// Valid request
	valid := CoordinationSnapshotRequest{
		SchemaVersion: CoordinationSchemaVersion,
		ProjectID:     "project-1",
	}
	if err := valid.Validate(ctx); err != nil {
		t.Fatalf("expected valid, got: %v", err)
	}

	// Unsupported schema version
	invalidVer := CoordinationSnapshotRequest{
		SchemaVersion: "cortexos.coordination.v0",
	}
	if err := invalidVer.Validate(ctx); !errors.Is(err, ErrCoordinationUnsupportedVersion) {
		t.Fatalf("expected ErrCoordinationUnsupportedVersion, got: %v", err)
	}

	// Canceled context
	canceledCtx, cancel := context.WithCancel(ctx)
	cancel()
	if err := valid.Validate(canceledCtx); !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got: %v", err)
	}

	// Secret in project ID
	secretProj := CoordinationSnapshotRequest{
		SchemaVersion: CoordinationSchemaVersion,
		ProjectID:     "api-key-secret-123",
	}
	if err := secretProj.Validate(ctx); !errors.Is(err, ErrCoordinationSanitization) {
		t.Fatalf("expected ErrCoordinationSanitization, got: %v", err)
	}

	// Windows path in project ID
	winPathProj := CoordinationSnapshotRequest{
		SchemaVersion: CoordinationSchemaVersion,
		ProjectID:     `C:\Users\Project`,
	}
	if err := winPathProj.Validate(ctx); !errors.Is(err, ErrCoordinationSanitization) {
		t.Fatalf("expected ErrCoordinationSanitization, got: %v", err)
	}
}

func TestCoordinationTaskValidation(t *testing.T) {
	validTask := CoordinationTask{
		ID:                "task-1",
		ProjectID:         "proj-1",
		Title:             "Compile frontend assets",
		AssignedStaffID:   "staff-1",
		AssignedStaffRole: "implementer",
		Status:            "running",
		Phase:             0,
		Dependencies:      []string{"dep-1"},
		IsBlocked:         false,
		Progress:          50,
		UpdatedAt:         time.Now().UTC(),
	}
	if err := validTask.Validate(); err != nil {
		t.Fatalf("expected valid task, got: %v", err)
	}

	// Empty ID
	emptyID := validTask
	emptyID.ID = ""
	if err := emptyID.Validate(); !errors.Is(err, ErrCoordinationInvalidRequest) {
		t.Fatalf("expected ErrCoordinationInvalidRequest, got: %v", err)
	}

	// Invalid ID characters
	invalidID := validTask
	invalidID.ID = "task/with/slashes"
	if err := invalidID.Validate(); !errors.Is(err, ErrCoordinationInvalidRequest) {
		t.Fatalf("expected ErrCoordinationInvalidRequest, got: %v", err)
	}

	// Title too long
	longTitle := validTask
	longTitle.Title = strings.Repeat("A", MaxCoordinationLabelLength+1)
	if err := longTitle.Validate(); !errors.Is(err, ErrCoordinationLimitExceeded) {
		t.Fatalf("expected ErrCoordinationLimitExceeded, got: %v", err)
	}

	// Absolute Unix path in title
	unixPathTitle := validTask
	unixPathTitle.Title = "Read /etc/passwd config"
	if err := unixPathTitle.Validate(); !errors.Is(err, ErrCoordinationSanitization) {
		t.Fatalf("expected ErrCoordinationSanitization for Unix path, got: %v", err)
	}

	// Absolute Windows path in title
	winPathTitle := validTask
	winPathTitle.Title = `Open C:\Windows\System32 config`
	if err := winPathTitle.Validate(); !errors.Is(err, ErrCoordinationSanitization) {
		t.Fatalf("expected ErrCoordinationSanitization for Windows path, got: %v", err)
	}

	// Secret token in title
	secretTitle := validTask
	secretTitle.Title = "Use Bearer 123456 token"
	if err := secretTitle.Validate(); !errors.Is(err, ErrCoordinationSanitization) {
		t.Fatalf("expected ErrCoordinationSanitization for secret, got: %v", err)
	}

	// Negative progress
	negProgress := validTask
	negProgress.Progress = -1
	if err := negProgress.Validate(); !errors.Is(err, ErrCoordinationInvalidRequest) {
		t.Fatalf("expected ErrCoordinationInvalidRequest for negative progress, got: %v", err)
	}

	// Over-100 progress
	overProgress := validTask
	overProgress.Progress = 101
	if err := overProgress.Validate(); !errors.Is(err, ErrCoordinationInvalidRequest) {
		t.Fatalf("expected ErrCoordinationInvalidRequest for >100 progress, got: %v", err)
	}
}

func TestStaffAssignmentStatusValidation(t *testing.T) {
	validStaff := StaffAssignmentStatus{
		StaffID:            "staff-lead",
		Name:               "Staff Lead",
		Role:               "coordinator",
		Availability:       "busy",
		CurrentTaskID:      "task-root",
		CurrentTaskTitle:   "Plan sprint execution",
		State:              "running",
		AssignedTaskCount:  3,
		CompletedTaskCount: 1,
	}
	if err := validStaff.Validate(); err != nil {
		t.Fatalf("expected valid staff, got: %v", err)
	}

	// Invalid ID
	badID := validStaff
	badID.StaffID = "staff with spaces"
	if err := badID.Validate(); !errors.Is(err, ErrCoordinationInvalidRequest) {
		t.Fatalf("expected ErrCoordinationInvalidRequest, got: %v", err)
	}

	// Secret in name
	secretName := validStaff
	secretName.Name = "BearerTokenStaff"
	if err := secretName.Validate(); !errors.Is(err, ErrCoordinationSanitization) {
		t.Fatalf("expected ErrCoordinationSanitization, got: %v", err)
	}
}

func TestCoordinationSnapshotBoundsValidation(t *testing.T) {
	validSnapshot := CoordinationSnapshot{
		SchemaVersion: CoordinationSchemaVersion,
		ProjectID:     "proj-demo",
		ActivePhase:   0,
		TotalPhases:   1,
		Tasks: []CoordinationTask{
			{
				ID:        "t-1",
				ProjectID: "proj-demo",
				Title:     "Task 1",
				Status:    "ready",
			},
		},
		Assignments: []StaffAssignmentStatus{
			{
				StaffID: "staff-1",
				Name:    "Staff One",
				Role:    "planner",
			},
		},
		Metrics: CoordinationMetrics{
			TotalTasks: 1,
			ReadyTasks: 1,
		},
		GeneratedAt: time.Now().UTC(),
	}
	if err := validSnapshot.Validate(); err != nil {
		t.Fatalf("expected valid snapshot, got: %v", err)
	}

	// Exceeds max tasks
	overflowTasks := validSnapshot
	overflowTasks.Tasks = make([]CoordinationTask, MaxCoordinationTasks+1)
	for i := range overflowTasks.Tasks {
		overflowTasks.Tasks[i] = CoordinationTask{
			ID:        "t-i",
			ProjectID: "proj-demo",
			Title:     "Task",
			Status:    "ready",
		}
	}
	if err := overflowTasks.Validate(); !errors.Is(err, ErrCoordinationLimitExceeded) {
		t.Fatalf("expected ErrCoordinationLimitExceeded for task overflow, got: %v", err)
	}

	// Exceeds max staff
	overflowStaff := validSnapshot
	overflowStaff.Assignments = make([]StaffAssignmentStatus, MaxCoordinationStaff+1)
	for i := range overflowStaff.Assignments {
		overflowStaff.Assignments[i] = StaffAssignmentStatus{
			StaffID: "s-i",
			Name:    "Staff",
			Role:    "worker",
		}
	}
	if err := overflowStaff.Validate(); !errors.Is(err, ErrCoordinationLimitExceeded) {
		t.Fatalf("expected ErrCoordinationLimitExceeded for staff overflow, got: %v", err)
	}
}

func TestComputeDependencyPhases(t *testing.T) {
	// Independent tasks
	independent := []CoordinationTask{
		{ID: "t-1", Title: "Task 1"},
		{ID: "t-2", Title: "Task 2"},
		{ID: "t-3", Title: "Task 3"},
	}
	phases, total := ComputeDependencyPhases(independent)
	if total != 1 {
		t.Fatalf("expected 1 phase for independent tasks, got %d", total)
	}
	for _, id := range []string{"t-1", "t-2", "t-3"} {
		if phases[id] != 0 {
			t.Fatalf("expected phase 0 for %s, got %d", id, phases[id])
		}
	}

	// Chain: t-1 -> t-2 -> t-3
	chain := []CoordinationTask{
		{ID: "t-1", Title: "Task 1"},
		{ID: "t-2", Title: "Task 2", Dependencies: []string{"t-1"}},
		{ID: "t-3", Title: "Task 3", Dependencies: []string{"t-2"}},
	}
	phases, total = ComputeDependencyPhases(chain)
	if total != 3 {
		t.Fatalf("expected 3 phases for chain, got %d", total)
	}
	if phases["t-1"] != 0 || phases["t-2"] != 1 || phases["t-3"] != 2 {
		t.Fatalf("expected phases {0, 1, 2}, got %+v", phases)
	}

	// Diamond: t-1 -> (t-2a, t-2b) -> t-3
	diamond := []CoordinationTask{
		{ID: "t-1", Title: "Task 1"},
		{ID: "t-2a", Title: "Task 2a", Dependencies: []string{"t-1"}},
		{ID: "t-2b", Title: "Task 2b", Dependencies: []string{"t-1"}},
		{ID: "t-3", Title: "Task 3", Dependencies: []string{"t-2a", "t-2b"}},
	}
	phases, total = ComputeDependencyPhases(diamond)
	if total != 3 {
		t.Fatalf("expected 3 phases for diamond, got %d", total)
	}
	if phases["t-1"] != 0 || phases["t-2a"] != 1 || phases["t-2b"] != 1 || phases["t-3"] != 2 {
		t.Fatalf("expected diamond phases {t-1:0, t-2a:1, t-2b:1, t-3:2}, got %+v", phases)
	}

	// Cycle protection: t-1 -> t-2 -> t-1
	cycle := []CoordinationTask{
		{ID: "t-1", Title: "Task 1", Dependencies: []string{"t-2"}},
		{ID: "t-2", Title: "Task 2", Dependencies: []string{"t-1"}},
	}
	// Must not panic or loop infinitely
	phases, total = ComputeDependencyPhases(cycle)
	if total < 1 {
		t.Fatalf("expected total >= 1, got %d", total)
	}
}

func TestDetermineBlockedState(t *testing.T) {
	taskStatuses := map[string]string{
		"dep-1": "success",
		"dep-2": "running",
		"dep-3": "ready",
	}

	// Terminal tasks are never blocked
	for _, termStatus := range []string{"success", "failed", "canceled"} {
		task := CoordinationTask{
			ID:           "test",
			Status:       termStatus,
			Dependencies: []string{"dep-2"},
		}
		blocked, _ := DetermineBlockedState(task, taskStatuses)
		if blocked {
			t.Fatalf("terminal task status %q must not be blocked", termStatus)
		}
	}

	// Task with satisfied dependencies is not blocked
	readyTask := CoordinationTask{
		ID:           "test",
		Status:       "ready",
		Dependencies: []string{"dep-1"},
	}
	blocked, reason := DetermineBlockedState(readyTask, taskStatuses)
	if blocked {
		t.Fatalf("task with satisfied dependencies should not be blocked, got reason: %s", reason)
	}

	// Task with pending dependencies is blocked
	blockedTask := CoordinationTask{
		ID:           "test",
		Status:       "ready",
		Dependencies: []string{"dep-1", "dep-2", "dep-3"},
	}
	blocked, reason = DetermineBlockedState(blockedTask, taskStatuses)
	if !blocked {
		t.Fatalf("task with uncompleted dependencies must be blocked")
	}
	if !strings.Contains(reason, "dep-2") || !strings.Contains(reason, "dep-3") {
		t.Fatalf("expected blockedReason to mention dep-2 and dep-3, got: %s", reason)
	}
}

func TestCoordinationEventPayloadValidation(t *testing.T) {
	validPayload := CoordinationEventPayload{
		SchemaVersion:   CoordinationSchemaVersion,
		EventType:       "assigned",
		TaskID:          "task-123",
		ProjectID:       "proj-alpha",
		StaffID:         "staff-omega",
		PreviousStaffID: "staff-beta",
		Status:          "ready",
		Phase:           1,
		IsBlocked:       false,
		Progress:        20,
		Timestamp:       time.Now().UTC(),
	}
	if err := validPayload.Validate(); err != nil {
		t.Fatalf("expected valid payload, got: %v", err)
	}

	// Unsupported schema version
	invalidVer := validPayload
	invalidVer.SchemaVersion = "bad.version"
	if err := invalidVer.Validate(); !errors.Is(err, ErrCoordinationUnsupportedVersion) {
		t.Fatalf("expected ErrCoordinationUnsupportedVersion, got: %v", err)
	}

	// Path in blocked reason
	pathPayload := validPayload
	pathPayload.BlockedReason = "Blocked on /etc/shadow"
	if err := pathPayload.Validate(); !errors.Is(err, ErrCoordinationSanitization) {
		t.Fatalf("expected ErrCoordinationSanitization for path in blockedReason, got: %v", err)
	}
}
