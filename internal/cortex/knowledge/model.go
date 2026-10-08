package knowledge

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"regexp"
	"strings"
	"time"
)

// ContractVersion is the stable domain contract version for knowledge items.
const ContractVersion = "cortexos.knowledge.v1"

// KnowledgeID is the unique identifier for a knowledge entity.
type KnowledgeID string

// ProjectID and WorkspaceID are opaque domain identifiers.
type ProjectID string
type WorkspaceID string

// KnowledgeKind identifies the functional categorization of knowledge.
type KnowledgeKind string

const (
	KindArchitecture KnowledgeKind = "architecture"
	KindDecision     KnowledgeKind = "decision"
	KindConvention   KnowledgeKind = "convention"
	KindRunbook      KnowledgeKind = "runbook"
	KindReference    KnowledgeKind = "reference"
	KindSynthesis    KnowledgeKind = "synthesis"
)

// LifecycleState models the governance lifecycle of a knowledge record.
type LifecycleState string

const (
	LifecycleDraft      LifecycleState = "draft"
	LifecycleActive     LifecycleState = "active"
	LifecycleDeprecated LifecycleState = "deprecated"
	LifecycleExpired    LifecycleState = "expired"
	LifecycleTombstone  LifecycleState = "tombstone"
)

// ValidationStatus represents the trust and verification gate.
// ADR-0005: Worker claims are strictly untrusted until verified by authority.
type ValidationStatus string

const (
	StatusUnvalidated ValidationStatus = "unvalidated"
	StatusVerified    ValidationStatus = "verified"
	StatusRejected    ValidationStatus = "rejected"
)

// Provenance captures source attribution, origin authority, and licensing.
type Provenance struct {
	SourceKind       string     `json:"sourceKind"`           // e.g., "worker_output", "vault_note", "user_input", "git_commit"
	SourceURI        string     `json:"sourceUri"`            // relative path or logical source identifier
	SourceHash       string     `json:"sourceHash,omitempty"` // authoritative source content hash
	Author           string     `json:"author"`               // human or logical agent attribution
	Attribution      string     `json:"attribution"`          // copyright or authorship statement
	License          string     `json:"license,omitempty"`    // SPDX or permission grant
	TaskID           string     `json:"taskId,omitempty"`
	ExecutionID      string     `json:"executionId,omitempty"`
	WorkerIdentity   string     `json:"workerIdentity,omitempty"` // evidence tracking only; not an authority handle
	CapturedAt       time.Time  `json:"capturedAt"`
	ValidatedAt      *time.Time `json:"validatedAt,omitempty"`
	ValidatedBy      string     `json:"validatedBy,omitempty"` // Inspector, Orchestra, or User
	ValidationReason string     `json:"validationReason,omitempty"`
}

// Item represents a single governed unit of durable knowledge.
type Item struct {
	ID               KnowledgeID      `json:"id"`
	WorkspaceID      WorkspaceID      `json:"workspaceId"`
	ProjectID        ProjectID        `json:"projectId"`
	Title            string           `json:"title"`
	Content          string           `json:"content"`
	Kind             KnowledgeKind    `json:"kind"`
	Tags             []string         `json:"tags,omitempty"`
	Provenance       Provenance       `json:"provenance"`
	Lifecycle        LifecycleState   `json:"lifecycle"`
	ValidationStatus ValidationStatus `json:"validationStatus"`
	ContentHash      string           `json:"contentHash"`
	Version          int              `json:"version"`
	CreatedAt        time.Time        `json:"createdAt"`
	UpdatedAt        time.Time        `json:"updatedAt"`
	ExpiresAt        *time.Time       `json:"expiresAt,omitempty"`
	SchemaVersion    string           `json:"schemaVersion"`
}

// HashContent produces a canonical sha256 checksum with prefix.
func HashContent(content string) string {
	sum := sha256.Sum256([]byte(content))
	return "sha256:" + hex.EncodeToString(sum[:])
}

var (
	emailPattern  = regexp.MustCompile(`(?i)\b[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Za-z]{2,}\b`)
	secretPattern = regexp.MustCompile(`(?i)\b(?:api[_-]?key|secret|token|password|credential)\s*[:=]\s*\S+`)
	bearerPattern = regexp.MustCompile(`(?i)Bearer\s+[A-Za-z0-9\-._~+/]+=*`)
)

// RedactedContent returns content stripped of standard secrets and credentials.
func (i Item) RedactedContent() string {
	res := i.Content
	res = emailPattern.ReplaceAllString(res, "[EMAIL]")
	res = secretPattern.ReplaceAllString(res, "[REDACTED]")
	res = bearerPattern.ReplaceAllString(res, "Bearer [REDACTED]")
	return res
}

// IsExpired checks if the item has passed its expiry deadline.
func (i Item) IsExpired(at time.Time) bool {
	return i.ExpiresAt != nil && at.After(*i.ExpiresAt)
}

// Validate ensures structural validity, format adherence, and fail-closed security.
func (i Item) Validate() error {
	if i.ID == "" {
		return newError(ErrInvalidRequest, "knowledge id is required")
	}
	if err := validateToken(string(i.ID), "knowledge id"); err != nil {
		return err
	}
	if i.WorkspaceID == "" {
		return newError(ErrInvalidRequest, "workspace id is required")
	}
	if err := validateToken(string(i.WorkspaceID), "workspace id"); err != nil {
		return err
	}
	if i.ProjectID == "" {
		return newError(ErrInvalidRequest, "project id is required")
	}
	if err := validateToken(string(i.ProjectID), "project id"); err != nil {
		return err
	}
	if strings.TrimSpace(i.Title) == "" {
		return newError(ErrInvalidRequest, "title is required")
	}
	if strings.TrimSpace(i.Content) == "" {
		return newError(ErrInvalidRequest, "content is required")
	}
	if !validKind(i.Kind) {
		return newError(ErrInvalidRequest, fmt.Sprintf("unsupported knowledge kind %q", i.Kind))
	}
	if !validLifecycle(i.Lifecycle) {
		return newError(ErrInvalidRequest, fmt.Sprintf("unsupported lifecycle state %q", i.Lifecycle))
	}
	if !validStatus(i.ValidationStatus) {
		return newError(ErrInvalidRequest, fmt.Sprintf("unsupported validation status %q", i.ValidationStatus))
	}
	if i.Version < 1 {
		return newError(ErrInvalidRequest, "version must be at least 1")
	}
	if i.CreatedAt.IsZero() || i.UpdatedAt.IsZero() {
		return newError(ErrInvalidRequest, "creation and update timestamps are required")
	}
	if i.UpdatedAt.Before(i.CreatedAt) {
		return newError(ErrInvalidRequest, "updatedAt must not precede createdAt")
	}
	if i.ExpiresAt != nil && i.ExpiresAt.Before(i.CreatedAt) {
		return newError(ErrInvalidRequest, "expiresAt must not precede createdAt")
	}
	if i.SchemaVersion != ContractVersion {
		return newError(ErrUnsupportedVersion, fmt.Sprintf("unsupported schema version %q", i.SchemaVersion))
	}
	expectedHash := HashContent(i.Content)
	if i.ContentHash != "" && i.ContentHash != expectedHash {
		return newError(ErrConflict, "content hash mismatch")
	}

	// Provenance verification
	if i.Provenance.SourceKind == "" {
		return newError(ErrInvalidRequest, "provenance sourceKind is required")
	}
	if i.Provenance.Author == "" {
		return newError(ErrInvalidRequest, "provenance author is required")
	}
	if i.Provenance.CapturedAt.IsZero() {
		return newError(ErrInvalidRequest, "provenance capturedAt timestamp is required")
	}

	// ADR-0005 security gate: Active knowledge CANNOT be unvalidated or rejected
	if i.Lifecycle == LifecycleActive && i.ValidationStatus != StatusVerified {
		return newError(ErrUntrustedClaim, "active knowledge must be verified by authority before promotion")
	}

	for _, tag := range i.Tags {
		if strings.ContainsAny(tag, "\x00\r\n") {
			return newError(ErrInvalidRequest, "tag contains control characters")
		}
	}
	return nil
}

func validKind(k KnowledgeKind) bool {
	switch k {
	case KindArchitecture, KindDecision, KindConvention, KindRunbook, KindReference, KindSynthesis:
		return true
	default:
		return false
	}
}

func validLifecycle(s LifecycleState) bool {
	switch s {
	case LifecycleDraft, LifecycleActive, LifecycleDeprecated, LifecycleExpired, LifecycleTombstone:
		return true
	default:
		return false
	}
}

func validStatus(s ValidationStatus) bool {
	switch s {
	case StatusUnvalidated, StatusVerified, StatusRejected:
		return true
	default:
		return false
	}
}

func validateToken(token, name string) error {
	if token == "" {
		return newError(ErrInvalidRequest, fmt.Sprintf("%s is required", name))
	}
	if len(token) > 128 {
		return newError(ErrInvalidRequest, fmt.Sprintf("%s exceeds max length of 128", name))
	}
	for i := 0; i < len(token); i++ {
		c := token[i]
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '-' || c == '_' || c == '.' {
			continue
		}
		return newError(ErrInvalidRequest, fmt.Sprintf("%s contains invalid character %q", name, c))
	}
	return nil
}
