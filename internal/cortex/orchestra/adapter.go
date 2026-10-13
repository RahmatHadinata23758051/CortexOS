package orchestra

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/RahmatHadinata23758051/CortexOS/internal/cortex/harness"
	"github.com/RahmatHadinata23758051/CortexOS/internal/logging"
)

// WorktreeInspector abstracts worktree verification (e.g. from workspace.Service).
type WorktreeInspector interface {
	InspectWorktreeState(ctx context.Context, worktreeID string) (WorktreeState, error)
}

// HarnessEvidenceReader provides evidence lookup for task executions.
type HarnessEvidenceReader interface {
	EvidenceForExecution(ctx context.Context, executionID string) ([]harness.EvidenceRecord, error)
}

// ProductionAdapterConfig holds dependencies for the production Orchestra adapter.
type ProductionAdapterConfig struct {
	TaskStore         TaskStore
	HarnessStore      HarnessEvidenceReader
	WorktreeInspector WorktreeInspector
	Inspector         Inspector
	Logger            logging.Logger
}

// ProductionAdapter implements application.OrchestraPort using real SQLite persistence,
// Harness evidence validation, worktree cleanliness verification, and Inspector acceptance (ADR-0005).
// MergeTask is serialized per-task to prevent concurrent merge races.
type ProductionAdapter struct {
	tasks       TaskStore
	harness     HarnessEvidenceReader
	worktrees   WorktreeInspector
	inspector   Inspector
	logger      logging.Logger
	mergeMu     sync.Mutex
	activeMerge map[TaskID]struct{}
}

// NewProductionAdapter constructs the production Orchestra adapter.
func NewProductionAdapter(config ProductionAdapterConfig) (*ProductionAdapter, error) {
	if config.TaskStore == nil {
		return nil, errors.New("task store is required")
	}
	if config.HarnessStore == nil {
		return nil, errors.New("harness evidence store is required")
	}
	if config.WorktreeInspector == nil {
		return nil, errors.New("worktree inspector is required")
	}
	if config.Inspector == nil {
		config.Inspector = MergeAuthority{Capability: MergeCapability}
	}
	if config.Logger == nil {
		config.Logger = logging.NopLogger{}
	}
	return &ProductionAdapter{
		tasks:       config.TaskStore,
		harness:     config.HarnessStore,
		worktrees:   config.WorktreeInspector,
		inspector:   config.Inspector,
		logger:      config.Logger,
		activeMerge: make(map[TaskID]struct{}),
	}, nil
}

// ListTasks returns all tasks from the durable store.
func (a *ProductionAdapter) ListTasks(ctx context.Context) ([]Task, error) {
	if a == nil || a.tasks == nil {
		return nil, ErrStoreUnavailable
	}
	return a.tasks.ListTasks(ctx)
}

// GetTask returns a single task by ID.
func (a *ProductionAdapter) GetTask(ctx context.Context, id TaskID) (Task, error) {
	if a == nil || a.tasks == nil {
		return Task{}, ErrStoreUnavailable
	}
	return a.tasks.GetTask(ctx, id)
}

// ListEvents returns the audit event timeline for a task.
func (a *ProductionAdapter) ListEvents(ctx context.Context, id TaskID) ([]TaskEvent, error) {
	if a == nil || a.tasks == nil {
		return nil, ErrStoreUnavailable
	}
	return a.tasks.ListEvents(ctx, id)
}

// CancelTask cancels an active or draft task.
func (a *ProductionAdapter) CancelTask(ctx context.Context, id TaskID) error {
	if a == nil || a.tasks == nil {
		return ErrStoreUnavailable
	}
	task, err := a.tasks.GetTask(ctx, id)
	if err != nil {
		return err
	}
	res, err := ApplyTransition(task, TransitionCancel)
	if err != nil {
		return err
	}
	if err := a.tasks.SaveTask(ctx, res.Task); err != nil {
		return err
	}
	return a.tasks.AppendEvent(ctx, TaskEvent{
		TaskID:     id,
		Type:       EventTaskCanceled,
		From:       task.Status,
		To:         res.Task.Status,
		Message:    "task canceled by operator request",
		OccurredAt: time.Now().UTC(),
	})
}

// RetryTask restarts a failed task.
func (a *ProductionAdapter) RetryTask(ctx context.Context, id TaskID) error {
	if a == nil || a.tasks == nil {
		return ErrStoreUnavailable
	}
	task, err := a.tasks.GetTask(ctx, id)
	if err != nil {
		return err
	}
	res, err := ApplyTransition(task, TransitionRetry)
	if err != nil {
		return err
	}
	if err := a.tasks.SaveTask(ctx, res.Task); err != nil {
		return err
	}
	return a.tasks.AppendEvent(ctx, TaskEvent{
		TaskID:     id,
		Type:       EventTaskRetryRequested,
		From:       task.Status,
		To:         res.Task.Status,
		Message:    "task retry requested",
		OccurredAt: time.Now().UTC(),
	})
}

// MergeTask governs authoritative task completion and plan unblocking (ADR-0005).
// It verifies:
// 1. Task is currently in TaskAwaitingInspection.
// 2. Worktree is clean and valid.
// 3. Governed audit evidence is present and passed.
// 4. MergeAuthority accepts the change.
// On acceptance:
// 1. Task transitions to TaskSuccess and is persisted in SQLite.
// 2. EventInspectionAccepted is appended to the timeline.
// 3. Plan graph is updated: dependent draft tasks whose dependencies have all completed become TaskReady.
func (a *ProductionAdapter) MergeTask(ctx context.Context, id TaskID, note string) (Task, error) {
	if a == nil || a.tasks == nil {
		return Task{}, ErrStoreUnavailable
	}
	if ctx == nil {
		return Task{}, fmt.Errorf("%w: context is required", ErrInspectionRejected)
	}
	if err := ctx.Err(); err != nil {
		return Task{}, err
	}

	startTime := time.Now()
	a.logger.InfoContext(ctx, "merge attempt started",
		"event", "orchestra_merge_started",
		"task_id", id,
		"note", note,
	)

	// In-process lock per task ID to prevent duplicate concurrent merge requests
	a.mergeMu.Lock()
	if _, running := a.activeMerge[id]; running {
		a.mergeMu.Unlock()
		a.logger.WarnContext(ctx, "merge already in progress",
			"event", "orchestra_merge_conflict",
			"task_id", id,
		)
		return Task{}, fmt.Errorf("%w: merge already in progress for task %s", ErrInspectionRejected, id)
	}
	a.activeMerge[id] = struct{}{}
	a.mergeMu.Unlock()

	defer func() {
		a.mergeMu.Lock()
		delete(a.activeMerge, id)
		a.mergeMu.Unlock()
	}()

	task, err := a.tasks.GetTask(ctx, id)
	if err != nil {
		a.logger.ErrorContext(ctx, "failed to load task for merge",
			"event", "orchestra_merge_load_failed",
			"task_id", id,
			"error", err.Error(),
			"duration_ms", time.Since(startTime).Milliseconds(),
		)
		return Task{}, err
	}

	if task.Status != TaskAwaitingInspection {
		a.logger.WarnContext(ctx, "invalid task status for merge",
			"event", "orchestra_merge_invalid_status",
			"task_id", id,
			"current_status", task.Status,
			"expected_status", TaskAwaitingInspection,
			"duration_ms", time.Since(startTime).Milliseconds(),
		)
		return Task{}, fmt.Errorf("%w: task status is %s, expected %s", ErrInspectionRejected, task.Status, TaskAwaitingInspection)
	}

	// 1. Resolve worktree state (strictly required)
	var wtState WorktreeState
	if task.WorktreeID == "" {
		a.logger.ErrorContext(ctx, "task missing worktree id",
			"event", "orchestra_merge_missing_worktree",
			"task_id", id,
			"duration_ms", time.Since(startTime).Milliseconds(),
		)
		return Task{}, fmt.Errorf("%w: task worktree id is missing", ErrInvalidWorktree)
	}
	state, wtErr := a.worktrees.InspectWorktreeState(ctx, task.WorktreeID)
	if wtErr != nil {
		a.logger.ErrorContext(ctx, "worktree inspection failed",
			"event", "orchestra_merge_worktree_inspect_failed",
			"task_id", id,
			"worktree_id", task.WorktreeID,
			"error", wtErr.Error(),
			"duration_ms", time.Since(startTime).Milliseconds(),
		)
		return Task{}, fmt.Errorf("%w: inspect worktree: %w", ErrInvalidWorktree, wtErr)
	}
	wtState = state
	a.logger.DebugContext(ctx, "worktree state resolved",
		"event", "orchestra_merge_worktree_resolved",
		"task_id", id,
		"worktree_id", task.WorktreeID,
		"worktree_revision", wtState.Revision,
		"worktree_dirty", wtState.Dirty,
		"worktree_valid", wtState.Valid,
		"duration_ms", time.Since(startTime).Milliseconds(),
	)

	// 2. Resolve execution and evidence
	exec, evidenceList, err := a.resolveExecutionAndEvidence(ctx, task)
	if err != nil {
		a.logger.ErrorContext(ctx, "evidence resolution failed",
			"event", "orchestra_merge_evidence_failed",
			"task_id", id,
			"error", err.Error(),
			"duration_ms", time.Since(startTime).Milliseconds(),
		)
		return Task{}, err
	}
	if exec.ID == "" {
		a.logger.ErrorContext(ctx, "execution record not found",
			"event", "orchestra_merge_no_execution",
			"task_id", id,
			"duration_ms", time.Since(startTime).Milliseconds(),
		)
		return Task{}, fmt.Errorf("%w: execution record not found for merge", ErrInspectionRejected)
	}
	a.logger.DebugContext(ctx, "execution and evidence resolved",
		"event", "orchestra_merge_evidence_resolved",
		"task_id", id,
		"execution_id", exec.ID,
		"evidence_count", len(evidenceList),
		"duration_ms", time.Since(startTime).Milliseconds(),
	)

	// 3. Invoke Inspector (fail-closed if authority or evidence rejected)
	inspReq := InspectionRequest{
		Task:         task,
		Execution:    exec,
		Worktree:     wtState,
		Evidence:     evidenceList,
		ChangedPaths: []string{},
	}

	result, err := a.inspector.Inspect(ctx, inspReq)
	if err != nil {
		rejectionEventErr := a.tasks.AppendEvent(ctx, TaskEvent{
			TaskID:      id,
			ExecutionID: exec.ID,
			Type:        EventInspectionRejected,
			From:        task.Status,
			To:          task.Status,
			Message:     fmt.Sprintf("inspection rejected: %v", err),
			OccurredAt:  time.Now().UTC(),
		})
		if rejectionEventErr != nil {
			a.logger.ErrorContext(ctx, "persist inspection rejection event failed",
				"event", "orchestra_merge_rejection_persist_failed",
				"task_id", id,
				"execution_id", exec.ID,
				"error", rejectionEventErr.Error(),
			)
			return Task{}, errors.Join(err, fmt.Errorf("persist inspection rejection event: %w", rejectionEventErr))
		}
		a.logger.WarnContext(ctx, "inspection rejected with error",
			"event", "orchestra_merge_rejected_error",
			"task_id", id,
			"execution_id", exec.ID,
			"error", err.Error(),
			"duration_ms", time.Since(startTime).Milliseconds(),
		)
		return Task{}, err
	}

	if !result.Accepted {
		rejectionErr := ErrInspectionRejected
		if result.Reason != "" {
			rejectionErr = fmt.Errorf("%w: %s", rejectionErr, result.Reason)
		}
		if eventErr := a.tasks.AppendEvent(ctx, TaskEvent{
			TaskID: id, ExecutionID: exec.ID, Type: EventInspectionRejected,
			From: task.Status, To: task.Status, Message: rejectionErr.Error(), OccurredAt: time.Now().UTC(),
		}); eventErr != nil {
			a.logger.ErrorContext(ctx, "persist inspection rejection event failed",
				"event", "orchestra_merge_rejection_persist_failed",
				"task_id", id,
				"execution_id", exec.ID,
				"error", eventErr.Error(),
			)
			return Task{}, errors.Join(rejectionErr, fmt.Errorf("persist inspection rejection event: %w", eventErr))
		}
		a.logger.WarnContext(ctx, "inspection rejected",
			"event", "orchestra_merge_rejected",
			"task_id", id,
			"execution_id", exec.ID,
			"reason", result.Reason,
			"evidence_count", len(evidenceList),
			"duration_ms", time.Since(startTime).Milliseconds(),
		)
		return Task{}, rejectionErr
	}

	// 4. Build atomic merge payload (merged task + event + unblocked dependent tasks + events)
	successTask := result.AcceptedTask
	successTask.UpdatedAt = time.Now().UTC()

	evMsg := "inspection accepted and merged"
	if note != "" {
		evMsg = fmt.Sprintf("inspection accepted: %s", note)
	}
	evidenceIDs := make([]string, 0, len(evidenceList))
	for _, ev := range evidenceList {
		evidenceIDs = append(evidenceIDs, ev.ID)
	}

	acceptedEvent := TaskEvent{
		TaskID:      id,
		ExecutionID: exec.ID,
		Type:        EventInspectionAccepted,
		From:        task.Status,
		To:          TaskSuccess,
		Message:     evMsg,
		EvidenceIDs: evidenceIDs,
		OccurredAt:  time.Now().UTC(),
	}

	unblockedTasks, validatedEvents, err := a.calculatePlanUnblocking(ctx, task.ProjectID, successTask.ID)
	if err != nil {
		a.logger.ErrorContext(ctx, "plan unblocking calculation failed",
			"event", "orchestra_merge_unblocking_failed",
			"task_id", id,
			"error", err.Error(),
			"duration_ms", time.Since(startTime).Milliseconds(),
		)
		return Task{}, fmt.Errorf("calculate plan unblocking: %w", err)
	}

	if len(unblockedTasks) > 0 {
		a.logger.InfoContext(ctx, "plan unblocking calculated",
			"event", "orchestra_merge_unblocked_tasks",
			"task_id", id,
			"unblocked_count", len(unblockedTasks),
			"unblocked_task_ids", taskIDsToStrings(unblockedTasks),
			"duration_ms", time.Since(startTime).Milliseconds(),
		)
	}

	commitPayload := MergeCommit{
		MergedTask:      successTask,
		AcceptedEvent:   acceptedEvent,
		UnblockedTasks:  unblockedTasks,
		ValidatedEvents: validatedEvents,
	}

	// 5. If store supports atomic CommitMerge, execute under single transaction
	if committer, ok := a.tasks.(MergeCommitter); ok {
		if err := committer.CommitMerge(ctx, commitPayload); err != nil {
			a.logger.ErrorContext(ctx, "atomic commit merge failed",
				"event", "orchestra_merge_atomic_commit_failed",
				"task_id", id,
				"error", err.Error(),
				"duration_ms", time.Since(startTime).Milliseconds(),
			)
			return Task{}, fmt.Errorf("atomic commit merge: %w", err)
		}
		a.logger.InfoContext(ctx, "merge committed atomically",
			"event", "orchestra_merge_committed",
			"task_id", id,
			"execution_id", exec.ID,
			"unblocked_count", len(unblockedTasks),
			"evidence_count", len(evidenceList),
			"duration_ms", time.Since(startTime).Milliseconds(),
		)
		return successTask, nil
	}

	// Fallback for non-atomic stores (e.g. testing doubles without MergeCommitter)
	if err := a.tasks.SaveTask(ctx, successTask); err != nil {
		a.logger.ErrorContext(ctx, "persist accepted task failed",
			"event", "orchestra_merge_persist_task_failed",
			"task_id", id,
			"error", err.Error(),
			"duration_ms", time.Since(startTime).Milliseconds(),
		)
		return Task{}, fmt.Errorf("persist accepted task: %w", err)
	}
	if err := a.tasks.AppendEvent(ctx, acceptedEvent); err != nil {
		a.logger.ErrorContext(ctx, "append inspection accepted event failed",
			"event", "orchestra_merge_persist_event_failed",
			"task_id", id,
			"error", err.Error(),
			"duration_ms", time.Since(startTime).Milliseconds(),
		)
		return Task{}, fmt.Errorf("append inspection accepted event: %w", err)
	}
	for i, ut := range unblockedTasks {
		if err := a.tasks.SaveTask(ctx, ut); err != nil {
			a.logger.ErrorContext(ctx, "persist unblocked task failed",
				"event", "orchestra_merge_persist_unblocked_failed",
				"task_id", id,
				"unblocked_task_id", ut.ID,
				"error", err.Error(),
				"duration_ms", time.Since(startTime).Milliseconds(),
			)
			return successTask, fmt.Errorf("persist unblocked task %s: %w", ut.ID, err)
		}
		if err := a.tasks.AppendEvent(ctx, validatedEvents[i]); err != nil {
			a.logger.ErrorContext(ctx, "append validated event failed",
				"event", "orchestra_merge_persist_validated_failed",
				"task_id", id,
				"unblocked_task_id", ut.ID,
				"error", err.Error(),
				"duration_ms", time.Since(startTime).Milliseconds(),
			)
			return successTask, fmt.Errorf("append validated event for %s: %w", ut.ID, err)
		}
	}

	a.logger.InfoContext(ctx, "merge committed (fallback)",
		"event", "orchestra_merge_committed_fallback",
		"task_id", id,
		"execution_id", exec.ID,
		"unblocked_count", len(unblockedTasks),
		"evidence_count", len(evidenceList),
		"duration_ms", time.Since(startTime).Milliseconds(),
	)
	return successTask, nil
}

func taskIDsToStrings(tasks []Task) []string {
	result := make([]string, 0, len(tasks))
	for _, t := range tasks {
		result = append(result, string(t.ID))
	}
	return result
}

// calculatePlanUnblocking determines which draft tasks become ready when the merged task succeeds.
func (a *ProductionAdapter) calculatePlanUnblocking(ctx context.Context, projectID string, newlySucceededID TaskID) ([]Task, []TaskEvent, error) {
	tasks, err := a.tasks.ListTasks(ctx)
	if err != nil {
		return nil, nil, err
	}

	var projectTasks []Task
	for _, t := range tasks {
		if projectID == "" || t.ProjectID == projectID {
			projectTasks = append(projectTasks, t)
		}
	}

	if len(projectTasks) == 0 {
		return nil, nil, nil
	}

	// Map current statuses, setting newlySucceededID to TaskSuccess
	statusMap := make(map[TaskID]TaskStatus, len(projectTasks))
	for _, t := range projectTasks {
		if t.ID == newlySucceededID {
			statusMap[t.ID] = TaskSuccess
		} else {
			statusMap[t.ID] = t.Status
		}
	}

	var unblockedTasks []Task
	var validatedEvents []TaskEvent
	now := time.Now().UTC()

	for _, t := range projectTasks {
		if t.Status != TaskDraft || len(t.Dependencies) == 0 {
			continue
		}

		allDone := true
		for _, depID := range t.Dependencies {
			if statusMap[depID] != TaskSuccess {
				allDone = false
				break
			}
		}

		if allDone {
			res, transErr := ApplyTransition(t, TransitionValidate)
			if transErr != nil {
				continue
			}
			res.Task.UpdatedAt = now
			unblockedTasks = append(unblockedTasks, res.Task)
			validatedEvents = append(validatedEvents, TaskEvent{
				TaskID:     t.ID,
				Type:       EventTaskValidated,
				From:       t.Status,
				To:         TaskReady,
				Message:    "all dependencies completed; task unblocked and ready",
				OccurredAt: now,
			})
		}
	}

	return unblockedTasks, validatedEvents, nil
}

func (a *ProductionAdapter) resolveExecutionAndEvidence(ctx context.Context, task Task) (Execution, []InspectionEvidence, error) {
	// Standard dispatcher convention is "{taskID}-{attemptCount}"
	candidateIDs := []ExecutionID{
		ExecutionID(fmt.Sprintf("%s-%d", task.ID, task.AttemptCount)),
		ExecutionID(fmt.Sprintf("%s-exec-%d", task.ID, task.AttemptCount)),
	}

	var exec Execution
	var found bool
	for _, cid := range candidateIDs {
		if e, err := a.tasks.GetExecution(ctx, cid); err == nil {
			exec = e
			found = true
			break
		}
	}

	if !found {
		// Fallback to searching events timeline for recorded execution ID
		events, _ := a.tasks.ListEvents(ctx, task.ID)
		for i := len(events) - 1; i >= 0; i-- {
			if events[i].ExecutionID != "" {
				if e, err := a.tasks.GetExecution(ctx, events[i].ExecutionID); err == nil {
					exec = e
					found = true
					break
				}
			}
		}
	}

	if !found {
		return Execution{}, nil, fmt.Errorf("%w: execution record for task %s not found in store", ErrInspectionRejected, task.ID)
	}

	records, hErr := a.harness.EvidenceForExecution(ctx, string(exec.ID))
	if hErr != nil {
		return Execution{}, nil, fmt.Errorf("harness evidence lookup failed: %w", hErr)
	}
	if len(records) == 0 {
		return Execution{}, nil, fmt.Errorf("%w: no governed audit evidence recorded for execution %s", ErrMissingEvidence, exec.ID)
	}

	evidenceList := make([]InspectionEvidence, 0, len(records))
	executionEvidenceSet := make(map[string]struct{}, len(exec.EvidenceIDs))
	for _, eid := range exec.EvidenceIDs {
		executionEvidenceSet[eid] = struct{}{}
	}
	if len(executionEvidenceSet) == 0 {
		return Execution{}, nil, fmt.Errorf("%w: execution %s has no pinned evidence IDs", ErrMissingEvidence, exec.ID)
	}
	for _, rec := range records {
		if rec.ExecutionID != string(exec.ID) {
			return Execution{}, nil, fmt.Errorf("%w: evidence %s execution mismatch", ErrInspectionRejected, rec.ID)
		}
		if rec.TaskID != string(task.ID) {
			return Execution{}, nil, fmt.Errorf("%w: evidence %s task mismatch", ErrInspectionRejected, rec.ID)
		}
		if rec.WorktreeID != task.WorktreeID {
			return Execution{}, nil, fmt.Errorf("%w: evidence %s worktree mismatch", ErrInspectionRejected, rec.ID)
		}
		if rec.Kind == "" {
			return Execution{}, nil, fmt.Errorf("%w: evidence %s missing kind", ErrInspectionRejected, rec.ID)
		}
		if _, ok := executionEvidenceSet[rec.ID]; !ok {
			return Execution{}, nil, fmt.Errorf("%w: evidence %s is not pinned on execution", ErrInspectionRejected, rec.ID)
		}
		evidenceList = append(evidenceList, InspectionEvidence{
			ID: rec.ID, Kind: string(rec.Kind), Passed: true, Summary: string(rec.Kind),
		})
	}

	// Exact-set check: every pinned evidence ID must appear in the governed records
	if len(evidenceList) != len(executionEvidenceSet) {
		return Execution{}, nil, fmt.Errorf("%w: execution %s evidence count mismatch (pinned=%d, governed=%d)", ErrInspectionRejected, exec.ID, len(executionEvidenceSet), len(evidenceList))
	}

	return exec, evidenceList, nil
}

// SortTasks returns tasks sorted by TaskID.
func SortTasks(tasks []Task) {
	sort.Slice(tasks, func(i, j int) bool { return tasks[i].ID < tasks[j].ID })
}
