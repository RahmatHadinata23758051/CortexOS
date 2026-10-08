package staff

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

func validDefinition() Definition {
	now := time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC)
	return Definition{
		Identity: Identity{ID: "staff-1", Name: "Builder"},
		Role:     RoleImplementer, Capabilities: []Capability{"coding", "test_run"},
		Permissions: PermissionSet{{ID: "read-worktree", Action: "read", Resource: "src/*", Effect: EffectAllow}},
		Skills:      []SkillReference{{ID: "go", Version: "v1"}},
		Memory:      []MemoryReference{{ID: "staff-memory", Kind: "episodic", Version: "v1"}},
		Workspace:   WorkspaceAssignment{WorkspaceID: "workspace-1", ProjectID: "project-1", WorktreeID: "worktree-1"},
		Lifecycle:   LifecycleActive, Availability: AvailabilityAvailable,
		CreatedAt: now, UpdatedAt: now, SchemaVersion: ContractVersion,
	}
}

func TestDefinitionValidateAllRoles(t *testing.T) {
	roles := []Role{
		RoleCoordinator,
		RolePlanner,
		RoleImplementer,
		RoleReviewer,
		RoleSpecialist,
	}
	for _, role := range roles {
		t.Run(string(role), func(t *testing.T) {
			d := validDefinition()
			d.Role = role
			if err := d.Validate(); err != nil {
				t.Fatalf("expected valid role %q to pass validation, got %v", role, err)
			}
		})
	}
}

func TestDefinitionValidateEdgeCases(t *testing.T) {
	if err := validDefinition().Validate(); err != nil {
		t.Fatalf("valid definition rejected: %v", err)
	}
	tests := []struct {
		name   string
		mutate func(*Definition)
		code   ErrorCode
	}{
		{"unknown version", func(d *Definition) { d.SchemaVersion = "cortexos.staff.v99" }, ErrUnsupportedVersion},
		{"empty version", func(d *Definition) { d.SchemaVersion = "" }, ErrUnsupportedVersion},
		{"missing id", func(d *Definition) { d.ID = "" }, ErrInvalidRequest},
		{"id with whitespace", func(d *Definition) { d.ID = "staff 1" }, ErrInvalidRequest},
		{"id with control char", func(d *Definition) { d.ID = "staff\n1" }, ErrInvalidRequest},
		{"id with invalid punctuation", func(d *Definition) { d.ID = "staff/1" }, ErrInvalidRequest},
		{"missing name", func(d *Definition) { d.Name = "" }, ErrInvalidRequest},
		{"whitespace name", func(d *Definition) { d.Name = "   " }, ErrInvalidRequest},
		{"name with control char", func(d *Definition) { d.Name = "Builder\x00" }, ErrInvalidRequest},
		{"unknown role", func(d *Definition) { d.Role = "operator" }, ErrInvalidRequest},
		{"empty role", func(d *Definition) { d.Role = "" }, ErrInvalidRequest},
		{"empty capabilities", func(d *Definition) { d.Capabilities = nil }, ErrInvalidRequest},
		{"empty capability token", func(d *Definition) { d.Capabilities = []Capability{""} }, ErrInvalidRequest},
		{"capability with invalid char", func(d *Definition) { d.Capabilities = []Capability{"code/test"} }, ErrInvalidRequest},
		{"duplicate capability", func(d *Definition) { d.Capabilities = []Capability{"coding", "coding"} }, ErrInvalidRequest},
		{"invalid permission", func(d *Definition) {
			d.Permissions = PermissionSet{{ID: "x", Action: "read", Resource: "../secret", Effect: EffectAllow}}
		}, ErrInvalidPermission},
		{"skill missing id", func(d *Definition) { d.Skills = []SkillReference{{ID: "", Version: "v1"}} }, ErrInvalidRequest},
		{"skill missing version", func(d *Definition) { d.Skills = []SkillReference{{ID: "go", Version: ""}} }, ErrInvalidRequest},
		{"skill duplicate id", func(d *Definition) {
			d.Skills = []SkillReference{{ID: "go", Version: "v1"}, {ID: "go", Version: "v2"}}
		}, ErrInvalidRequest},
		{"memory missing id", func(d *Definition) { d.Memory = []MemoryReference{{ID: "", Kind: "episodic", Version: "v1"}} }, ErrInvalidRequest},
		{"memory missing kind", func(d *Definition) { d.Memory = []MemoryReference{{ID: "mem-1", Kind: "", Version: "v1"}} }, ErrInvalidRequest},
		{"memory missing version", func(d *Definition) { d.Memory = []MemoryReference{{ID: "mem-1", Kind: "episodic", Version: ""}} }, ErrInvalidRequest},
		{"memory duplicate id", func(d *Definition) {
			d.Memory = []MemoryReference{{ID: "mem-1", Kind: "episodic", Version: "v1"}, {ID: "mem-1", Kind: "semantic", Version: "v2"}}
		}, ErrInvalidRequest},
		{"missing workspace", func(d *Definition) { d.Workspace.WorkspaceID = "" }, ErrInvalidRequest},
		{"workspace with invalid token", func(d *Definition) { d.Workspace.WorkspaceID = "ws/1" }, ErrInvalidRequest},
		{"project with invalid token", func(d *Definition) { d.Workspace.ProjectID = "proj/1" }, ErrInvalidRequest},
		{"worktree with invalid token", func(d *Definition) { d.Workspace.WorktreeID = "wt/1" }, ErrInvalidRequest},
		{"invalid lifecycle", func(d *Definition) { d.Lifecycle = "starting" }, ErrInvalidRequest},
		{"empty lifecycle", func(d *Definition) { d.Lifecycle = "" }, ErrInvalidRequest},
		{"invalid availability", func(d *Definition) { d.Availability = "standby" }, ErrInvalidRequest},
		{"empty availability", func(d *Definition) { d.Availability = "" }, ErrInvalidRequest},
		{"zero createdAt", func(d *Definition) { d.CreatedAt = time.Time{} }, ErrInvalidRequest},
		{"zero updatedAt", func(d *Definition) { d.UpdatedAt = time.Time{} }, ErrInvalidRequest},
		{"updatedAt before createdAt", func(d *Definition) {
			d.UpdatedAt = d.CreatedAt.Add(-time.Hour)
		}, ErrInvalidRequest},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			d := validDefinition()
			test.mutate(&d)
			err := d.Validate()
			if ErrorCodeOf(err) != test.code {
				t.Fatalf("error code = %q, want %q; err=%v", ErrorCodeOf(err), test.code, err)
			}
		})
	}
}

func TestNewDefinitionCopiesCollections(t *testing.T) {
	d := validDefinition()
	copyDef, err := NewDefinition(d)
	if err != nil {
		t.Fatal(err)
	}
	d.Capabilities[0] = "changed"
	d.Permissions[0].ID = "changed"
	d.Skills[0].ID = "changed"
	d.Memory[0].ID = "changed"
	if copyDef.Capabilities[0] == "changed" || copyDef.Permissions[0].ID == "changed" ||
		copyDef.Skills[0].ID == "changed" || copyDef.Memory[0].ID == "changed" {
		t.Fatal("validated definition retained caller-owned collection storage")
	}
}

func TestNewDefinitionValidationFailure(t *testing.T) {
	invalid := validDefinition()
	invalid.SchemaVersion = "cortexos.staff.v99"
	_, err := NewDefinition(invalid)
	if ErrorCodeOf(err) != ErrUnsupportedVersion {
		t.Fatalf("expected ErrUnsupportedVersion, got %v", err)
	}
}

func TestFilterValidate(t *testing.T) {
	ws := WorkspaceID("ws-1")
	proj := ProjectID("proj-1")
	role := RoleImplementer
	validFilter := Filter{
		WorkspaceID: &ws,
		ProjectID:   &proj,
		Role:        &role,
		ActiveOnly:  true,
	}
	if err := validFilter.Validate(); err != nil {
		t.Fatalf("valid filter rejected: %v", err)
	}

	// Empty filter is valid
	if err := (Filter{}).Validate(); err != nil {
		t.Fatalf("empty filter rejected: %v", err)
	}

	badWS := WorkspaceID("ws/1")
	if err := (Filter{WorkspaceID: &badWS}).Validate(); ErrorCodeOf(err) != ErrInvalidRequest {
		t.Fatalf("invalid workspace filter unexpectedly accepted: %v", err)
	}

	badProj := ProjectID("proj/1")
	if err := (Filter{ProjectID: &badProj}).Validate(); ErrorCodeOf(err) != ErrInvalidRequest {
		t.Fatalf("invalid project filter unexpectedly accepted: %v", err)
	}

	badRole := Role("invalid-role")
	if err := (Filter{Role: &badRole}).Validate(); ErrorCodeOf(err) != ErrInvalidRequest {
		t.Fatalf("invalid role filter unexpectedly accepted: %v", err)
	}
}

func TestRouterRequestValidate(t *testing.T) {
	validReq := RouterRequest{
		WorkspaceID:          "ws-1",
		ProjectID:            "proj-1",
		RoleHint:             RoleImplementer,
		RequiredCapabilities: []Capability{"coding"},
		ExcludeIDs:           []StaffID{"staff-2"},
	}
	if err := validReq.Validate(); err != nil {
		t.Fatalf("valid router request rejected: %v", err)
	}

	// Missing workspace
	bad := validReq
	bad.WorkspaceID = ""
	if err := bad.Validate(); ErrorCodeOf(err) != ErrInvalidRequest {
		t.Fatalf("missing workspace id unexpectedly accepted: %v", err)
	}

	// Invalid workspace token
	bad = validReq
	bad.WorkspaceID = "ws/invalid"
	if err := bad.Validate(); ErrorCodeOf(err) != ErrInvalidRequest {
		t.Fatalf("invalid workspace token unexpectedly accepted: %v", err)
	}

	// Invalid project token
	bad = validReq
	bad.ProjectID = "proj/invalid"
	if err := bad.Validate(); ErrorCodeOf(err) != ErrInvalidRequest {
		t.Fatalf("invalid project token unexpectedly accepted: %v", err)
	}

	// Invalid role hint
	bad = validReq
	bad.RoleHint = "unknown-role"
	if err := bad.Validate(); ErrorCodeOf(err) != ErrInvalidRequest {
		t.Fatalf("invalid role hint unexpectedly accepted: %v", err)
	}

	// Invalid capability token
	bad = validReq
	bad.RequiredCapabilities = []Capability{"invalid/cap"}
	if err := bad.Validate(); ErrorCodeOf(err) != ErrInvalidRequest {
		t.Fatalf("invalid capability token unexpectedly accepted: %v", err)
	}

	// Invalid exclude ID token
	bad = validReq
	bad.ExcludeIDs = []StaffID{"invalid/id"}
	if err := bad.Validate(); ErrorCodeOf(err) != ErrInvalidRequest {
		t.Fatalf("invalid exclude ID token unexpectedly accepted: %v", err)
	}
}

func TestStableErrorClassification(t *testing.T) {
	err := WrapError(ErrConflict, "conflict", errors.New("internal detail"))
	if ErrorCodeOf(err) != ErrConflict || !strings.HasPrefix(err.Error(), "staff.conflict:") {
		t.Fatalf("unexpected error classification: code=%q err=%v", ErrorCodeOf(err), err)
	}

	if !IsNotFound(newError(ErrNotFound, "absent")) {
		t.Fatal("expected IsNotFound to return true")
	}
	if !IsPermissionDenied(newError(ErrPermissionDenied, "denied")) {
		t.Fatal("expected IsPermissionDenied to return true")
	}
	if !IsInvalidRequest(newError(ErrInvalidRequest, "invalid")) {
		t.Fatal("expected IsInvalidRequest to return true")
	}
	if !IsUnsupportedVersion(newError(ErrUnsupportedVersion, "version")) {
		t.Fatal("expected IsUnsupportedVersion to return true")
	}

	// Context cancellations
	if ErrorCodeOf(context.Canceled) != ErrCanceled {
		t.Fatalf("context.Canceled code = %q, want %q", ErrorCodeOf(context.Canceled), ErrCanceled)
	}
	if ErrorCodeOf(context.DeadlineExceeded) != ErrCanceled {
		t.Fatalf("context.DeadlineExceeded code = %q, want %q", ErrorCodeOf(context.DeadlineExceeded), ErrCanceled)
	}
	if CanceledError(nil) != nil {
		t.Fatal("CanceledError(nil) must be nil")
	}
	if err := CanceledError(context.Canceled); ErrorCodeOf(err) != ErrCanceled {
		t.Fatalf("CanceledError code = %q, want %q", ErrorCodeOf(err), ErrCanceled)
	}

	// Nil error
	if ErrorCodeOf(nil) != "" {
		t.Fatalf("ErrorCodeOf(nil) must be empty, got %q", ErrorCodeOf(nil))
	}

	// Error formatting
	var nilErr *Error
	if nilErr.Error() != "<nil>" {
		t.Fatalf("nil error formatting = %q", nilErr.Error())
	}
	bareErr := &Error{Code: ErrInternal}
	if bareErr.Error() != string(ErrInternal) {
		t.Fatalf("bare error formatting = %q", bareErr.Error())
	}
}
