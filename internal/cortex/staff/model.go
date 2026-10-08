package staff

import (
	"fmt"
	"strings"
	"time"
)

// ContractVersion is the version of the persisted and bridge-visible Staff
// definition. Unknown versions are rejected rather than silently downgraded.
const ContractVersion = "cortexos.staff.v1"

// StaffID is the stable logical identity of a Staff member. It is deliberately
// not a worker ID, process ID, CLI identity, or engine handle.
type StaffID string

// WorkspaceID, ProjectID, and WorktreeID are opaque domain identifiers. They
// do not contain filesystem paths or process information.
type WorkspaceID string
type ProjectID string
type WorktreeID string

// Role identifies the responsibility assigned to a logical Staff member.
type Role string

const (
	RoleCoordinator Role = "coordinator"
	RolePlanner     Role = "planner"
	RoleImplementer Role = "implementer"
	RoleReviewer    Role = "reviewer"
	RoleSpecialist  Role = "specialist"
)

// Capability is a product-level ability, not a direct process or OS grant.
type Capability string

// LifecycleState is owned by Staff management and is independent of worker
// process lifecycle.
type LifecycleState string

const (
	LifecycleActive   LifecycleState = "active"
	LifecycleInactive LifecycleState = "inactive"
	LifecycleRetired  LifecycleState = "retired"
)

// AvailabilityState is an observation used by later assignment/routing code.
// It does not identify or reserve a worker process.
type AvailabilityState string

const (
	AvailabilityAvailable   AvailabilityState = "available"
	AvailabilityBusy        AvailabilityState = "busy"
	AvailabilityUnavailable AvailabilityState = "unavailable"
	AvailabilityOffline     AvailabilityState = "offline"
)

// Identity contains only stable, human-facing Staff identity.
type Identity struct {
	ID   StaffID `json:"id"`
	Name string  `json:"name"`
}

// SkillReference points to a separately governed skill definition. It does not
// embed instructions, provider output, credentials, or executable content.
type SkillReference struct {
	ID      string `json:"id"`
	Version string `json:"version"`
}

// MemoryReference points to memory owned by a later memory adapter. Staff
// contracts carry references only, never memory contents.
type MemoryReference struct {
	ID      string `json:"id"`
	Kind    string `json:"kind"`
	Version string `json:"version"`
}

// WorkspaceAssignment identifies the approved scope for a Staff member. Paths,
// process handles, and worker-pool assignments intentionally do not appear.
type WorkspaceAssignment struct {
	WorkspaceID WorkspaceID `json:"workspaceId"`
	ProjectID   ProjectID   `json:"projectId,omitempty"`
	WorktreeID  WorktreeID  `json:"worktreeId,omitempty"`
}

// Definition is the complete safe, versioned logical Staff contract. It has no
// CLI/process identity and no success/merge authority.
type Definition struct {
	Identity
	Role          Role                `json:"role"`
	Capabilities  []Capability        `json:"capabilities"`
	Permissions   PermissionSet       `json:"permissions"`
	Skills        []SkillReference    `json:"skills"`
	Memory        []MemoryReference   `json:"memory"`
	Workspace     WorkspaceAssignment `json:"workspace"`
	Lifecycle     LifecycleState      `json:"lifecycle"`
	Availability  AvailabilityState   `json:"availability"`
	CreatedAt     time.Time           `json:"createdAt"`
	UpdatedAt     time.Time           `json:"updatedAt"`
	SchemaVersion string              `json:"schemaVersion"`
}

// Staff is an alias for the versioned logical Staff definition. Keeping one
// representation prevents identity and definition drift across adapters.
type Staff = Definition

// NewDefinition defensively copies collections so callers cannot mutate a
// validated definition behind the contract boundary.
func NewDefinition(def Definition) (Definition, error) {
	def.Capabilities = append([]Capability(nil), def.Capabilities...)
	def.Permissions = append(PermissionSet(nil), def.Permissions...)
	def.Skills = append([]SkillReference(nil), def.Skills...)
	def.Memory = append([]MemoryReference(nil), def.Memory...)
	if err := def.Validate(); err != nil {
		return Definition{}, err
	}
	return def, nil
}

func (i Identity) Validate() error {
	if err := validateToken(string(i.ID), "id"); err != nil {
		return err
	}
	if err := validateDisplayName(i.Name, "name"); err != nil {
		return err
	}
	return nil
}

// Validate checks every field that can affect identity, routing, policy, or
// bridge serialization. Unknown contract versions fail closed.
func (d Definition) Validate() error {
	if d.SchemaVersion != ContractVersion {
		return newError(ErrUnsupportedVersion, fmt.Sprintf("unsupported schema version %q", d.SchemaVersion))
	}
	if err := d.Identity.Validate(); err != nil {
		return err
	}
	if !validRole(d.Role) {
		return newError(ErrInvalidRequest, fmt.Sprintf("unsupported role %q", d.Role))
	}
	if len(d.Capabilities) == 0 {
		return newError(ErrInvalidRequest, "at least one capability is required")
	}
	seenCapabilities := make(map[Capability]struct{}, len(d.Capabilities))
	for _, capability := range d.Capabilities {
		if err := validateToken(string(capability), "capability"); err != nil {
			return err
		}
		if _, exists := seenCapabilities[capability]; exists {
			return newError(ErrInvalidRequest, fmt.Sprintf("duplicate capability %q", capability))
		}
		seenCapabilities[capability] = struct{}{}
	}
	if err := d.Permissions.Validate(); err != nil {
		return err
	}
	seenSkills := make(map[string]struct{}, len(d.Skills))
	for _, skill := range d.Skills {
		if err := validateToken(skill.ID, "skill id"); err != nil {
			return err
		}
		if err := validateToken(skill.Version, "skill version"); err != nil {
			return err
		}
		if _, exists := seenSkills[skill.ID]; exists {
			return newError(ErrInvalidRequest, fmt.Sprintf("duplicate skill %q", skill.ID))
		}
		seenSkills[skill.ID] = struct{}{}
	}
	seenMemory := make(map[string]struct{}, len(d.Memory))
	for _, memory := range d.Memory {
		if err := validateToken(memory.ID, "memory reference id"); err != nil {
			return err
		}
		if err := validateToken(memory.Kind, "memory reference kind"); err != nil {
			return err
		}
		if err := validateToken(memory.Version, "memory reference version"); err != nil {
			return err
		}
		if _, exists := seenMemory[memory.ID]; exists {
			return newError(ErrInvalidRequest, fmt.Sprintf("duplicate memory reference %q", memory.ID))
		}
		seenMemory[memory.ID] = struct{}{}
	}
	if err := d.Workspace.Validate(); err != nil {
		return err
	}
	if !validLifecycle(d.Lifecycle) {
		return newError(ErrInvalidRequest, fmt.Sprintf("unsupported lifecycle %q", d.Lifecycle))
	}
	if !validAvailability(d.Availability) {
		return newError(ErrInvalidRequest, fmt.Sprintf("unsupported availability %q", d.Availability))
	}
	if d.CreatedAt.IsZero() || d.UpdatedAt.IsZero() {
		return newError(ErrInvalidRequest, "createdAt and updatedAt are required")
	}
	if d.UpdatedAt.Before(d.CreatedAt) {
		return newError(ErrInvalidRequest, "updatedAt must not precede createdAt")
	}
	return nil
}

func (w WorkspaceAssignment) Validate() error {
	if err := validateToken(string(w.WorkspaceID), "workspace id"); err != nil {
		return err
	}
	if w.ProjectID != "" {
		if err := validateToken(string(w.ProjectID), "project id"); err != nil {
			return err
		}
	}
	if w.WorktreeID != "" {
		if err := validateToken(string(w.WorktreeID), "worktree id"); err != nil {
			return err
		}
	}
	return nil
}

func validateToken(value, field string) error {
	if value == "" {
		return newError(ErrInvalidRequest, field+" is required")
	}
	if strings.TrimSpace(value) != value || strings.ContainsAny(value, "\x00\r\n") {
		return newError(ErrInvalidRequest, field+" contains invalid whitespace")
	}
	for _, r := range value {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' || r == '-' || r == '.' || r == ':' {
			continue
		}
		return newError(ErrInvalidRequest, fmt.Sprintf("%s contains unsupported character", field))
	}
	return nil
}

func validateDisplayName(value, field string) error {
	if strings.TrimSpace(value) == "" {
		return newError(ErrInvalidRequest, field+" is required")
	}
	if strings.ContainsAny(value, "\x00\r\n") {
		return newError(ErrInvalidRequest, field+" contains control characters")
	}
	return nil
}

func validRole(value Role) bool {
	return value == RoleCoordinator || value == RolePlanner || value == RoleImplementer || value == RoleReviewer || value == RoleSpecialist
}

func validLifecycle(value LifecycleState) bool {
	return value == LifecycleActive || value == LifecycleInactive || value == LifecycleRetired
}

func validAvailability(value AvailabilityState) bool {
	return value == AvailabilityAvailable || value == AvailabilityBusy || value == AvailabilityUnavailable || value == AvailabilityOffline
}
