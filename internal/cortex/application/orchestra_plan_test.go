package application

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/RahmatHadinata23758051/CortexOS/internal/cortex/orchestra"
)

type mockOrchestraTaskStore struct {
	tasks []orchestra.Task
}

type testCoordinatorBrain struct {
	output string
}

func (b *testCoordinatorBrain) Name() string { return "test-coordinator" }

func (b *testCoordinatorBrain) Dispatch(ctx context.Context, req BrainDispatchRequest) (BrainDispatchResult, error) {
	if err := ctx.Err(); err != nil {
		return BrainDispatchResult{}, err
	}
	return BrainDispatchResult{TaskID: req.TaskID, State: "completed", Output: b.output, SchemaVersion: BrainBridgeContract}, nil
}

func (b *testCoordinatorBrain) DispatchStream(ctx context.Context, req BrainDispatchRequest, handler StreamChunkHandler) (BrainDispatchResult, error) {
	return b.Dispatch(ctx, req)
}

func (m *mockOrchestraTaskStore) ListTasks(context.Context) ([]orchestra.Task, error) {
	return m.tasks, nil
}

func (m *mockOrchestraTaskStore) GetTask(_ context.Context, id orchestra.TaskID) (orchestra.Task, error) {
	for _, t := range m.tasks {
		if t.ID == id {
			return t, nil
		}
	}
	return orchestra.Task{}, orchestra.ErrNotFound
}

func (m *mockOrchestraTaskStore) ListEvents(context.Context, orchestra.TaskID) ([]orchestra.TaskEvent, error) {
	return nil, nil
}

func (m *mockOrchestraTaskStore) CancelTask(context.Context, orchestra.TaskID) error {
	return nil
}

func (m *mockOrchestraTaskStore) RetryTask(context.Context, orchestra.TaskID) error {
	return nil
}

func (m *mockOrchestraTaskStore) MergeTask(context.Context, orchestra.TaskID, string) (orchestra.Task, error) {
	return orchestra.Task{}, nil
}

func (m *mockOrchestraTaskStore) CreateTask(_ context.Context, task orchestra.Task) (orchestra.Task, error) {
	m.tasks = append(m.tasks, task)
	return task, nil
}

func TestEvaluatePlanConfirmationPolicy(t *testing.T) {
	t.Run("single step standard complexity auto-dispatches", func(t *testing.T) {
		tasks := []OrchestraPlanProposedTask{
			{Index: 0, Title: "Inspect code", Role: "reviewer", Action: "inspect", AcceptanceCriteria: []string{"Reviewed"}},
		}
		reqConf, risk := EvaluatePlanConfirmationPolicy(tasks, nil)
		if reqConf {
			t.Errorf("expected auto-dispatch (false), got %v", reqConf)
		}
		if risk != "low" {
			t.Errorf("expected risk low, got %s", risk)
		}
	})

	t.Run("multi-step plan requires confirmation", func(t *testing.T) {
		tasks := []OrchestraPlanProposedTask{
			{Index: 0, Title: "Step 1", Role: "planner", Action: "plan", AcceptanceCriteria: []string{"Done"}},
			{Index: 1, Title: "Step 2", Role: "implementer", Action: "build", AcceptanceCriteria: []string{"Built"}, Dependencies: []int{0}},
		}
		reqConf, risk := EvaluatePlanConfirmationPolicy(tasks, nil)
		if !reqConf {
			t.Errorf("expected confirmation required (true), got %v", reqConf)
		}
		if risk != "medium" {
			t.Errorf("expected risk medium, got %s", risk)
		}
	})

	t.Run("destructive single step requires confirmation", func(t *testing.T) {
		tasks := []OrchestraPlanProposedTask{
			{Index: 0, Title: "Delete file", Role: "implementer", Action: "delete", AcceptanceCriteria: []string{"Removed"}},
		}
		reqConf, risk := EvaluatePlanConfirmationPolicy(tasks, nil)
		if !reqConf {
			t.Errorf("expected confirmation required (true) for destructive action, got %v", reqConf)
		}
		if risk != "high" {
			t.Errorf("expected risk high, got %s", risk)
		}
	})

	t.Run("explicit override false auto-dispatches", func(t *testing.T) {
		override := false
		tasks := []OrchestraPlanProposedTask{
			{Index: 0, Title: "Multi 1", Role: "implementer", Action: "build", AcceptanceCriteria: []string{"Done"}},
			{Index: 1, Title: "Multi 2", Role: "implementer", Action: "build", AcceptanceCriteria: []string{"Done"}},
		}
		reqConf, _ := EvaluatePlanConfirmationPolicy(tasks, &override)
		if reqConf {
			t.Errorf("expected override false, got %v", reqConf)
		}
	})
}

func TestOrchestraPlanManager_ProposeAndAutoDispatch(t *testing.T) {
	mockStore := &mockOrchestraTaskStore{}
	mgr := NewOrchestraPlanManager(mockStore)

	ctx := context.Background()
	req := OrchestraPlanProposalRequest{
		CorrelationID: "corr-101",
		ProjectID:     "proj-alpha",
		UserPrompt:    "Check system status",
		Title:         "System check",
		SchemaVersion: OrchestraPlanSchemaVersion,
		Tasks: []OrchestraPlanProposedTask{
			{
				Index:               0,
				Title:               "Read status",
				Role:                "reviewer",
				Action:              "inspect",
				AcceptanceCriteria:  []string{"Status checked"},
				EstimatedComplexity: "trivial",
			},
		},
	}

	proposal, err := mgr.ProposePlan(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if proposal.Status != "auto_dispatched" {
		t.Errorf("expected status 'auto_dispatched', got %s", proposal.Status)
	}
	if proposal.RequiresConfirmation {
		t.Errorf("expected requiresConfirmation=false")
	}
	if len(proposal.DispatchedTaskIDs) != 1 {
		t.Fatalf("expected 1 dispatched task ID, got %d", len(proposal.DispatchedTaskIDs))
	}
	if len(mockStore.tasks) != 1 {
		t.Fatalf("expected 1 task created in Orchestra store, got %d", len(mockStore.tasks))
	}
	if mockStore.tasks[0].ProjectID != "proj-alpha" {
		t.Errorf("expected project proj-alpha, got %s", mockStore.tasks[0].ProjectID)
	}
}

func TestOrchestraPlanManager_ProposeConfirmFlow(t *testing.T) {
	mockStore := &mockOrchestraTaskStore{}
	mgr := NewOrchestraPlanManager(mockStore)

	ctx := context.Background()
	req := OrchestraPlanProposalRequest{
		CorrelationID: "corr-202",
		ProjectID:     "proj-beta",
		UserPrompt:    "Implement feature X",
		Title:         "Feature X Plan",
		SchemaVersion: OrchestraPlanSchemaVersion,
		Tasks: []OrchestraPlanProposedTask{
			{
				Index:              0,
				Title:              "Design interface",
				Role:               "architect",
				CallSign:           "Architect-1",
				Action:             "spec",
				AcceptanceCriteria: []string{"Spec approved"},
			},
			{
				Index:              1,
				Title:              "Write code",
				Role:               "developer",
				CallSign:           "Coder-1",
				Action:             "code",
				AcceptanceCriteria: []string{"Tests pass"},
				Dependencies:       []int{0},
			},
		},
	}

	proposal, err := mgr.ProposePlan(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error proposing: %v", err)
	}

	if proposal.Status != "proposed" {
		t.Fatalf("expected status 'proposed', got %s", proposal.Status)
	}
	if !proposal.RequiresConfirmation {
		t.Fatalf("expected requiresConfirmation=true")
	}
	if len(mockStore.tasks) != 0 {
		t.Fatalf("expected 0 tasks dispatched before confirmation, got %d", len(mockStore.tasks))
	}

	// Confirm
	confirmRes, err := mgr.ConfirmPlan(ctx, OrchestraPlanConfirmRequest{
		PlanID:        proposal.PlanID,
		SchemaVersion: OrchestraPlanSchemaVersion,
	})
	if err != nil {
		t.Fatalf("unexpected error confirming: %v", err)
	}

	if confirmRes.Status != "confirmed" {
		t.Errorf("expected confirm status 'confirmed', got %s", confirmRes.Status)
	}
	if len(confirmRes.DispatchedTaskIDs) != 2 {
		t.Fatalf("expected 2 dispatched task IDs, got %d", len(confirmRes.DispatchedTaskIDs))
	}
	if len(mockStore.tasks) != 2 {
		t.Fatalf("expected 2 tasks in Orchestra store, got %d", len(mockStore.tasks))
	}

	// Verify dependencies wired correctly
	t0 := mockStore.tasks[0]
	t1 := mockStore.tasks[1]
	if len(t1.Dependencies) != 1 || string(t1.Dependencies[0]) != string(t0.ID) {
		t.Errorf("expected t1 to depend on t0 (%s), got %v", t0.ID, t1.Dependencies)
	}

	// Double-confirm must fail
	_, err = mgr.ConfirmPlan(ctx, OrchestraPlanConfirmRequest{
		PlanID:        proposal.PlanID,
		SchemaVersion: OrchestraPlanSchemaVersion,
	})
	if err != ErrOrchestraPlanAlreadyDecided {
		t.Errorf("expected ErrOrchestraPlanAlreadyDecided, got %v", err)
	}
}

func TestOrchestraPlanManager_RejectFlow(t *testing.T) {
	mockStore := &mockOrchestraTaskStore{}
	mgr := NewOrchestraPlanManager(mockStore)

	ctx := context.Background()
	req := OrchestraPlanProposalRequest{
		CorrelationID: "corr-303",
		ProjectID:     "proj-gamma",
		UserPrompt:    "Refactor module",
		Title:         "Refactor",
		SchemaVersion: OrchestraPlanSchemaVersion,
		Tasks: []OrchestraPlanProposedTask{
			{
				Index:              0,
				Title:              "Task 1",
				Role:               "coder",
				Action:             "code",
				AcceptanceCriteria: []string{"Done"},
			},
			{
				Index:              1,
				Title:              "Task 2",
				Role:               "tester",
				Action:             "test",
				AcceptanceCriteria: []string{"Passed"},
			},
		},
	}

	proposal, err := mgr.ProposePlan(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error proposing: %v", err)
	}

	err = mgr.RejectPlan(ctx, OrchestraPlanRejectRequest{
		PlanID:        proposal.PlanID,
		Reason:        "Too risky right now",
		SchemaVersion: OrchestraPlanSchemaVersion,
	})
	if err != nil {
		t.Fatalf("unexpected error rejecting: %v", err)
	}

	p, err := mgr.GetPlan(ctx, proposal.PlanID)
	if err != nil {
		t.Fatalf("unexpected error getting plan: %v", err)
	}
	if p.Status != "rejected" {
		t.Errorf("expected status 'rejected', got %s", p.Status)
	}
	if len(mockStore.tasks) != 0 {
		t.Errorf("expected 0 dispatched tasks for rejected plan, got %d", len(mockStore.tasks))
	}
}

func TestOrchestraPlanManager_InquiryDoesNotSpawnWorkers(t *testing.T) {
	mockStore := &mockOrchestraTaskStore{}
	mgr := NewOrchestraPlanManager(mockStore)
	mgr.SetBrain(&testCoordinatorBrain{output: `{"intent":"inquiry","reply":"Halo dari model."}`})

	ctx := context.Background()
	req := OrchestraPlanProposalRequest{
		CorrelationID: "corr-inquiry-1",
		ProjectID:     "proj-test",
		UserPrompt:    "halo",
		SchemaVersion: OrchestraPlanSchemaVersion,
	}

	proposal, err := mgr.ProposePlan(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if proposal.Status != "answered" {
		t.Errorf("expected status 'answered', got %q", proposal.Status)
	}
	if proposal.Reply == "" {
		t.Errorf("expected non-empty reply")
	}
	if len(mockStore.tasks) != 0 {
		t.Fatalf("expected 0 tasks created in Orchestra store, got %d", len(mockStore.tasks))
	}
	if len(proposal.DispatchedTaskIDs) != 0 {
		t.Fatalf("expected 0 dispatched tasks, got %d", len(proposal.DispatchedTaskIDs))
	}
}

func TestOrchestraPlanManager_ActionPromptWithoutTasksAutoDerivesAndDispatches(t *testing.T) {
	mockStore := &mockOrchestraTaskStore{}
	mgr := NewOrchestraPlanManager(mockStore)
	mgr.SetBrain(&testCoordinatorBrain{output: `{"intent":"action","title":"Inventory system","role":"implementer","action":"build the inventory module","acceptanceCriteria":["Inventory works"]}`})

	ctx := context.Background()
	req := OrchestraPlanProposalRequest{
		CorrelationID: "corr-action-1",
		ProjectID:     "proj-test",
		UserPrompt:    "buat sistem inventory baru",
		SchemaVersion: OrchestraPlanSchemaVersion,
	}

	proposal, err := mgr.ProposePlan(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if proposal.Status != "auto_dispatched" {
		t.Errorf("expected status 'auto_dispatched', got %q", proposal.Status)
	}
	if len(mockStore.tasks) != 1 {
		t.Fatalf("expected 1 task created, got %d", len(mockStore.tasks))
	}
	if mockStore.tasks[0].AssignedStaffID != "implementer-1" {
		t.Errorf("expected call sign implementer-1, got %q", mockStore.tasks[0].AssignedStaffID)
	}
}

func TestParseOrchestraDecision(t *testing.T) {
	decision, err := parseOrchestraDecision("```json\n{\"intent\":\"inquiry\",\"reply\":\"model answer\"}\n```")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if decision.Intent != "inquiry" || decision.Reply != "model answer" {
		t.Fatalf("unexpected decision: %+v", decision)
	}
}

func TestOrchestraPlanManager_FailsClosedWithoutCoordinator(t *testing.T) {
	mgr := NewOrchestraPlanManager(&mockOrchestraTaskStore{})
	_, err := mgr.ProposePlan(context.Background(), OrchestraPlanProposalRequest{
		CorrelationID: "corr-no-brain",
		ProjectID:     "proj-test",
		UserPrompt:    "halo",
		SchemaVersion: OrchestraPlanSchemaVersion,
	})
	if !errors.Is(err, ErrOrchestraPlanModelUnavailable) {
		t.Fatalf("expected model-unavailable error, got %v", err)
	}
}

func TestOrchestraPlanManager_BoundedCapacity(t *testing.T) {
	mgr := NewOrchestraPlanManager(nil)
	ctx := context.Background()

	// Propose 105 plans to trigger eviction
	for i := 0; i < 105; i++ {
		corrID := fmt.Sprintf("corr-bound-%d", i)
		_, err := mgr.ProposePlan(ctx, OrchestraPlanProposalRequest{
			CorrelationID: corrID,
			ProjectID:     "test",
			UserPrompt:    "Test prompt",
			Title:         "Test",
			SchemaVersion: OrchestraPlanSchemaVersion,
			Tasks: []OrchestraPlanProposedTask{
				{
					Index:              0,
					Title:              "T",
					Role:               "R",
					Action:             "A",
					AcceptanceCriteria: []string{"C"},
				},
			},
		})
		if err != nil {
			t.Fatalf("unexpected error at %d: %v", i, err)
		}
		// ensure monotonically advancing timestamp for deterministic eviction
		time.Sleep(1 * time.Millisecond)
	}

	if len(mgr.plans) > 100 {
		t.Errorf("expected map bounded at 100 entries, got %d", len(mgr.plans))
	}
}
