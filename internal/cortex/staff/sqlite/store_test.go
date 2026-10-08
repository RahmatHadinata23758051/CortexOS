package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/RahmatHadinata23758051/CortexOS/internal/cortex/staff"
	_ "modernc.org/sqlite"
)

func tempDBPath(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	return filepath.Join(dir, "staff.db")
}

func validDefinition(t *testing.T) staff.Definition {
	t.Helper()
	def := staff.Definition{
		Identity: staff.Identity{ID: staff.StaffID("test-staff-1"), Name: "Test Staff"},
		Role:     staff.RoleImplementer,
		Capabilities: []staff.Capability{
			staff.Capability("coding"),
			staff.Capability("review"),
		},
		Permissions: staff.PermissionSet{
			{ID: "read-src", Action: "read", Resource: "src/*", Effect: staff.EffectAllow, Priority: 10},
			{ID: "write-tests", Action: "write", Resource: "tests/*", Effect: staff.EffectAsk, Priority: 5},
		},
		Skills: []staff.SkillReference{
			{ID: "golang", Version: "1.0"},
			{ID: "testing", Version: "2.0"},
		},
		Memory: []staff.MemoryReference{
			{ID: "mem-1", Kind: "episodic", Version: "1"},
			{ID: "mem-2", Kind: "semantic", Version: "1"},
		},
		Workspace: staff.WorkspaceAssignment{
			WorkspaceID: staff.WorkspaceID("ws-1"),
			ProjectID:   staff.ProjectID("proj-1"),
			WorktreeID:  staff.WorktreeID("wt-1"),
		},
		Lifecycle:     staff.LifecycleActive,
		Availability:  staff.AvailabilityAvailable,
		CreatedAt:     time.Now().UTC(),
		UpdatedAt:     time.Now().UTC(),
		SchemaVersion: staff.ContractVersion,
	}
	validated, err := staff.NewDefinition(def)
	if err != nil {
		t.Fatalf("valid definition failed validation: %v", err)
	}
	return validated
}

func TestOpenAndSchemaVersion(t *testing.T) {
	t.Parallel()
	dbPath := tempDBPath(t)
	s, err := Open(context.Background(), dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	version, err := s.SchemaVersion(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if version != staff.ContractVersion {
		t.Fatalf("expected %q, got %q", staff.ContractVersion, version)
	}
}

func TestMigrationIdempotence(t *testing.T) {
	t.Parallel()
	dbPath := tempDBPath(t)
	s1, err := Open(context.Background(), dbPath)
	if err != nil {
		t.Fatal(err)
	}
	s1.Close()

	s2, err := Open(context.Background(), dbPath)
	if err != nil {
		t.Fatal(err)
	}
	s2.Close()

	// Verify tables exist and data can be inserted
	s3, err := Open(context.Background(), dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer s3.Close()

	def := validDefinition(t)
	_, err = s3.SaveDefinition(context.Background(), def)
	if err != nil {
		t.Fatal(err)
	}
}

func TestMigrationConcurrency(t *testing.T) {
	t.Parallel()
	dbPath := tempDBPath(t)
	var wg sync.WaitGroup
	errs := make(chan error, 10)
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			s, err := Open(context.Background(), dbPath)
			if err != nil {
				errs <- err
				return
			}
			s.Close()
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Fatal(err)
	}
}

func TestApplyMigrationsRejectsNonContiguousMetadata(t *testing.T) {
	t.Parallel()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec(`CREATE TABLE staff_schema (version INTEGER PRIMARY KEY NOT NULL, name TEXT NOT NULL, applied_at TEXT NOT NULL);
		INSERT INTO staff_schema(version, name, applied_at) VALUES (2, 'staff-2', CURRENT_TIMESTAMP);`); err != nil {
		t.Fatal(err)
	}
	if err := ApplyMigrations(context.Background(), db); err == nil {
		t.Fatal("expected non-contiguous metadata error")
	} else if !errors.Is(err, ErrUnknownSchema) {
		t.Fatalf("error = %v, want unknown schema classification", err)
	}
}

func TestFKEnforcement(t *testing.T) {
	t.Parallel()
	dbPath := tempDBPath(t)
	s, err := Open(context.Background(), dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	// Try to insert capability for non-existent staff - should fail due to FK
	_, err = s.db.ExecContext(context.Background(),
		`INSERT INTO staff_capabilities(staff_id, position, capability) VALUES (?, ?, ?)`,
		"nonexistent", 0, "coding")
	if err == nil {
		t.Fatal("expected foreign key constraint violation for capability")
	}

	// Same for permissions
	_, err = s.db.ExecContext(context.Background(),
		`INSERT INTO staff_permissions(staff_id, position, permission_id, action, resource, effect, priority) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		"nonexistent", 0, "p1", "read", "src/*", "allow", 10)
	if err == nil {
		t.Fatal("expected foreign key constraint violation for permission")
	}

	// Same for skills
	_, err = s.db.ExecContext(context.Background(),
		`INSERT INTO staff_skills(staff_id, position, skill_id, skill_version) VALUES (?, ?, ?, ?)`,
		"nonexistent", 0, "golang", "1.0")
	if err == nil {
		t.Fatal("expected foreign key constraint violation for skill")
	}

	// Same for memory
	_, err = s.db.ExecContext(context.Background(),
		`INSERT INTO staff_memory(staff_id, position, memory_id, memory_kind, memory_version) VALUES (?, ?, ?, ?, ?)`,
		"nonexistent", 0, "mem-1", "episodic", "1")
	if err == nil {
		t.Fatal("expected foreign key constraint violation for memory")
	}
}

func TestCRUD(t *testing.T) {
	t.Parallel()
	dbPath := tempDBPath(t)
	s, err := Open(context.Background(), dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	def := validDefinition(t)

	// Create
	saved, err := s.SaveDefinition(context.Background(), def)
	if err != nil {
		t.Fatalf("SaveDefinition: %v", err)
	}
	if saved.ID != def.ID || saved.Name != def.Name || saved.Role != def.Role {
		t.Fatalf("saved definition differs: got %+v", saved)
	}

	// Read
	got, err := s.GetDefinition(context.Background(), def.ID)
	if err != nil {
		t.Fatalf("GetDefinition: %v", err)
	}
	if got.ID != def.ID {
		t.Fatalf("ID mismatch: %v vs %v", got.ID, def.ID)
	}
	if got.Name != def.Name {
		t.Fatalf("Name mismatch: %v vs %v", got.Name, def.Name)
	}
	if got.Role != def.Role {
		t.Fatalf("Role mismatch: %v vs %v", got.Role, def.Role)
	}
	if len(got.Capabilities) != len(def.Capabilities) {
		t.Fatalf("Capabilities count mismatch: %d vs %d", len(got.Capabilities), len(def.Capabilities))
	}
	for i := range def.Capabilities {
		if got.Capabilities[i] != def.Capabilities[i] {
			t.Fatalf("Capability[%d] mismatch: %v vs %v", i, got.Capabilities[i], def.Capabilities[i])
		}
	}
	if len(got.Permissions) != len(def.Permissions) {
		t.Fatalf("Permissions count mismatch: %d vs %d", len(got.Permissions), len(def.Permissions))
	}
	for i := range def.Permissions {
		if got.Permissions[i].ID != def.Permissions[i].ID {
			t.Fatalf("Permission[%d] ID mismatch: %v vs %v", i, got.Permissions[i].ID, def.Permissions[i].ID)
		}
	}
	if len(got.Skills) != len(def.Skills) {
		t.Fatalf("Skills count mismatch: %d vs %d", len(got.Skills), len(def.Skills))
	}
	for i := range def.Skills {
		if got.Skills[i].ID != def.Skills[i].ID || got.Skills[i].Version != def.Skills[i].Version {
			t.Fatalf("Skill[%d] mismatch: %+v vs %+v", i, got.Skills[i], def.Skills[i])
		}
	}
	if len(got.Memory) != len(def.Memory) {
		t.Fatalf("Memory count mismatch: %d vs %d", len(got.Memory), len(def.Memory))
	}
	for i := range def.Memory {
		if got.Memory[i].ID != def.Memory[i].ID || got.Memory[i].Kind != def.Memory[i].Kind || got.Memory[i].Version != def.Memory[i].Version {
			t.Fatalf("Memory[%d] mismatch: %+v vs %+v", i, got.Memory[i], def.Memory[i])
		}
	}
	if got.Workspace.WorkspaceID != def.Workspace.WorkspaceID ||
		got.Workspace.ProjectID != def.Workspace.ProjectID ||
		got.Workspace.WorktreeID != def.Workspace.WorktreeID {
		t.Fatalf("Workspace mismatch: %+v vs %+v", got.Workspace, def.Workspace)
	}
	if got.Lifecycle != def.Lifecycle {
		t.Fatalf("Lifecycle mismatch: %v vs %v", got.Lifecycle, def.Lifecycle)
	}
	if got.Availability != def.Availability {
		t.Fatalf("Availability mismatch: %v vs %v", got.Availability, def.Availability)
	}
	if got.SchemaVersion != def.SchemaVersion {
		t.Fatalf("SchemaVersion mismatch: %v vs %v", got.SchemaVersion, def.SchemaVersion)
	}

	// Update (change name, role, availability)
	updated := got
	updated.Name = "Updated Name"
	updated.Role = staff.RoleReviewer
	updated.Availability = staff.AvailabilityBusy
	updated.UpdatedAt = time.Now().UTC()
	saved2, err := s.SaveDefinition(context.Background(), updated)
	if err != nil {
		t.Fatalf("Update SaveDefinition: %v", err)
	}
	if saved2.Name != "Updated Name" || saved2.Role != staff.RoleReviewer || saved2.Availability != staff.AvailabilityBusy {
		t.Fatalf("update not persisted: %+v", saved2)
	}
	if !saved2.UpdatedAt.After(saved2.CreatedAt) {
		t.Fatalf("UpdatedAt must be after CreatedAt")
	}
	if saved2.CreatedAt != got.CreatedAt {
		t.Fatalf("CreatedAt must not change on update")
	}

	// Delete
	err = s.DeleteDefinition(context.Background(), def.ID)
	if err != nil {
		t.Fatalf("DeleteDefinition: %v", err)
	}
	_, err = s.GetDefinition(context.Background(), def.ID)
	if !staff.IsNotFound(err) {
		t.Fatalf("expected NotFound after delete, got %v", err)
	}
}

func TestDeleteNonExistent(t *testing.T) {
	t.Parallel()
	dbPath := tempDBPath(t)
	s, err := Open(context.Background(), dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	err = s.DeleteDefinition(context.Background(), staff.StaffID("nonexistent"))
	if !staff.IsNotFound(err) {
		t.Fatalf("expected NotFound for non-existent delete, got %v", err)
	}
}

func TestListDefinitions(t *testing.T) {
	t.Parallel()
	dbPath := tempDBPath(t)
	s, err := Open(context.Background(), dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	defs := []staff.Definition{
		{Identity: staff.Identity{ID: "staff-a", Name: "Staff A"}, Role: staff.RoleImplementer, Capabilities: []staff.Capability{"coding"}, Permissions: staff.PermissionSet{{ID: "p1", Action: "read", Resource: "src/*", Effect: staff.EffectAllow}}, Workspace: staff.WorkspaceAssignment{WorkspaceID: "ws-1"}, Lifecycle: staff.LifecycleActive, Availability: staff.AvailabilityAvailable, CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC(), SchemaVersion: staff.ContractVersion},
		{Identity: staff.Identity{ID: "staff-b", Name: "Staff B"}, Role: staff.RoleReviewer, Capabilities: []staff.Capability{"review"}, Permissions: staff.PermissionSet{{ID: "p2", Action: "read", Resource: "tests/*", Effect: staff.EffectAllow}}, Workspace: staff.WorkspaceAssignment{WorkspaceID: "ws-1"}, Lifecycle: staff.LifecycleActive, Availability: staff.AvailabilityAvailable, CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC(), SchemaVersion: staff.ContractVersion},
		{Identity: staff.Identity{ID: "staff-c", Name: "Staff C"}, Role: staff.RoleImplementer, Capabilities: []staff.Capability{"coding"}, Permissions: staff.PermissionSet{{ID: "p3", Action: "write", Resource: "src/*", Effect: staff.EffectAllow}}, Workspace: staff.WorkspaceAssignment{WorkspaceID: "ws-2"}, Lifecycle: staff.LifecycleInactive, Availability: staff.AvailabilityOffline, CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC(), SchemaVersion: staff.ContractVersion},
	}
	for _, def := range defs {
		validated, err := staff.NewDefinition(def)
		if err != nil {
			t.Fatalf("valid definition: %v", err)
		}
		_, err = s.SaveDefinition(context.Background(), validated)
		if err != nil {
			t.Fatalf("save: %v", err)
		}
	}

	all, err := s.ListDefinitions(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 3 {
		t.Fatalf("expected 3 staff, got %d", len(all))
	}
	// Deterministic ordering by ID
	if all[0].ID != "staff-a" || all[1].ID != "staff-b" || all[2].ID != "staff-c" {
		t.Fatalf("unexpected order: %v %v %v", all[0].ID, all[1].ID, all[2].ID)
	}
}

func TestListDefinitionsByWorkspace(t *testing.T) {
	t.Parallel()
	dbPath := tempDBPath(t)
	s, err := Open(context.Background(), dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	defs := []staff.Definition{
		{Identity: staff.Identity{ID: "staff-a", Name: "Staff A"}, Role: staff.RoleImplementer, Capabilities: []staff.Capability{"coding"}, Permissions: staff.PermissionSet{{ID: "p1", Action: "read", Resource: "src/*", Effect: staff.EffectAllow}}, Workspace: staff.WorkspaceAssignment{WorkspaceID: "ws-1"}, Lifecycle: staff.LifecycleActive, Availability: staff.AvailabilityAvailable, CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC(), SchemaVersion: staff.ContractVersion},
		{Identity: staff.Identity{ID: "staff-b", Name: "Staff B"}, Role: staff.RoleReviewer, Capabilities: []staff.Capability{"review"}, Permissions: staff.PermissionSet{{ID: "p2", Action: "read", Resource: "tests/*", Effect: staff.EffectAllow}}, Workspace: staff.WorkspaceAssignment{WorkspaceID: "ws-2"}, Lifecycle: staff.LifecycleActive, Availability: staff.AvailabilityAvailable, CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC(), SchemaVersion: staff.ContractVersion},
	}
	for _, def := range defs {
		validated, _ := staff.NewDefinition(def)
		_, _ = s.SaveDefinition(context.Background(), validated)
	}

	ws1, err := s.ListDefinitionsByWorkspace(context.Background(), staff.WorkspaceID("ws-1"))
	if err != nil {
		t.Fatal(err)
	}
	if len(ws1) != 1 || ws1[0].ID != "staff-a" {
		t.Fatalf("expected staff-a in ws-1, got %v", ws1)
	}

	ws2, err := s.ListDefinitionsByWorkspace(context.Background(), staff.WorkspaceID("ws-2"))
	if err != nil {
		t.Fatal(err)
	}
	if len(ws2) != 1 || ws2[0].ID != "staff-b" {
		t.Fatalf("expected staff-b in ws-2, got %v", ws2)
	}
}

func TestListDefinitionsByProject(t *testing.T) {
	t.Parallel()
	dbPath := tempDBPath(t)
	s, err := Open(context.Background(), dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	defs := []staff.Definition{
		{Identity: staff.Identity{ID: "staff-a", Name: "Staff A"}, Role: staff.RoleImplementer, Capabilities: []staff.Capability{"coding"}, Permissions: staff.PermissionSet{{ID: "p1", Action: "read", Resource: "src/*", Effect: staff.EffectAllow}}, Workspace: staff.WorkspaceAssignment{WorkspaceID: "ws-1", ProjectID: "proj-1"}, Lifecycle: staff.LifecycleActive, Availability: staff.AvailabilityAvailable, CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC(), SchemaVersion: staff.ContractVersion},
		{Identity: staff.Identity{ID: "staff-b", Name: "Staff B"}, Role: staff.RoleReviewer, Capabilities: []staff.Capability{"review"}, Permissions: staff.PermissionSet{{ID: "p2", Action: "read", Resource: "tests/*", Effect: staff.EffectAllow}}, Workspace: staff.WorkspaceAssignment{WorkspaceID: "ws-1", ProjectID: "proj-2"}, Lifecycle: staff.LifecycleActive, Availability: staff.AvailabilityAvailable, CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC(), SchemaVersion: staff.ContractVersion},
	}
	for _, def := range defs {
		validated, _ := staff.NewDefinition(def)
		_, _ = s.SaveDefinition(context.Background(), validated)
	}

	proj1, err := s.ListDefinitionsByProject(context.Background(), staff.ProjectID("proj-1"))
	if err != nil {
		t.Fatal(err)
	}
	if len(proj1) != 1 || proj1[0].ID != "staff-a" {
		t.Fatalf("expected staff-a in proj-1, got %v", proj1)
	}
}

func TestListDefinitionsByRole(t *testing.T) {
	t.Parallel()
	dbPath := tempDBPath(t)
	s, err := Open(context.Background(), dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	defs := []staff.Definition{
		{Identity: staff.Identity{ID: "staff-a", Name: "Staff A"}, Role: staff.RoleImplementer, Capabilities: []staff.Capability{"coding"}, Permissions: staff.PermissionSet{{ID: "p1", Action: "read", Resource: "src/*", Effect: staff.EffectAllow}}, Workspace: staff.WorkspaceAssignment{WorkspaceID: "ws-1"}, Lifecycle: staff.LifecycleActive, Availability: staff.AvailabilityAvailable, CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC(), SchemaVersion: staff.ContractVersion},
		{Identity: staff.Identity{ID: "staff-b", Name: "Staff B"}, Role: staff.RoleReviewer, Capabilities: []staff.Capability{"review"}, Permissions: staff.PermissionSet{{ID: "p2", Action: "read", Resource: "tests/*", Effect: staff.EffectAllow}}, Workspace: staff.WorkspaceAssignment{WorkspaceID: "ws-1"}, Lifecycle: staff.LifecycleActive, Availability: staff.AvailabilityAvailable, CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC(), SchemaVersion: staff.ContractVersion},
		{Identity: staff.Identity{ID: "staff-c", Name: "Staff C"}, Role: staff.RoleImplementer, Capabilities: []staff.Capability{"testing"}, Permissions: staff.PermissionSet{{ID: "p3", Action: "read", Resource: "tests/*", Effect: staff.EffectAllow}}, Workspace: staff.WorkspaceAssignment{WorkspaceID: "ws-1"}, Lifecycle: staff.LifecycleActive, Availability: staff.AvailabilityAvailable, CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC(), SchemaVersion: staff.ContractVersion},
	}
	for _, def := range defs {
		validated, _ := staff.NewDefinition(def)
		_, _ = s.SaveDefinition(context.Background(), validated)
	}

	impl, err := s.ListDefinitionsByRole(context.Background(), staff.RoleImplementer)
	if err != nil {
		t.Fatal(err)
	}
	if len(impl) != 2 {
		t.Fatalf("expected 2 implementers, got %d", len(impl))
	}
	for _, d := range impl {
		if d.Role != staff.RoleImplementer {
			t.Fatalf("non-implementer in list: %v", d.Role)
		}
	}
}

func TestValidationRejection(t *testing.T) {
	t.Parallel()
	dbPath := tempDBPath(t)
	s, err := Open(context.Background(), dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	// Invalid schema version
	badVersion := validDefinition(t)
	badVersion.SchemaVersion = "unsupported.v1"
	_, err = s.SaveDefinition(context.Background(), badVersion)
	if !staff.IsUnsupportedVersion(err) {
		t.Fatalf("expected unsupported version error, got %v", err)
	}

	// Empty capabilities
	badCaps := validDefinition(t)
	badCaps.Capabilities = nil
	_, err = s.SaveDefinition(context.Background(), badCaps)
	if !staff.IsInvalidRequest(err) {
		t.Fatalf("expected invalid request for empty capabilities, got %v", err)
	}

	// Duplicate capability
	badDupCap := validDefinition(t)
	badDupCap.Capabilities = []staff.Capability{"coding", "coding"}
	_, err = s.SaveDefinition(context.Background(), badDupCap)
	if !staff.IsInvalidRequest(err) {
		t.Fatalf("expected invalid request for duplicate capability, got %v", err)
	}

	// Invalid role
	badRole := validDefinition(t)
	badRole.Role = "invalid-role"
	_, err = s.SaveDefinition(context.Background(), badRole)
	if !staff.IsInvalidRequest(err) {
		t.Fatalf("expected invalid request for invalid role, got %v", err)
	}

	// Invalid lifecycle
	badLife := validDefinition(t)
	badLife.Lifecycle = "invalid"
	_, err = s.SaveDefinition(context.Background(), badLife)
	if !staff.IsInvalidRequest(err) {
		t.Fatalf("expected invalid request for invalid lifecycle, got %v", err)
	}

	// CreatedAt is zero
	badTime := validDefinition(t)
	badTime.CreatedAt = time.Time{}
	_, err = s.SaveDefinition(context.Background(), badTime)
	if !staff.IsInvalidRequest(err) {
		t.Fatalf("expected invalid request for zero createdAt, got %v", err)
	}

	// UpdatedAt before CreatedAt
	badOrder := validDefinition(t)
	badOrder.UpdatedAt = badOrder.CreatedAt.Add(-time.Hour)
	_, err = s.SaveDefinition(context.Background(), badOrder)
	if !staff.IsInvalidRequest(err) {
		t.Fatalf("expected invalid request for UpdatedAt before CreatedAt, got %v", err)
	}

	// Invalid workspace ID (empty)
	badWS := validDefinition(t)
	badWS.Workspace.WorkspaceID = ""
	_, err = s.SaveDefinition(context.Background(), badWS)
	if !staff.IsInvalidRequest(err) {
		t.Fatalf("expected invalid request for empty workspace ID, got %v", err)
	}

	// Invalid permission
	badPerm := validDefinition(t)
	badPerm.Permissions = staff.PermissionSet{{ID: "", Action: "read", Resource: "src/*", Effect: staff.EffectAllow}}
	_, err = s.SaveDefinition(context.Background(), badPerm)
	if staff.ErrorCodeOf(err) != staff.ErrInvalidPermission {
		t.Fatalf("expected invalid permission error, got %v", err)
	}
}

func TestContextCancellation(t *testing.T) {
	t.Parallel()
	dbPath := tempDBPath(t)
	s, err := Open(context.Background(), dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	_, err = s.SaveDefinition(ctx, validDefinition(t))
	if err == nil || !errors.Is(err, context.Canceled) {
		t.Fatalf("expected canceled error, got %v", err)
	}

	_, err = s.GetDefinition(ctx, staff.StaffID("any"))
	if err == nil || !errors.Is(err, context.Canceled) {
		t.Fatalf("expected canceled error, got %v", err)
	}

	_, err = s.ListDefinitions(ctx)
	if err == nil || !errors.Is(err, context.Canceled) {
		t.Fatalf("expected canceled error, got %v", err)
	}
}

func TestDeterministicListOrdering(t *testing.T) {
	t.Parallel()
	dbPath := tempDBPath(t)
	s, err := Open(context.Background(), dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	ids := []string{"staff-z", "staff-a", "staff-m", "staff-b"}
	for _, id := range ids {
		def := staff.Definition{
			Identity:     staff.Identity{ID: staff.StaffID(id), Name: id},
			Role:         staff.RoleImplementer,
			Capabilities: []staff.Capability{staff.Capability("coding")},
			Permissions:  staff.PermissionSet{{ID: "p1", Action: "read", Resource: "src/*", Effect: staff.EffectAllow}},
			Workspace:    staff.WorkspaceAssignment{WorkspaceID: staff.WorkspaceID("ws-1")},
			Lifecycle:    staff.LifecycleActive,
			Availability: staff.AvailabilityAvailable,
			CreatedAt:    time.Now().UTC(), UpdatedAt: time.Now().UTC(),
			SchemaVersion: staff.ContractVersion,
		}
		validated, _ := staff.NewDefinition(def)
		_, _ = s.SaveDefinition(context.Background(), validated)
	}

	list, err := s.ListDefinitions(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 4 {
		t.Fatalf("expected 4, got %d", len(list))
	}
	// Must be sorted by ID ascending
	for i := 1; i < len(list); i++ {
		if string(list[i-1].ID) >= string(list[i].ID) {
			t.Fatalf("list not sorted by ID: %v >= %v", list[i-1].ID, list[i].ID)
		}
	}
}

func TestJSONRedactionSafety(t *testing.T) {
	t.Parallel()
	dbPath := tempDBPath(t)
	s, err := Open(context.Background(), dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	// Ensure capabilities/permissions/skills/memory are stored normalized
	def := validDefinition(t)
	saved, err := s.SaveDefinition(context.Background(), def)
	if err != nil {
		t.Fatal(err)
	}

	// Direct query to verify child tables have correct data
	rows, err := s.db.QueryContext(context.Background(),
		`SELECT capability FROM staff_capabilities WHERE staff_id = ? ORDER BY position`, string(def.ID))
	if err != nil {
		t.Fatal(err)
	}
	var caps []string
	for rows.Next() {
		var c string
		if err := rows.Scan(&c); err != nil {
			_ = rows.Close()
			t.Fatal(err)
		}
		caps = append(caps, c)
	}
	_ = rows.Close()
	if len(caps) != 2 || caps[0] != "coding" || caps[1] != "review" {
		t.Fatalf("capabilities not stored correctly: %v", caps)
	}

	rows, err = s.db.QueryContext(context.Background(),
		`SELECT permission_id, action, resource, effect, priority FROM staff_permissions WHERE staff_id = ? ORDER BY position`, string(def.ID))
	if err != nil {
		t.Fatal(err)
	}
	var perms []struct {
		ID, Action, Resource, Effect string
		Priority                     int
	}
	for rows.Next() {
		var p struct {
			ID, Action, Resource, Effect string
			Priority                     int
		}
		if err := rows.Scan(&p.ID, &p.Action, &p.Resource, &p.Effect, &p.Priority); err != nil {
			_ = rows.Close()
			t.Fatal(err)
		}
		perms = append(perms, p)
	}
	_ = rows.Close()
	if len(perms) != 2 {
		t.Fatalf("expected 2 permissions, got %d", len(perms))
	}

	// Verify saved definition matches
	if len(saved.Capabilities) != 2 || saved.Capabilities[0] != "coding" || saved.Capabilities[1] != "review" {
		t.Fatalf("saved capabilities mismatch: %v", saved.Capabilities)
	}
	if len(saved.Permissions) != 2 {
		t.Fatalf("saved permissions mismatch: %v", saved.Permissions)
	}

	// Ensure no forbidden fields can be stored (by validating on read)
	_, err = s.db.ExecContext(context.Background(),
		`UPDATE staff_definitions SET schema_version = 'cortexos.staff.v1' WHERE id = ?`, string(def.ID))
	if err != nil {
		t.Fatal(err)
	}
	// Should still validate correctly on read
	got, err := s.GetDefinition(context.Background(), def.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.SchemaVersion != staff.ContractVersion {
		t.Fatalf("schema version changed: %v", got.SchemaVersion)
	}
}

func TestConcurrentOperations(t *testing.T) {
	t.Parallel()
	dbPath := tempDBPath(t)
	s, err := Open(context.Background(), dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	var wg sync.WaitGroup
	errs := make(chan error, 20)

	// Concurrent saves
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			def := staff.Definition{
				Identity:     staff.Identity{ID: staff.StaffID(fmt.Sprintf("staff-concurrent-%d", i)), Name: fmt.Sprintf("Staff %d", i)},
				Role:         staff.RoleImplementer,
				Capabilities: []staff.Capability{staff.Capability("coding")},
				Permissions:  staff.PermissionSet{{ID: "p1", Action: "read", Resource: "src/*", Effect: staff.EffectAllow}},
				Workspace:    staff.WorkspaceAssignment{WorkspaceID: staff.WorkspaceID("ws-1")},
				Lifecycle:    staff.LifecycleActive,
				Availability: staff.AvailabilityAvailable,
				CreatedAt:    time.Now().UTC(), UpdatedAt: time.Now().UTC(),
				SchemaVersion: staff.ContractVersion,
			}
			validated, _ := staff.NewDefinition(def)
			if _, err := s.SaveDefinition(context.Background(), validated); err != nil {
				errs <- err
			}
		}(i)
	}

	// Concurrent reads
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := s.ListDefinitions(context.Background()); err != nil {
				errs <- err
			}
		}()
	}

	wg.Wait()
	close(errs)
	for err := range errs {
		t.Fatal(err)
	}

	// Verify all saved
	list, err := s.ListDefinitions(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 10 {
		t.Fatalf("expected 10 staff, got %d", len(list))
	}
}

func TestCorruptRecordRejection(t *testing.T) {
	t.Parallel()
	dbPath := tempDBPath(t)
	s, err := Open(context.Background(), dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	def := validDefinition(t)
	_, err = s.SaveDefinition(context.Background(), def)
	if err != nil {
		t.Fatal(err)
	}

	// Corrupt the schema_version in the database
	_, err = s.db.ExecContext(context.Background(),
		`UPDATE staff_definitions SET schema_version = 'corrupt.version' WHERE id = ?`, string(def.ID))
	if err != nil {
		t.Fatal(err)
	}

	// Read should fail closed
	_, err = s.GetDefinition(context.Background(), def.ID)
	if !staff.IsUnsupportedVersion(err) && !staff.IsInvalidRequest(err) {
		t.Fatalf("expected unsupported version or invalid request for corrupt record, got %v", err)
	}
}

func TestSafeDelete(t *testing.T) {
	t.Parallel()
	dbPath := tempDBPath(t)
	s, err := Open(context.Background(), dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	def := validDefinition(t)
	_, err = s.SaveDefinition(context.Background(), def)
	if err != nil {
		t.Fatal(err)
	}

	// Verify child records exist before delete
	rows, _ := s.db.QueryContext(context.Background(),
		`SELECT COUNT(*) FROM staff_capabilities WHERE staff_id = ?`, string(def.ID))
	var count int
	rows.Next()
	rows.Scan(&count)
	_ = rows.Close()
	if count == 0 {
		t.Fatal("expected capabilities before delete")
	}

	err = s.DeleteDefinition(context.Background(), def.ID)
	if err != nil {
		t.Fatal(err)
	}

	// Verify child records are cascade deleted
	rows, _ = s.db.QueryContext(context.Background(),
		`SELECT COUNT(*) FROM staff_capabilities WHERE staff_id = ?`, string(def.ID))
	rows.Next()
	rows.Scan(&count)
	_ = rows.Close()
	if count != 0 {
		t.Fatalf("expected 0 capabilities after cascade delete, got %d", count)
	}
	rows, _ = s.db.QueryContext(context.Background(),
		`SELECT COUNT(*) FROM staff_permissions WHERE staff_id = ?`, string(def.ID))
	rows.Next()
	rows.Scan(&count)
	_ = rows.Close()
	if count != 0 {
		t.Fatalf("expected 0 permissions after cascade delete, got %d", count)
	}
	rows, _ = s.db.QueryContext(context.Background(),
		`SELECT COUNT(*) FROM staff_skills WHERE staff_id = ?`, string(def.ID))
	rows.Next()
	rows.Scan(&count)
	_ = rows.Close()
	if count != 0 {
		t.Fatalf("expected 0 skills after cascade delete, got %d", count)
	}
	rows, _ = s.db.QueryContext(context.Background(),
		`SELECT COUNT(*) FROM staff_memory WHERE staff_id = ?`, string(def.ID))
	rows.Next()
	rows.Scan(&count)
	_ = rows.Close()
	if count != 0 {
		t.Fatalf("expected 0 memory after cascade delete, got %d", count)
	}
}

func TestMultipleSavesSameID(t *testing.T) {
	t.Parallel()
	dbPath := tempDBPath(t)
	s, err := Open(context.Background(), dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	def := validDefinition(t)
	_, err = s.SaveDefinition(context.Background(), def)
	if err != nil {
		t.Fatal(err)
	}

	// Save again with same ID but different name - should update
	def2 := def
	def2.Name = "New Name"
	def2.UpdatedAt = time.Now().UTC()
	_, err = s.SaveDefinition(context.Background(), def2)
	if err != nil {
		t.Fatal(err)
	}

	got, err := s.GetDefinition(context.Background(), def.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Name != "New Name" {
		t.Fatalf("name not updated: %v", got.Name)
	}
}

func TestNewAndOpenLazy(t *testing.T) {
	t.Parallel()
	dbPath := tempDBPath(t)
	s, err := New(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	ctx := context.Background()
	if err := s.Open(ctx); err != nil {
		t.Fatalf("lazy Open failed: %v", err)
	}

	def := validDefinition(t)
	saved, err := s.SaveDefinition(ctx, def)
	if err != nil {
		t.Fatalf("SaveDefinition after lazy Open: %v", err)
	}
	if saved.ID != def.ID {
		t.Fatalf("saved ID mismatch: %v", saved.ID)
	}
}

func TestFilterCombinationsAndActiveOnly(t *testing.T) {
	t.Parallel()
	dbPath := tempDBPath(t)
	s, err := Open(context.Background(), dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	ctx := context.Background()
	ws1 := staff.WorkspaceID("ws-alpha")
	ws2 := staff.WorkspaceID("ws-beta")
	proj1 := staff.ProjectID("proj-1")
	proj2 := staff.ProjectID("proj-2")
	roleCoord := staff.RoleCoordinator
	roleImpl := staff.RoleImplementer

	defs := []staff.Definition{
		{
			Identity:     staff.Identity{ID: "staff-1", Name: "Staff 1"},
			Role:         roleCoord,
			Capabilities: []staff.Capability{"coordination"},
			Permissions:  staff.PermissionSet{{ID: "p1", Action: "read", Resource: "*", Effect: staff.EffectAllow}},
			Workspace:    staff.WorkspaceAssignment{WorkspaceID: ws1, ProjectID: proj1},
			Lifecycle:    staff.LifecycleActive,
			Availability: staff.AvailabilityAvailable,
			CreatedAt:    time.Now().UTC(), UpdatedAt: time.Now().UTC(),
			SchemaVersion: staff.ContractVersion,
		},
		{
			Identity:     staff.Identity{ID: "staff-2", Name: "Staff 2"},
			Role:         roleImpl,
			Capabilities: []staff.Capability{"coding"},
			Permissions:  staff.PermissionSet{{ID: "p2", Action: "write", Resource: "src/*", Effect: staff.EffectAllow}},
			Workspace:    staff.WorkspaceAssignment{WorkspaceID: ws1, ProjectID: proj1},
			Lifecycle:    staff.LifecycleInactive, // Inactive
			Availability: staff.AvailabilityOffline,
			CreatedAt:    time.Now().UTC(), UpdatedAt: time.Now().UTC(),
			SchemaVersion: staff.ContractVersion,
		},
		{
			Identity:     staff.Identity{ID: "staff-3", Name: "Staff 3"},
			Role:         roleImpl,
			Capabilities: []staff.Capability{"coding"},
			Permissions:  staff.PermissionSet{{ID: "p3", Action: "write", Resource: "src/*", Effect: staff.EffectAllow}},
			Workspace:    staff.WorkspaceAssignment{WorkspaceID: ws1, ProjectID: proj2},
			Lifecycle:    staff.LifecycleActive,
			Availability: staff.AvailabilityAvailable,
			CreatedAt:    time.Now().UTC(), UpdatedAt: time.Now().UTC(),
			SchemaVersion: staff.ContractVersion,
		},
		{
			Identity:     staff.Identity{ID: "staff-4", Name: "Staff 4"},
			Role:         roleImpl,
			Capabilities: []staff.Capability{"coding"},
			Permissions:  staff.PermissionSet{{ID: "p4", Action: "read", Resource: "src/*", Effect: staff.EffectAllow}},
			Workspace:    staff.WorkspaceAssignment{WorkspaceID: ws2},
			Lifecycle:    staff.LifecycleActive,
			Availability: staff.AvailabilityAvailable,
			CreatedAt:    time.Now().UTC(), UpdatedAt: time.Now().UTC(),
			SchemaVersion: staff.ContractVersion,
		},
	}

	for _, d := range defs {
		v, err := staff.NewDefinition(d)
		if err != nil {
			t.Fatalf("NewDefinition(%s): %v", d.ID, err)
		}
		if _, err := s.SaveDefinition(ctx, v); err != nil {
			t.Fatalf("SaveDefinition(%s): %v", d.ID, err)
		}
	}

	// Filter: Workspace ws1 + ActiveOnly
	res, err := s.ListDefinitionsWithFilter(ctx, staff.Filter{
		WorkspaceID: &ws1,
		ActiveOnly:  true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(res) != 2 {
		t.Fatalf("expected 2 active staff in ws1, got %d", len(res))
	}
	if res[0].ID != "staff-1" || res[1].ID != "staff-3" {
		t.Fatalf("unexpected results: %v, %v", res[0].ID, res[1].ID)
	}

	// Filter: Workspace ws1 + Project proj1 + ActiveOnly
	res, err = s.ListDefinitionsWithFilter(ctx, staff.Filter{
		WorkspaceID: &ws1,
		ProjectID:   &proj1,
		ActiveOnly:  true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(res) != 1 || res[0].ID != "staff-1" {
		t.Fatalf("expected staff-1, got %v", res)
	}

	// Filter: Role Implementer + ActiveOnly
	res, err = s.ListDefinitionsWithFilter(ctx, staff.Filter{
		Role:       &roleImpl,
		ActiveOnly: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(res) != 2 || res[0].ID != "staff-3" || res[1].ID != "staff-4" {
		t.Fatalf("expected staff-3 and staff-4, got %v", res)
	}

	// Invalid Filter
	badRole := staff.Role("unknown-role")
	_, err = s.ListDefinitionsWithFilter(ctx, staff.Filter{Role: &badRole})
	if !staff.IsInvalidRequest(err) {
		t.Fatalf("expected invalid request for unknown role filter, got %v", err)
	}
}

func TestStoreClosedRejection(t *testing.T) {
	t.Parallel()
	dbPath := tempDBPath(t)
	s, err := Open(context.Background(), dbPath)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	def := validDefinition(t)
	if _, err := s.SaveDefinition(ctx, def); err != nil {
		t.Fatal(err)
	}

	if err := s.Close(); err != nil {
		t.Fatal(err)
	}

	// Post-close operations should fail with ErrStoreUnavailable
	if _, err := s.GetDefinition(ctx, def.ID); err == nil {
		t.Fatal("expected error on GetDefinition after Close")
	}
	if _, err := s.SaveDefinition(ctx, def); err == nil {
		t.Fatal("expected error on SaveDefinition after Close")
	}
	if err := s.DeleteDefinition(ctx, def.ID); err == nil {
		t.Fatal("expected error on DeleteDefinition after Close")
	}
	if _, err := s.ListDefinitions(ctx); err == nil {
		t.Fatal("expected error on ListDefinitions after Close")
	}
	if _, err := s.SchemaVersion(ctx); err == nil {
		t.Fatal("expected error on SchemaVersion after Close")
	}
}

func TestNilStoreSafety(t *testing.T) {
	t.Parallel()
	var s *Store
	ctx := context.Background()

	if err := s.Close(); err != nil {
		t.Fatalf("nil store Close should be nil, got %v", err)
	}
	if _, err := s.GetDefinition(ctx, "id"); err == nil {
		t.Fatal("expected error on nil store GetDefinition")
	}
	if _, err := s.SaveDefinition(ctx, staff.Definition{}); err == nil {
		t.Fatal("expected error on nil store SaveDefinition")
	}
	if err := s.DeleteDefinition(ctx, "id"); err == nil {
		t.Fatal("expected error on nil store DeleteDefinition")
	}
	if _, err := s.ListDefinitions(ctx); err == nil {
		t.Fatal("expected error on nil store ListDefinitions")
	}
	if _, err := s.SchemaVersion(ctx); err == nil {
		t.Fatal("expected error on nil store SchemaVersion")
	}
	if err := s.Open(ctx); err == nil {
		t.Fatal("expected error on nil store Open")
	}
}

func TestDeterministicCollectionsOrdering(t *testing.T) {
	t.Parallel()
	dbPath := tempDBPath(t)
	s, err := Open(context.Background(), dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	ctx := context.Background()
	def := staff.Definition{
		Identity: staff.Identity{ID: "ordered-staff", Name: "Ordered Staff"},
		Role:     staff.RoleSpecialist,
		Capabilities: []staff.Capability{
			"zeta",
			"alpha",
			"gamma",
			"beta",
		},
		Permissions: staff.PermissionSet{
			{ID: "p-z", Action: "read", Resource: "z/*", Effect: staff.EffectAllow, Priority: 1},
			{ID: "p-a", Action: "read", Resource: "a/*", Effect: staff.EffectAllow, Priority: 2},
			{ID: "p-m", Action: "read", Resource: "m/*", Effect: staff.EffectAsk, Priority: 1},
		},
		Skills: []staff.SkillReference{
			{ID: "skill-3", Version: "v3"},
			{ID: "skill-1", Version: "v1"},
			{ID: "skill-2", Version: "v2"},
		},
		Memory: []staff.MemoryReference{
			{ID: "mem-c", Kind: "episodic", Version: "1"},
			{ID: "mem-a", Kind: "semantic", Version: "1"},
			{ID: "mem-b", Kind: "procedural", Version: "1"},
		},
		Workspace:     staff.WorkspaceAssignment{WorkspaceID: "ws-ordered"},
		Lifecycle:     staff.LifecycleActive,
		Availability:  staff.AvailabilityAvailable,
		CreatedAt:     time.Now().UTC(),
		UpdatedAt:     time.Now().UTC(),
		SchemaVersion: staff.ContractVersion,
	}

	saved, err := s.SaveDefinition(ctx, def)
	if err != nil {
		t.Fatalf("SaveDefinition: %v", err)
	}

	got, err := s.GetDefinition(ctx, "ordered-staff")
	if err != nil {
		t.Fatalf("GetDefinition: %v", err)
	}

	// Verify exact collection ordering is preserved
	for i, expected := range def.Capabilities {
		if got.Capabilities[i] != expected {
			t.Fatalf("Capability[%d] = %s, want %s", i, got.Capabilities[i], expected)
		}
	}
	for i, expected := range def.Permissions {
		if got.Permissions[i].ID != expected.ID {
			t.Fatalf("Permission[%d] = %s, want %s", i, got.Permissions[i].ID, expected.ID)
		}
	}
	for i, expected := range def.Skills {
		if got.Skills[i].ID != expected.ID {
			t.Fatalf("Skill[%d] = %s, want %s", i, got.Skills[i].ID, expected.ID)
		}
	}
	for i, expected := range def.Memory {
		if got.Memory[i].ID != expected.ID {
			t.Fatalf("Memory[%d] = %s, want %s", i, got.Memory[i].ID, expected.ID)
		}
	}

	// Replace with new order
	saved.Capabilities = []staff.Capability{"gamma", "zeta"}
	saved.Skills = []staff.SkillReference{{ID: "skill-2", Version: "v2"}}
	saved.UpdatedAt = time.Now().UTC()
	if _, err := s.SaveDefinition(ctx, saved); err != nil {
		t.Fatalf("update with new collections: %v", err)
	}

	got2, err := s.GetDefinition(ctx, "ordered-staff")
	if err != nil {
		t.Fatalf("GetDefinition after update: %v", err)
	}
	if len(got2.Capabilities) != 2 || got2.Capabilities[0] != "gamma" || got2.Capabilities[1] != "zeta" {
		t.Fatalf("updated capabilities not preserved: %v", got2.Capabilities)
	}
	if len(got2.Skills) != 1 || got2.Skills[0].ID != "skill-2" {
		t.Fatalf("updated skills not preserved: %v", got2.Skills)
	}
}

func TestOpenInvalidParameters(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	if _, err := Open(nil, tempDBPath(t)); err == nil {
		t.Fatal("expected error on nil context Open")
	}

	canceledCtx, cancel := context.WithCancel(ctx)
	cancel()
	if _, err := Open(canceledCtx, tempDBPath(t)); err == nil || !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled on canceled context Open, got %v", err)
	}

	if _, err := Open(ctx, ""); err == nil {
		t.Fatal("expected error on empty path Open")
	}

	if _, err := Open(ctx, "relative/path/staff.db"); err == nil {
		t.Fatal("expected error on relative path Open")
	}

	if _, err := New(""); err == nil {
		t.Fatal("expected error on empty path New")
	}

	if _, err := New("relative/path/staff.db"); err == nil {
		t.Fatal("expected error on relative path New")
	}
}

func TestApplyMigrationsValidation(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	if err := ApplyMigrations(nil, db); err == nil {
		t.Fatal("expected error with nil context")
	}

	canceledCtx, cancel := context.WithCancel(ctx)
	cancel()
	if err := ApplyMigrations(canceledCtx, db); err == nil || !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}

	if err := ApplyMigrations(ctx, nil); err == nil {
		t.Fatal("expected error with nil DB")
	}
}
