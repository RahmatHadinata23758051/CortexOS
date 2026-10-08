package application

import (
	"context"
	"errors"
	"time"

	"github.com/RahmatHadinata23758051/CortexOS/internal/cortex/staff"
)

const StaffBridgeSchemaVersion = "cortexos.staff.bridge.v1"

var ErrStaffInvalidRequest = errors.New("invalid staff bridge request")

// StaffPort is the application boundary for Staff observability. It exposes
// safe, typed queries without leaking SQL, process handles, raw provider output,
// secrets, or internal permission details.
type StaffPort interface {
	// GetStaff returns a safe summary of a Staff definition by ID.
	GetStaff(ctx context.Context, id staff.StaffID) (StaffSummary, error)
	// ListStaff returns all Staff summaries with optional workspace filter.
	ListStaff(ctx context.Context, filter StaffListFilter) ([]StaffSummary, error)
	// ListStaffByWorkspace returns Staff summaries assigned to a workspace.
	ListStaffByWorkspace(ctx context.Context, workspaceID staff.WorkspaceID) ([]StaffSummary, error)
	// GetStaffCapabilities returns capability metadata for all registered Staff capabilities.
	GetStaffCapabilities(ctx context.Context) ([]StaffCapabilitySummary, error)
	// GetStaffAssignment returns the current workspace assignment for a Staff member.
	GetStaffAssignment(ctx context.Context, id staff.StaffID) (StaffAssignmentSummary, error)
}

// StaffSummary is the safe, serialized form of a Staff definition.
// It intentionally omits: permission patterns, skill references, memory references,
// process identity, execution evidence, and success/merge authority.
type StaffSummary struct {
	ID            staff.StaffID           `json:"id"`
	Name          string                  `json:"name"`
	Role          staff.Role              `json:"role"`
	Capabilities  []staff.Capability      `json:"capabilities"`
	WorkspaceID   staff.WorkspaceID       `json:"workspaceId"`
	ProjectID     staff.ProjectID         `json:"projectId,omitempty"`
	WorktreeID    staff.WorktreeID        `json:"worktreeId,omitempty"`
	Lifecycle     staff.LifecycleState    `json:"lifecycle"`
	Availability  staff.AvailabilityState `json:"availability"`
	SchemaVersion string                  `json:"schemaVersion"`
	CreatedAt     time.Time               `json:"createdAt"`
	UpdatedAt     time.Time               `json:"updatedAt"`
}

// StaffListFilter is a safe query filter for Staff summaries.
type StaffListFilter struct {
	WorkspaceID *staff.WorkspaceID
	Role        *staff.Role
	ActiveOnly  bool
}

// StaffCapabilitySummary is the safe, serialized form of a Staff capability mapping.
type StaffCapabilitySummary struct {
	Capability       staff.Capability `json:"capability"`
	Description      string           `json:"description,omitempty"`
	RequiredTools    []string         `json:"requiredTools"`
	PreferredEngines []string         `json:"preferredEngines,omitempty"`
	AllowedEngines   []string         `json:"allowedEngines,omitempty"`
	MinMemoryMB      int              `json:"minMemoryMB,omitempty"`
	RequiresAsk      bool             `json:"requiresAsk,omitempty"`
	SchemaVersion    string           `json:"schemaVersion"`
}

// StaffAssignmentSummary is the safe, serialized form of a Staff workspace assignment.
// It does not expose filesystem paths, process handles, or worker pool details.
type StaffAssignmentSummary struct {
	StaffID       staff.StaffID     `json:"staffId"`
	WorkspaceID   staff.WorkspaceID `json:"workspaceId"`
	ProjectID     staff.ProjectID   `json:"projectId,omitempty"`
	WorktreeID    staff.WorktreeID  `json:"worktreeId,omitempty"`
	SchemaVersion string            `json:"schemaVersion"`
}

// StaffBridgeError is the stable, generic error returned across the Staff bridge.
type StaffBridgeError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func (e *StaffBridgeError) Error() string {
	if e == nil {
		return "<nil>"
	}
	return e.Code + ": " + e.Message
}

type StaffCapabilitiesRequest struct {
	SchemaVersion string `json:"schemaVersion"`
}

type StaffListRequest struct {
	SchemaVersion string          `json:"schemaVersion"`
	Filter        StaffListFilter `json:"filter,omitempty"`
}

type StaffByWorkspaceRequest struct {
	SchemaVersion string            `json:"schemaVersion"`
	WorkspaceID   staff.WorkspaceID `json:"workspaceId"`
}

type StaffRequest struct {
	SchemaVersion string        `json:"schemaVersion"`
	StaffID       staff.StaffID `json:"staffId"`
}

type StaffAssignmentRequest struct {
	SchemaVersion string        `json:"schemaVersion"`
	StaffID       staff.StaffID `json:"staffId"`
}

// validateStaffRequest checks context cancellation and schema version.
func validateStaffRequest(ctx context.Context, schemaVersion string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if schemaVersion != StaffBridgeSchemaVersion {
		return errors.New("staff: unsupported schema version " + schemaVersion)
	}
	return nil
}

func staffBridgeError(err error) *StaffBridgeError {
	if err == nil {
		return nil
	}
	code := "staff.unknown"
	message := err.Error()
	var staffErr *staff.Error
	if errors.As(err, &staffErr) {
		code = string(staffErr.Code)
	} else if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		code = string(staff.ErrCanceled)
		message = "operation canceled"
	}
	return &StaffBridgeError{Code: code, Message: message}
}
