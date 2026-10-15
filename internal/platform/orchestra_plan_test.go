package platform

import (
	"context"
	"testing"

	"github.com/RahmatHadinata23758051/CortexOS/internal/cortex/application"
)

type planPortStub struct {
	proposed   application.OrchestraPlanProposal
	confirmRes application.OrchestraPlanConfirmResult
}

func (s *planPortStub) ProposePlan(_ context.Context, req application.OrchestraPlanProposalRequest) (application.OrchestraPlanProposal, error) {
	return s.proposed, nil
}

func (s *planPortStub) ConfirmPlan(_ context.Context, req application.OrchestraPlanConfirmRequest) (application.OrchestraPlanConfirmResult, error) {
	return s.confirmRes, nil
}

func (s *planPortStub) RejectPlan(_ context.Context, req application.OrchestraPlanRejectRequest) error {
	return nil
}

func (s *planPortStub) GetPlan(_ context.Context, planID string) (application.OrchestraPlanProposal, error) {
	return s.proposed, nil
}

func TestBridge_ProposeOrchestraPlanEmitsEvent(t *testing.T) {
	service := application.NewService()
	stub := &planPortStub{
		proposed: application.OrchestraPlanProposal{
			PlanID:               "plan-test-1",
			CorrelationID:        "corr-1",
			Status:               "proposed",
			RequiresConfirmation: true,
			SchemaVersion:        application.OrchestraPlanSchemaVersion,
		},
	}
	service.SetOrchestraPlan(stub)
	b := NewBridge(service)

	var emittedEvent string
	var emittedPayload any
	b.RegisterEventHook(func(name string, payload any) {
		emittedEvent = name
		emittedPayload = payload
	})

	res, err := b.ProposeOrchestraPlan(application.OrchestraPlanProposalRequest{
		CorrelationID: "corr-1",
		UserPrompt:    "Build something",
		Title:         "Build",
		SchemaVersion: application.OrchestraPlanSchemaVersion,
		Tasks: []application.OrchestraPlanProposedTask{
			{Index: 0, Title: "Step 1", Role: "coder", Action: "code", AcceptanceCriteria: []string{"A"}},
			{Index: 1, Title: "Step 2", Role: "tester", Action: "test", AcceptanceCriteria: []string{"B"}},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if emittedEvent != EventOrchestraPlanProposed {
		t.Errorf("expected event %q, got %q", EventOrchestraPlanProposed, emittedEvent)
	}
	p, ok := emittedPayload.(application.OrchestraPlanProposal)
	if !ok || p.PlanID != res.PlanID {
		t.Errorf("payload does not match result: %+v", emittedPayload)
	}
}

func TestBridge_ProposeOrchestraPlanEmitsAutoDispatchedEvent(t *testing.T) {
	service := application.NewService()
	stub := &planPortStub{
		proposed: application.OrchestraPlanProposal{
			PlanID:               "plan-test-auto",
			CorrelationID:        "corr-auto",
			Status:               "auto_dispatched",
			RequiresConfirmation: false,
			SchemaVersion:        application.OrchestraPlanSchemaVersion,
		},
	}
	service.SetOrchestraPlan(stub)
	b := NewBridge(service)

	var emittedEvent string
	b.RegisterEventHook(func(name string, payload any) {
		emittedEvent = name
	})

	_, err := b.ProposeOrchestraPlan(application.OrchestraPlanProposalRequest{
		CorrelationID: "corr-auto",
		UserPrompt:    "Inspect file",
		Title:         "Inspect",
		SchemaVersion: application.OrchestraPlanSchemaVersion,
		Tasks: []application.OrchestraPlanProposedTask{
			{Index: 0, Title: "Inspect", Role: "reviewer", Action: "inspect", AcceptanceCriteria: []string{"A"}},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if emittedEvent != EventOrchestraPlanAutoDispatched {
		t.Errorf("expected event %q, got %q", EventOrchestraPlanAutoDispatched, emittedEvent)
	}
}

func TestBridge_ConfirmOrchestraPlanEmitsEvent(t *testing.T) {
	service := application.NewService()
	stub := &planPortStub{
		confirmRes: application.OrchestraPlanConfirmResult{
			PlanID:            "plan-test-confirm",
			Status:            "confirmed",
			DispatchedTaskIDs: []string{"task-1", "task-2"},
			SchemaVersion:     application.OrchestraPlanSchemaVersion,
		},
	}
	service.SetOrchestraPlan(stub)
	b := NewBridge(service)

	var emittedEvent string
	b.RegisterEventHook(func(name string, payload any) {
		emittedEvent = name
	})

	res, err := b.ConfirmOrchestraPlan(application.OrchestraPlanConfirmRequest{
		PlanID:        "plan-test-confirm",
		SchemaVersion: application.OrchestraPlanSchemaVersion,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if emittedEvent != EventOrchestraPlanConfirmed {
		t.Errorf("expected event %q, got %q", EventOrchestraPlanConfirmed, emittedEvent)
	}
	if res.Status != "confirmed" {
		t.Errorf("expected confirmed status, got %s", res.Status)
	}
}

func TestBridge_RejectOrchestraPlanEmitsEvent(t *testing.T) {
	service := application.NewService()
	stub := &planPortStub{}
	service.SetOrchestraPlan(stub)
	b := NewBridge(service)

	var emittedEvent string
	b.RegisterEventHook(func(name string, payload any) {
		emittedEvent = name
	})

	err := b.RejectOrchestraPlan(application.OrchestraPlanRejectRequest{
		PlanID:        "plan-test-reject",
		Reason:        "Cancelled by user",
		SchemaVersion: application.OrchestraPlanSchemaVersion,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if emittedEvent != EventOrchestraPlanRejected {
		t.Errorf("expected event %q, got %q", EventOrchestraPlanRejected, emittedEvent)
	}
}

type planWorkerPortStub struct {
	syncResult application.WorkerCardsResult
}

func (w *planWorkerPortStub) Sync(_ context.Context, _ application.WorkerCardsRequest) (application.WorkerCardsResult, error) {
	return w.syncResult, nil
}

func (w *planWorkerPortStub) Transition(_ context.Context, _ string, _ application.WorkerLifecycleState, _ string) (application.WorkerCard, application.WorkerLifecycleEvent, error) {
	return application.WorkerCard{}, application.WorkerLifecycleEvent{}, nil
}

type planCoordPortStub struct {
	snapshot application.CoordinationSnapshot
}

func (c *planCoordPortStub) GetCoordinationSnapshot(_ context.Context, _ application.CoordinationSnapshotRequest) (application.CoordinationSnapshot, error) {
	return c.snapshot, nil
}

func TestBridge_AutoDispatchSyncsWorkersAndCoordination(t *testing.T) {
	service := application.NewService()
	stub := &planPortStub{
		proposed: application.OrchestraPlanProposal{
			PlanID:               "plan-auto-sync",
			CorrelationID:        "corr-auto-sync",
			Status:               "auto_dispatched",
			RequiresConfirmation: false,
			SchemaVersion:        application.OrchestraPlanSchemaVersion,
		},
	}
	service.SetOrchestraPlan(stub)
	workerStub := &planWorkerPortStub{
		syncResult: application.WorkerCardsResult{
			Workers: []application.WorkerCard{
				{WorkerID: "w-1", State: application.WorkerSpawned},
			},
			Events: []application.WorkerLifecycleEvent{
				{WorkerID: "w-1", To: application.WorkerSpawned},
			},
		},
	}
	service.SetWorkerLifecycle(workerStub)
	coordStub := &planCoordPortStub{
		snapshot: application.CoordinationSnapshot{
			ActivePhase: 0,
			TotalPhases: 1,
		},
	}
	service.SetCoordination(coordStub)
	b := NewBridge(service)

	emitted := make([]string, 0)
	b.RegisterEventHook(func(name string, _ any) {
		emitted = append(emitted, name)
	})

	_, err := b.ProposeOrchestraPlan(application.OrchestraPlanProposalRequest{
		CorrelationID: "corr-auto-sync",
		UserPrompt:    "Inspect file",
		Title:         "Inspect",
		ProjectID:     "proj-1",
		SchemaVersion: application.OrchestraPlanSchemaVersion,
		Tasks: []application.OrchestraPlanProposedTask{
			{Index: 0, Title: "Inspect", Role: "reviewer", Action: "inspect", AcceptanceCriteria: []string{"A"}},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expectedEvents := []string{
		EventOrchestraPlanAutoDispatched,
		EventOrchestraWorkerSpawned,
		EventCoordinationChanged,
	}
	for _, expected := range expectedEvents {
		found := false
		for _, e := range emitted {
			if e == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected emitted event %q in %v", expected, emitted)
		}
	}
}
