package staff

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/RahmatHadinata23758051/CortexOS/internal/cortex/harness"
)

// ContractVersion is the versioned contract for skill definitions.
const SkillContractVersion = "cortexos.skill.v1"

// SkillID is the unique identifier for a skill.
type SkillID string

// Skill is a governed, versioned capability bundle that can be injected into
// execution envelopes. It carries prompt templates, applicability rules, and
// capability requirements but does not grant authority by itself.
type Skill struct {
	ID              SkillID         `json:"id"`
	Name            string          `json:"name"`
	Description     string          `json:"description"`
	Version         string          `json:"version"`
	Applicability   Applicability   `json:"applicability"`
	PromptTemplates PromptTemplates `json:"promptTemplates"`
	Capabilities    []Capability    `json:"capabilities"`
	RequiresAsk     bool            `json:"requiresAsk"`
	MaxContextChars int             `json:"maxContextChars"`
	SourceURI       string          `json:"sourceUri"`
	Author          string          `json:"author"`
	License         string          `json:"license,omitempty"`
	CreatedAt       time.Time       `json:"createdAt"`
	UpdatedAt       time.Time       `json:"updatedAt"`
	SchemaVersion   string          `json:"schemaVersion"`
}

// Applicability defines when and where a skill may be used.
type Applicability struct {
	AllowedRoles         []Role        `json:"allowedRoles,omitempty"`
	AllowedWorkspaces    []WorkspaceID `json:"allowedWorkspaces,omitempty"`
	RequiredCapabilities []Capability  `json:"requiredCapabilities,omitempty"`
	ExcludedWorkspaces   []WorkspaceID `json:"excludedWorkspaces,omitempty"`
}

// PromptTemplates contains the injection templates.
type PromptTemplates struct {
	System    string `json:"system"`
	User      string `json:"user"`
	Assistant string `json:"assistant,omitempty"`
}

// SkillPort is the narrow interface for resolving skill definitions.
// It does not expose raw storage or unvalidated queries.
type SkillPort interface {
	Open(ctx context.Context) error
	Close() error
	SchemaVersion(ctx context.Context) (string, error)
	// Get retrieves a skill by ID and version.
	Get(ctx context.Context, id SkillID, version string) (Skill, error)
	// List returns all available skills.
	List(ctx context.Context) ([]Skill, error)
	// ListForStaff returns skills applicable to a Staff definition.
	ListForStaff(ctx context.Context, def Definition) ([]Skill, error)
}

// SkillFilter is a safe, validated query for skills.
type SkillFilter struct {
	Role        *Role
	WorkspaceID *WorkspaceID
	Capability  *Capability
}

func (f SkillFilter) Validate() error {
	if f.Role != nil && !validRole(*f.Role) {
		return newError(ErrInvalidRequest, fmt.Sprintf("unsupported role %q in filter", *f.Role))
	}
	if f.WorkspaceID != nil {
		if err := validateToken(string(*f.WorkspaceID), "workspace id"); err != nil {
			return WrapError(ErrInvalidRequest, "invalid workspace filter", err)
		}
	}
	if f.Capability != nil {
		if err := validateToken(string(*f.Capability), "capability"); err != nil {
			return WrapError(ErrInvalidRequest, "invalid capability filter", err)
		}
	}
	return nil
}

// SkillInjection is the result of preparing a skill for execution envelope injection.
type SkillInjection struct {
	SkillID         SkillID       `json:"skillId"`
	SkillVersion    string        `json:"skillVersion"`
	Resolved        ResolvedSkill `json:"resolved"`
	InjectedAt      time.Time     `json:"injectedAt"`
	ContractVersion string        `json:"contractVersion"`
}

// ToHarness converts the governed injection into the Harness-neutral envelope
// representation. It intentionally does not translate capabilities into policy;
// Harness policy remains the sole authorization boundary.
func (i SkillInjection) ToHarness() harness.SkillInjection {
	capabilities := make([]string, 0, len(i.Resolved.Capabilities))
	for _, capability := range i.Resolved.Capabilities {
		capabilities = append(capabilities, string(capability))
	}
	return harness.SkillInjection{
		SkillID:         string(i.SkillID),
		SkillVersion:    i.SkillVersion,
		SystemPrompt:    i.Resolved.SystemPrompt,
		UserPrompt:      i.Resolved.UserPrompt,
		Capabilities:    capabilities,
		RequiresAsk:     i.Resolved.RequiresAsk,
		MaxContextChars: i.Resolved.MaxContextChars,
		SourceURI:       i.Resolved.SourceURI,
		Author:          i.Resolved.Author,
		License:         i.Resolved.License,
		InjectedAt:      i.InjectedAt,
		ContractVersion: i.ContractVersion,
	}
}

// ResolvedSkill is the fully expanded skill ready for envelope injection.
type ResolvedSkill struct {
	SystemPrompt    string       `json:"systemPrompt"`
	UserPrompt      string       `json:"userPrompt"`
	Capabilities    []Capability `json:"capabilities"`
	RequiresAsk     bool         `json:"requiresAsk"`
	MaxContextChars int          `json:"maxContextChars"`
	SourceURI       string       `json:"sourceUri"`
	Author          string       `json:"author"`
	License         string       `json:"license,omitempty"`
}

// PrepareInjection resolves and sanitizes a skill for safe injection.
// It redacts secrets, bounds size, and validates applicability against the staff.
func PrepareInjection(skill Skill, def Definition, contextData string) (SkillInjection, error) {
	if err := skill.Validate(); err != nil {
		return SkillInjection{}, err
	}
	if err := def.Validate(); err != nil {
		return SkillInjection{}, WrapError(ErrInvalidRequest, "invalid staff definition", err)
	}

	// Check applicability
	if !skill.Applicability.Allows(def) {
		return SkillInjection{}, newError(ErrPermissionDenied, "skill not applicable to staff")
	}

	// Resolve templates with context data
	resolved := ResolvedSkill{
		SystemPrompt:    redactSecrets(skill.PromptTemplates.System),
		UserPrompt:      redactSecrets(expandTemplate(skill.PromptTemplates.User, contextData)),
		Capabilities:    append([]Capability(nil), skill.Capabilities...),
		RequiresAsk:     skill.RequiresAsk,
		MaxContextChars: skill.MaxContextChars,
		SourceURI:       skill.SourceURI,
		Author:          skill.Author,
		License:         skill.License,
	}

	// Bound prompt sizes
	if len(resolved.SystemPrompt) > skill.MaxContextChars/2 {
		resolved.SystemPrompt = resolved.SystemPrompt[:skill.MaxContextChars/2]
	}
	if len(resolved.UserPrompt) > skill.MaxContextChars {
		resolved.UserPrompt = resolved.UserPrompt[:skill.MaxContextChars]
	}

	return SkillInjection{
		SkillID:         skill.ID,
		SkillVersion:    skill.Version,
		Resolved:        resolved,
		InjectedAt:      time.Now().UTC(),
		ContractVersion: SkillContractVersion,
	}, nil
}

func (a Applicability) Allows(def Definition) bool {
	if len(a.AllowedRoles) > 0 {
		matched := false
		for _, r := range a.AllowedRoles {
			if def.Role == r {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}
	if len(a.AllowedWorkspaces) > 0 {
		matched := false
		for _, ws := range a.AllowedWorkspaces {
			if def.Workspace.WorkspaceID == ws {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}
	if len(a.ExcludedWorkspaces) > 0 {
		for _, ws := range a.ExcludedWorkspaces {
			if def.Workspace.WorkspaceID == ws {
				return false
			}
		}
	}
	if len(a.RequiredCapabilities) > 0 {
		hasAll := true
		for _, req := range a.RequiredCapabilities {
			found := false
			for _, cap := range def.Capabilities {
				if cap == req {
					found = true
					break
				}
			}
			if !found {
				hasAll = false
				break
			}
		}
		if !hasAll {
			return false
		}
	}
	return true
}

func (s Skill) Validate() error {
	if s.ID == "" {
		return newError(ErrInvalidRequest, "skill id is required")
	}
	if s.Version == "" {
		return newError(ErrInvalidRequest, "skill version is required")
	}
	if strings.TrimSpace(s.Name) == "" {
		return newError(ErrInvalidRequest, "skill name is required")
	}
	if s.SchemaVersion != SkillContractVersion {
		return newError(ErrUnsupportedVersion, fmt.Sprintf("unsupported schema version %q", s.SchemaVersion))
	}
	if len(s.Capabilities) == 0 {
		return newError(ErrInvalidRequest, "skill must declare at least one capability")
	}
	for _, cap := range s.Capabilities {
		if err := validateToken(string(cap), "capability"); err != nil {
			return WrapError(ErrInvalidRequest, "invalid capability in skill", err)
		}
	}
	if s.MaxContextChars <= 0 {
		return newError(ErrInvalidRequest, "maxContextChars must be positive")
	}
	if s.MaxContextChars > 100000 {
		return newError(ErrInvalidRequest, "maxContextChars exceeds limit of 100000")
	}
	if strings.TrimSpace(s.SystemPrompt()) == "" && strings.TrimSpace(s.UserPrompt()) == "" {
		return newError(ErrInvalidRequest, "skill must have at least system or user prompt template")
	}
	if s.RequiresAsk && len(s.Capabilities) == 0 {
		return newError(ErrInvalidRequest, "requiresAsk skill must declare capabilities")
	}
	return nil
}

func (s Skill) SystemPrompt() string { return s.PromptTemplates.System }
func (s Skill) UserPrompt() string   { return s.PromptTemplates.User }

var (
	skillSecretRegex = regexp.MustCompile(`(?i)\b(?:api[_-]?key|secret|token|password|credential)\s*[:=]\s*\S+`)
	skillBearerRegex = regexp.MustCompile(`(?i)Bearer\s+[A-Za-z0-9\-._~+/]+=*`)
	skillEmailRegex  = regexp.MustCompile(`(?i)\b[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Za-z]{2,}\b`)
)

func redactSecrets(s string) string {
	res := s
	res = skillEmailRegex.ReplaceAllString(res, "[EMAIL]")
	res = skillSecretRegex.ReplaceAllString(res, "[REDACTED]")
	res = skillBearerRegex.ReplaceAllString(res, "Bearer [REDACTED]")
	return res
}

func expandTemplate(tmpl, data string) string {
	if strings.TrimSpace(tmpl) == "" {
		return ""
	}
	// Simple placeholder replacement: {{.Context}} -> data
	result := strings.ReplaceAll(tmpl, "{{.Context}}", data)
	result = strings.ReplaceAll(result, "{{.Data}}", data)
	return result
}

// SkillStore is an in-memory thread-safe implementation of SkillPort.
type SkillStore struct {
	mu     sync.RWMutex
	skills map[SkillID]map[string]Skill // id -> version -> skill
}

func NewSkillStore() *SkillStore {
	return &SkillStore{
		skills: make(map[SkillID]map[string]Skill),
	}
}

func (s *SkillStore) Open(ctx context.Context) error {
	return nil
}

func (s *SkillStore) Close() error {
	return nil
}

func (s *SkillStore) SchemaVersion(ctx context.Context) (string, error) {
	return SkillContractVersion, nil
}

func (s *SkillStore) Get(ctx context.Context, id SkillID, version string) (Skill, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	versions, ok := s.skills[id]
	if !ok {
		return Skill{}, WrapError(ErrNotFound, fmt.Sprintf("skill %q not found", id), nil)
	}
	if version == "" || version == "latest" {
		var latest Skill
		for _, v := range versions {
			if latest.Version == "" || compareSkillVersions(v.Version, latest.Version) > 0 {
				latest = v
			}
		}
		if latest.Version == "" {
			return Skill{}, WrapError(ErrNotFound, fmt.Sprintf("skill %q has no versions", id), nil)
		}
		return latest, nil
	}
	skill, ok := versions[version]
	if !ok {
		return Skill{}, WrapError(ErrNotFound, fmt.Sprintf("skill %q version %q not found", id, version), nil)
	}
	return skill, nil
}

func compareSkillVersions(v1, v2 string) int {
	p1 := strings.Split(v1, ".")
	p2 := strings.Split(v2, ".")
	maxLen := len(p1)
	if len(p2) > maxLen {
		maxLen = len(p2)
	}
	for i := 0; i < maxLen; i++ {
		var s1, s2 string
		if i < len(p1) {
			s1 = p1[i]
		}
		if i < len(p2) {
			s2 = p2[i]
		}
		if s1 != s2 {
			if len(s1) != len(s2) {
				return len(s1) - len(s2)
			}
			return strings.Compare(s1, s2)
		}
	}
	return 0
}

func (s *SkillStore) List(ctx context.Context) ([]Skill, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var res []Skill
	for _, versions := range s.skills {
		for _, skill := range versions {
			res = append(res, skill)
		}
	}
	sort.Slice(res, func(i, j int) bool {
		if res[i].ID != res[j].ID {
			return res[i].ID < res[j].ID
		}
		return res[i].Version < res[j].Version
	})
	return res, nil
}

func (s *SkillStore) ListForStaff(ctx context.Context, def Definition) ([]Skill, error) {
	all, err := s.List(ctx)
	if err != nil {
		return nil, err
	}
	var res []Skill
	for _, skill := range all {
		if skill.Applicability.Allows(def) {
			res = append(res, skill)
		}
	}
	return res, nil
}

func (s *SkillStore) Save(ctx context.Context, skill Skill) (Skill, error) {
	if err := skill.Validate(); err != nil {
		return Skill{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.skills[skill.ID]; !ok {
		s.skills[skill.ID] = make(map[string]Skill)
	}
	s.skills[skill.ID][skill.Version] = skill
	return skill, nil
}

func DefaultSkills() []Skill {
	now := time.Now().UTC()
	return []Skill{
		{
			ID:          "coding-assistant",
			Name:        "General Coding Assistant",
			Description: "Provides general coding guidance, refactoring, and analysis",
			Version:     "1.0.0",
			Applicability: Applicability{
				AllowedRoles:         []Role{RoleImplementer, RolePlanner, RoleReviewer},
				RequiredCapabilities: []Capability{"coding"},
			},
			PromptTemplates: PromptTemplates{
				System: "You are a coding assistant. Provide helpful, accurate, and safe code suggestions.",
				User:   "{{.Context}}\n\nTask: {{.Data}}",
			},
			Capabilities:    []Capability{"coding", "analysis", "refactor"},
			RequiresAsk:     false,
			MaxContextChars: 8192,
			SourceURI:       "skills/coding-assistant",
			Author:          "cortexos",
			CreatedAt:       now,
			UpdatedAt:       now,
			SchemaVersion:   SkillContractVersion,
		},
		{
			ID:          "security-review",
			Name:        "Security Code Review",
			Description: "Specialist skill for security-focused code review",
			Version:     "1.0.0",
			Applicability: Applicability{
				AllowedRoles:         []Role{RoleReviewer, RoleSpecialist},
				RequiredCapabilities: []Capability{"security_audit", "review"},
			},
			PromptTemplates: PromptTemplates{
				System: "You are a security specialist. Review code for vulnerabilities, unsafe patterns, and compliance issues.",
				User:   "Code to review:\n{{.Context}}\n\nProvide findings with severity and remediation.",
			},
			Capabilities:    []Capability{"security_audit", "review", "analysis"},
			RequiresAsk:     true,
			MaxContextChars: 16384,
			SourceURI:       "skills/security-review",
			Author:          "cortexos",
			CreatedAt:       now,
			UpdatedAt:       now,
			SchemaVersion:   SkillContractVersion,
		},
		{
			ID:          "test-generation",
			Name:        "Test Generation Assistant",
			Description: "Generates unit tests and integration tests for code",
			Version:     "1.0.0",
			Applicability: Applicability{
				AllowedRoles:         []Role{RoleImplementer, RoleReviewer},
				RequiredCapabilities: []Capability{"test_gen", "coding"},
			},
			PromptTemplates: PromptTemplates{
				System: "You are a test generation assistant. Create comprehensive, idiomatic tests.",
				User:   "Code under test:\n{{.Context}}\n\nGenerate tests for the specified behavior.",
			},
			Capabilities:    []Capability{"test_gen", "coding", "analysis"},
			RequiresAsk:     false,
			MaxContextChars: 8192,
			SourceURI:       "skills/test-generation",
			Author:          "cortexos",
			CreatedAt:       now,
			UpdatedAt:       now,
			SchemaVersion:   SkillContractVersion,
		},
	}
}

func SeedSkillStore(ctx context.Context, store *SkillStore) error {
	for _, skill := range DefaultSkills() {
		if _, err := store.Save(ctx, skill); err != nil {
			return err
		}
	}
	return nil
}

// JSONMarshaler/Unmarshaler for safe transport
func (s Skill) MarshalJSON() ([]byte, error) {
	type Alias Skill
	return json.Marshal(struct {
		Alias
		SchemaVersion string `json:"schemaVersion"`
	}{
		Alias:         Alias(s),
		SchemaVersion: SkillContractVersion,
	})
}

// InjectSkillIntoEnvelope attaches a sanitized skill injection to the execution
// envelope if the staff definition possesses matching skill references and
// applicability rules allow it.
// It verifies the skill's source, redacts secrets, bounds prompt size, and ensures
// the skill cannot escalate capabilities beyond what the staff definition permits.
// The envelope's policy decision is NOT modified — Harness remains the sole authority.
func InjectSkillIntoEnvelope(envelope *harness.ExecutionEnvelope, def Definition, skillStore SkillPort, contextData string) error {
	if envelope == nil {
		return newError(ErrInvalidRequest, "envelope is required")
	}
	if skillStore == nil {
		return newError(ErrInvalidRequest, "skill store is required")
	}
	if err := def.Validate(); err != nil {
		return WrapError(ErrInvalidRequest, "invalid staff definition", err)
	}

	// Query applicable skills for this staff definition
	skills, err := skillStore.ListForStaff(context.Background(), def)
	if err != nil {
		return WrapError(ErrInternal, "failed to list skills for staff", err)
	}

	var matchedSkill *Skill
	// Prefer skills explicitly referenced by the staff definition
	for _, ref := range def.Skills {
		for i := range skills {
			if string(skills[i].ID) == ref.ID {
				matchedSkill = &skills[i]
				break
			}
		}
		if matchedSkill != nil {
			break
		}
	}
	// Fall back to first applicable skill if none explicitly referenced
	if matchedSkill == nil && len(skills) > 0 {
		matchedSkill = &skills[0]
	}
	if matchedSkill == nil {
		return nil // No skill applies; envelope remains un-injected
	}

	// Prepare injection (redacts secrets, bounds size, checks applicability)
	injection, err := PrepareInjection(*matchedSkill, def, contextData)
	if err != nil {
		return WrapError(ErrPermissionDenied, "skill injection preparation failed", err)
	}

	// Prevent capability escalation: skill capabilities must be subset of staff capabilities
	staffCaps := make(map[Capability]bool, len(def.Capabilities))
	for _, cap := range def.Capabilities {
		staffCaps[cap] = true
	}
	for _, cap := range matchedSkill.Capabilities {
		if !staffCaps[cap] {
			return newError(ErrPermissionDenied, fmt.Sprintf("skill requires capability %q not granted to staff %q", cap, def.Identity.ID))
		}
	}

	// Validate source URI and author to prevent untrusted external script execution
	if strings.Contains(matchedSkill.SourceURI, "..") || strings.ContainsAny(matchedSkill.SourceURI, "\x00\r\n") {
		return newError(ErrInvalidRequest, "skill source URI contains invalid path traversal or control characters")
	}
	if strings.TrimSpace(matchedSkill.Author) == "" {
		return newError(ErrInvalidRequest, "skill author is required for provenance tracking")
	}

	harnessInjection := injection.ToHarness()
	envelope.SkillInjection = &harnessInjection
	return nil
}
