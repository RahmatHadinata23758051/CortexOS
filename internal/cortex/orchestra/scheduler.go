package orchestra

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/RahmatHadinata23758051/CortexOS/internal/cortex/harness"
	"github.com/RahmatHadinata23758051/CortexOS/internal/cortex/staff"
)

var (
	ErrDuplicateDispatch = errors.New("orchestra: duplicate dispatch")
	ErrWorkerBusy        = errors.New("orchestra: worker is busy")
	ErrInvalidDispatch   = errors.New("orchestra: invalid dispatch")
	ErrAssignmentQueued  = errors.New("orchestra: task assignment queued, awaiting staff availability")
)

type Executor interface {
	Execute(context.Context, ExecutionEnvelope) error
}

// ExecutionContextProvider enriches a dispatch with governed logical Staff,
// skill, memory, and assignment context. It cannot grant Harness permissions.
type ExecutionContextProvider interface {
	PrepareExecution(context.Context, Task, Execution) (ExecutionEnvelope, error)
}

type ExecutionEnvelope struct {
	TaskID          TaskID                  `json:"taskId"`
	ProjectID       string                  `json:"projectId"`
	WorktreeID      string                  `json:"worktreeId"`
	WorkspaceID     string                  `json:"workspaceId,omitempty"`
	Attempt         int                     `json:"attempt"`
	Title           string                  `json:"title"`
	TraceID         string                  `json:"traceId,omitempty"`
	Staff           *staff.Definition       `json:"staff,omitempty"`
	Skill           *harness.SkillInjection `json:"skill,omitempty"`
	Memory          []AdvisoryMemory        `json:"memory,omitempty"`
	Assignment      AssignmentProvenance    `json:"assignment,omitempty"`
	SelectedAdapter string                  `json:"selectedAdapter,omitempty"`
}

type Dispatcher struct {
	mu         sync.Mutex
	store      TaskStore
	exec       Executor
	active     map[TaskID]*activeExecution
	workerID   string
	staffSched *staff.Scheduler
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

// SetStaffScheduler attaches a staff scheduler for availability-aware dispatch.
// The dispatcher will emit assignment events for audit and will retry queued
// tasks when staff becomes available.
func (d *Dispatcher) SetStaffScheduler(sched *staff.Scheduler) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.staffSched = sched
	if sched != nil {
		sched.SetEventSink(func(event staff.AssignmentEvent) {
			taskID := TaskID(event.Assignment.Request.TaskID)
			var eventType EventType
			switch event.Type {
			case staff.EventAssignmentQueued:
				eventType = EventTaskQueued
			case staff.EventAssignmentAssigned:
				eventType = EventTaskAssigned
			case staff.EventAssignmentRejected:
				eventType = EventTaskAssignmentRejected
			case staff.EventAssignmentReassigned:
				eventType = EventTaskReassigned
			case staff.EventAssignmentCanceled:
				eventType = EventTaskCanceled
			case staff.EventAssignmentReleased:
				return // Clean logical release, no duplicate task event
			default:
				return // Unknown event type, skip
			}
			_ = d.store.AppendEvent(context.Background(), TaskEvent{
				TaskID: taskID, Type: eventType, Message: event.Assignment.Reason,
				OccurredAt: event.OccurredAt,
			})
		})
	}
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
		envelope := ExecutionEnvelope{TaskID: result.Task.ID, ProjectID: result.Task.ProjectID, WorktreeID: result.Task.WorktreeID, Attempt: result.Task.AttemptCount, Title: result.Task.Title, TraceID: string(execution.ID)}
		if provider, ok := d.exec.(ExecutionContextProvider); ok {
			prepared, prepareErr := provider.PrepareExecution(execCtx, result.Task, execution)
			if prepareErr != nil {
				cancel()
				delete(d.active, taskID)
				_ = d.store.SaveTask(ctx, task)
				return Execution{}, prepareErr
			}
			envelope = prepared
		}
		go d.execute(execCtx, execution, envelope)
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
			defer active.Cancel()
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
		if d.staffSched != nil {
			_ = d.staffSched.Release(context.WithoutCancel(ctx), string(task.ID))
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
	if d.staffSched != nil {
		_ = d.staffSched.Cancel(context.WithoutCancel(ctx), string(task.ID))
	}
	delete(d.active, taskID)
	return nil
}

func (d *Dispatcher) execute(ctx context.Context, execution Execution, envelope ExecutionEnvelope) {
	completeCtx := context.WithoutCancel(ctx)
	if err := d.exec.Execute(ctx, envelope); err != nil {
		var evidenceIDs []string
		if collector, ok := d.exec.(interface{ LastEvidenceIDs(ExecutionID) []string }); ok {
			evidenceIDs = collector.LastEvidenceIDs(execution.ID)
		}
		_ = d.Complete(completeCtx, execution.ID, TaskFailed, evidenceIDs)
		return
	}
	evidenceIDs := []string{"worker-evidence"}
	if collector, ok := d.exec.(interface{ LastEvidenceIDs(ExecutionID) []string }); ok {
		if collected := collector.LastEvidenceIDs(execution.ID); len(collected) > 0 {
			evidenceIDs = collected
		}
	}
	_ = d.Complete(completeCtx, execution.ID, TaskAwaitingInspection, evidenceIDs)
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
