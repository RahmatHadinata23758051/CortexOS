package orchestra

import (
	"errors"
	"fmt"
)

var (
	ErrInvalidTask       = errors.New("orchestra: invalid task")
	ErrInvalidTransition = errors.New("orchestra: invalid task transition")
	ErrTerminalTask      = errors.New("orchestra: task is terminal")
)

type TransitionResult struct {
	Task    Task
	Changed bool
}

func ValidateTask(task Task) error {
	if task.ID == "" || task.ProjectID == "" || task.WorktreeID == "" || task.Title == "" {
		return fmt.Errorf("%w: id, project, worktree, and title are required", ErrInvalidTask)
	}
	if len(task.AcceptanceCriteria) == 0 {
		return fmt.Errorf("%w: at least one acceptance criterion is required", ErrInvalidTask)
	}
	if task.MaxAttempts < 1 {
		return fmt.Errorf("%w: max attempts must be positive", ErrInvalidTask)
	}
	if task.AttemptCount < 0 || task.AttemptCount > task.MaxAttempts {
		return fmt.Errorf("%w: attempt count is outside the allowed range", ErrInvalidTask)
	}
	if task.SchemaVersion != "" && task.SchemaVersion != ContractVersion {
		return fmt.Errorf("%w: unsupported schema version", ErrInvalidTask)
	}
	return nil
}

func ApplyTransition(task Task, transition Transition) (TransitionResult, error) {
	if err := ValidateTask(task); err != nil {
		return TransitionResult{}, err
	}
	if task.Status == "" {
		task.Status = TaskDraft
	}
	if task.SchemaVersion == "" {
		task.SchemaVersion = ContractVersion
	}

	to, ok := transitionTarget(task.Status, transition)
	if !ok {
		if isTerminal(task.Status) {
			return TransitionResult{}, fmt.Errorf("%w: %s cannot accept %s", ErrTerminalTask, task.Status, transition)
		}
		return TransitionResult{}, fmt.Errorf("%w: %s cannot accept %s", ErrInvalidTransition, task.Status, transition)
	}
	if to == task.Status {
		return TransitionResult{}, fmt.Errorf("%w: %s cannot accept %s (no-op)", ErrInvalidTransition, task.Status, transition)
	}
	if transition == TransitionDispatch {
		task.AttemptCount++
	}
	task.Status = to
	return TransitionResult{Task: task, Changed: true}, nil
}

func transitionTarget(status TaskStatus, transition Transition) (TaskStatus, bool) {
	switch transition {
	case TransitionValidate:
		return TaskReady, status == TaskDraft
	case TransitionDispatch:
		return TaskRunning, status == TaskReady
	case TransitionCollectEvidence:
		return TaskAwaitingInspection, status == TaskRunning
	case TransitionInspectAccepted:
		return TaskSuccess, status == TaskAwaitingInspection
	case TransitionInspectRejected, TransitionExecutionFailed:
		return TaskFailed, status == TaskAwaitingInspection || status == TaskRunning
	case TransitionCancel:
		return TaskCanceled, status == TaskDraft || status == TaskReady || status == TaskRunning
	case TransitionRetry:
		return TaskReady, status == TaskFailed
	default:
		return "", false
	}
}

func isTerminal(status TaskStatus) bool {
	return status == TaskSuccess || status == TaskCanceled
}
