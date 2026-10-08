package staff

import (
	"context"
	"errors"
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/RahmatHadinata23758051/CortexOS/internal/cortex/harness"
)

// fakeEngineAdapter implements harness.EngineAdapter for testing.
type fakeEngineAdapter struct {
	kind         harness.ToolKind
	caps         []harness.ToolCapability
	name         string
	version      string
	healthErr    error
	executed     bool
	lastEnvelope harness.ExecutionEnvelope
}

func newFakeAdapter(name string, kind harness.ToolKind, caps ...harness.ToolCapability) *fakeEngineAdapter {
	return &fakeEngineAdapter{
		name:    name,
		kind:    kind,
		caps:    caps,
		version: "1.0.0",
	}
}

func (a *fakeEngineAdapter) Kind() harness.ToolKind { return a.kind }
func (a *fakeEngineAdapter) Capabilities() []harness.ToolCapability {
	return append([]harness.ToolCapability(nil), a.caps...)
}
func (a *fakeEngineAdapter) Execute(ctx harness.ExecutionContext, envelope harness.ExecutionEnvelope) (harness.ToolResult, error) {
	a.executed = true
	a.lastEnvelope = envelope
	return harness.ToolResult{
		ContractVersion: harness.HarnessContractVersion,
		ExecutionID:     envelope.ExecutionID,
		TaskID:          envelope.TaskID,
		ToolName:        envelope.ToolName,
		Status:          harness.ToolStatusSuccess,
	}, nil
}
func (a *fakeEngineAdapter) Health(ctx harness.ExecutionContext) error {
	return a.healthErr
}
func (a *fakeEngineAdapter) Describe() harness.AdapterDescriptor {
	return harness.AdapterDescriptor{
		Name:          a.name,
		Kind:          a.kind,
		Version:       a.version,
		Capabilities:  a.caps,
		SchemaVersion: harness.HarnessContractVersion,
	}
}

func setupTestRouter(t *testing.T) (*StaffEngineRouter, Store, *harness.BasicRouter) {
	t.Helper()

	hRouter := harness.NewBasicRouter(harness.DefaultRoutingPolicy())

	// Register Native adapter
	native := newFakeAdapter("native-worker-1", harness.ToolKindNative,
		harness.CapabilityShell, harness.CapabilityFileRead, harness.CapabilityFileWrite,
		harness.CapabilityFileEdit, harness.CapabilityGit, harness.CapabilitySearch,
		harness.CapabilityTaskPlan, harness.CapabilityCodeNavigation, harness.CapabilityTestRun,
		harness.CapabilityLint, harness.CapabilityBuild)
	err := hRouter.Register(native, harness.ExtendedAdapterDescriptor{
		AdapterDescriptor: native.Describe(),
		Identity: harness.AdapterIdentity{
			Name:          native.name,
			InstanceID:    "native-inst-1",
			Kind:          native.kind,
			Class:         harness.EngineClassNative,
			Version:       native.version,
			SchemaVersion: harness.HarnessContractVersion,
		},
		ResourceReq: harness.DefaultResourceRequirements(harness.EngineClassNative),
	})
	if err != nil {
		t.Fatalf("failed to register native adapter: %v", err)
	}

	// Register Pi adapter
	pi := newFakeAdapter("pi-worker-1", harness.ToolKindPi,
		harness.CapabilityCoding, harness.CapabilityAnalysis, harness.CapabilityRefactor,
		harness.CapabilityDebug, harness.CapabilityReview, harness.CapabilityTestGen,
		harness.CapabilityDocGen)
	err = hRouter.Register(pi, harness.ExtendedAdapterDescriptor{
		AdapterDescriptor: pi.Describe(),
		Identity: harness.AdapterIdentity{
			Name:          pi.name,
			InstanceID:    "pi-inst-1",
			Kind:          pi.kind,
			Class:         harness.EngineClassPi,
			Version:       pi.version,
			SchemaVersion: harness.HarnessContractVersion,
		},
		ResourceReq: harness.DefaultResourceRequirements(harness.EngineClassPi),
	})
	if err != nil {
		t.Fatalf("failed to register pi adapter: %v", err)
	}

	// Register OMP adapter
	omp := newFakeAdapter("omp-worker-1", harness.ToolKindOMP,
		harness.CapabilitySpecialist, harness.CapabilityRecovery,
		harness.CapabilitySecurityAudit, harness.CapabilityDeepDebug,
		harness.CapabilityArchitectureReview)
	err = hRouter.Register(omp, harness.ExtendedAdapterDescriptor{
		AdapterDescriptor: omp.Describe(),
		Identity: harness.AdapterIdentity{
			Name:          omp.name,
			InstanceID:    "omp-inst-1",
			Kind:          omp.kind,
			Class:         harness.EngineClassOMP,
			Version:       omp.version,
			SchemaVersion: harness.HarnessContractVersion,
		},
		ResourceReq: harness.DefaultResourceRequirements(harness.EngineClassOMP),
	})
	if err != nil {
		t.Fatalf("failed to register omp adapter: %v", err)
	}

	registry := DefaultCapabilityRegistry()
	memStore := newMemStore()
	router, err := NewStaffEngineRouter(memStore, hRouter, registry)
	if err != nil {
		t.Fatalf("failed to create staff engine router: %v", err)
	}

	return router, memStore, hRouter
}

// memStore is a thread-safe in-memory implementation of staff.Store for testing.
type memStore struct {
	mu   sync.RWMutex
	data map[StaffID]Definition
}

func newMemStore() *memStore {
	return &memStore{data: make(map[StaffID]Definition)}
}

func (s *memStore) Open(ctx context.Context) error {
	return nil
}

func (s *memStore) Close() error {
	return nil
}

func (s *memStore) SchemaVersion(ctx context.Context) (string, error) {
	return ContractVersion, nil
}

func (s *memStore) SaveDefinition(ctx context.Context, def Definition) (Definition, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data[def.ID] = def
	return def, nil
}

func (s *memStore) GetDefinition(ctx context.Context, id StaffID) (Definition, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	def, ok := s.data[id]
	if !ok {
		return Definition{}, WrapError(ErrNotFound, "staff definition not found", nil)
	}
	return def, nil
}

func (s *memStore) DeleteDefinition(ctx context.Context, id StaffID) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.data[id]; !ok {
		return WrapError(ErrNotFound, "staff definition not found", nil)
	}
	delete(s.data, id)
	return nil
}

func (s *memStore) ListDefinitions(ctx context.Context) ([]Definition, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]Definition, 0, len(s.data))
	for _, def := range s.data {
		result = append(result, def)
	}
	sort.Slice(result, func(i, j int) bool { return string(result[i].ID) < string(result[j].ID) })
	return result, nil
}

func (s *memStore) ListDefinitionsByWorkspace(ctx context.Context, id WorkspaceID) ([]Definition, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]Definition, 0)
	for _, def := range s.data {
		if def.Workspace.WorkspaceID == id {
			result = append(result, def)
		}
	}
	sort.Slice(result, func(i, j int) bool { return string(result[i].ID) < string(result[j].ID) })
	return result, nil
}

func (s *memStore) ListDefinitionsByProject(ctx context.Context, id ProjectID) ([]Definition, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]Definition, 0)
	for _, def := range s.data {
		if def.Workspace.ProjectID == id {
			result = append(result, def)
		}
	}
	sort.Slice(result, func(i, j int) bool { return string(result[i].ID) < string(result[j].ID) })
	return result, nil
}

func (s *memStore) ListDefinitionsByRole(ctx context.Context, role Role) ([]Definition, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]Definition, 0)
	for _, def := range s.data {
		if def.Role == role {
			result = append(result, def)
		}
	}
	sort.Slice(result, func(i, j int) bool { return string(result[i].ID) < string(result[j].ID) })
	return result, nil
}

func createStaff(t *testing.T, store Store, id string, role Role, caps []Capability, perms PermissionSet, ws string, avail AvailabilityState) Definition {
	t.Helper()
	now := time.Now().UTC()
	def, err := NewDefinition(Definition{
		Identity:      Identity{ID: StaffID(id), Name: "Staff " + id},
		Role:          role,
		Capabilities:  caps,
		Permissions:   perms,
		Workspace:     WorkspaceAssignment{WorkspaceID: WorkspaceID(ws)},
		Lifecycle:     LifecycleActive,
		Availability:  avail,
		CreatedAt:     now,
		UpdatedAt:     now,
		SchemaVersion: ContractVersion,
	})
	if err != nil {
		t.Fatalf("invalid staff definition: %v", err)
	}
	saved, err := store.SaveDefinition(context.Background(), def)
	if err != nil {
		t.Fatalf("failed to save staff definition: %v", err)
	}
	return saved
}

func TestSelectCandidates(t *testing.T) {
	router, store, _ := setupTestRouter(t)
	ctx := context.Background()

	// Seed Staff members
	createStaff(t, store, "staff-coder-1", RoleImplementer, []Capability{"coding", "file_read", "file_write"},
		PermissionSet{{ID: "p1", Action: "read", Resource: "src/*", Effect: EffectAllow}}, "ws-alpha", AvailabilityAvailable)
	createStaff(t, store, "staff-coder-2", RoleImplementer, []Capability{"coding", "file_read", "file_write"},
		PermissionSet{{ID: "p1", Action: "read", Resource: "src/*", Effect: EffectAllow}}, "ws-alpha", AvailabilityBusy)
	createStaff(t, store, "staff-reviewer-1", RoleReviewer, []Capability{"review", "file_read"},
		PermissionSet{{ID: "p1", Action: "read", Resource: "src/*", Effect: EffectAllow}}, "ws-alpha", AvailabilityAvailable)
	createStaff(t, store, "staff-other-ws", RoleImplementer, []Capability{"coding"},
		PermissionSet{{ID: "p1", Action: "read", Resource: "src/*", Effect: EffectAllow}}, "ws-beta", AvailabilityAvailable)

	// Test 1: Find coders in ws-alpha
	candidates, err := router.SelectCandidates(ctx, RouterRequest{
		WorkspaceID:          "ws-alpha",
		RequiredCapabilities: []Capability{"coding"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(candidates) != 2 {
		t.Fatalf("expected 2 candidates, got %d", len(candidates))
	}
	// Available should be prioritized over Busy
	if candidates[0].ID != "staff-coder-1" {
		t.Errorf("expected available coder first, got %s", candidates[0].ID)
	}
	if candidates[1].ID != "staff-coder-2" {
		t.Errorf("expected busy coder second, got %s", candidates[1].ID)
	}

	// Test 2: Role hint prioritization
	candidates, err = router.SelectCandidates(ctx, RouterRequest{
		WorkspaceID:          "ws-alpha",
		RoleHint:             RoleReviewer,
		RequiredCapabilities: []Capability{"file_read"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(candidates) != 3 {
		t.Fatalf("expected 3 candidates with file_read, got %d", len(candidates))
	}
	// RoleReviewer should come first because of RoleHint
	if candidates[0].Role != RoleReviewer || candidates[0].ID != "staff-reviewer-1" {
		t.Errorf("expected reviewer first due to role hint, got %s (%s)", candidates[0].ID, candidates[0].Role)
	}

	// Test 3: Exclude IDs
	candidates, err = router.SelectCandidates(ctx, RouterRequest{
		WorkspaceID:          "ws-alpha",
		RequiredCapabilities: []Capability{"coding"},
		ExcludeIDs:           []StaffID{"staff-coder-1"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(candidates) != 1 || candidates[0].ID != "staff-coder-2" {
		t.Errorf("expected only staff-coder-2 after exclusion, got %v", candidates)
	}
}

func TestRouteTaskMatrix(t *testing.T) {
	router, store, _ := setupTestRouter(t)
	ctx := context.Background()

	// Implementer for Pi coding
	createStaff(t, store, "staff-dev", RoleImplementer, []Capability{"coding", "file_edit"},
		PermissionSet{
			{ID: "allow-src", Action: "edit", Resource: "src/*", Effect: EffectAllow, Priority: 10},
			{ID: "deny-secret", Action: "edit", Resource: "secrets/*", Effect: EffectDeny, Priority: 20},
		}, "ws-prod", AvailabilityAvailable)

	// Specialist for OMP security audit
	createStaff(t, store, "staff-sec", RoleSpecialist, []Capability{"security_audit", "specialist"},
		PermissionSet{
			{ID: "allow-all", Action: "audit", Resource: "*", Effect: EffectAllow, Priority: 10},
		}, "ws-prod", AvailabilityAvailable)

	// Case 1: Route coding task -> should select staff-dev and Pi engine adapter
	decision, err := router.RouteTask(ctx, TaskRouteRequest{
		TaskID:               "task-101",
		WorkspaceID:          "ws-prod",
		RequiredCapabilities: []Capability{"coding"},
		Action:               "edit",
		Resource:             "src/main.go",
	})
	if err != nil {
		t.Fatalf("RouteTask failed: %v", err)
	}
	if decision.StaffID != "staff-dev" {
		t.Errorf("expected staff-dev, got %s", decision.StaffID)
	}
	if decision.EngineClass != harness.EngineClassPi {
		t.Errorf("expected Pi engine class for coding, got %s", decision.EngineClass)
	}
	if !decision.PolicyDecision.Allowed {
		t.Errorf("expected policy allowed, got %+v", decision.PolicyDecision)
	}
	if decision.DeterministicRouteKey == "" {
		t.Errorf("expected non-empty deterministic route key")
	}

	// Case 2: Policy denial -> should fail with ErrPermissionDenied
	_, err = router.RouteTask(ctx, TaskRouteRequest{
		TaskID:               "task-102",
		WorkspaceID:          "ws-prod",
		RequiredCapabilities: []Capability{"coding"},
		Action:               "edit",
		Resource:             "secrets/key.pem",
	})
	if err == nil {
		t.Fatal("expected permission denied error, got nil")
	}
	if !IsPermissionDenied(err) {
		t.Errorf("expected ErrPermissionDenied, got: %v", err)
	}

	// Case 3: Route security audit -> should select staff-sec and OMP engine adapter
	secDecision, err := router.RouteTask(ctx, TaskRouteRequest{
		TaskID:               "task-103",
		WorkspaceID:          "ws-prod",
		RequiredCapabilities: []Capability{"security_audit"},
		Action:               "audit",
		Resource:             "src/auth.go",
	})
	if err != nil {
		t.Fatalf("RouteTask security audit failed: %v", err)
	}
	if secDecision.StaffID != "staff-sec" {
		t.Errorf("expected staff-sec, got %s", secDecision.StaffID)
	}
	if secDecision.EngineClass != harness.EngineClassOMP {
		t.Errorf("expected OMP engine class for security_audit, got %s", secDecision.EngineClass)
	}

	// Case 4: Context cancellation
	canceledCtx, cancel := context.WithCancel(ctx)
	cancel()
	_, err = router.RouteTask(canceledCtx, TaskRouteRequest{
		TaskID:               "task-104",
		WorkspaceID:          "ws-prod",
		RequiredCapabilities: []Capability{"coding"},
	})
	if !errors.Is(err, context.Canceled) && ErrorCodeOf(err) != ErrCanceled {
		t.Errorf("expected canceled error, got: %v", err)
	}
}

func TestRouteTaskSpecificStaffIDValidation(t *testing.T) {
	router, store, _ := setupTestRouter(t)
	ctx := context.Background()

	// Inactive staff
	now := time.Now().UTC()
	inactiveDef, _ := NewDefinition(Definition{
		Identity:      Identity{ID: "staff-inactive", Name: "Staff Inactive"},
		Role:          RoleImplementer,
		Capabilities:  []Capability{"coding"},
		Permissions:   PermissionSet{{ID: "p1", Action: "read", Resource: "*", Effect: EffectAllow}},
		Workspace:     WorkspaceAssignment{WorkspaceID: "ws-test"},
		Lifecycle:     LifecycleInactive,
		Availability:  AvailabilityAvailable,
		CreatedAt:     now,
		UpdatedAt:     now,
		SchemaVersion: ContractVersion,
	})
	_, _ = store.SaveDefinition(ctx, inactiveDef)

	_, err := router.RouteTask(ctx, TaskRouteRequest{
		TaskID:               "task-201",
		WorkspaceID:          "ws-test",
		StaffID:              "staff-inactive",
		RequiredCapabilities: []Capability{"coding"},
	})
	if err == nil || ErrorCodeOf(err) != ErrInactiveStaff {
		t.Errorf("expected ErrInactiveStaff, got %v", err)
	}

	// Non-existent staff
	_, err = router.RouteTask(ctx, TaskRouteRequest{
		TaskID:               "task-202",
		WorkspaceID:          "ws-test",
		StaffID:              "staff-nonexistent",
		RequiredCapabilities: []Capability{"coding"},
	})
	if err == nil || !IsNotFound(err) {
		t.Errorf("expected ErrNotFound for non-existent staff, got %v", err)
	}
}
