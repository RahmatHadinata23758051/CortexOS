package platform

import (
	"context"
	"testing"

	"github.com/RahmatHadinata23758051/CortexOS/internal/cortex/application"
	"github.com/RahmatHadinata23758051/CortexOS/internal/cortex/orchestra"
)

type routePortStub struct{}

func (routePortStub) RouteBrainToOrchestra(_ context.Context, req application.BrainOrchestraRouteRequest) (application.BrainOrchestraRouteResult, error) {
	return application.BrainOrchestraRouteResult{
		CorrelationID:   req.CorrelationID,
		OrchestraTaskID: "route-task",
		Status:          "accepted",
		SchemaVersion:   application.BrainOrchestraBridgeSchemaVersion,
	}, nil
}

type bridgeOrchestraStub struct{}

func (bridgeOrchestraStub) ListTasks(context.Context) ([]orchestra.Task, error) { return nil, nil }
func (bridgeOrchestraStub) GetTask(context.Context, orchestra.TaskID) (orchestra.Task, error) {
	return orchestra.Task{}, orchestra.ErrNotFound
}
func (bridgeOrchestraStub) ListEvents(context.Context, orchestra.TaskID) ([]orchestra.TaskEvent, error) {
	return nil, nil
}
func (bridgeOrchestraStub) CancelTask(context.Context, orchestra.TaskID) error { return nil }
func (bridgeOrchestraStub) RetryTask(context.Context, orchestra.TaskID) error  { return nil }
func (bridgeOrchestraStub) MergeTask(context.Context, orchestra.TaskID, string) (orchestra.Task, error) {
	return orchestra.Task{}, nil
}
func (bridgeOrchestraStub) CreateTask(context.Context, orchestra.Task) (orchestra.Task, error) {
	return orchestra.Task{}, nil
}

func TestBridgeRouteBrainTaskToOrchestraEmitsLifecycleEvent(t *testing.T) {
	service := application.NewServiceWithBrainOrchestra(routePortStub{})
	b := NewBridge(service)

	var eventName string
	var payload application.BrainOrchestraRouteResult
	unsubscribe := b.RegisterEventHook(func(name string, data any) {
		eventName = name
		payload, _ = data.(application.BrainOrchestraRouteResult)
	})
	defer unsubscribe()

	result, err := b.RouteBrainTaskToOrchestra(application.BrainOrchestraRouteRequest{
		CorrelationID:      "corr-platform",
		UserPrompt:         "route",
		Title:              "Route",
		AcceptanceCriteria: []string{"done"},
		SchemaVersion:      application.BrainOrchestraBridgeSchemaVersion,
	})
	if err != nil {
		t.Fatalf("route returned error: %v", err)
	}
	if result.Status != "accepted" || eventName != EventBrainRouteAccepted {
		t.Fatalf("result/event = %#v/%q", result, eventName)
	}
	if payload.CorrelationID != "corr-platform" {
		t.Fatalf("unexpected event payload: %#v", payload)
	}
}

type routePortWithGroundingStub struct{}

func (routePortWithGroundingStub) RouteBrainToOrchestra(_ context.Context, req application.BrainOrchestraRouteRequest) (application.BrainOrchestraRouteResult, error) {
	return application.BrainOrchestraRouteResult{
		CorrelationID:   req.CorrelationID,
		OrchestraTaskID: "route-grounded",
		Status:          "accepted",
		GroundedContext: []application.GroundedContextRef{
			{ID: "doc-1", Title: "Spec", SourceLabel: "docs/spec.md"},
		},
		SchemaVersion: application.BrainOrchestraBridgeSchemaVersion,
	}, nil
}

type groundingStub struct{}

func (groundingStub) GroundRoutingContext(_ context.Context, req application.KnowledgeContextRequest) (application.KnowledgeContextBundle, error) {
	return application.KnowledgeContextBundle{
		Items: []application.KnowledgeContextItem{
			{ID: "doc-1", Title: "Spec", SourceLabel: "docs/spec.md", Snippet: "Architecture plan", Score: 0.95},
		},
		TotalChars:    17,
		SchemaVersion: application.KnowledgeContextSchemaVersion,
	}, nil
}

func TestBridgeRouteBrainTaskEmitsGroundingEventWhenContextPresent(t *testing.T) {
	service := application.NewServiceWithBrainOrchestra(routePortWithGroundingStub{})
	b := NewBridge(service)

	emitted := make(map[string]bool)
	unsubscribe := b.RegisterEventHook(func(name string, _ any) {
		emitted[name] = true
	})
	defer unsubscribe()

	_, err := b.RouteBrainTaskToOrchestra(application.BrainOrchestraRouteRequest{
		CorrelationID:      "corr-grounded",
		UserPrompt:         "plan with context",
		Title:              "Plan",
		AcceptanceCriteria: []string{"done"},
		SchemaVersion:      application.BrainOrchestraBridgeSchemaVersion,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !emitted[EventKnowledgeContextGrounded] || !emitted[EventBrainRouteAccepted] {
		t.Fatalf("expected grounding and route accepted events, got %#v", emitted)
	}
}

func TestBridgeGroundRoutingContextMethodAndEvent(t *testing.T) {
	service := application.NewService()
	service.SetKnowledgeGrounding(groundingStub{})
	b := NewBridge(service)

	var emittedEvent string
	unsubscribe := b.RegisterEventHook(func(name string, _ any) {
		emittedEvent = name
	})
	defer unsubscribe()

	bundle, err := b.GroundRoutingContext(application.KnowledgeContextRequest{
		ProjectID:     "proj-1",
		Query:         "architecture",
		Budget:        application.DefaultKnowledgeContextBudget(),
		SchemaVersion: application.KnowledgeContextSchemaVersion,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(bundle.Items) != 1 || bundle.Items[0].Title != "Spec" {
		t.Fatalf("unexpected bundle: %#v", bundle)
	}
	if emittedEvent != EventKnowledgeContextGrounded {
		t.Fatalf("expected %q, got %q", EventKnowledgeContextGrounded, emittedEvent)
	}
}
