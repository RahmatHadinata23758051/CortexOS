package orchestra

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

var (
	ErrDuplicateDispatch = errors.New("orchestra: duplicate dispatch")
	ErrWorkerBusy        = errors.New("orchestra: worker is busy")
	ErrInvalidDispatch   = errors.New("orchestra: invalid dispatch")
)

type Executor interface {
	Execute(context.Context, ExecutionEnvelope) error
}

type ExecutionEnvelope struct {
	TaskID     TaskID `json:"taskId"`
	ProjectID  string `json:"projectId"`
	WorktreeID string `json:"worktreeId"`
	Attempt    int    `json:"attempt"`
	Title      string `json:"title"`
}

type Dispatcher struct {
	mu       sync.Mutex
	store    TaskStore
	exec     Executor
	active   map[TaskID]*activeExecution
	workerID string
}

type activeExecution struct {
	Execution Execution
	Cancel    context.CancelFunc
	WorkerID  string
	terminal  atomic.Bool
}

func NewDispatcher(store TaskStore, exec Executor, workerID ...string) *Dispatcher {
	id := ""
	if len(workerID) > 0 {
		id = workerID[0]
	}
	return &Dispatcher{store: store, exec: exec, active: make(map[TaskID]*activeExecution), workerID: id}
}

func (d *Dispatcher) Dispatch(ctx context.Context, taskID TaskID) (Execution, error) {
	if err := ctx.Err(); err != nil {
		return Execution{}, err
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	task, err := d.store.GetTask(ctx, taskID)
	if err != nil {
		return Execution{}, err
	}
	if err := ValidateTask(task); err != nil {
		return Execution{}, err
	}
	if _, active := d.active[taskID]; active {
		return Execution{}, fmt.Errorf("%w: task %q is already active", ErrDuplicateDispatch, taskID)
	}
	if task.Status == TaskSuccess || task.Status == TaskCanceled {
		return Execution{}, fmt.Errorf("%w: task %q is terminal", ErrInvalidDispatch, taskID)
	}
	if task.Status != TaskReady && task.Status != TaskFailed && task.Status != TaskDraft {
		return Execution{}, fmt.Errorf("%w: task %q is not dispatchable", ErrInvalidDispatch, taskID)
	}
	if task.AttemptCount >= task.MaxAttempts {
		return Execution{}, fmt.Errorf("%w: task %q exceeded retry budget", ErrInvalidDispatch, taskID)
	}
	result, err := ApplyTransition(task, TransitionDispatch)
	if err != nil {
		return Execution{}, err
	}
	execution := Execution{
		ID:      ExecutionID(fmt.Sprintf("%s-%d", result.Task.ID, result.Task.AttemptCount)),
		TaskID:  result.Task.ID,
		Attempt: result.Task.AttemptCount,
		Status:  TaskRunning,
	}
	if err := d.store.SaveTask(ctx, result.Task); err != nil {
		return Execution{}, err
	}
	if err := d.store.SaveExecution(ctx, execution); err != nil {
		return Execution{}, err
	}
	if err := d.store.AppendEvent(ctx, TaskEvent{TaskID: task.ID, ExecutionID: execution.ID, Type: EventTaskDispatched, From: task.Status, To: result.Task.Status}); err != nil {
		return Execution{}, err
	}
	execCtx, cancel := context.WithCancel(ctx)
	active := &activeExecution{Execution: execution, Cancel: cancel, WorkerID: d.workerID}
	d.active[taskID] = active
	if d.exec != nil {
		go d.execute(execCtx, execution, ExecutionEnvelope{TaskID: result.Task.ID, ProjectID: result.Task.ProjectID, WorktreeID: result.Task.WorktreeID, Attempt: result.Task.AttemptCount, Title: result.Task.Title})
	}
	return execution, nil
}

func (d *Dispatcher) Complete(ctx context.Context, executionID ExecutionID, outcome TaskStatus, evidenceIDs []string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	for taskID, active := range d.active {
		execution := active.Execution
		if execution.ID != executionID {
			continue
		}
		if active.terminal.Load() {
			return fmt.Errorf("%w: execution %q already completed", ErrInvalidDispatch, executionID)
		}
		task, err := d.store.GetTask(ctx, taskID)
		if err != nil {
			return err
		}
		transition := TransitionCollectEvidence
		eventType := EventEvidenceCollected
		to := TaskAwaitingInspection
		if outcome == TaskFailed {
			transition = TransitionExecutionFailed
			eventType = EventExecutionFailed
			to = TaskFailed
		} else if outcome != TaskAwaitingInspection {
			return fmt.Errorf("%w: unsupported completion outcome %q", ErrInvalidDispatch, outcome)
		}
		result, err := ApplyTransition(task, transition)
		if err != nil {
			return err
		}
		active.terminal.Store(true)
		if active.Cancel != nil {
			active.Cancel()
		}
		if err := d.store.SaveTask(ctx, result.Task); err != nil {
			return err
		}
		now := time.Now().UTC()
		execution.Status = to
		execution.FinishedAt = &now
		execution.EvidenceIDs = append([]string(nil), evidenceIDs...)
		if err := d.store.SaveExecution(ctx, execution); err != nil {
			return err
		}
		if err := d.store.AppendEvent(ctx, TaskEvent{TaskID: task.ID, ExecutionID: executionID, Type: eventType, From: task.Status, To: to, EvidenceIDs: evidenceIDs}); err != nil {
			return err
		}
		delete(d.active, taskID)
		return nil
	}
	return ErrNotFound
}

func (d *Dispatcher) Cancel(ctx context.Context, taskID TaskID) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	active, exists := d.active[taskID]
	if !exists {
		return fmt.Errorf("%w: task %q is not active", ErrInvalidDispatch, taskID)
	}
	if active.terminal.Load() {
		return fmt.Errorf("%w: task %q already completed", ErrTerminalTask, taskID)
	}
	active.terminal.Store(true)
	if active.Cancel != nil {
		active.Cancel()
	}
	task, err := d.store.GetTask(ctx, taskID)
	if err != nil {
		return err
	}
	result, err := ApplyTransition(task, TransitionCancel)
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	execution := active.Execution
	execution.Status = TaskCanceled
	execution.FinishedAt = &now
	if err := d.store.SaveExecution(ctx, execution); err != nil {
		return err
	}
	if err := d.store.SaveTask(ctx, result.Task); err != nil {
		return err
	}
	if err := d.store.AppendEvent(ctx, TaskEvent{TaskID: task.ID, Type: EventTaskCanceled, From: task.Status, To: result.Task.Status}); err != nil {
		return err
	}
	delete(d.active, taskID)
	return nil
}

func (d *Dispatcher) execute(ctx context.Context, execution Execution, envelope ExecutionEnvelope) {
	if err := d.exec.Execute(ctx, envelope); err != nil {
		_ = d.Complete(ctx, execution.ID, TaskFailed, nil)
		return
	}
	_ = d.Complete(ctx, execution.ID, TaskAwaitingInspection, []string{"worker-evidence"})
}

// VerifyWorkerIdentity checks if a workerID matches the current active worker for an execution.
func (d *Dispatcher) VerifyWorkerIdentity(taskID TaskID, executionID ExecutionID, workerID string) bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	for tid, active := range d.active {
		if tid == taskID && active.Execution.ID == executionID {
			if d.workerID != "" && workerID != d.workerID {
				return false
			}
			return true
		}
	}
	return false
}
