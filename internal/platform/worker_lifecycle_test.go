package platform

import (
	"context"
	"testing"
	"time"

	"github.com/RahmatHadinata23758051/CortexOS/internal/cortex/application"
)

type workerPortStub struct {
	result application.WorkerCardsResult
	card   application.WorkerCard
	event  application.WorkerLifecycleEvent
}

func (s *workerPortStub) Sync(_ context.Context, _ application.WorkerCardsRequest) (application.WorkerCardsResult, error) {
	return s.result, nil
}

func (s *workerPortStub) Transition(_ context.Context, _ string, to application.WorkerLifecycleState, reason string) (application.WorkerCard, application.WorkerLifecycleEvent, error) {
	return s.card, s.event, nil
}

func TestBridge_GetWorkerCardsEmitsSpawnedEvent(t *testing.T) {
	service := application.NewService()
	stub := &workerPortStub{
		result: application.WorkerCardsResult{
			Workers: []application.WorkerCard{
				{WorkerID: "w-1", State: application.WorkerSpawned},
			},
			Events: []application.WorkerLifecycleEvent{
				{WorkerID: "w-1", To: application.WorkerSpawned, OccurredAt: time.Now().UTC()},
			},
		},
	}
	service.SetWorkerLifecycle(stub)
	b := NewBridge(service)

	var emittedEvent string
	b.RegisterEventHook(func(name string, _ any) {
		emittedEvent = name
	})

	_, err := b.GetWorkerCards(application.WorkerCardsRequest{
		SchemaVersion: application.WorkerLifecycleSchemaVersion,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if emittedEvent != EventOrchestraWorkerSpawned {
		t.Errorf("expected %q, got %q", EventOrchestraWorkerSpawned, emittedEvent)
	}
}

func TestBridge_TransitionWorkerEmitsUnspawnedEvent(t *testing.T) {
	service := application.NewService()
	stub := &workerPortStub{
		card: application.WorkerCard{WorkerID: "w-1", State: application.WorkerUnspawned},
		event: application.WorkerLifecycleEvent{
			WorkerID: "w-1",
			To:       application.WorkerUnspawned,
		},
	}
	service.SetWorkerLifecycle(stub)
	b := NewBridge(service)

	var emittedEvent string
	b.RegisterEventHook(func(name string, _ any) {
		emittedEvent = name
	})

	_, err := b.TransitionWorker(application.WorkerTransitionRequest{
		WorkerID:      "w-1",
		State:         application.WorkerUnspawned,
		SchemaVersion: application.WorkerLifecycleSchemaVersion,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if emittedEvent != EventOrchestraWorkerUnspawned {
		t.Errorf("expected %q, got %q", EventOrchestraWorkerUnspawned, emittedEvent)
	}
}

func TestBridge_TransitionWorkerEmitsUpdatedEvent(t *testing.T) {
	service := application.NewService()
	stub := &workerPortStub{
		card: application.WorkerCard{WorkerID: "w-2", State: application.WorkerRunning},
		event: application.WorkerLifecycleEvent{
			WorkerID: "w-2",
			From:     application.WorkerSpawned,
			To:       application.WorkerRunning,
		},
	}
	service.SetWorkerLifecycle(stub)
	b := NewBridge(service)

	var emittedEvent string
	b.RegisterEventHook(func(name string, _ any) {
		emittedEvent = name
	})

	_, err := b.TransitionWorker(application.WorkerTransitionRequest{
		WorkerID:      "w-2",
		State:         application.WorkerRunning,
		SchemaVersion: application.WorkerLifecycleSchemaVersion,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if emittedEvent != EventOrchestraWorkerUpdated {
		t.Errorf("expected %q, got %q", EventOrchestraWorkerUpdated, emittedEvent)
	}
}
