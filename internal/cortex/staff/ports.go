package staff

import (
	"context"
	"fmt"
	"time"
)

// Store is the narrow persistence interface for Staff definitions. It does not
// expose SQL, raw database handles, or unvalidated queries.
type Store interface {
	// Open initializes the store. It must be called before any other method.
	Open(context.Context) error
	// Close releases all resources.
	Close() error
	// SchemaVersion returns the contract version of the persisted schema.
	SchemaVersion(context.Context) (string, error)

	// SaveDefinition inserts or updates a Staff definition. The definition must
	// pass Validate() before being passed to this method.
	SaveDefinition(context.Context, Definition) (Definition, error)
	// GetDefinition retrieves a Staff definition by ID.
	GetDefinition(context.Context, StaffID) (Definition, error)
	// DeleteDefinition removes a Staff definition.
	DeleteDefinition(context.Context, StaffID) error
	// ListDefinitions returns all Staff definitions.
	ListDefinitions(context.Context) ([]Definition, error)
	// ListDefinitionsByWorkspace returns Staff definitions assigned to a workspace.
	ListDefinitionsByWorkspace(context.Context, WorkspaceID) ([]Definition, error)
	// ListDefinitionsByProject returns Staff definitions assigned to a project.
	ListDefinitionsByProject(context.Context, ProjectID) ([]Definition, error)
	// ListDefinitionsByRole returns Staff definitions with a specific role.
	ListDefinitionsByRole(context.Context, Role) ([]Definition, error)
}

// Filter represents a safe, validated query for Staff definitions. It avoids
// stringly-typed or SQL-like filters that would couple callers to persistence.
type Filter struct {
	WorkspaceID *WorkspaceID
	ProjectID   *ProjectID
	Role        *Role
	ActiveOnly  bool
}

func (f Filter) Validate() error {
	if f.WorkspaceID != nil {
		if err := validateToken(string(*f.WorkspaceID), "workspace id"); err != nil {
			return WrapError(ErrInvalidRequest, "invalid workspace filter", err)
		}
	}
	if f.ProjectID != nil {
		if err := validateToken(string(*f.ProjectID), "project id"); err != nil {
			return WrapError(ErrInvalidRequest, "invalid project filter", err)
		}
	}
	if f.Role != nil {
		if err := validateToken(string(*f.Role), "role"); err != nil {
			return WrapError(ErrInvalidRequest, "invalid role filter", err)
		}
		if !validRole(*f.Role) {
			return newError(ErrInvalidRequest, fmt.Sprintf("unsupported role %q in filter", *f.Role))
		}
	}
	return nil
}

// Router selects Staff candidates for a given assignment request. It does not
// bind to workers, processes, or scheduling; those are later Harness/Orchestra
// responsibilities.
type Router interface {
	// SelectCandidates returns Staff definitions that match the request.
	// The result is a filtered, ordered set of logical Staff — not worker handles.
	SelectCandidates(context.Context, RouterRequest) ([]Definition, error)
}

// RouterRequest is a capability and scope query, not a process dispatch.
type RouterRequest struct {
	RequiredCapabilities []Capability
	WorkspaceID          WorkspaceID
	ProjectID            ProjectID
	RoleHint             Role
	ExcludeIDs           []StaffID
}

func (r RouterRequest) Validate() error {
	if r.WorkspaceID == "" {
		return newError(ErrInvalidRequest, "workspace id is required")
	}
	if err := validateToken(string(r.WorkspaceID), "workspace id"); err != nil {
		return WrapError(ErrInvalidRequest, "invalid workspace id", err)
	}
	if r.ProjectID != "" {
		if err := validateToken(string(r.ProjectID), "project id"); err != nil {
			return WrapError(ErrInvalidRequest, "invalid project id", err)
		}
	}
	if r.RoleHint != "" {
		if err := validateToken(string(r.RoleHint), "role hint"); err != nil {
			return WrapError(ErrInvalidRequest, "invalid role hint", err)
		}
		if !validRole(r.RoleHint) {
			return newError(ErrInvalidRequest, fmt.Sprintf("unsupported role hint %q", r.RoleHint))
		}
	}
	for _, cap := range r.RequiredCapabilities {
		if err := validateToken(string(cap), "capability"); err != nil {
			return WrapError(ErrInvalidRequest, "invalid capability", err)
		}
	}
	for _, id := range r.ExcludeIDs {
		if err := validateToken(string(id), "exclude staff id"); err != nil {
			return WrapError(ErrInvalidRequest, "invalid exclude id", err)
		}
	}
	return nil
}

// MemoryMetadata describes a memory entity without exposing memory content,
// provider embeddings, or raw storage paths.
type MemoryMetadata struct {
	ID          string    `json:"id"`
	Kind        string    `json:"kind"`
	Version     string    `json:"version"`
	ContentHash string    `json:"contentHash,omitempty"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

// MemoryPort is the narrow interface for resolving memory references owned
// by later Phase 5 memory adapters. Staff definitions only carry references;
// content retrieval, embedding, and RAG happen behind this port.
type MemoryPort interface {
	ResolveReference(ctx context.Context, staffID StaffID, ref MemoryReference) (MemoryMetadata, error)
	ListReferences(ctx context.Context, staffID StaffID) ([]MemoryReference, error)
}

// AssignmentPort is the narrow interface for updating workspace and project
// assignment for a Staff member. It does not spawn or kill worker processes.
type AssignmentPort interface {
	AssignWorkspace(ctx context.Context, id StaffID, assignment WorkspaceAssignment) (Definition, error)
	UnassignWorkspace(ctx context.Context, id StaffID) (Definition, error)
	GetAssignment(ctx context.Context, id StaffID) (WorkspaceAssignment, error)
}

// BridgePort is the narrow interface consumed by application/bridge layers
// to expose safe Staff queries to desktop presentation.
type BridgePort interface {
	GetStaffSummary(ctx context.Context, id StaffID) (Summary, error)
	ListStaffSummaries(ctx context.Context) ([]Summary, error)
	ListStaffByWorkspace(ctx context.Context, workspaceID WorkspaceID) ([]Summary, error)
}

// AssignmentNotifier is an optional interface for components that must react
// to Staff workspace assignment changes. It does not trigger worker lifecycle.
type AssignmentNotifier interface {
	NotifyAssignmentChanged(context.Context, Definition) error
}
