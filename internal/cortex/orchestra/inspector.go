package orchestra

import (
	"context"
	"errors"
	"fmt"
	"sort"
)

var (
	ErrInspectionRejected  = errors.New("orchestra: inspection rejected")
	ErrMissingEvidence     = errors.New("orchestra: required evidence is missing")
	ErrDirtyWorktree       = errors.New("orchestra: worktree is dirty")
	ErrUnrelatedChanges    = errors.New("orchestra: unrelated changes are present")
	ErrInvalidWorktree     = errors.New("orchestra: worktree state is invalid")
	ErrSelfReportedSuccess = errors.New("orchestra: worker success is not trusted")
)

type WorktreeState struct {
	WorktreeID string
	Revision   string
	Dirty      bool
	Valid      bool
}

type InspectionEvidence struct {
	ID      string
	Kind    string
	Passed  bool
	Summary string
}

type InspectionRequest struct {
	Task         Task
	Execution    Execution
	Worktree     WorktreeState
	Evidence     []InspectionEvidence
	ChangedPaths []string
}

type InspectionResult struct {
	Accepted     bool
	MergeReady   bool
	Reason       string
	AcceptedTask Task
}

type Inspector interface {
	Inspect(context.Context, InspectionRequest) (InspectionResult, error)
}

type MergeAuthority struct{}

func (MergeAuthority) Inspect(ctx context.Context, request InspectionRequest) (InspectionResult, error) {
	if ctx == nil {
		return InspectionResult{}, fmt.Errorf("%w: context is required", ErrInspectionRejected)
	}
	if err := ctx.Err(); err != nil {
		return InspectionResult{}, err
	}
	if err := ValidateTask(request.Task); err != nil {
		return InspectionResult{}, fmt.Errorf("%w: invalid task: %w", ErrInspectionRejected, err)
	}
	if request.Execution.ID == "" || request.Execution.TaskID != request.Task.ID {
		return InspectionResult{}, fmt.Errorf("%w: execution does not belong to task", ErrInspectionRejected)
	}
	if request.Execution.Status == TaskSuccess {
		return InspectionResult{}, fmt.Errorf("%w: execution cannot self-report trusted success", ErrSelfReportedSuccess)
	}
	if request.Worktree.WorktreeID == "" || request.Worktree.WorktreeID != request.Task.WorktreeID || !request.Worktree.Valid {
		return InspectionResult{}, fmt.Errorf("%w: worktree identity or validity check failed", ErrInvalidWorktree)
	}
	if request.Worktree.Dirty {
		return InspectionResult{}, ErrDirtyWorktree
	}
	if len(request.Evidence) == 0 {
		return InspectionResult{}, ErrMissingEvidence
	}

	for _, evidence := range request.Evidence {
		if evidence.ID == "" || evidence.Kind == "" || !evidence.Passed {
			return InspectionResult{}, fmt.Errorf("%w: evidence is incomplete or failed", ErrInspectionRejected)
		}
	}

	paths := append([]string(nil), request.ChangedPaths...)
	sort.Strings(paths)
	for i := 1; i < len(paths); i++ {
		if paths[i] == paths[i-1] {
			return InspectionResult{}, fmt.Errorf("%w: changed path %q is duplicated", ErrUnrelatedChanges, paths[i])
		}
	}

	accepted, err := ApplyTransition(request.Task, TransitionInspectAccepted)
	if err != nil {
		return InspectionResult{}, fmt.Errorf("%w: task is not awaiting inspection: %w", ErrInspectionRejected, err)
	}
	return InspectionResult{
		Accepted:     true,
		MergeReady:   len(paths) >= 0,
		Reason:       "acceptance criteria and governed evidence passed",
		AcceptedTask: accepted.Task,
	}, nil
}
