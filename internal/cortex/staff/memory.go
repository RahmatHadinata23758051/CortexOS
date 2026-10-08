package staff

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"
)

// MemoryKind represents the category of memory.
type MemoryKind string

const (
	MemoryKindShortTerm MemoryKind = "short_term"
	MemoryKindLongTerm  MemoryKind = "long_term"
	MemoryKindEpisodic  MemoryKind = "episodic"
)

// MemoryContext is a typed, scoped memory entry for a Staff member.
// It carries provenance metadata and is advisory only — it NEVER grants
// execution authority, tool permissions, or success/merge verdicts.
type MemoryContext struct {
	ID          string      `json:"id"`
	StaffID     StaffID     `json:"staffId"`
	Kind        MemoryKind  `json:"kind"`
	Content     string      `json:"content"`
	Provenance  Provenance  `json:"provenance"`
	Relevance   float64     `json:"relevance"` // 0.0 to 1.0
	Priority    int         `json:"priority"`  // higher = more important
	CreatedAt   time.Time   `json:"createdAt"`
	UpdatedAt   time.Time   `json:"updatedAt"`
	ExpiresAt   *time.Time  `json:"expiresAt,omitempty"`
	WorkspaceID WorkspaceID `json:"workspaceId"`
	Tags        []string    `json:"tags,omitempty"`
}

// Provenance tracks the origin and lineage of a memory entry.
type Provenance struct {
	Source      string    `json:"source"` // e.g., "user_input", "tool_output", "skill_injection", "memory_synthesis"
	TaskID      string    `json:"taskId,omitempty"`
	ExecutionID string    `json:"executionId,omitempty"`
	Author      string    `json:"author,omitempty"`
	Attribution string    `json:"attribution,omitempty"`
	License     string    `json:"license,omitempty"`
	Timestamp   time.Time `json:"timestamp"`
}

// MemorySelectionRequest defines criteria for selecting relevant memory contexts.
type MemorySelectionRequest struct {
	StaffID       StaffID
	WorkspaceID   WorkspaceID
	ProjectID     ProjectID
	Kinds         []MemoryKind
	MaxItems      int
	MaxTotalChars int
	MinRelevance  float64
	RequiredTags  []string
	ExcludeTags   []string
	PreferRecency bool
	Query         string // optional semantic query for future embedding-based selection
}

// MemorySelectionResult is the deterministic, bounded set of selected memory contexts.
type MemorySelectionResult struct {
	ContractVersion  string          `json:"contractVersion"`
	StaffID          StaffID         `json:"staffId"`
	SelectedContexts []MemoryContext `json:"selectedContexts"`
	TotalChars       int             `json:"totalChars"`
	SelectionReason  string          `json:"selectionReason"`
	SelectedAt       time.Time       `json:"selectedAt"`
}

// MemoryContextPort is the narrow interface for memory context operations.
// It does not expose raw storage, database handles, or unvalidated queries.
type MemoryContextPort interface {
	// Save stores or updates a memory context. The context must pass Validate() before passing.
	Save(ctx context.Context, memory MemoryContext) (MemoryContext, error)
	// Get retrieves a memory context by ID.
	Get(ctx context.Context, id string) (MemoryContext, error)
	// Delete removes a memory context by ID.
	Delete(ctx context.Context, id string) error
	// List returns memory contexts matching the filter.
	List(ctx context.Context, filter MemoryFilter) ([]MemoryContext, error)
	// Select returns a deterministic, bounded set of relevant memory contexts.
	Select(ctx context.Context, req MemorySelectionRequest) (MemorySelectionResult, error)
	// PruneExpired removes all expired memory contexts.
	PruneExpired(ctx context.Context) (int, error)
}

// MemoryFilter is a safe, validated query for memory contexts.
type MemoryFilter struct {
	StaffID     *StaffID
	WorkspaceID *WorkspaceID
	ProjectID   *ProjectID
	Kind        *MemoryKind
	MinPriority int
	MaxPriority int
	ActiveOnly  bool // if true, excludes expired entries
}

func (f MemoryFilter) Validate() error {
	if f.StaffID != nil {
		if err := validateToken(string(*f.StaffID), "staff id"); err != nil {
			return WrapError(ErrInvalidRequest, "invalid staff filter", err)
		}
	}
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
	if f.Kind != nil && !validMemoryKind(*f.Kind) {
		return newError(ErrInvalidRequest, fmt.Sprintf("unsupported memory kind %q", *f.Kind))
	}
	return nil
}

// Validate checks that a memory context is well-formed and safe.
// It rejects any content that could be used as execution authority.
func (m MemoryContext) Validate() error {
	if m.ID == "" {
		return newError(ErrInvalidRequest, "memory id is required")
	}
	if err := validateToken(m.ID, "memory id"); err != nil {
		return err
	}
	if m.StaffID == "" {
		return newError(ErrInvalidRequest, "staff id is required")
	}
	if err := validateToken(string(m.StaffID), "staff id"); err != nil {
		return err
	}
	if m.WorkspaceID == "" {
		return newError(ErrInvalidRequest, "workspace id is required")
	}
	if err := validateToken(string(m.WorkspaceID), "workspace id"); err != nil {
		return err
	}
	if !validMemoryKind(m.Kind) {
		return newError(ErrInvalidRequest, fmt.Sprintf("unsupported memory kind %q", m.Kind))
	}
	if strings.TrimSpace(m.Content) == "" {
		return newError(ErrInvalidRequest, "memory content is required")
	}
	if m.Relevance < 0 || m.Relevance > 1 {
		return newError(ErrInvalidRequest, "relevance must be in range [0, 1]")
	}
	if m.Priority < 0 {
		return newError(ErrInvalidRequest, "priority must not be negative")
	}
	if m.CreatedAt.IsZero() || m.UpdatedAt.IsZero() {
		return newError(ErrInvalidRequest, "createdAt and updatedAt are required")
	}
	if m.UpdatedAt.Before(m.CreatedAt) {
		return newError(ErrInvalidRequest, "updatedAt must not precede createdAt")
	}
	if m.ExpiresAt != nil && m.ExpiresAt.Before(m.CreatedAt) {
		return newError(ErrInvalidRequest, "expiresAt must not precede createdAt")
	}
	if m.Provenance.Timestamp.IsZero() {
		return newError(ErrInvalidRequest, "provenance timestamp is required")
	}
	for _, tag := range m.Tags {
		if strings.ContainsAny(tag, "\x00\r\n") {
			return newError(ErrInvalidRequest, "tag contains control characters")
		}
	}
	return nil
}

// IsExpired returns true if the memory context has expired.
func (m MemoryContext) IsExpired(at time.Time) bool {
	return m.ExpiresAt != nil && at.After(*m.ExpiresAt)
}

var (
	emailRegex  = regexp.MustCompile(`(?i)\b[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Za-z]{2,}\b`)
	secretRegex = regexp.MustCompile(`(?i)\b(?:api[_-]?key|secret|token|password|credential)\s*[:=]\s*\S+`)
	bearerRegex = regexp.MustCompile(`(?i)Bearer\s+[A-Za-z0-9\-._~+/]+=*`)
)

// RedactedContent returns a copy of the memory with sensitive patterns redacted.
// This is advisory — the caller is responsible for applying redaction before
// passing memory into any execution envelope or untrusted context.
func (m MemoryContext) RedactedContent() string {
	redacted := m.Content
	redacted = emailRegex.ReplaceAllString(redacted, "[EMAIL]")
	redacted = secretRegex.ReplaceAllString(redacted, "[REDACTED]")
	redacted = bearerRegex.ReplaceAllString(redacted, "Bearer [REDACTED]")
	return redacted
}

func validMemoryKind(kind MemoryKind) bool {
	return kind == MemoryKindShortTerm || kind == MemoryKindLongTerm || kind == MemoryKindEpisodic
}

// MemorySelectionContractVersion is the stable contract version for memory selection results.
const MemorySelectionContractVersion = "cortexos.memory.selection.v1"

// NewMemorySelectionResult creates a validated selection result.
func NewMemorySelectionResult(staffID StaffID, contexts []MemoryContext, reason string) (MemorySelectionResult, error) {
	res := MemorySelectionResult{
		ContractVersion:  MemorySelectionContractVersion,
		StaffID:          staffID,
		SelectedContexts: contexts,
		SelectionReason:  reason,
		SelectedAt:       time.Now().UTC(),
	}
	for _, ctx := range contexts {
		res.TotalChars += len(ctx.Content)
	}
	return res, nil
}
