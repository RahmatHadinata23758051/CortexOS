package staff

import (
	"testing"

	"github.com/RahmatHadinata23758051/CortexOS/internal/cortex/harness"
)

func TestDefaultCapabilityRegistry(t *testing.T) {
	r := DefaultCapabilityRegistry()
	if err := r.Validate(); err != nil {
		t.Fatalf("DefaultCapabilityRegistry validation failed: %v", err)
	}

	// Verify all roles have defaults
	roles := []Role{RoleCoordinator, RolePlanner, RoleImplementer, RoleReviewer, RoleSpecialist}
	for _, role := range roles {
		defaults := r.GetRoleDefaults(role)
		if len(defaults) == 0 {
			t.Errorf("role %q has no default capabilities", role)
		}
	}

	// Verify engine class mapping completeness
	classes := []harness.EngineClass{harness.EngineClassNative, harness.EngineClassPi, harness.EngineClassOMP}
	for _, class := range classes {
		caps := r.EngineClassCapabilities(class)
		if len(caps) == 0 {
			t.Errorf("engine class %q has no capabilities", class)
		}
	}
}

func TestCapabilityRegistryMappings(t *testing.T) {
	r := DefaultCapabilityRegistry()

	// Test shell mapping
	m, ok := r.GetMapping(Capability("shell"))
	if !ok {
		t.Fatal("shell capability not found")
	}
	if len(m.RequiredTools) != 1 || m.RequiredTools[0] != harness.CapabilityShell {
		t.Errorf("unexpected required tools: %v", m.RequiredTools)
	}
	if len(m.AllowedEngines) != 1 || m.AllowedEngines[0] != harness.EngineClassNative {
		t.Errorf("unexpected allowed engines: %v", m.AllowedEngines)
	}

	// Test coding mapping
	m, ok = r.GetMapping(Capability("coding"))
	if !ok {
		t.Fatal("coding capability not found")
	}
	if len(m.RequiredTools) != 1 || m.RequiredTools[0] != harness.CapabilityCoding {
		t.Errorf("unexpected required tools: %v", m.RequiredTools)
	}
	if len(m.PreferredEngines) != 1 || m.PreferredEngines[0] != harness.EngineClassPi {
		t.Errorf("unexpected preferred engines: %v", m.PreferredEngines)
	}

	// Test specialist mapping
	m, ok = r.GetMapping(Capability("specialist"))
	if !ok {
		t.Fatal("specialist capability not found")
	}
	if len(m.RequiredTools) != 1 || m.RequiredTools[0] != harness.CapabilitySpecialist {
		t.Errorf("unexpected required tools: %v", m.RequiredTools)
	}
	if len(m.PreferredEngines) != 1 || m.PreferredEngines[0] != harness.EngineClassOMP {
		t.Errorf("unexpected preferred engines: %v", m.PreferredEngines)
	}
}

func TestCapabilityRegistryDeterministicOrdering(t *testing.T) {
	r := DefaultCapabilityRegistry()

	caps := r.AllCapabilities()
	if len(caps) == 0 {
		t.Fatal("no capabilities returned")
	}
	for i := 1; i < len(caps); i++ {
		if string(caps[i]) < string(caps[i-1]) {
			t.Errorf("capabilities not sorted: %s before %s", caps[i-1], caps[i])
		}
	}

	roles := r.AllRoles()
	if len(roles) == 0 {
		t.Fatal("no roles returned")
	}
	for i := 1; i < len(roles); i++ {
		if string(roles[i]) < string(roles[i-1]) {
			t.Errorf("roles not sorted: %s before %s", roles[i-1], roles[i])
		}
	}
}
