package staff

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// Summary is the safe bridge representation of a Staff definition. It is
// intentionally smaller than Definition: permission patterns and memory/skill
// references are not exposed to untrusted presentation consumers by default.
type Summary struct {
	ID            StaffID           `json:"id"`
	Name          string            `json:"name"`
	Role          Role              `json:"role"`
	Capabilities  []Capability      `json:"capabilities"`
	WorkspaceID   WorkspaceID       `json:"workspaceId"`
	ProjectID     ProjectID         `json:"projectId,omitempty"`
	WorktreeID    WorktreeID        `json:"worktreeId,omitempty"`
	Lifecycle     LifecycleState    `json:"lifecycle"`
	Availability  AvailabilityState `json:"availability"`
	SchemaVersion string            `json:"schemaVersion"`
	CreatedAt     time.Time         `json:"createdAt"`
	UpdatedAt     time.Time         `json:"updatedAt"`
}

// ForbiddenKeys are field names that must never appear in a serialized or
// deserialized Staff definition. They represent process identity, secrets,
// execution authority, and raw provider output that are excluded by contract.
var ForbiddenKeys = map[string]struct{}{
	"workerID":       {},
	"processID":      {},
	"pid":            {},
	"cliIdentity":    {},
	"processHandle":  {},
	"secret":         {},
	"secrets":        {},
	"token":          {},
	"tokens":         {},
	"apiKey":         {},
	"password":       {},
	"credential":     {},
	"credentials":    {},
	"rawProviderOut": {},
	"providerOutput": {},
	"selfApprove":    {},
	"mergeAuthority": {},
	"successVerdict": {},
	"executionID":    {},
}

// isForbiddenKey returns true if a key name matches any forbidden field
// regardless of casing, underscores, or hyphens.
func isForbiddenKey(key string) bool {
	normalized := strings.ToLower(strings.ReplaceAll(strings.ReplaceAll(key, "_", ""), "-", ""))
	for fk := range ForbiddenKeys {
		normFK := strings.ToLower(strings.ReplaceAll(strings.ReplaceAll(fk, "_", ""), "-", ""))
		if normalized == normFK {
			return true
		}
	}
	return false
}

func (d *Definition) UnmarshalJSON(data []byte) error {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	for k := range raw {
		if _, forbidden := ForbiddenKeys[k]; forbidden || isForbiddenKey(k) {
			return newError(ErrInvalidRequest, fmt.Sprintf("forbidden field %q in Staff definition", k))
		}
	}
	type alias Definition
	if err := json.Unmarshal(data, (*alias)(d)); err != nil {
		return err
	}
	return d.Validate()
}

func (d Definition) Summary() Summary {
	return Summary{
		ID: d.ID, Name: d.Name, Role: d.Role,
		Capabilities: append([]Capability(nil), d.Capabilities...),
		WorkspaceID:  d.Workspace.WorkspaceID, ProjectID: d.Workspace.ProjectID,
		WorktreeID: d.Workspace.WorktreeID, Lifecycle: d.Lifecycle,
		Availability: d.Availability, SchemaVersion: d.SchemaVersion,
		CreatedAt: d.CreatedAt, UpdatedAt: d.UpdatedAt,
	}
}

// MarshalJSON validates before serialization so malformed definitions cannot
// cross a contract boundary. Definition fields contain no secret or process
// handle by construction; provider output and execution evidence are absent.
func (d Definition) MarshalJSON() ([]byte, error) {
	if err := d.Validate(); err != nil {
		return nil, err
	}
	type alias Definition
	return json.Marshal(alias(d))
}

// MarshalJSON makes the safe summary explicit and validates enum-bearing data.
func (s Summary) MarshalJSON() ([]byte, error) {
	if err := validateToken(string(s.ID), "id"); err != nil {
		return nil, err
	}
	if err := validateDisplayName(s.Name, "name"); err != nil {
		return nil, err
	}
	if err := validateToken(string(s.WorkspaceID), "workspace id"); err != nil {
		return nil, err
	}
	if s.ProjectID != "" {
		if err := validateToken(string(s.ProjectID), "project id"); err != nil {
			return nil, err
		}
	}
	if s.WorktreeID != "" {
		if err := validateToken(string(s.WorktreeID), "worktree id"); err != nil {
			return nil, err
		}
	}
	if !validRole(s.Role) {
		return nil, newError(ErrInvalidRequest, fmt.Sprintf("unsupported role %q in summary", s.Role))
	}
	if !validLifecycle(s.Lifecycle) || !validAvailability(s.Availability) {
		return nil, newError(ErrInvalidRequest, "invalid summary lifecycle or availability")
	}
	if s.SchemaVersion != ContractVersion {
		return nil, newError(ErrUnsupportedVersion, "unsupported summary schema version")
	}
	type alias Summary
	return json.Marshal(alias(s))
}

// JSONSafe returns a bridge-safe, deterministic representation of a Staff
// definition. It exists for callers that need to make the boundary explicit.
func (d Definition) JSONSafe() (Summary, error) {
	if err := d.Validate(); err != nil {
		return Summary{}, err
	}
	return d.Summary(), nil
}

// isForbiddenSummaryKey returns true if a key name is forbidden in a Summary
// (includes Definition forbidden keys plus fields that must never appear in summaries).
func isForbiddenSummaryKey(key string) bool {
	if isForbiddenKey(key) {
		return true
	}
	normalized := strings.ToLower(strings.ReplaceAll(strings.ReplaceAll(key, "_", ""), "-", ""))
	summaryForbidden := map[string]struct{}{
		"permissions": {},
		"skills":      {},
		"memory":      {},
	}
	_, ok := summaryForbidden[normalized]
	return ok
}

func (s *Summary) UnmarshalJSON(data []byte) error {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	for k := range raw {
		if isForbiddenSummaryKey(k) {
			return newError(ErrInvalidRequest, fmt.Sprintf("forbidden field %q in Staff summary", k))
		}
	}
	type alias Summary
	if err := json.Unmarshal(data, (*alias)(s)); err != nil {
		return err
	}
	// Validate fields that could be forged in JSON
	if err := validateToken(string(s.ID), "id"); err != nil {
		return err
	}
	if err := validateDisplayName(s.Name, "name"); err != nil {
		return err
	}
	if err := validateToken(string(s.WorkspaceID), "workspace id"); err != nil {
		return err
	}
	if s.ProjectID != "" {
		if err := validateToken(string(s.ProjectID), "project id"); err != nil {
			return err
		}
	}
	if s.WorktreeID != "" {
		if err := validateToken(string(s.WorktreeID), "worktree id"); err != nil {
			return err
		}
	}
	if !validRole(s.Role) {
		return newError(ErrInvalidRequest, fmt.Sprintf("unsupported role %q in summary", s.Role))
	}
	if !validLifecycle(s.Lifecycle) {
		return newError(ErrInvalidRequest, fmt.Sprintf("unsupported lifecycle %q in summary", s.Lifecycle))
	}
	if !validAvailability(s.Availability) {
		return newError(ErrInvalidRequest, fmt.Sprintf("unsupported availability %q in summary", s.Availability))
	}
	if s.SchemaVersion != ContractVersion {
		return newError(ErrUnsupportedVersion, fmt.Sprintf("unsupported summary schema version %q", s.SchemaVersion))
	}
	return nil
}
