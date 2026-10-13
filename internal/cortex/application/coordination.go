package application

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"
)

// CoordinationSchemaVersion is the stable schema identifier for multi-Staff coordination.
const CoordinationSchemaVersion = "cortexos.coordination.v1"

// Coordination bounded limits to protect memory and render budgets.
const (
	MaxCoordinationTasks         = 100
	MaxCoordinationStaff         = 50
	MaxCoordinationLabelLength   = 128
	MaxCoordinationSummaryLength = 512
)

var (
	ErrCoordinationInvalidRequest     = errors.New("coordination: invalid request")
	ErrCoordinationUnsupportedVersion = errors.New("coordination: unsupported schema version")
	ErrCoordinationLimitExceeded      = errors.New("coordination: capacity limit exceeded")
	ErrCoordinationSanitization       = errors.New("coordination: sanitization violation")
)

var (
	rxCoordValidID     = regexp.MustCompile(`^[a-zA-Z0-9_\-\.]{1,64}$`)
	rxCoordAbsPathWin  = regexp.MustCompile(`(?i)[a-z]:[\\/]`)
	rxCoordAbsPathUnix = regexp.MustCompile(`(?:^|[\s"'(=])/(?:Users|home|etc|var|tmp|private|root|opt|usr)(?:/|\b)`)
	rxCoordSecret      = regexp.MustCompile(`(?i)(api[_-]?key|secret|bearer|password|private[_-]?key|access[_-]?token|credential)`)
)

// CoordinationPort is the application boundary for multi-Staff coordination observability.
// It exposes projected, deterministic coordination facts derived from authoritative
// Orchestra and Staff state without leaking process handles, paths, or secrets.
type CoordinationPort interface {
	GetCoordinationSnapshot(ctx context.Context, request CoordinationSnapshotRequest) (CoordinationSnapshot, error)
}

// CoordinationSnapshotRequest specifies the parameters for querying a coordination snapshot.
type CoordinationSnapshotRequest struct {
	SchemaVersion string `json:"schemaVersion"`
	ProjectID     string `json:"projectId,omitempty"`
}

// Validate ensures the request meets the schema contract.
func (r CoordinationSnapshotRequest) Validate(ctx context.Context) error {
	if ctx != nil {
		if err := ctx.Err(); err != nil {
			return err
		}
	}
	if r.SchemaVersion != CoordinationSchemaVersion {
		return fmt.Errorf("%w: expected %s, got %q", ErrCoordinationUnsupportedVersion, CoordinationSchemaVersion, r.SchemaVersion)
	}
	if r.ProjectID != "" {
		if err := validateCoordinationString("projectId", r.ProjectID, MaxCoordinationLabelLength); err != nil {
			return err
		}
	}
	return nil
}

// CoordinationTask represents a single governed Orchestra task projected for multi-Staff coordination.
type CoordinationTask struct {
	ID                string    `json:"id"`
	ProjectID         string    `json:"projectId"`
	Title             string    `json:"title"`
	AssignedStaffID   string    `json:"assignedStaffId,omitempty"`
	AssignedStaffRole string    `json:"assignedStaffRole,omitempty"`
	Status            string    `json:"status"`
	Phase             int       `json:"phase"`
	Dependencies      []string  `json:"dependencies"`
	IsBlocked         bool      `json:"isBlocked"`
	BlockedReason     string    `json:"blockedReason,omitempty"`
	Attempt           int       `json:"attempt"`
	MaxAttempts       int       `json:"maxAttempts"`
	Progress          int       `json:"progress"`
	UpdatedAt         time.Time `json:"updatedAt"`
}

// Validate verifies task invariants and sanitization.
func (t CoordinationTask) Validate() error {
	if err := validateCoordinationToken("id", t.ID); err != nil {
		return err
	}
	if err := validateCoordinationString("title", t.Title, MaxCoordinationLabelLength); err != nil {
		return err
	}
	if t.AssignedStaffID != "" {
		if err := validateCoordinationToken("assignedStaffId", t.AssignedStaffID); err != nil {
			return err
		}
	}
	if t.AssignedStaffRole != "" {
		if err := validateCoordinationString("assignedStaffRole", t.AssignedStaffRole, 32); err != nil {
			return err
		}
	}
	if t.BlockedReason != "" {
		if err := validateCoordinationString("blockedReason", t.BlockedReason, MaxCoordinationSummaryLength); err != nil {
			return err
		}
	}
	for _, dep := range t.Dependencies {
		if err := validateCoordinationToken("dependency", dep); err != nil {
			return err
		}
	}
	if t.Progress < 0 || t.Progress > 100 {
		return fmt.Errorf("%w: progress must be between 0 and 100", ErrCoordinationInvalidRequest)
	}
	return nil
}

// StaffAssignmentStatus represents a logical Staff member's role and state in the coordination plan.
type StaffAssignmentStatus struct {
	StaffID            string `json:"staffId"`
	Name               string `json:"name"`
	Role               string `json:"role"`
	Availability       string `json:"availability"`
	CurrentTaskID      string `json:"currentTaskId,omitempty"`
	CurrentTaskTitle   string `json:"currentTaskTitle,omitempty"`
	State              string `json:"state"` // "idle" | "running" | "blocked" | "awaiting_inspection" | "completed" | "failed"
	AssignedTaskCount  int    `json:"assignedTaskCount"`
	CompletedTaskCount int    `json:"completedTaskCount"`
}

// Validate verifies staff assignment invariants and sanitization.
func (s StaffAssignmentStatus) Validate() error {
	if err := validateCoordinationToken("staffId", s.StaffID); err != nil {
		return err
	}
	if err := validateCoordinationString("name", s.Name, MaxCoordinationLabelLength); err != nil {
		return err
	}
	if err := validateCoordinationString("role", s.Role, 32); err != nil {
		return err
	}
	if s.CurrentTaskID != "" {
		if err := validateCoordinationToken("currentTaskId", s.CurrentTaskID); err != nil {
			return err
		}
	}
	if s.CurrentTaskTitle != "" {
		if err := validateCoordinationString("currentTaskTitle", s.CurrentTaskTitle, MaxCoordinationLabelLength); err != nil {
			return err
		}
	}
	return nil
}

// CoordinationMetrics summarizes the aggregate health and progress of the coordination plan.
type CoordinationMetrics struct {
	TotalTasks         int `json:"totalTasks"`
	ReadyTasks         int `json:"readyTasks"`
	RunningTasks       int `json:"runningTasks"`
	BlockedTasks       int `json:"blockedTasks"`
	AwaitingInspection int `json:"awaitingInspection"`
	CompletedTasks     int `json:"completedTasks"`
	FailedTasks        int `json:"failedTasks"`
	CanceledTasks      int `json:"canceledTasks"`
	ActiveStaffCount   int `json:"activeStaffCount"`
	TotalStaffCount    int `json:"totalStaffCount"`
}

// CoordinationSnapshot is the deterministic, bounded snapshot of multi-Staff coordination.
type CoordinationSnapshot struct {
	SchemaVersion string                  `json:"schemaVersion"`
	ProjectID     string                  `json:"projectId,omitempty"`
	ActivePhase   int                     `json:"activePhase"`
	TotalPhases   int                     `json:"totalPhases"`
	Tasks         []CoordinationTask      `json:"tasks"`
	Assignments   []StaffAssignmentStatus `json:"assignments"`
	Metrics       CoordinationMetrics     `json:"metrics"`
	GeneratedAt   time.Time               `json:"generatedAt"`
}

// Validate checks bounds, schema version, and nested item sanitization.
func (s CoordinationSnapshot) Validate() error {
	if s.SchemaVersion != CoordinationSchemaVersion {
		return fmt.Errorf("%w: expected %s, got %q", ErrCoordinationUnsupportedVersion, CoordinationSchemaVersion, s.SchemaVersion)
	}
	if len(s.Tasks) > MaxCoordinationTasks {
		return fmt.Errorf("%w: task count %d exceeds maximum %d", ErrCoordinationLimitExceeded, len(s.Tasks), MaxCoordinationTasks)
	}
	if len(s.Assignments) > MaxCoordinationStaff {
		return fmt.Errorf("%w: staff count %d exceeds maximum %d", ErrCoordinationLimitExceeded, len(s.Assignments), MaxCoordinationStaff)
	}
	for _, task := range s.Tasks {
		if err := task.Validate(); err != nil {
			return err
		}
	}
	for _, staff := range s.Assignments {
		if err := staff.Validate(); err != nil {
			return err
		}
	}
	return nil
}

// CoordinationEventPayload represents the bounded data carried in coordination lifecycle events.
type CoordinationEventPayload struct {
	SchemaVersion   string    `json:"schemaVersion"`
	EventType       string    `json:"eventType"` // "assigned" | "reassigned" | "progress" | "blocked" | "completed" | "canceled"
	TaskID          string    `json:"taskId"`
	ProjectID       string    `json:"projectId,omitempty"`
	StaffID         string    `json:"staffId,omitempty"`
	PreviousStaffID string    `json:"previousStaffId,omitempty"`
	Role            string    `json:"role,omitempty"`
	Status          string    `json:"status"`
	Phase           int       `json:"phase"`
	IsBlocked       bool      `json:"isBlocked"`
	BlockedReason   string    `json:"blockedReason,omitempty"`
	Progress        int       `json:"progress"`
	Timestamp       time.Time `json:"timestamp"`
}

// Validate verifies the payload schema and sanitization.
func (p CoordinationEventPayload) Validate() error {
	if p.SchemaVersion != CoordinationSchemaVersion {
		return fmt.Errorf("%w: expected %s, got %q", ErrCoordinationUnsupportedVersion, CoordinationSchemaVersion, p.SchemaVersion)
	}
	if err := validateCoordinationToken("taskId", p.TaskID); err != nil {
		return err
	}
	if p.StaffID != "" {
		if err := validateCoordinationToken("staffId", p.StaffID); err != nil {
			return err
		}
	}
	if p.PreviousStaffID != "" {
		if err := validateCoordinationToken("previousStaffId", p.PreviousStaffID); err != nil {
			return err
		}
	}
	if p.BlockedReason != "" {
		if err := validateCoordinationString("blockedReason", p.BlockedReason, MaxCoordinationSummaryLength); err != nil {
			return err
		}
	}
	return nil
}

// validateCoordinationToken ensures identifiers are safe ASCII tokens.
func validateCoordinationToken(field, val string) error {
	trimmed := strings.TrimSpace(val)
	if trimmed == "" {
		return fmt.Errorf("%w: %s is required", ErrCoordinationInvalidRequest, field)
	}
	if !rxCoordValidID.MatchString(trimmed) {
		return fmt.Errorf("%w: %s %q contains invalid characters", ErrCoordinationInvalidRequest, field, val)
	}
	return nil
}

// validateCoordinationString ensures strings are bounded and do not leak paths or secrets.
func validateCoordinationString(field, val string, maxLen int) error {
	if len(val) > maxLen {
		return fmt.Errorf("%w: %s exceeds max length %d", ErrCoordinationLimitExceeded, field, maxLen)
	}
	if rxCoordAbsPathWin.MatchString(val) || rxCoordAbsPathUnix.MatchString(val) {
		return fmt.Errorf("%w: %s contains absolute path", ErrCoordinationSanitization, field)
	}
	if rxCoordSecret.MatchString(val) {
		return fmt.Errorf("%w: %s contains potential secret or credential token", ErrCoordinationSanitization, field)
	}
	return nil
}

// ComputeDependencyPhases computes deterministic topological phases for tasks.
// Returns a map of taskID -> phase (0-indexed). If cycles or missing dependencies exist,
// tasks degrade gracefully to the maximum discovered phase rather than failing.
func ComputeDependencyPhases(tasks []CoordinationTask) (map[string]int, int) {
	taskMap := make(map[string]CoordinationTask, len(tasks))
	for _, t := range tasks {
		taskMap[t.ID] = t
	}

	phases := make(map[string]int, len(tasks))
	visiting := make(map[string]bool)

	var getPhase func(id string, depth int) int
	getPhase = func(id string, depth int) int {
		if depth > len(tasks) {
			return 0 // cycle protection
		}
		if p, ok := phases[id]; ok {
			return p
		}
		if visiting[id] {
			return 0 // cycle fallback
		}
		visiting[id] = true
		defer func() { visiting[id] = false }()

		task, exists := taskMap[id]
		if !exists || len(task.Dependencies) == 0 {
			phases[id] = 0
			return 0
		}

		maxDepPhase := -1
		for _, depID := range task.Dependencies {
			depPhase := getPhase(depID, depth+1)
			if depPhase > maxDepPhase {
				maxDepPhase = depPhase
			}
		}

		calculated := maxDepPhase + 1
		phases[id] = calculated
		return calculated
	}

	maxPhase := 0
	for _, t := range tasks {
		p := getPhase(t.ID, 0)
		if p > maxPhase {
			maxPhase = p
		}
	}

	totalPhases := 0
	if len(tasks) > 0 {
		totalPhases = maxPhase + 1
	}

	return phases, totalPhases
}

// DetermineBlockedState evaluates if a task is currently blocked by unfinished dependencies.
func DetermineBlockedState(task CoordinationTask, taskStatuses map[string]string) (bool, string) {
	if task.Status == "success" || task.Status == "canceled" || task.Status == "failed" {
		return false, ""
	}
	var uncompleted []string
	for _, depID := range task.Dependencies {
		status := taskStatuses[depID]
		if status != "success" {
			uncompleted = append(uncompleted, depID)
		}
	}
	if len(uncompleted) > 0 {
		sort.Strings(uncompleted)
		return true, fmt.Sprintf("awaiting dependencies: %s", strings.Join(uncompleted, ", "))
	}
	return false, ""
}
