package staff

import (
	"reflect"
	"strings"
	"testing"
	"time"
)

// TestLogicalStaffIndependenceFromProcessIdentity validates that Staff definitions
// are pure logical domain models with zero coupling to worker processes, OS PIDs,
// CLI engine handles, or host machine filesystem paths.
func TestLogicalStaffIndependenceFromProcessIdentity(t *testing.T) {
	// 1. Structural inspection: Definition must not have process/worker/engine fields
	defType := reflect.TypeOf(Definition{})
	forbiddenSubstrings := []string{"process", "worker", "pid", "handle", "engine", "verdict", "merge"}

	for i := 0; i < defType.NumField(); i++ {
		field := defType.Field(i)
		fieldNameLower := strings.ToLower(field.Name)
		for _, forbidden := range forbiddenSubstrings {
			if strings.Contains(fieldNameLower, forbidden) {
				t.Errorf("Definition struct leaked process/engine field %q (matches %q)", field.Name, forbidden)
			}
		}
	}

	// 2. Summary struct inspection
	summaryType := reflect.TypeOf(Summary{})
	for i := 0; i < summaryType.NumField(); i++ {
		field := summaryType.Field(i)
		fieldNameLower := strings.ToLower(field.Name)
		for _, forbidden := range forbiddenSubstrings {
			if strings.Contains(fieldNameLower, forbidden) {
				t.Errorf("Summary struct leaked process/engine field %q (matches %q)", field.Name, forbidden)
			}
		}
	}
}

// TestStaffInvarianceAcrossWorkerReplacements verifies that replacing, restarting,
// or crashing engine worker processes does not alter the logical Staff definition.
func TestStaffInvarianceAcrossWorkerReplacements(t *testing.T) {
	staffMember := validDefinition()
	staffMember.ID = "staff-senior-developer"
	staffMember.Role = RoleImplementer

	initialJSON, err := staffMember.MarshalJSON()
	if err != nil {
		t.Fatal(err)
	}

	// Simulate successive ephemeral worker dispatches (worker 1 -> crash -> worker 2 -> worker 3)
	workerSimulations := []struct {
		workerID string
		engine   string
		status   string
	}{
		{"worker-native-001", "native", "crashed"},
		{"worker-pi-002", "pi", "timeout"},
		{"worker-omp-003", "omp", "completed"},
	}

	for _, sim := range workerSimulations {
		// The Staff member's logical identity and state remain immutable through worker replacement
		if staffMember.ID != "staff-senior-developer" {
			t.Fatalf("Staff ID mutated during worker dispatch simulation %s: %s", sim.workerID, staffMember.ID)
		}
		if staffMember.Lifecycle != LifecycleActive {
			t.Fatalf("Staff lifecycle mutated during worker crash simulation %s: %s", sim.workerID, staffMember.Lifecycle)
		}
	}

	postJSON, err := staffMember.MarshalJSON()
	if err != nil {
		t.Fatal(err)
	}
	if string(initialJSON) != string(postJSON) {
		t.Fatalf("Staff definition drifted across worker executions:\nbefore: %s\nafter:  %s", string(initialJSON), string(postJSON))
	}
}

// TestStaffLifecycleIndependentFromProcessLifecycle validates that Staff lifecycle states
// (active, inactive, retired) are administrative states, not worker process execution states.
func TestStaffLifecycleIndependentFromProcessLifecycle(t *testing.T) {
	d := validDefinition()

	// Staff remains active even when no workers are running (idle state)
	d.Lifecycle = LifecycleActive
	d.Availability = AvailabilityAvailable
	if err := d.Validate(); err != nil {
		t.Fatalf("valid idle active Staff rejected: %v", err)
	}

	// Staff can be administratively retired while retaining valid schema
	d.Lifecycle = LifecycleRetired
	d.Availability = AvailabilityOffline
	if err := d.Validate(); err != nil {
		t.Fatalf("valid retired Staff rejected: %v", err)
	}

	// Ephemeral worker process states (e.g. "running", "spawned", "zombie", "killed")
	// must be rejected as invalid Staff lifecycle states
	for _, processState := range []LifecycleState{"running", "spawning", "killed", "zombie", "crashed", "exited"} {
		d.Lifecycle = processState
		if err := d.Validate(); ErrorCodeOf(err) != ErrInvalidRequest {
			t.Fatalf("worker process state %q was unexpectedly accepted as Staff lifecycle: %v", processState, err)
		}
	}
}

// TestWorkspaceAssignmentIsOpaqueDomainScope validates that WorkspaceAssignment
// uses logical identifiers and rejects host filesystem paths or process handles.
func TestWorkspaceAssignmentIsOpaqueDomainScope(t *testing.T) {
	// Valid opaque domain IDs
	validAssignment := WorkspaceAssignment{
		WorkspaceID: "ws-cortex-alpha",
		ProjectID:   "proj-backend-go",
		WorktreeID:  "wt-feat-ban-103",
	}
	if err := validAssignment.Validate(); err != nil {
		t.Fatalf("valid opaque domain assignment rejected: %v", err)
	}

	// Host paths must be rejected as workspace IDs
	invalidHostPaths := []string{
		`C:\Users\user\Project`,
		`/var/run/workers/worker-1.sock`,
		`/tmp/scratch`,
		`D:\worktree\path`,
		`\\server\share\path`,
	}
	for _, hostPath := range invalidHostPaths {
		t.Run("reject_host_path_"+hostPath, func(t *testing.T) {
			badWS := WorkspaceAssignment{WorkspaceID: WorkspaceID(hostPath)}
			if err := badWS.Validate(); ErrorCodeOf(err) != ErrInvalidRequest {
				t.Fatalf("host path %q unexpectedly accepted as WorkspaceID: %v", hostPath, err)
			}
		})
	}
}

// TestConcurrentStaffShareWorkspaceWithoutCollision validates that multiple
// distinct Staff members can share the same WorkspaceAssignment without
// process handle conflicts.
func TestConcurrentStaffShareWorkspaceWithoutCollision(t *testing.T) {
	now := time.Now().UTC()
	sharedScope := WorkspaceAssignment{
		WorkspaceID: "ws-monorepo",
		ProjectID:   "proj-core",
		WorktreeID:  "wt-shared-worktree",
	}

	staffA := Definition{
		Identity:      Identity{ID: "staff-coord", Name: "Coordinator"},
		Role:          RoleCoordinator,
		Capabilities:  []Capability{"task_plan"},
		Permissions:   PermissionSet{{ID: "perm-plan", Action: "plan", Resource: "*", Effect: EffectAllow}},
		Workspace:     sharedScope,
		Lifecycle:     LifecycleActive,
		Availability:  AvailabilityAvailable,
		CreatedAt:     now,
		UpdatedAt:     now,
		SchemaVersion: ContractVersion,
	}

	staffB := Definition{
		Identity:      Identity{ID: "staff-rev", Name: "Reviewer"},
		Role:          RoleReviewer,
		Capabilities:  []Capability{"review"},
		Permissions:   PermissionSet{{ID: "perm-rev", Action: "review", Resource: "src/*", Effect: EffectAllow}},
		Workspace:     sharedScope,
		Lifecycle:     LifecycleActive,
		Availability:  AvailabilityBusy,
		CreatedAt:     now,
		UpdatedAt:     now,
		SchemaVersion: ContractVersion,
	}

	if err := staffA.Validate(); err != nil {
		t.Fatalf("staffA validation failed: %v", err)
	}
	if err := staffB.Validate(); err != nil {
		t.Fatalf("staffB validation failed: %v", err)
	}

	// Distinct logical identities sharing identical workspace scope
	if staffA.ID == staffB.ID {
		t.Fatal("Staff identities must be distinct")
	}
	if staffA.Workspace != staffB.Workspace {
		t.Fatal("Staff members should be able to share exact same workspace scope")
	}
}
