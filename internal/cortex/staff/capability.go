package staff

import (
	"fmt"
	"sort"

	"github.com/RahmatHadinata23758051/CortexOS/internal/cortex/harness"
)

// CapabilityRegistry maps Staff capabilities to Harness tool capabilities and engine classes.
// It is a static, versioned registry used for deterministic routing.
type CapabilityRegistry struct {
	Version         string
	Mappings        map[Capability]CapabilityMapping
	RoleDefaults    map[Role][]Capability
	EngineClassCaps map[harness.EngineClass][]harness.ToolCapability
}

const RegistryVersion = "cortexos.staff.capability.v1"

// CapabilityMapping defines how a Staff capability translates to Harness capabilities
// and which engine classes can fulfill it.
type CapabilityMapping struct {
	StaffCapability  Capability
	RequiredTools    []harness.ToolCapability
	PreferredEngines []harness.EngineClass
	AllowedEngines   []harness.EngineClass
	ExcludedEngines  []harness.EngineClass
	MinMemoryMB      int
	RequiresAsk      bool
	Description      string
}

// DefaultCapabilityRegistry returns the canonical capability registry.
// It is deterministic and versioned; unknown versions must fail closed.
func DefaultCapabilityRegistry() *CapabilityRegistry {
	r := &CapabilityRegistry{
		Version:      RegistryVersion,
		Mappings:     make(map[Capability]CapabilityMapping),
		RoleDefaults: make(map[Role][]Capability),
		EngineClassCaps: map[harness.EngineClass][]harness.ToolCapability{
			harness.EngineClassNative: {
				harness.CapabilityShell, harness.CapabilityFileRead, harness.CapabilityFileWrite,
				harness.CapabilityFileEdit, harness.CapabilityGit, harness.CapabilitySearch,
				harness.CapabilityTaskPlan, harness.CapabilityCodeNavigation, harness.CapabilityTestRun,
				harness.CapabilityLint, harness.CapabilityBuild,
			},
			harness.EngineClassPi: {
				harness.CapabilityCoding, harness.CapabilityAnalysis, harness.CapabilityRefactor,
				harness.CapabilityDebug, harness.CapabilityReview, harness.CapabilityTestGen,
				harness.CapabilityDocGen,
			},
			harness.EngineClassOMP: {
				harness.CapabilitySpecialist, harness.CapabilityRecovery,
				harness.CapabilitySecurityAudit, harness.CapabilityDeepDebug,
				harness.CapabilityArchitectureReview,
			},
		},
	}
	r.registerDefaults()
	return r
}

func (r *CapabilityRegistry) registerDefaults() {
	// Core capabilities
	r.Mappings[Capability("shell")] = CapabilityMapping{
		StaffCapability:  Capability("shell"),
		RequiredTools:    []harness.ToolCapability{harness.CapabilityShell},
		PreferredEngines: []harness.EngineClass{harness.EngineClassNative},
		AllowedEngines:   []harness.EngineClass{harness.EngineClassNative},
		MinMemoryMB:      50,
		Description:      "Execute commands in isolated worktree",
	}
	r.Mappings[Capability("file_read")] = CapabilityMapping{
		StaffCapability:  Capability("file_read"),
		RequiredTools:    []harness.ToolCapability{harness.CapabilityFileRead},
		PreferredEngines: []harness.EngineClass{harness.EngineClassNative},
		AllowedEngines:   []harness.EngineClass{harness.EngineClassNative},
		MinMemoryMB:      50,
		Description:      "Read files within worktree boundary",
	}
	r.Mappings[Capability("file_write")] = CapabilityMapping{
		StaffCapability:  Capability("file_write"),
		RequiredTools:    []harness.ToolCapability{harness.CapabilityFileWrite},
		PreferredEngines: []harness.EngineClass{harness.EngineClassNative},
		AllowedEngines:   []harness.EngineClass{harness.EngineClassNative},
		MinMemoryMB:      50,
		Description:      "Write files within worktree boundary",
	}
	r.Mappings[Capability("file_edit")] = CapabilityMapping{
		StaffCapability:  Capability("file_edit"),
		RequiredTools:    []harness.ToolCapability{harness.CapabilityFileEdit},
		PreferredEngines: []harness.EngineClass{harness.EngineClassNative},
		AllowedEngines:   []harness.EngineClass{harness.EngineClassNative},
		MinMemoryMB:      50,
		Description:      "Edit files within worktree boundary",
	}
	r.Mappings[Capability("git")] = CapabilityMapping{
		StaffCapability:  Capability("git"),
		RequiredTools:    []harness.ToolCapability{harness.CapabilityGit},
		PreferredEngines: []harness.EngineClass{harness.EngineClassNative},
		AllowedEngines:   []harness.EngineClass{harness.EngineClassNative},
		MinMemoryMB:      50,
		Description:      "Execute git operations on worktree",
	}
	r.Mappings[Capability("search")] = CapabilityMapping{
		StaffCapability:  Capability("search"),
		RequiredTools:    []harness.ToolCapability{harness.CapabilitySearch},
		PreferredEngines: []harness.EngineClass{harness.EngineClassNative},
		AllowedEngines:   []harness.EngineClass{harness.EngineClassNative},
		MinMemoryMB:      50,
		Description:      "Search content within worktree",
	}
	r.Mappings[Capability("task_plan")] = CapabilityMapping{
		StaffCapability:  Capability("task_plan"),
		RequiredTools:    []harness.ToolCapability{harness.CapabilityTaskPlan},
		PreferredEngines: []harness.EngineClass{harness.EngineClassNative},
		AllowedEngines:   []harness.EngineClass{harness.EngineClassNative},
		MinMemoryMB:      50,
		Description:      "Task and plan coordination",
	}
	r.Mappings[Capability("code_navigation")] = CapabilityMapping{
		StaffCapability:  Capability("code_navigation"),
		RequiredTools:    []harness.ToolCapability{harness.CapabilityCodeNavigation},
		PreferredEngines: []harness.EngineClass{harness.EngineClassNative},
		AllowedEngines:   []harness.EngineClass{harness.EngineClassNative},
		MinMemoryMB:      50,
		Description:      "Navigate symbols and references",
	}
	r.Mappings[Capability("test_run")] = CapabilityMapping{
		StaffCapability:  Capability("test_run"),
		RequiredTools:    []harness.ToolCapability{harness.CapabilityTestRun},
		PreferredEngines: []harness.EngineClass{harness.EngineClassNative},
		AllowedEngines:   []harness.EngineClass{harness.EngineClassNative},
		MinMemoryMB:      100,
		Description:      "Run tests in isolated sandbox",
	}
	r.Mappings[Capability("lint")] = CapabilityMapping{
		StaffCapability:  Capability("lint"),
		RequiredTools:    []harness.ToolCapability{harness.CapabilityLint},
		PreferredEngines: []harness.EngineClass{harness.EngineClassNative},
		AllowedEngines:   []harness.EngineClass{harness.EngineClassNative},
		MinMemoryMB:      100,
		Description:      "Run linters in isolated sandbox",
	}
	r.Mappings[Capability("build")] = CapabilityMapping{
		StaffCapability:  Capability("build"),
		RequiredTools:    []harness.ToolCapability{harness.CapabilityBuild},
		PreferredEngines: []harness.EngineClass{harness.EngineClassNative},
		AllowedEngines:   []harness.EngineClass{harness.EngineClassNative},
		MinMemoryMB:      100,
		Description:      "Run build commands in isolated sandbox",
	}

	// Pi engine capabilities
	r.Mappings[Capability("coding")] = CapabilityMapping{
		StaffCapability:  Capability("coding"),
		RequiredTools:    []harness.ToolCapability{harness.CapabilityCoding},
		PreferredEngines: []harness.EngineClass{harness.EngineClassPi},
		AllowedEngines:   []harness.EngineClass{harness.EngineClassPi, harness.EngineClassNative},
		MinMemoryMB:      200,
		Description:      "General coding assistance",
	}
	r.Mappings[Capability("analysis")] = CapabilityMapping{
		StaffCapability:  Capability("analysis"),
		RequiredTools:    []harness.ToolCapability{harness.CapabilityAnalysis},
		PreferredEngines: []harness.EngineClass{harness.EngineClassPi},
		AllowedEngines:   []harness.EngineClass{harness.EngineClassPi},
		MinMemoryMB:      200,
		Description:      "Code analysis and understanding",
	}
	r.Mappings[Capability("refactor")] = CapabilityMapping{
		StaffCapability:  Capability("refactor"),
		RequiredTools:    []harness.ToolCapability{harness.CapabilityRefactor},
		PreferredEngines: []harness.EngineClass{harness.EngineClassPi},
		AllowedEngines:   []harness.EngineClass{harness.EngineClassPi},
		MinMemoryMB:      200,
		Description:      "Code refactoring",
	}
	r.Mappings[Capability("debug")] = CapabilityMapping{
		StaffCapability:  Capability("debug"),
		RequiredTools:    []harness.ToolCapability{harness.CapabilityDebug},
		PreferredEngines: []harness.EngineClass{harness.EngineClassPi},
		AllowedEngines:   []harness.EngineClass{harness.EngineClassPi, harness.EngineClassNative},
		MinMemoryMB:      200,
		Description:      "Debugging assistance",
	}
	r.Mappings[Capability("review")] = CapabilityMapping{
		StaffCapability:  Capability("review"),
		RequiredTools:    []harness.ToolCapability{harness.CapabilityReview},
		PreferredEngines: []harness.EngineClass{harness.EngineClassPi},
		AllowedEngines:   []harness.EngineClass{harness.EngineClassPi, harness.EngineClassNative},
		MinMemoryMB:      200,
		Description:      "Code review assistance",
	}
	r.Mappings[Capability("test_gen")] = CapabilityMapping{
		StaffCapability:  Capability("test_gen"),
		RequiredTools:    []harness.ToolCapability{harness.CapabilityTestGen},
		PreferredEngines: []harness.EngineClass{harness.EngineClassPi},
		AllowedEngines:   []harness.EngineClass{harness.EngineClassPi},
		MinMemoryMB:      200,
		Description:      "Test generation",
	}
	r.Mappings[Capability("doc_gen")] = CapabilityMapping{
		StaffCapability:  Capability("doc_gen"),
		RequiredTools:    []harness.ToolCapability{harness.CapabilityDocGen},
		PreferredEngines: []harness.EngineClass{harness.EngineClassPi},
		AllowedEngines:   []harness.EngineClass{harness.EngineClassPi},
		MinMemoryMB:      200,
		Description:      "Documentation generation",
	}

	// OMP specialist capabilities
	r.Mappings[Capability("specialist")] = CapabilityMapping{
		StaffCapability:  Capability("specialist"),
		RequiredTools:    []harness.ToolCapability{harness.CapabilitySpecialist},
		PreferredEngines: []harness.EngineClass{harness.EngineClassOMP},
		AllowedEngines:   []harness.EngineClass{harness.EngineClassOMP},
		MinMemoryMB:      300,
		Description:      "Specialist domain tasks",
	}
	r.Mappings[Capability("recovery")] = CapabilityMapping{
		StaffCapability:  Capability("recovery"),
		RequiredTools:    []harness.ToolCapability{harness.CapabilityRecovery},
		PreferredEngines: []harness.EngineClass{harness.EngineClassOMP},
		AllowedEngines:   []harness.EngineClass{harness.EngineClassOMP},
		MinMemoryMB:      300,
		Description:      "Recovery and remediation",
	}
	r.Mappings[Capability("security_audit")] = CapabilityMapping{
		StaffCapability:  Capability("security_audit"),
		RequiredTools:    []harness.ToolCapability{harness.CapabilitySecurityAudit},
		PreferredEngines: []harness.EngineClass{harness.EngineClassOMP},
		AllowedEngines:   []harness.EngineClass{harness.EngineClassOMP},
		MinMemoryMB:      300,
		Description:      "Security audit and review",
	}
	r.Mappings[Capability("deep_debug")] = CapabilityMapping{
		StaffCapability:  Capability("deep_debug"),
		RequiredTools:    []harness.ToolCapability{harness.CapabilityDeepDebug},
		PreferredEngines: []harness.EngineClass{harness.EngineClassOMP},
		AllowedEngines:   []harness.EngineClass{harness.EngineClassOMP},
		MinMemoryMB:      300,
		Description:      "Deep debugging and diagnostics",
	}
	r.Mappings[Capability("architecture_review")] = CapabilityMapping{
		StaffCapability:  Capability("architecture_review"),
		RequiredTools:    []harness.ToolCapability{harness.CapabilityArchitectureReview},
		PreferredEngines: []harness.EngineClass{harness.EngineClassOMP},
		AllowedEngines:   []harness.EngineClass{harness.EngineClassOMP},
		MinMemoryMB:      300,
		Description:      "Architecture review and design",
	}

	// Role default capabilities
	r.RoleDefaults[RoleCoordinator] = []Capability{
		Capability("task_plan"), Capability("git"), Capability("search"),
	}
	r.RoleDefaults[RolePlanner] = []Capability{
		Capability("task_plan"), Capability("code_navigation"), Capability("analysis"),
	}
	r.RoleDefaults[RoleImplementer] = []Capability{
		Capability("coding"), Capability("file_read"), Capability("file_write"),
		Capability("file_edit"), Capability("git"), Capability("test_run"), Capability("build"),
	}
	r.RoleDefaults[RoleReviewer] = []Capability{
		Capability("review"), Capability("analysis"), Capability("code_navigation"),
		Capability("lint"), Capability("test_run"),
	}
	r.RoleDefaults[RoleSpecialist] = []Capability{
		Capability("specialist"), Capability("deep_debug"), Capability("security_audit"),
		Capability("architecture_review"), Capability("recovery"),
	}
}

// Validate checks the registry for internal consistency.
func (r *CapabilityRegistry) Validate() error {
	if r.Version != RegistryVersion {
		return fmt.Errorf("unsupported capability registry version %q", r.Version)
	}
	if len(r.Mappings) == 0 {
		return fmt.Errorf("empty capability mappings")
	}
	if len(r.RoleDefaults) == 0 {
		return fmt.Errorf("empty role defaults")
	}
	for cap, mapping := range r.Mappings {
		if mapping.StaffCapability != cap {
			return fmt.Errorf("capability key mismatch: %q vs %q", cap, mapping.StaffCapability)
		}
		if len(mapping.RequiredTools) == 0 {
			return fmt.Errorf("capability %q has no required tools", cap)
		}
		if len(mapping.AllowedEngines) == 0 {
			return fmt.Errorf("capability %q has no allowed engines", cap)
		}
	}
	for role, caps := range r.RoleDefaults {
		if !validRole(role) {
			return fmt.Errorf("invalid role %q in defaults", role)
		}
		for _, cap := range caps {
			if _, ok := r.Mappings[cap]; !ok {
				return fmt.Errorf("role %q references unknown capability %q", role, cap)
			}
		}
	}
	return nil
}

// GetMapping returns the capability mapping for a Staff capability.
func (r *CapabilityRegistry) GetMapping(cap Capability) (CapabilityMapping, bool) {
	m, ok := r.Mappings[cap]
	return m, ok
}

// GetRoleDefaults returns the default capabilities for a role.
func (r *CapabilityRegistry) GetRoleDefaults(role Role) []Capability {
	caps := r.RoleDefaults[role]
	return append([]Capability(nil), caps...)
}

// GetAllowedEngines returns the allowed engine classes for a Staff capability.
func (r *CapabilityRegistry) GetAllowedEngines(cap Capability) []harness.EngineClass {
	m, ok := r.GetMapping(cap)
	if !ok {
		return nil
	}
	return append([]harness.EngineClass(nil), m.AllowedEngines...)
}

// GetPreferredEngines returns the preferred engine classes for a Staff capability.
func (r *CapabilityRegistry) GetPreferredEngines(cap Capability) []harness.EngineClass {
	m, ok := r.GetMapping(cap)
	if !ok {
		return nil
	}
	return append([]harness.EngineClass(nil), m.PreferredEngines...)
}

// GetRequiredTools returns the Harness tool capabilities required for a Staff capability.
func (r *CapabilityRegistry) GetRequiredTools(cap Capability) []harness.ToolCapability {
	m, ok := r.GetMapping(cap)
	if !ok {
		return nil
	}
	return append([]harness.ToolCapability(nil), m.RequiredTools...)
}

// EngineClassCapabilities returns the tool capabilities for an engine class.
func (r *CapabilityRegistry) EngineClassCapabilities(class harness.EngineClass) []harness.ToolCapability {
	caps, ok := r.EngineClassCaps[class]
	if !ok {
		return nil
	}
	return append([]harness.ToolCapability(nil), caps...)
}

// AllCapabilities returns all registered Staff capabilities in sorted order.
func (r *CapabilityRegistry) AllCapabilities() []Capability {
	caps := make([]Capability, 0, len(r.Mappings))
	for cap := range r.Mappings {
		caps = append(caps, cap)
	}
	sort.Slice(caps, func(i, j int) bool { return string(caps[i]) < string(caps[j]) })
	return caps
}

// AllRoles returns all roles with defaults in sorted order.
func (r *CapabilityRegistry) AllRoles() []Role {
	roles := make([]Role, 0, len(r.RoleDefaults))
	for role := range r.RoleDefaults {
		roles = append(roles, role)
	}
	sort.Slice(roles, func(i, j int) bool { return string(roles[i]) < string(roles[j]) })
	return roles
}
