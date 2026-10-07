package orchestra

import (
	"errors"
	"testing"
)

func validTask() Task {
	return Task{
		ID: "task-1", ProjectID: "project-1", WorktreeID: "worktree-1", Title: "run validation",
		AcceptanceCriteria: []string{"validation passes"}, MaxAttempts: 2, Status: TaskDraft,
	}
}

func TestApplyTransitionLifecycle(t *testing.T) {
	task := validTask()
	for _, step := range []struct {
		transition Transition
		status     TaskStatus
	}{
		{TransitionValidate, TaskReady},
		{TransitionDispatch, TaskRunning},
		{TransitionCollectEvidence, TaskAwaitingInspection},
		{TransitionInspectAccepted, TaskSuccess},
	} {
		result, err := ApplyTransition(task, step.transition)
		if err != nil {
			t.Fatalf("%s: %v", step.transition, err)
		}
		if result.Task.Status != step.status {
			t.Fatalf("%s: got %s, want %s", step.transition, result.Task.Status, step.status)
		}
		task = result.Task
	}
	if task.AttemptCount != 1 {
		t.Fatalf("attempt count = %d, want 1", task.AttemptCount)
	}
}

func TestApplyTransitionRejectsSelfApprovalAndTerminalMutation(t *testing.T) {
	task := validTask()
	if _, err := ApplyTransition(task, TransitionInspectAccepted); !errors.Is(err, ErrInvalidTransition) {
		t.Fatalf("expected invalid transition, got %v", err)
	}

	task.Status = TaskSuccess
	if _, err := ApplyTransition(task, TransitionCancel); !errors.Is(err, ErrTerminalTask) {
		t.Fatalf("expected terminal error, got %v", err)
	}
}

func TestApplyTransitionRetryIsExplicitAndIdempotent(t *testing.T) {
	task := validTask()
	task.Status = TaskFailed
	result, err := ApplyTransition(task, TransitionRetry)
	if err != nil || result.Task.Status != TaskReady || !result.Changed {
		t.Fatalf("retry result = %#v, err = %v", result, err)
	}

	result, err = ApplyTransition(result.Task, TransitionRetry)
	if err == nil || !errors.Is(err, ErrInvalidTransition) {
		t.Fatalf("expected invalid repeated retry, got %#v, %v", result, err)
	}
}

func TestApplyTransitionNoOpIsExplicitlyRejected(t *testing.T) {
	task := validTask()
	task.Status = TaskReady
	result, err := ApplyTransition(task, TransitionValidate)
	if err == nil || !errors.Is(err, ErrInvalidTransition) {
		t.Fatalf("expected no-op transition rejection, got %#v, %v", result, err)
	}
}

func TestValidateTaskBoundsAttempts(t *testing.T) {
	task := validTask()
	task.AttemptCount = 3
	if err := ValidateTask(task); !errors.Is(err, ErrInvalidTask) {
		t.Fatalf("expected invalid attempt count, got %v", err)
	}
}
