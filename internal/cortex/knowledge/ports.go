package knowledge

import (
	"context"
	"fmt"
	"time"
)

// Store is the narrow persistence interface for knowledge items.
// It does not expose raw storage, SQL, or unvalidated queries.
type Store interface {
	Open(ctx context.Context) error
	Close() error
	SchemaVersion(ctx context.Context) (string, error)

	// Save creates or updates a knowledge item. The item must pass Validate().
	Save(ctx context.Context, item Item) (Item, error)
	// Get retrieves a knowledge item by ID.
	Get(ctx context.Context, id KnowledgeID) (Item, error)
	// Delete marks an item as tombstoned (soft delete).
	Delete(ctx context.Context, id KnowledgeID) error
	// List returns items matching the filter.
	List(ctx context.Context, filter Filter) ([]Item, error)
	// PruneExpired removes all expired items.
	PruneExpired(ctx context.Context) (int, error)
}

// Filter is a safe, validated query for knowledge items.
type Filter struct {
	WorkspaceID      *WorkspaceID
	ProjectID        *ProjectID
	Kind             *KnowledgeKind
	Lifecycle        *LifecycleState
	ValidationStatus *ValidationStatus
	ActiveOnly       bool
	MinVersion       int
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
	if f.Kind != nil && !validKind(*f.Kind) {
		return newError(ErrInvalidRequest, fmt.Sprintf("unsupported kind %q", *f.Kind))
	}
	if f.Lifecycle != nil && !validLifecycle(*f.Lifecycle) {
		return newError(ErrInvalidRequest, fmt.Sprintf("unsupported lifecycle %q", *f.Lifecycle))
	}
	if f.ValidationStatus != nil && !validStatus(*f.ValidationStatus) {
		return newError(ErrInvalidRequest, fmt.Sprintf("unsupported validation status %q", *f.ValidationStatus))
	}
	return nil
}

// KnowledgePort is the application-facing interface for knowledge lifecycle operations.
// It enforces validation gates and provenance preservation.
type KnowledgePort interface {
	// Capture creates a new draft knowledge item from untrusted input.
	// The item starts with ValidationStatusUnvalidated and LifecycleDraft.
	Capture(ctx context.Context, req CaptureRequest) (Item, error)
	// Validate promotes an item to Verified/Active by authority.
	Validate(ctx context.Context, id KnowledgeID, validatedBy, reason string) (Item, error)
	// Reject marks an item as rejected; it cannot become Active.
	Reject(ctx context.Context, id KnowledgeID, validatedBy, reason string) (Item, error)
	// Update amends content with new provenance and increments version.
	// ValidationStatus resets to Unvalidated and requires re-validation.
	Update(ctx context.Context, req UpdateRequest) (Item, error)
	// Correct applies a content correction with full provenance.
	Correct(ctx context.Context, req CorrectionRequest) (Item, error)
	// Deprecate marks an active item as deprecated.
	Deprecate(ctx context.Context, id KnowledgeID) (Item, error)
	// Expire processes lifecycle expiry transitions.
	Expire(ctx context.Context, id KnowledgeID) (Item, error)
	// Get retrieves a knowledge item.
	Get(ctx context.Context, id KnowledgeID) (Item, error)
	// List queries items with safe filters.
	List(ctx context.Context, filter Filter) ([]Item, error)
	// PruneExpired removes all expired items.
	PruneExpired(ctx context.Context) (int, error)
}

// CaptureRequest is the input for creating a new knowledge item.
type CaptureRequest struct {
	WorkspaceID WorkspaceID
	ProjectID   ProjectID
	Title       string
	Content     string
	Kind        KnowledgeKind
	Tags        []string
	Provenance  Provenance
	ExpiresAt   *time.Time
}

// UpdateRequest is the input for updating a knowledge item.
type UpdateRequest struct {
	ID         KnowledgeID
	Title      string
	Content    string
	Kind       KnowledgeKind
	Tags       []string
	Provenance Provenance
	ExpiresAt  *time.Time
}

// CorrectionRequest represents a provenance-tracked content correction.
type CorrectionRequest struct {
	ID         KnowledgeID
	Content    string
	Reason     string
	Author     string
	License    string
	Provenance Provenance
}
