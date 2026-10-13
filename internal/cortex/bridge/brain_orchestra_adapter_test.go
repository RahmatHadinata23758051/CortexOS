package bridge

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"

	"github.com/RahmatHadinata23758051/CortexOS/internal/cortex/application"
	"github.com/RahmatHadinata23758051/CortexOS/internal/cortex/orchestra"
)

type testOrchestraPort struct {
	tasks []orchestra.Task
	mu    sync.Mutex
	err   error
}

func (t *testOrchestraPort) ListTasks(context.Context) ([]orchestra.Task, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	return append([]orchestra.Task(nil), t.tasks...), t.err
}
func (t *testOrchestraPort) GetTask(_ context.Context, id orchestra.TaskID) (orchestra.Task, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	for _, task := range t.tasks {
		if task.ID == id {
			return task, nil
		}
	}
	return orchestra.Task{}, orchestra.ErrNotFound
}
func (t *testOrchestraPort) ListEvents(context.Context, orchestra.TaskID) ([]orchestra.TaskEvent, error) {
	return nil, nil
}
func (t *testOrchestraPort) CancelTask(context.Context, orchestra.TaskID) error { return nil }
func (t *testOrchestraPort) RetryTask(context.Context, orchestra.TaskID) error  { return nil }
func (t *testOrchestraPort) MergeTask(context.Context, orchestra.TaskID, string) (orchestra.Task, error) {
	return orchestra.Task{}, nil
}
func (t *testOrchestraPort) CreateTask(_ context.Context, task orchestra.Task) (orchestra.Task, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.err != nil {
		return orchestra.Task{}, t.err
	}
	for _, existing := range t.tasks {
		if existing.ID == task.ID {
			return orchestra.Task{}, orchestra.ErrDuplicateDispatch
		}
	}
	t.tasks = append(t.tasks, task)
	return task, nil
}

func validRoutingRequest() application.BrainOrchestraRouteRequest {
	return application.BrainOrchestraRouteRequest{
		CorrelationID:      "corr-1",
		UserPrompt:         "implement the feature",
		Title:              "Implement the feature",
		AcceptanceCriteria: []string{"tests pass"},
		SchemaVersion:      application.BrainOrchestraBridgeSchemaVersion,
	}
}

func TestRoutingAdapter_HappyPath(t *testing.T) {
	fakeBrain := application.NewFakeBrainAdapter("test-brain")
	// Set canned response that returns a synthetic test plan
	fakeBrain.SetResult("corr-1-plan", application.BrainDispatchResult{
		TaskID:        "corr-1-plan",
		State:         "completed",
		Output:        "PLAN:2|Setup project|Implement feature",
		SchemaVersion: application.BrainBridgeContract,
	})

	orch := &testOrchestraPort{}
	adapter, err := NewBrainOrchestraRoutingAdapter(BrainOrchestraRoutingAdapterConfig{
		Brain:     fakeBrain,
		Orchestra: orch,
		ProjectID: "proj-123",
	})
	if err != nil {
		t.Fatalf("failed to create adapter: %v", err)
	}

	ctx := context.Background()
	result, err := adapter.RouteBrainToOrchestra(ctx, validRoutingRequest())
	if err != nil {
		t.Fatalf("RouteBrainToOrchestra failed: %v", err)
	}

	if result.Status != "accepted" {
		t.Fatalf("expected accepted status, got %q", result.Status)
	}
	if result.CorrelationID != "corr-1" {
		t.Fatalf("expected correlationId corr-1, got %q", result.CorrelationID)
	}
	if !strings.HasPrefix(result.OrchestraTaskID, "route-corr-1") {
		t.Fatalf("unexpected orchestraTaskId %q", result.OrchestraTaskID)
	}
	if result.SchemaVersion != application.BrainOrchestraBridgeSchemaVersion {
		t.Fatalf("unexpected schema version %q", result.SchemaVersion)
	}

	// Verify task in Orchestra
	tasks, _ := orch.ListTasks(ctx)
	if len(tasks) != 1 {
		t.Fatalf("expected 1 task in Orchestra, got %d", len(tasks))
	}
	if tasks[0].ID != orchestra.TaskID(result.OrchestraTaskID) {
		t.Fatalf("task ID mismatch: %q vs %q", tasks[0].ID, result.OrchestraTaskID)
	}
	if tasks[0].ProjectID != "proj-123" {
		t.Fatalf("expected project ID proj-123, got %q", tasks[0].ProjectID)
	}
	if tasks[0].Status != orchestra.TaskDraft {
		t.Fatalf("expected draft status, got %q", tasks[0].Status)
	}
}

type routingGroundingStub struct {
	bundle application.KnowledgeContextBundle
	err    error
}

func (s routingGroundingStub) GroundRoutingContext(context.Context, application.KnowledgeContextRequest) (application.KnowledgeContextBundle, error) {
	return s.bundle, s.err
}

func TestRoutingAdapter_GroundsPlanAndReturnsContextRefs(t *testing.T) {
	fakeBrain := application.NewFakeBrainAdapter("test-brain")
	fakeBrain.SetResult("corr-ground-plan", application.BrainDispatchResult{
		TaskID: "corr-ground-plan", State: "completed", Output: "PLAN:1|Grounded step", SchemaVersion: application.BrainBridgeContract,
	})
	orch := &testOrchestraPort{}
	adapter, err := NewBrainOrchestraRoutingAdapter(BrainOrchestraRoutingAdapterConfig{
		Brain: fakeBrain, Orchestra: orch, ProjectID: "proj-1",
		Grounding: routingGroundingStub{bundle: application.KnowledgeContextBundle{
			Items: []application.KnowledgeContextItem{{ID: "doc-1", Title: "Architecture", SourceLabel: "docs/arch.md", Snippet: "safe context", Score: 0.9}},
			SchemaVersion: application.KnowledgeContextSchemaVersion,
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	request := validRoutingRequest()
	request.CorrelationID = "corr-ground"
	result, err := adapter.RouteBrainToOrchestra(context.Background(), request)
	if err != nil {
		t.Fatalf("grounded route failed: %v", err)
	}
	if len(result.GroundedContext) != 1 || result.GroundedContext[0].ID != "doc-1" {
		t.Fatalf("unexpected grounded refs: %#v", result.GroundedContext)
	}
}

func TestRoutingAdapter_DuplicateCorrelationId(t *testing.T) {
	fakeBrain := application.NewFakeBrainAdapter("test-brain")
	fakeBrain.SetResult("corr-dup-plan", application.BrainDispatchResult{
		TaskID:        "corr-dup-plan",
		State:         "completed",
		Output:        "PLAN:1|Single step",
		SchemaVersion: application.BrainBridgeContract,
	})

	orch := &testOrchestraPort{}
	adapter, err := NewBrainOrchestraRoutingAdapter(BrainOrchestraRoutingAdapterConfig{
		Brain:     fakeBrain,
		Orchestra: orch,
	})
	if err != nil {
		t.Fatal(err)
	}

	req := validRoutingRequest()
	req.CorrelationID = "corr-dup"

	ctx := context.Background()
	res1, err := adapter.RouteBrainToOrchestra(ctx, req)
	if err != nil {
		t.Fatalf("first call failed: %v", err)
	}

	// Second call with same correlation ID should return cached result without creating a second task
	res2, err := adapter.RouteBrainToOrchestra(ctx, req)
	if err != nil {
		t.Fatalf("second call failed: %v", err)
	}

	if res1.OrchestraTaskID != res2.OrchestraTaskID {
		t.Fatalf("expected matching task IDs, got %q vs %q", res1.OrchestraTaskID, res2.OrchestraTaskID)
	}

	tasks, _ := orch.ListTasks(ctx)
	if len(tasks) != 1 {
		t.Fatalf("expected exactly 1 task in store, got %d", len(tasks))
	}
}

func TestRoutingAdapter_InvalidRequestFailsClosed(t *testing.T) {
	fakeBrain := application.NewFakeBrainAdapter("test-brain")
	orch := &testOrchestraPort{}
	adapter, _ := NewBrainOrchestraRoutingAdapter(BrainOrchestraRoutingAdapterConfig{
		Brain:     fakeBrain,
		Orchestra: orch,
	})

	badReq := validRoutingRequest()
	badReq.SchemaVersion = "wrong"

	_, err := adapter.RouteBrainToOrchestra(context.Background(), badReq)
	if err == nil || !errors.Is(err, application.ErrBrainOrchestraInvalidRequest) {
		t.Fatalf("expected ErrBrainOrchestraInvalidRequest, got %v", err)
	}
}

func TestRoutingAdapter_UnstructuredBrainOutputFailsClosed(t *testing.T) {
	fakeBrain := application.NewFakeBrainAdapter("test-brain")
	// Return non-structured output
	fakeBrain.SetResult("corr-bad-plan", application.BrainDispatchResult{
		TaskID:        "corr-bad-plan",
		State:         "completed",
		Output:        "I cannot help with that.",
		SchemaVersion: application.BrainBridgeContract,
	})

	orch := &testOrchestraPort{}
	adapter, _ := NewBrainOrchestraRoutingAdapter(BrainOrchestraRoutingAdapterConfig{
		Brain:     fakeBrain,
		Orchestra: orch,
	})

	req := validRoutingRequest()
	req.CorrelationID = "corr-bad"

	res, err := adapter.RouteBrainToOrchestra(context.Background(), req)
	if err == nil {
		t.Fatalf("expected error on unstructured output, got result %#v", res)
	}
	if res.Status != "rejected" {
		t.Fatalf("expected rejected status, got %q", res.Status)
	}

	tasks, _ := orch.ListTasks(context.Background())
	if len(tasks) != 0 {
		t.Fatalf("expected 0 tasks in store, got %d", len(tasks))
	}
}

func TestRoutingAdapter_ContextCancellation(t *testing.T) {
	fakeBrain := application.NewFakeBrainAdapter("test-brain")
	orch := &testOrchestraPort{}
	adapter, _ := NewBrainOrchestraRoutingAdapter(BrainOrchestraRoutingAdapterConfig{
		Brain:     fakeBrain,
		Orchestra: orch,
	})

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // canceled immediately

	res, err := adapter.RouteBrainToOrchestra(ctx, validRoutingRequest())
	if err == nil {
		t.Fatal("expected cancellation error")
	}
	if res.Status != "canceled" {
		t.Fatalf("expected canceled status, got %q", res.Status)
	}

	tasks, _ := orch.ListTasks(context.Background())
	if len(tasks) != 0 {
		t.Fatalf("expected 0 tasks in store after cancel, got %d", len(tasks))
	}
}

func TestRoutingAdapter_OrchestraFailureFailsClosed(t *testing.T) {
	fakeBrain := application.NewFakeBrainAdapter("test-brain")
	fakeBrain.SetResult("corr-err-plan", application.BrainDispatchResult{
		TaskID:        "corr-err-plan",
		State:         "completed",
		Output:        "PLAN:1|Step",
		SchemaVersion: application.BrainBridgeContract,
	})

	orch := &testOrchestraPort{err: errors.New("db disk failure")}
	adapter, _ := NewBrainOrchestraRoutingAdapter(BrainOrchestraRoutingAdapterConfig{
		Brain:     fakeBrain,
		Orchestra: orch,
	})

	req := validRoutingRequest()
	req.CorrelationID = "corr-err"

	res, err := adapter.RouteBrainToOrchestra(context.Background(), req)
	if err == nil {
		t.Fatal("expected error when orchestra store fails")
	}
	if res.Status != "rejected" {
		t.Fatalf("expected rejected status, got %q", res.Status)
	}
}
