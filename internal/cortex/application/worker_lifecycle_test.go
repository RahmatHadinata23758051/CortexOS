package application

import (
	"context"
	"testing"
	"time"

	"github.com/RahmatHadinata23758051/CortexOS/internal/cortex/orchestra"
	"github.com/RahmatHadinata23758051/CortexOS/internal/cortex/staff"
)

type mockOrchestraTasks struct {
	tasks []orchestra.Task
}

func (m *mockOrchestraTasks) ListTasks(context.Context) ([]orchestra.Task, error) {
	return m.tasks, nil
}
func (m *mockOrchestraTasks) GetTask(context.Context, orchestra.TaskID) (orchestra.Task, error) {
	return orchestra.Task{}, orchestra.ErrNotFound
}
func (m *mockOrchestraTasks) ListEvents(context.Context, orchestra.TaskID) ([]orchestra.TaskEvent, error) {
	return nil, nil
}
func (m *mockOrchestraTasks) CancelTask(context.Context, orchestra.TaskID) error { return nil }
func (m *mockOrchestraTasks) RetryTask(context.Context, orchestra.TaskID) error  { return nil }
func (m *mockOrchestraTasks) MergeTask(context.Context, orchestra.TaskID, string) (orchestra.Task, error) {
	return orchestra.Task{}, nil
}
func (m *mockOrchestraTasks) CreateTask(context.Context, orchestra.Task) (orchestra.Task, error) {
	return orchestra.Task{}, nil
}

func TestWorkerLifecycleManager_SyncFromOrchestraTasks(t *testing.T) {
	tasks := &mockOrchestraTasks{
		tasks: []orchestra.Task{
			{
				ID:              "task-10",
				ProjectID:       "proj-alpha",
				Title:           "Implement auth",
				AssignedStaffID: staff.StaffID("Coder-1"),
				Status:          orchestra.TaskRunning,
				CreatedAt:       time.Now().UTC(),
			},
		},
	}
	mgr := NewWorkerLifecycleManager(tasks)
	ctx := context.Background()

	result, err := mgr.Sync(ctx, WorkerCardsRequest{
		SchemaVersion: WorkerLifecycleSchemaVersion,
		ProjectID:     "proj-alpha",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Workers) != 1 {
		t.Fatalf("expected 1 worker card, got %d", len(result.Workers))
	}
	worker := result.Workers[0]
	if worker.WorkerID != "worker-task-10" {
		t.Errorf("unexpected worker ID: %s", worker.WorkerID)
	}
	if worker.State != WorkerRunning {
		t.Errorf("expected state running, got %s", worker.State)
	}
}

func TestWorkerLifecycleManager_UnspawnFlow(t *testing.T) {
	tasks := &mockOrchestraTasks{
		tasks: []orchestra.Task{
			{
				ID:              "task-20",
				ProjectID:       "proj-beta",
				AssignedStaffID: staff.StaffID("Reviewer-1"),
				Status:          orchestra.TaskSuccess,
			},
		},
	}
	mgr := NewWorkerLifecycleManager(tasks)
	ctx := context.Background()

	_, err := mgr.Sync(ctx, WorkerCardsRequest{
		SchemaVersion: WorkerLifecycleSchemaVersion,
		ProjectID:     "proj-beta",
	})
	if err != nil {
		t.Fatalf("unexpected sync error: %v", err)
	}

	// Transition to unspawned
	card, event, err := mgr.Transition(ctx, "worker-task-20", WorkerUnspawned, "task accepted by Orchestra")
	if err != nil {
		t.Fatalf("unexpected transition error: %v", err)
	}
	if card.State != WorkerUnspawned {
		t.Errorf("expected state unspawned, got %s", card.State)
	}
	if event.To != WorkerUnspawned {
		t.Errorf("expected event to unspawned, got %s", event.To)
	}

	// Active list without history should omit unspawned worker
	active, err := mgr.Sync(ctx, WorkerCardsRequest{
		SchemaVersion: WorkerLifecycleSchemaVersion,
		ProjectID:     "proj-beta",
	})
	if err != nil {
		t.Fatalf("unexpected sync error: %v", err)
	}
	if len(active.Workers) != 0 {
		t.Errorf("expected 0 active workers after unspawn, got %d", len(active.Workers))
	}

	// List with history should retain it
	history, err := mgr.Sync(ctx, WorkerCardsRequest{
		SchemaVersion:  WorkerLifecycleSchemaVersion,
		ProjectID:      "proj-beta",
		IncludeHistory: true,
	})
	if err != nil {
		t.Fatalf("unexpected sync error: %v", err)
	}
	if len(history.Workers) != 1 {
		t.Errorf("expected 1 worker in history, got %d", len(history.Workers))
	}
}
