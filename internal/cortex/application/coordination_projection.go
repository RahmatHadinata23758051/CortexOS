package application

import (
	"context"
	"sort"
	"sync"
	"time"

	"github.com/RahmatHadinata23758051/CortexOS/internal/cortex/orchestra"
	"github.com/RahmatHadinata23758051/CortexOS/internal/cortex/staff"
)

// CoordinationSource reads authoritative Orchestra and Staff state.
// Implementations must return sanitized domain records; the projection validates
// again before exposing any bridge-visible snapshot.
type CoordinationSource interface {
	ListTasks(context.Context) ([]orchestra.Task, error)
	ListStaff(context.Context) ([]StaffSummary, error)
}

// CoordinationProjection is a thread-safe, bounded read model for Office/Cockpit.
// It coalesces source reads into a stable snapshot and does not own domain mutations.
type CoordinationProjection struct {
	mu     sync.RWMutex
	source CoordinationSource
	now    func() time.Time
	cached CoordinationSnapshot
	has    bool
}

// NewCoordinationProjection creates a projection over the authoritative source.
func NewCoordinationProjection(source CoordinationSource) *CoordinationProjection {
	return &CoordinationProjection{source: source, now: time.Now}
}

// GetCoordinationSnapshot implements CoordinationPort by refreshing the projected view.
func (p *CoordinationProjection) GetCoordinationSnapshot(ctx context.Context, request CoordinationSnapshotRequest) (CoordinationSnapshot, error) {
	return p.Refresh(ctx, request)
}

// Refresh reads, projects, validates, and atomically publishes a new snapshot.
func (p *CoordinationProjection) Refresh(ctx context.Context, request CoordinationSnapshotRequest) (CoordinationSnapshot, error) {
	if err := request.Validate(ctx); err != nil {
		return CoordinationSnapshot{}, err
	}
	if p == nil || p.source == nil {
		return CoordinationSnapshot{}, ErrCoordinationInvalidRequest
	}

	tasks, err := p.source.ListTasks(ctx)
	if err != nil {
		return CoordinationSnapshot{}, err
	}
	staffRecords, err := p.source.ListStaff(ctx)
	if err != nil {
		return CoordinationSnapshot{}, err
	}
	if len(tasks) > MaxCoordinationTasks || len(staffRecords) > MaxCoordinationStaff {
		return CoordinationSnapshot{}, ErrCoordinationLimitExceeded
	}

	snapshot := projectCoordination(tasks, staffRecords, request.ProjectID, p.now().UTC())
	if err := snapshot.Validate(); err != nil {
		return CoordinationSnapshot{}, err
	}

	p.mu.Lock()
	p.cached = snapshot
	p.has = true
	p.mu.Unlock()
	return cloneCoordinationSnapshot(snapshot), nil
}

// Snapshot returns the most recently published snapshot, if any.
func (p *CoordinationProjection) Snapshot(ctx context.Context, request CoordinationSnapshotRequest) (CoordinationSnapshot, error) {
	if err := request.Validate(ctx); err != nil {
		return CoordinationSnapshot{}, err
	}
	if p == nil {
		return CoordinationSnapshot{}, ErrCoordinationInvalidRequest
	}
	p.mu.RLock()
	defer p.mu.RUnlock()
	if !p.has {
		return CoordinationSnapshot{}, ErrCoordinationInvalidRequest
	}
	if request.ProjectID != "" && p.cached.ProjectID != request.ProjectID {
		return CoordinationSnapshot{}, ErrCoordinationInvalidRequest
	}
	return cloneCoordinationSnapshot(p.cached), nil
}

func projectCoordination(tasks []orchestra.Task, staffRecords []StaffSummary, projectID string, generatedAt time.Time) CoordinationSnapshot {
	projectedTasks := make([]CoordinationTask, 0, len(tasks))
	statuses := make(map[string]string, len(tasks))
	for _, task := range tasks {
		if projectID != "" && task.ProjectID != projectID {
			continue
		}
		status := string(task.Status)
		statuses[string(task.ID)] = status
		deps := make([]string, 0, len(task.Dependencies))
		for _, d := range task.Dependencies {
			deps = append(deps, string(d))
		}
		projectedTasks = append(projectedTasks, CoordinationTask{
			ID:              string(task.ID),
			ProjectID:       task.ProjectID,
			Title:           task.Title,
			AssignedStaffID: string(task.AssignedStaffID),
			Status:          status,
			Dependencies:    deps,
			Attempt:         task.AttemptCount,
			MaxAttempts:     task.MaxAttempts,
			Progress:        coordinationProgress(status),
			UpdatedAt:       task.UpdatedAt,
		})
	}

	phases, totalPhases := ComputeDependencyPhases(projectedTasks)
	for i := range projectedTasks {
		projectedTasks[i].Phase = phases[projectedTasks[i].ID]
		blocked, reason := DetermineBlockedState(projectedTasks[i], statuses)
		projectedTasks[i].IsBlocked = blocked
		projectedTasks[i].BlockedReason = reason
	}
	sort.Slice(projectedTasks, func(i, j int) bool {
		if projectedTasks[i].Phase != projectedTasks[j].Phase {
			return projectedTasks[i].Phase < projectedTasks[j].Phase
		}
		return projectedTasks[i].ID < projectedTasks[j].ID
	})

	staffStatuses := make([]StaffAssignmentStatus, 0, len(staffRecords))
	seenStaff := make(map[string]struct{}, len(staffRecords))
	for _, member := range staffRecords {
		if projectID != "" && member.ProjectID != "" && string(member.ProjectID) != projectID {
			continue
		}
		assigned := make([]CoordinationTask, 0)
		for _, task := range projectedTasks {
			if task.AssignedStaffID == string(member.ID) {
				assigned = append(assigned, task)
			}
		}
		status := "idle"
		var current CoordinationTask
		if len(assigned) > 0 {
			current = assigned[0]
			status = coordinationStaffState(current)
		}
		completed := 0
		for _, task := range assigned {
			if task.Status == string(orchestra.TaskSuccess) {
				completed++
			}
		}
		seenStaff[string(member.ID)] = struct{}{}
		staffStatuses = append(staffStatuses, StaffAssignmentStatus{
			StaffID:            string(member.ID),
			Name:               member.Name,
			Role:               string(member.Role),
			Availability:       string(member.Availability),
			CurrentTaskID:      current.ID,
			CurrentTaskTitle:   current.Title,
			State:              status,
			AssignedTaskCount:  len(assigned),
			CompletedTaskCount: completed,
		})
	}

	// Synthesize assignment entries from authoritative Orchestra task assignments
	// when no Staff summary is available. Orchestra owns the assignment fact; the
	// projection only derives a bounded, sanitized presentation entry from it.
	for _, task := range projectedTasks {
		if task.AssignedStaffID == "" {
			continue
		}
		if _, ok := seenStaff[task.AssignedStaffID]; ok {
			continue
		}
		seenStaff[task.AssignedStaffID] = struct{}{}
		assigned := make([]CoordinationTask, 0)
		for _, candidate := range projectedTasks {
			if candidate.AssignedStaffID == task.AssignedStaffID {
				assigned = append(assigned, candidate)
			}
		}
		completed := 0
		for _, candidate := range assigned {
			if candidate.Status == string(orchestra.TaskSuccess) {
				completed++
			}
		}
		staffStatuses = append(staffStatuses, StaffAssignmentStatus{
			StaffID:            task.AssignedStaffID,
			Name:               task.AssignedStaffID,
			Role:               task.AssignedStaffRole,
			Availability:       "",
			CurrentTaskID:      task.ID,
			CurrentTaskTitle:   task.Title,
			State:              coordinationStaffState(task),
			AssignedTaskCount:  len(assigned),
			CompletedTaskCount: completed,
		})
	}
	sort.Slice(staffStatuses, func(i, j int) bool { return staffStatuses[i].StaffID < staffStatuses[j].StaffID })

	metrics := CoordinationMetrics{TotalTasks: len(projectedTasks), TotalStaffCount: len(staffStatuses)}
	maxPhase := 0
	for _, task := range projectedTasks {
		switch task.Status {
		case string(orchestra.TaskReady):
			metrics.ReadyTasks++
		case string(orchestra.TaskRunning):
			metrics.RunningTasks++
		case string(orchestra.TaskAwaitingInspection):
			metrics.AwaitingInspection++
		case string(orchestra.TaskSuccess):
			metrics.CompletedTasks++
		case string(orchestra.TaskFailed):
			metrics.FailedTasks++
		case string(orchestra.TaskCanceled):
			metrics.CanceledTasks++
		}
		if task.IsBlocked {
			metrics.BlockedTasks++
		}
		if task.Phase > maxPhase {
			maxPhase = task.Phase
		}
	}
	for _, member := range staffStatuses {
		if member.State != "idle" && member.Availability != string(staff.AvailabilityOffline) {
			metrics.ActiveStaffCount++
		}
	}

	activePhase := 0
	for _, task := range projectedTasks {
		if task.Status == string(orchestra.TaskRunning) || task.Status == string(orchestra.TaskAwaitingInspection) {
			activePhase = task.Phase
			break
		}
	}
	if len(projectedTasks) == 0 {
		totalPhases = 0
		maxPhase = 0
	}
	if totalPhases == 0 && len(projectedTasks) > 0 {
		totalPhases = maxPhase + 1
	}

	return CoordinationSnapshot{
		SchemaVersion: CoordinationSchemaVersion,
		ProjectID:     projectID,
		ActivePhase:   activePhase,
		TotalPhases:   totalPhases,
		Tasks:         projectedTasks,
		Assignments:   staffStatuses,
		Metrics:       metrics,
		GeneratedAt:   generatedAt,
	}
}

func coordinationProgress(status string) int {
	switch status {
	case string(orchestra.TaskDraft):
		return 0
	case string(orchestra.TaskReady):
		return 20
	case string(orchestra.TaskRunning):
		return 50
	case string(orchestra.TaskAwaitingInspection):
		return 80
	case string(orchestra.TaskSuccess), string(orchestra.TaskFailed), string(orchestra.TaskCanceled):
		return 100
	default:
		return 0
	}
}

func coordinationStaffState(task CoordinationTask) string {
	if task.IsBlocked {
		return "blocked"
	}
	switch task.Status {
	case string(orchestra.TaskRunning):
		return "running"
	case string(orchestra.TaskAwaitingInspection):
		return "awaiting_inspection"
	case string(orchestra.TaskSuccess):
		return "completed"
	case string(orchestra.TaskFailed):
		return "failed"
	default:
		return "idle"
	}
}

func cloneCoordinationSnapshot(snapshot CoordinationSnapshot) CoordinationSnapshot {
	clone := snapshot
	clone.Tasks = append([]CoordinationTask(nil), snapshot.Tasks...)
	for i := range clone.Tasks {
		clone.Tasks[i].Dependencies = append([]string(nil), snapshot.Tasks[i].Dependencies...)
	}
	clone.Assignments = append([]StaffAssignmentStatus(nil), snapshot.Assignments...)
	return clone
}
