package harness

import (
	"context"
	"errors"
	"testing"
)

func TestRouterBasicRegistration(t *testing.T) {
	router := NewBasicRouter(DefaultRoutingPolicy())
	engine := NewFakeEngine(ToolKindNative, CapabilityShell, CapabilityFileRead)

	desc := ExtendedAdapterDescriptor{
		AdapterDescriptor: engine.Describe(),
		Identity: AdapterIdentity{
			Name:          "test-native",
			InstanceID:    "test-native-1",
			Kind:          ToolKindNative,
			Class:         EngineClassNative,
			Version:       "1.0.0",
			SchemaVersion: HarnessContractVersion,
		},
		ResourceReq: DefaultResourceRequirements(EngineClassNative),
	}

	if err := router.Register(engine, desc); err != nil {
		t.Fatalf("Register: %v", err)
	}

	list := router.List()
	if len(list) != 1 {
		t.Fatalf("expected 1 adapter, got %d", len(list))
	}
}

func TestRouterDuplicateRegistration(t *testing.T) {
	router := NewBasicRouter(DefaultRoutingPolicy())
	engine := NewFakeEngine(ToolKindNative, CapabilityShell)

	desc := ExtendedAdapterDescriptor{
		AdapterDescriptor: engine.Describe(),
		Identity: AdapterIdentity{
			Name:          "test-native",
			InstanceID:    "test-native-1",
			Kind:          ToolKindNative,
			Class:         EngineClassNative,
			Version:       "1.0.0",
			SchemaVersion: HarnessContractVersion,
		},
		ResourceReq: DefaultResourceRequirements(EngineClassNative),
	}

	if err := router.Register(engine, desc); err != nil {
		t.Fatal(err)
	}

	err := router.Register(engine, desc)
	if err == nil || !errors.Is(err, ErrDuplicateTool) {
		t.Fatalf("expected duplicate error, got %v", err)
	}
}

func TestRouterFindByCapabilities(t *testing.T) {
	router := NewBasicRouter(DefaultRoutingPolicy())

	nativeEngine := NewFakeEngine(ToolKindNative, CapabilityShell, CapabilityFileRead)
	router.Register(nativeEngine, ExtendedAdapterDescriptor{
		AdapterDescriptor: nativeEngine.Describe(),
		Identity: AdapterIdentity{
			Name: "native", InstanceID: "native-1", Kind: ToolKindNative,
			Class: EngineClassNative, Version: "1.0.0", SchemaVersion: HarnessContractVersion,
		},
		ResourceReq: DefaultResourceRequirements(EngineClassNative),
	})

	piEngine := NewFakeEngine(ToolKindPi, CapabilityFileRead, CapabilityFileWrite, CapabilityCodeNavigation)
	router.Register(piEngine, ExtendedAdapterDescriptor{
		AdapterDescriptor: piEngine.Describe(),
		Identity: AdapterIdentity{
			Name: "pi", InstanceID: "pi-1", Kind: ToolKindPi,
			Class: EngineClassPi, Version: "1.0.0", SchemaVersion: HarnessContractVersion,
		},
		ResourceReq: DefaultResourceRequirements(EngineClassPi),
	})

	ctx := context.Background()

	selection, err := router.Find(ctx, []ToolCapability{CapabilityShell})
	if err != nil {
		t.Fatalf("Find shell: %v", err)
	}
	if selection.Adapter.Kind() != ToolKindNative {
		t.Fatalf("expected native for shell, got %v", selection.Adapter.Kind())
	}

	selection, err = router.Find(ctx, []ToolCapability{CapabilityCodeNavigation})
	if err != nil {
		t.Fatalf("Find code nav: %v", err)
	}
	if selection.Adapter.Kind() != ToolKindPi {
		t.Fatalf("expected pi for code nav, got %v", selection.Adapter.Kind())
	}

	selection, err = router.Find(ctx, []ToolCapability{CapabilityFileRead})
	if err != nil {
		t.Fatalf("Find file read: %v", err)
	}
	if selection.Adapter.Kind() != ToolKindNative {
		t.Fatalf("expected native for file read (preferred class), got %v", selection.Adapter.Kind())
	}
}

func TestRouterNoEligibleAdapter(t *testing.T) {
	router := NewBasicRouter(DefaultRoutingPolicy())

	engine := NewFakeEngine(ToolKindNative, CapabilityShell)
	router.Register(engine, ExtendedAdapterDescriptor{
		AdapterDescriptor: engine.Describe(),
		Identity: AdapterIdentity{
			Name: "native", InstanceID: "native-1", Kind: ToolKindNative,
			Class: EngineClassNative, Version: "1.0.0", SchemaVersion: HarnessContractVersion,
		},
		ResourceReq: DefaultResourceRequirements(EngineClassNative),
	})

	ctx := context.Background()
	_, err := router.Find(ctx, []ToolCapability{CapabilityCodeNavigation})
	if err == nil {
		t.Fatal("expected no eligible adapter for code navigation")
	}
	if !errors.Is(err, ErrNoEligibleAdapter) {
		t.Fatalf("expected ErrNoEligibleAdapter, got %v", err)
	}
}

func TestRouterPolicyDeniedEngine(t *testing.T) {
	policy := DefaultRoutingPolicy()
	policy.DeniedClasses = []EngineClass{EngineClassPi}
	router := NewBasicRouter(policy)

	nativeEngine := NewFakeEngine(ToolKindNative, CapabilityFileRead)
	router.Register(nativeEngine, ExtendedAdapterDescriptor{
		AdapterDescriptor: nativeEngine.Describe(),
		Identity: AdapterIdentity{
			Name: "native", InstanceID: "native-1", Kind: ToolKindNative,
			Class: EngineClassNative, Version: "1.0.0", SchemaVersion: HarnessContractVersion,
		},
		ResourceReq: DefaultResourceRequirements(EngineClassNative),
	})

	piEngine := NewFakeEngine(ToolKindPi, CapabilityFileRead, CapabilityCodeNavigation)
	router.Register(piEngine, ExtendedAdapterDescriptor{
		AdapterDescriptor: piEngine.Describe(),
		Identity: AdapterIdentity{
			Name: "pi", InstanceID: "pi-1", Kind: ToolKindPi,
			Class: EngineClassPi, Version: "1.0.0", SchemaVersion: HarnessContractVersion,
		},
		ResourceReq: DefaultResourceRequirements(EngineClassPi),
	})

	ctx := context.Background()
	_, err := router.Find(ctx, []ToolCapability{CapabilityCodeNavigation})
	if err == nil {
		t.Fatal("expected policy denied for Pi class")
	}
	if !errors.Is(err, ErrPolicyDeniedEngine) {
		t.Fatalf("expected ErrPolicyDeniedEngine, got %v", err)
	}
}

func TestRouterUnhealthyAdapter(t *testing.T) {
	router := NewBasicRouter(DefaultRoutingPolicy())

	engine := NewFakeEngine(ToolKindNative, CapabilityShell)
	engine.SetHealthError(errors.New("unhealthy"))

	router.Register(engine, ExtendedAdapterDescriptor{
		AdapterDescriptor: engine.Describe(),
		Identity: AdapterIdentity{
			Name: "native", InstanceID: "native-1", Kind: ToolKindNative,
			Class: EngineClassNative, Version: "1.0.0", SchemaVersion: HarnessContractVersion,
		},
		ResourceReq: DefaultResourceRequirements(EngineClassNative),
	})

	ctx := context.Background()
	_, err := router.Find(ctx, []ToolCapability{CapabilityShell})
	if err == nil {
		t.Fatal("expected unhealthy adapter error")
	}
	if !errors.Is(err, ErrAdapterUnhealthy) {
		t.Fatalf("expected ErrAdapterUnhealthy, got %v", err)
	}
}

func TestRouterDeterministicTieBreak(t *testing.T) {
	policy := DefaultRoutingPolicy()
	policy.PreferredClasses = []EngineClass{EngineClassNative}
	router := NewBasicRouter(policy)

	engine1 := NewFakeEngine(ToolKindNative, CapabilityFileRead)
	engine1.SetExecuteFunc(func(ctx ExecutionContext, env ExecutionEnvelope) (ToolResult, error) {
		return ToolResult{Status: ToolStatusSuccess}, nil
	})
	router.Register(engine1, ExtendedAdapterDescriptor{
		AdapterDescriptor: engine1.Describe(),
		Identity: AdapterIdentity{
			Name: "native-a", InstanceID: "native-a-1", Kind: ToolKindNative,
			Class: EngineClassNative, Version: "1.0.0", SchemaVersion: HarnessContractVersion,
		},
		ResourceReq: DefaultResourceRequirements(EngineClassNative),
	})

	engine2 := NewFakeEngine(ToolKindNative, CapabilityFileRead)
	engine2.SetExecuteFunc(func(ctx ExecutionContext, env ExecutionEnvelope) (ToolResult, error) {
		return ToolResult{Status: ToolStatusSuccess}, nil
	})
	router.Register(engine2, ExtendedAdapterDescriptor{
		AdapterDescriptor: engine2.Describe(),
		Identity: AdapterIdentity{
			Name: "native-b", InstanceID: "native-b-1", Kind: ToolKindNative,
			Class: EngineClassNative, Version: "1.0.0", SchemaVersion: HarnessContractVersion,
		},
		ResourceReq: DefaultResourceRequirements(EngineClassNative),
	})

	ctx := context.Background()

	selection1, _ := router.Find(ctx, []ToolCapability{CapabilityFileRead})
	selection2, _ := router.Find(ctx, []ToolCapability{CapabilityFileRead})

	if selection1.TieBreakKey != selection2.TieBreakKey {
		t.Fatalf("deterministic tie-break failed: %s != %s", selection1.TieBreakKey, selection2.TieBreakKey)
	}

	if selection1.TieBreakKey != "native-a|native-a-1" {
		t.Fatalf("expected native-a, got %s", selection1.TieBreakKey)
	}
}

func TestRouterBoundedFallback(t *testing.T) {
	policy := DefaultRoutingPolicy()
	policy.MaxFallbackAttempts = 1
	policy.RequireHealthEnforcement = true
	router := NewBasicRouter(policy)

	healthyEngine := NewFakeEngine(ToolKindNative, CapabilityFileRead)
	router.Register(healthyEngine, ExtendedAdapterDescriptor{
		AdapterDescriptor: healthyEngine.Describe(),
		Identity: AdapterIdentity{
			Name: "healthy", InstanceID: "healthy-1", Kind: ToolKindNative,
			Class: EngineClassNative, Version: "1.0.0", SchemaVersion: HarnessContractVersion,
		},
		ResourceReq: DefaultResourceRequirements(EngineClassNative),
	})

	unhealthyEngine := NewFakeEngine(ToolKindNative, CapabilityFileRead)
	unhealthyEngine.SetHealthError(errors.New("unhealthy"))
	router.Register(unhealthyEngine, ExtendedAdapterDescriptor{
		AdapterDescriptor: unhealthyEngine.Describe(),
		Identity: AdapterIdentity{
			Name: "unhealthy", InstanceID: "unhealthy-1", Kind: ToolKindNative,
			Class: EngineClassNative, Version: "1.0.0", SchemaVersion: HarnessContractVersion,
		},
		ResourceReq: DefaultResourceRequirements(EngineClassNative),
	})

	ctx := context.Background()
	selection, err := router.Find(ctx, []ToolCapability{CapabilityFileRead})
	if err != nil {
		t.Fatalf("Find with fallback: %v", err)
	}
	if selection.Descriptor.Identity.Name != "healthy" {
		t.Fatalf("expected fallback to healthy, got %s", selection.Descriptor.Identity.Name)
	}
	if selection.AttemptedCount != 1 {
		t.Fatalf("expected one health check before selecting the healthy adapter, got %d", selection.AttemptedCount)
	}
}

func TestRouterVersionCompatibility(t *testing.T) {
	router := NewBasicRouter(DefaultRoutingPolicy())

	oldEngine := NewFakeEngine(ToolKindNative, CapabilityFileRead)
	router.Register(oldEngine, ExtendedAdapterDescriptor{
		AdapterDescriptor: oldEngine.Describe(),
		Identity: AdapterIdentity{
			Name: "old", InstanceID: "old-1", Kind: ToolKindNative,
			Class: EngineClassNative, Version: "0.9.0", SchemaVersion: HarnessContractVersion,
		},
		ResourceReq:       DefaultResourceRequirements(EngineClassNative),
		VersionConstraint: VersionConstraint{Minimum: "1.0.0"},
	})

	ctx := context.Background()
	constraints := RoutingConstraints{RequiredAdapterVersion: VersionConstraint{Minimum: "1.0.0"}}
	_, err := router.FindWithConstraints(ctx, []ToolCapability{CapabilityFileRead}, constraints)
	if err == nil {
		t.Fatal("expected version incompatible error")
	}
	if !errors.Is(err, ErrVersionIncompatible) {
		t.Fatalf("expected ErrVersionIncompatible, got %v", err)
	}
}

func TestRouterExcludeClasses(t *testing.T) {
	router := NewBasicRouter(DefaultRoutingPolicy())

	nativeEngine := NewFakeEngine(ToolKindNative, CapabilityFileRead)
	router.Register(nativeEngine, ExtendedAdapterDescriptor{
		AdapterDescriptor: nativeEngine.Describe(),
		Identity: AdapterIdentity{
			Name: "native", InstanceID: "native-1", Kind: ToolKindNative,
			Class: EngineClassNative, Version: "1.0.0", SchemaVersion: HarnessContractVersion,
		},
		ResourceReq: DefaultResourceRequirements(EngineClassNative),
	})

	piEngine := NewFakeEngine(ToolKindPi, CapabilityFileRead, CapabilityCodeNavigation)
	router.Register(piEngine, ExtendedAdapterDescriptor{
		AdapterDescriptor: piEngine.Describe(),
		Identity: AdapterIdentity{
			Name: "pi", InstanceID: "pi-1", Kind: ToolKindPi,
			Class: EngineClassPi, Version: "1.0.0", SchemaVersion: HarnessContractVersion,
		},
		ResourceReq: DefaultResourceRequirements(EngineClassPi),
	})

	ctx := context.Background()
	constraints := RoutingConstraints{ExcludeClasses: []EngineClass{EngineClassNative}}
	selection, err := router.FindWithConstraints(ctx, []ToolCapability{CapabilityFileRead}, constraints)
	if err != nil {
		t.Fatalf("Find with exclude: %v", err)
	}
	if selection.Adapter.Kind() != ToolKindPi {
		t.Fatalf("expected Pi when native excluded, got %v", selection.Adapter.Kind())
	}
}

func TestRouterHealthChecking(t *testing.T) {
	router := NewBasicRouter(DefaultRoutingPolicy())

	engine := NewFakeEngine(ToolKindNative, CapabilityShell)
	router.Register(engine, ExtendedAdapterDescriptor{
		AdapterDescriptor: engine.Describe(),
		Identity: AdapterIdentity{
			Name: "native", InstanceID: "native-1", Kind: ToolKindNative,
			Class: EngineClassNative, Version: "1.0.0", SchemaVersion: HarnessContractVersion,
		},
		ResourceReq: DefaultResourceRequirements(EngineClassNative),
	})

	ctx := context.Background()

	health := router.Health(ctx)
	if len(health) != 1 {
		t.Fatalf("expected 1 health entry, got %d", len(health))
	}

	check := router.CheckHealth(ctx, "native")
	if !check.Healthy {
		t.Fatalf("expected healthy, got unhealthy: %s", check.Reason)
	}
	if check.CheckCount != 2 {
		t.Fatalf("expected check count 2 after Health() call, got %d", check.CheckCount)
	}

	engine.SetHealthError(errors.New("failed"))
	check = router.CheckHealth(ctx, "native")
	if check.Healthy {
		t.Fatal("expected unhealthy after error")
	}
	if check.FailCount != 1 {
		t.Fatalf("expected fail count 1, got %d", check.FailCount)
	}
	if check.ConsecutiveFails != 1 {
		t.Fatalf("expected consecutive fails 1, got %d", check.ConsecutiveFails)
	}

	engine.SetHealthError(nil)
	check = router.CheckHealth(ctx, "native")
	if !check.Healthy {
		t.Fatal("expected healthy after recovery")
	}
	if check.ConsecutiveFails != 0 {
		t.Fatalf("expected consecutive fails reset to 0, got %d", check.ConsecutiveFails)
	}
}

func TestRouterAllowedAdapters(t *testing.T) {
	policy := DefaultRoutingPolicy()
	policy.AllowedAdapters = []string{"allowed-engine"}
	router := NewBasicRouter(policy)

	allowed := NewFakeEngine(ToolKindNative, CapabilityFileRead)
	router.Register(allowed, ExtendedAdapterDescriptor{
		AdapterDescriptor: allowed.Describe(),
		Identity: AdapterIdentity{
			Name: "allowed-engine", InstanceID: "allowed-1", Kind: ToolKindNative,
			Class: EngineClassNative, Version: "1.0.0", SchemaVersion: HarnessContractVersion,
		},
		ResourceReq: DefaultResourceRequirements(EngineClassNative),
	})

	denied := NewFakeEngine(ToolKindNative, CapabilityFileRead)
	router.Register(denied, ExtendedAdapterDescriptor{
		AdapterDescriptor: denied.Describe(),
		Identity: AdapterIdentity{
			Name: "denied-engine", InstanceID: "denied-1", Kind: ToolKindNative,
			Class: EngineClassNative, Version: "1.0.0", SchemaVersion: HarnessContractVersion,
		},
		ResourceReq: DefaultResourceRequirements(EngineClassNative),
	})

	ctx := context.Background()
	selection, err := router.Find(ctx, []ToolCapability{CapabilityFileRead})
	if err != nil {
		t.Fatalf("Find with allowed list: %v", err)
	}
	if selection.Descriptor.Identity.Name != "allowed-engine" {
		t.Fatalf("expected allowed-engine, got %s", selection.Descriptor.Identity.Name)
	}
}

func TestRouterMinMemoryConstraint(t *testing.T) {
	router := NewBasicRouter(DefaultRoutingPolicy())

	smallEngine := NewFakeEngine(ToolKindNative, CapabilityFileRead)
	router.Register(smallEngine, ExtendedAdapterDescriptor{
		AdapterDescriptor: smallEngine.Describe(),
		Identity: AdapterIdentity{
			Name: "small", InstanceID: "small-1", Kind: ToolKindNative,
			Class: EngineClassNative, Version: "1.0.0", SchemaVersion: HarnessContractVersion,
		},
		ResourceReq: ResourceRequirement{MinMemoryMB: 50, PeakMemoryMB: 100},
	})

	largeEngine := NewFakeEngine(ToolKindPi, CapabilityFileRead)
	router.Register(largeEngine, ExtendedAdapterDescriptor{
		AdapterDescriptor: largeEngine.Describe(),
		Identity: AdapterIdentity{
			Name: "large", InstanceID: "large-1", Kind: ToolKindPi,
			Class: EngineClassPi, Version: "1.0.0", SchemaVersion: HarnessContractVersion,
		},
		ResourceReq: ResourceRequirement{MinMemoryMB: 200, PeakMemoryMB: 500},
	})

	ctx := context.Background()
	constraints := RoutingConstraints{MinMemoryMB: 300}
	selection, err := router.FindWithConstraints(ctx, []ToolCapability{CapabilityFileRead}, constraints)
	if err != nil {
		t.Fatalf("Find with min memory: %v", err)
	}
	if selection.Descriptor.Identity.Name != "large" {
		t.Fatalf("expected large engine for 300MB min, got %s", selection.Descriptor.Identity.Name)
	}
}

func TestRouterPreferInstanceID(t *testing.T) {
	router := NewBasicRouter(DefaultRoutingPolicy())

	engine1 := NewFakeEngine(ToolKindNative, CapabilityFileRead)
	router.Register(engine1, ExtendedAdapterDescriptor{
		AdapterDescriptor: engine1.Describe(),
		Identity: AdapterIdentity{
			Name: "native", InstanceID: "instance-1", Kind: ToolKindNative,
			Class: EngineClassNative, Version: "1.0.0", SchemaVersion: HarnessContractVersion,
		},
		ResourceReq: DefaultResourceRequirements(EngineClassNative),
	})

	engine2 := NewFakeEngine(ToolKindNative, CapabilityFileRead)
	router.Register(engine2, ExtendedAdapterDescriptor{
		AdapterDescriptor: engine2.Describe(),
		Identity: AdapterIdentity{
			Name: "native", InstanceID: "instance-2", Kind: ToolKindNative,
			Class: EngineClassNative, Version: "1.0.0", SchemaVersion: HarnessContractVersion,
		},
		ResourceReq: DefaultResourceRequirements(EngineClassNative),
	})

	ctx := context.Background()
	constraints := RoutingConstraints{PreferInstanceID: "instance-2"}
	selection, err := router.FindWithConstraints(ctx, []ToolCapability{CapabilityFileRead}, constraints)
	if err != nil {
		t.Fatalf("Find with prefer instance: %v", err)
	}
	if selection.Descriptor.Identity.InstanceID != "instance-2" {
		t.Fatalf("expected instance-2, got %s", selection.Descriptor.Identity.InstanceID)
	}
}

func TestRouterSchemaVersionConstraint(t *testing.T) {
	router := NewBasicRouter(DefaultRoutingPolicy())

	engine := NewFakeEngine(ToolKindNative, CapabilityFileRead)
	router.Register(engine, ExtendedAdapterDescriptor{
		AdapterDescriptor: engine.Describe(),
		Identity: AdapterIdentity{
			Name: "native", InstanceID: "native-1", Kind: ToolKindNative,
			Class: EngineClassNative, Version: "1.0.0", SchemaVersion: "other.version",
		},
		ResourceReq: DefaultResourceRequirements(EngineClassNative),
	})

	ctx := context.Background()
	constraints := RoutingConstraints{RequiredSchemaVersion: HarnessContractVersion}
	_, err := router.FindWithConstraints(ctx, []ToolCapability{CapabilityFileRead}, constraints)
	if err == nil {
		t.Fatal("expected no eligible adapter for schema mismatch")
	}
	if !errors.Is(err, ErrNoEligibleAdapter) {
		t.Fatalf("expected ErrNoEligibleAdapter, got %v", err)
	}
}

func TestRouterRetryOrder(t *testing.T) {
	router := NewBasicRouter(DefaultRoutingPolicy())

	engine1 := NewFakeEngine(ToolKindNative, CapabilityFileRead)
	router.Register(engine1, ExtendedAdapterDescriptor{
		AdapterDescriptor: engine1.Describe(),
		Identity: AdapterIdentity{
			Name: "first", InstanceID: "first-1", Kind: ToolKindNative,
			Class: EngineClassNative, Version: "1.0.0", SchemaVersion: HarnessContractVersion,
		},
		ResourceReq: DefaultResourceRequirements(EngineClassNative),
	})

	engine2 := NewFakeEngine(ToolKindPi, CapabilityFileRead)
	router.Register(engine2, ExtendedAdapterDescriptor{
		AdapterDescriptor: engine2.Describe(),
		Identity: AdapterIdentity{
			Name: "second", InstanceID: "second-1", Kind: ToolKindPi,
			Class: EngineClassPi, Version: "1.0.0", SchemaVersion: HarnessContractVersion,
		},
		ResourceReq: DefaultResourceRequirements(EngineClassPi),
	})

	ctx := context.Background()
	selection, _ := router.Find(ctx, []ToolCapability{CapabilityFileRead})

	if len(selection.RetryOrder) != 1 {
		t.Fatalf("expected 1 retry alternative, got %d", len(selection.RetryOrder))
	}
	if selection.RetryOrder[0] != "second|second-1" {
		t.Fatalf("expected second as retry, got %s", selection.RetryOrder[0])
	}
}

func TestRouterHealthEnforcementDisabled(t *testing.T) {
	policy := DefaultRoutingPolicy()
	policy.RequireHealthEnforcement = false
	router := NewBasicRouter(policy)

	engine := NewFakeEngine(ToolKindNative, CapabilityFileRead)
	engine.SetHealthError(errors.New("unhealthy"))
	router.Register(engine, ExtendedAdapterDescriptor{
		AdapterDescriptor: engine.Describe(),
		Identity: AdapterIdentity{
			Name: "native", InstanceID: "native-1", Kind: ToolKindNative,
			Class: EngineClassNative, Version: "1.0.0", SchemaVersion: HarnessContractVersion,
		},
		ResourceReq: DefaultResourceRequirements(EngineClassNative),
	})

	ctx := context.Background()
	selection, err := router.Find(ctx, []ToolCapability{CapabilityFileRead})
	if err != nil {
		t.Fatalf("Find with health disabled: %v", err)
	}
	if selection.Adapter.Kind() != ToolKindNative {
		t.Fatalf("expected native adapter despite unhealthy, got %v", selection.Adapter.Kind())
	}
	if selection.Reason != "eligible adapter; health enforcement disabled" {
		t.Fatalf("unexpected reason: %s", selection.Reason)
	}
}

func TestRouterMaxFallbackAttemptsZero(t *testing.T) {
	policy := DefaultRoutingPolicy()
	policy.MaxFallbackAttempts = 0
	policy.RequireHealthEnforcement = true
	router := NewBasicRouter(policy)

	unhealthy1 := NewFakeEngine(ToolKindNative, CapabilityFileRead)
	unhealthy1.SetHealthError(errors.New("unhealthy"))
	router.Register(unhealthy1, ExtendedAdapterDescriptor{
		AdapterDescriptor: unhealthy1.Describe(),
		Identity: AdapterIdentity{
			Name: "unhealthy1", InstanceID: "unhealthy1-1", Kind: ToolKindNative,
			Class: EngineClassNative, Version: "1.0.0", SchemaVersion: HarnessContractVersion,
		},
		ResourceReq: DefaultResourceRequirements(EngineClassNative),
	})

	unhealthy2 := NewFakeEngine(ToolKindNative, CapabilityFileRead)
	unhealthy2.SetHealthError(errors.New("unhealthy"))
	router.Register(unhealthy2, ExtendedAdapterDescriptor{
		AdapterDescriptor: unhealthy2.Describe(),
		Identity: AdapterIdentity{
			Name: "unhealthy2", InstanceID: "unhealthy2-1", Kind: ToolKindNative,
			Class: EngineClassNative, Version: "1.0.0", SchemaVersion: HarnessContractVersion,
		},
		ResourceReq: DefaultResourceRequirements(EngineClassNative),
	})

	ctx := context.Background()
	_, err := router.Find(ctx, []ToolCapability{CapabilityFileRead})
	if err == nil {
		t.Fatal("expected bounded fallback error with zero max attempts")
	}
	if !errors.Is(err, ErrAdapterUnhealthy) {
		t.Fatalf("expected ErrAdapterUnhealthy, got %v", err)
	}
}
