package platform

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/RahmatHadinata23758051/CortexOS/internal/cortex/application"
	"github.com/RahmatHadinata23758051/CortexOS/internal/cortex/workspace"
)

func TestEventEmitterRegisterAndEmit(t *testing.T) {
	t.Parallel()

	emitter := NewEventEmitter()
	var receivedEvent string
	var receivedPayload any
	var mu sync.Mutex

	unsubscribe := emitter.RegisterHook(func(eventName string, payload any) {
		mu.Lock()
		receivedEvent = eventName
		receivedPayload = payload
		mu.Unlock()
	})

	emitter.Emit("test.event", "test-payload")
	time.Sleep(10 * time.Millisecond)

	mu.Lock()
	if receivedEvent != "test.event" {
		mu.Unlock()
		t.Fatalf("expected event 'test.event', got %q", receivedEvent)
	}
	if receivedPayload != "test-payload" {
		mu.Unlock()
		t.Fatalf("expected payload 'test-payload', got %v", receivedPayload)
	}
	mu.Unlock()

	// Unsubscribe and verify no more events
	unsubscribe()
	emitter.Emit("test.event2", "test-payload2")
	time.Sleep(10 * time.Millisecond)

	mu.Lock()
	if receivedEvent != "test.event" {
		mu.Unlock()
		t.Fatalf("event should not have changed after unsubscribe, got %q", receivedEvent)
	}
	mu.Unlock()
}

func TestEventEmitterContextAware(t *testing.T) {
	t.Parallel()

	emitter := NewEventEmitter()
	var emittedEvents []string

	emitter.RegisterHook(func(eventName string, payload any) {
		emittedEvents = append(emittedEvents, eventName)
	})

	// Without Wails context, only hooks fire
	emitter.Emit("event.a", nil)
	emitter.Emit("event.b", nil)
	time.Sleep(10 * time.Millisecond)

	if len(emittedEvents) != 2 || emittedEvents[0] != "event.a" || emittedEvents[1] != "event.b" {
		t.Fatalf("expected 2 hook events, got %v", emittedEvents)
	}
}

func TestEventEmitterSetContext(t *testing.T) {
	t.Parallel()

	emitter := NewEventEmitter()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	emitter.SetContext(ctx)

	if emitter.GetContext() != ctx {
		t.Fatal("context not set")
	}

	emitter.SetContext(nil)
	got := emitter.GetContext()
	if got == ctx {
		t.Fatal("context should be background after nil, not cancelled ctx")
	}
}

func TestCockpitBridgeEventHooks(t *testing.T) {
	t.Parallel()

	memory := workspace.NewMemoryWorkspace()
	service, err := workspace.NewService(workspace.Dependencies{
		State: memory, Projects: memory, Worktrees: memory, Retrieval: memory,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := memory.RegisterProject(context.Background(), workspace.Project{
		ID:             "project-1",
		Name:           "Project",
		RepositoryRoot: "C:/Projects/project-1",
		VaultRoot:      "C:/Projects/project-1/vault",
	}); err != nil {
		t.Fatal(err)
	}

	appService := application.NewServiceWithWorkspace(service)
	cockpitBridge := NewCockpitBridge(appService)

	var events []string
	unsubscribe := cockpitBridge.RegisterEventHook(func(eventName string, payload any) {
		events = append(events, eventName)
	})

	_, err = cockpitBridge.GetCockpitRuntime()
	if err != nil {
		t.Fatal(err)
	}
	_, err = cockpitBridge.GetCockpitWorkspace(application.CockpitWorkspaceRequest{
		SchemaVersion: application.CockpitBridgeSchemaVersion,
	})
	if err != nil {
		t.Fatal(err)
	}

	project, err := cockpitBridge.RegisterCockpitProject(application.CockpitProjectRequest{
		SchemaVersion:  application.CockpitBridgeSchemaVersion,
		ID:             "project-2",
		Name:           "Project Two",
		RepositoryRoot: "C:/Projects/project-2",
		VaultRoot:      "C:/Projects/project-2/vault",
	})
	if err != nil {
		t.Fatal(err)
	}
	if project.ID != "project-2" {
		t.Fatalf("project = %#v", project)
	}

	err = cockpitBridge.RebuildCockpitRetrieval(application.CockpitMutationRequest{
		SchemaVersion: application.CockpitBridgeSchemaVersion,
		ProjectID:     "project-1",
	})
	if err != nil {
		t.Fatal(err)
	}

	time.Sleep(10 * time.Millisecond)

	expectedEvents := []string{
		EventCockpitRuntimeChanged,
		EventCockpitWorkspaceChanged,
		EventCockpitProjectRegistered,
		EventCockpitRetrievalRebuilt,
	}
	if len(events) != len(expectedEvents) {
		t.Fatalf("expected %d events, got %d: %v", len(expectedEvents), len(events), events)
	}
	for i, exp := range expectedEvents {
		if events[i] != exp {
			t.Fatalf("event %d: expected %q, got %q", i, exp, events[i])
		}
	}

	// Verify unsubscribe works
	unsubscribe()
	_, _ = cockpitBridge.GetCockpitRuntime()
	time.Sleep(10 * time.Millisecond)
	if len(events) != len(expectedEvents) {
		t.Fatalf("events should not grow after unsubscribe, got %v", events)
	}
}

func TestBridgeEventHooks(t *testing.T) {
	t.Parallel()

	memory := workspace.NewMemoryWorkspace()
	service, err := workspace.NewService(workspace.Dependencies{
		State: memory, Projects: memory, Worktrees: memory, Retrieval: memory,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := memory.RegisterProject(context.Background(), workspace.Project{
		ID:             "project-1",
		Name:           "Project",
		RepositoryRoot: "C:/Projects/project-1",
		VaultRoot:      "C:/Projects/project-1/vault",
	}); err != nil {
		t.Fatal(err)
	}

	appService := application.NewServiceWithWorkspace(service)
	bridge := NewBridge(appService)

	var events []string
	unsubscribe := bridge.RegisterEventHook(func(eventName string, payload any) {
		events = append(events, eventName)
	})

	_, _ = bridge.GetCockpitRuntime()
	_, _ = bridge.GetCockpitWorkspace(application.CockpitWorkspaceRequest{
		SchemaVersion: application.CockpitBridgeSchemaVersion,
	})
	_, _ = bridge.RegisterCockpitProject(application.CockpitProjectRequest{
		SchemaVersion:  application.CockpitBridgeSchemaVersion,
		ID:             "project-2",
		Name:           "Project Two",
		RepositoryRoot: "C:/Projects/project-2",
		VaultRoot:      "C:/Projects/project-2/vault",
	})
	_ = bridge.RebuildCockpitRetrieval(application.CockpitMutationRequest{
		SchemaVersion: application.CockpitBridgeSchemaVersion,
		ProjectID:     "project-1",
	})

	time.Sleep(10 * time.Millisecond)

	expectedEvents := []string{
		EventCockpitRuntimeChanged,
		EventCockpitWorkspaceChanged,
		EventCockpitProjectRegistered,
		EventCockpitRetrievalRebuilt,
	}
	if len(events) != len(expectedEvents) {
		t.Fatalf("expected %d events, got %d: %v", len(expectedEvents), len(events), events)
	}
	for i, exp := range expectedEvents {
		if events[i] != exp {
			t.Fatalf("event %d: expected %q, got %q", i, exp, events[i])
		}
	}

	unsubscribe()
	_, _ = bridge.GetCockpitRuntime()
	time.Sleep(10 * time.Millisecond)
	if len(events) != len(expectedEvents) {
		t.Fatalf("events should not grow after unsubscribe, got %v", events)
	}
}

func TestEmitEventManual(t *testing.T) {
	t.Parallel()

	emitter := NewEventEmitter()
	var received []string
	emitter.RegisterHook(func(eventName string, payload any) {
		received = append(received, eventName)
	})
	emitter.Emit("manual.event", "payload")
	time.Sleep(10 * time.Millisecond)
	if len(received) != 1 || received[0] != "manual.event" {
		t.Fatalf("manual emit failed: %v", received)
	}
}