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
