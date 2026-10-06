package orchestra

import (
	"context"
	"errors"
	"testing"
)

type recordingExecutor struct {
	err       error
	envelopes []ExecutionEnvelope
}

func (e *recordingExecutor) Execute(ctx context.Context, envelope ExecutionEnvelope) error {
	e.envelopes = append(e.envelopes, envelope)
	if e.err != nil {
		return e.err
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
		return nil
	}
}

func seededDispatcher(t *testing.T, task Task) (*Dispatcher, *MemoryTaskStore) {
	t.Helper()
	task.Status = TaskReady
	store := NewMemoryTaskStore()
	if err := store.SaveTask(context.Background(), task); err != nil {
		t.Fatal(err)
	}
	return NewDispatcher(store, nil), store
}

func TestDispatchTransitionsTaskAndRecordsEvent(t *testing.T) {
	task := validTask()
	dispatcher, store := seededDispatcher(t, task)
	execution, err := dispatcher.Dispatch(context.Background(), task.ID)
	if err != nil {
		t.Fatal(err)
	}
	if execution.TaskID != task.ID || execution.Attempt != 1 || execution.ID != "task-1-1" {
		t.Fatalf("execution = %#v", execution)
	}
	saved, err := store.GetTask(context.Background(), task.ID)
	if err != nil {
		t.Fatal(err)
	}
	if saved.Status != TaskRunning || saved.AttemptCount != 1 {
		t.Fatalf("saved = %#v", saved)
	}
	events, err := store.ListEvents(context.Background(), task.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 || events[0].Type != EventTaskDispatched {
		t.Fatalf("events = %#v", events)
	}
}

func TestDispatchRejectsDuplicateAndTerminalTasks(t *testing.T) {
	task := validTask()
	dispatcher, _ := seededDispatcher(t, task)
	if _, err := dispatcher.Dispatch(context.Background(), task.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := dispatcher.Dispatch(context.Background(), task.ID); !errors.Is(err, ErrDuplicateDispatch) {
		t.Fatalf("duplicate dispatch error = %v", err)
	}
	task.Status = TaskCanceled
	if _, err := dispatcher.Dispatch(context.Background(), task.ID); !errors.Is(err, ErrDuplicateDispatch) {
		t.Fatalf("terminal dispatch error = %v", err)
	}
}

func TestCancelAndCompleteAreMutuallyExclusive(t *testing.T) {
	task := validTask()
	dispatcher, store := seededDispatcher(t, task)
	execution, err := dispatcher.Dispatch(context.Background(), task.ID)
	if err != nil {
		t.Fatal(err)
	}
	if err := dispatcher.Cancel(context.Background(), task.ID); err != nil {
		t.Fatal(err)
	}
	if err := dispatcher.Complete(context.Background(), execution.ID, TaskAwaitingInspection, nil); err == nil {
		t.Fatal("expected completion error after cancel")
	}
	saved, err := store.GetTask(context.Background(), task.ID)
	if err != nil {
		t.Fatal(err)
	}
	if saved.Status != TaskCanceled {
		t.Fatalf("saved = %#v", saved)
	}
}

func TestCompleteMovesToInspectionOnlyThroughOrchestra(t *testing.T) {
	task := validTask()
	dispatcher, store := seededDispatcher(t, task)
	execution, err := dispatcher.Dispatch(context.Background(), task.ID)
	if err != nil {
		t.Fatal(err)
	}
	if err := dispatcher.Complete(context.Background(), execution.ID, TaskAwaitingInspection, []string{"evidence-1"}); err != nil {
		t.Fatal(err)
	}
	saved, err := store.GetTask(context.Background(), task.ID)
	if err != nil {
		t.Fatal(err)
	}
	if saved.Status != TaskAwaitingInspection {
		t.Fatalf("saved = %#v", saved)
	}
	events, err := store.ListEvents(context.Background(), task.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 2 || events[1].Type != EventEvidenceCollected {
		t.Fatalf("events = %#v", events)
	}
}

func TestDispatchHonorsRetryBudget(t *testing.T) {
	task := validTask()
	task.MaxAttempts = 1
	dispatcher, _ := seededDispatcher(t, task)
	if _, err := dispatcher.Dispatch(context.Background(), task.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := dispatcher.Dispatch(context.Background(), task.ID); err == nil {
		t.Fatal("expected duplicate dispatch error")
	}
}
