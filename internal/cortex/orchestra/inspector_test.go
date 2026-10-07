package orchestra

import (
	"context"
	"errors"
	"testing"
)

func validInspectionRequest() InspectionRequest {
	task := validTask()
	task.Status = TaskAwaitingInspection
	exec := Execution{
		ID:      "task-1-1",
		TaskID:  task.ID,
		Attempt: 1,
		Status:  TaskAwaitingInspection,
	}
	worktree := WorktreeState{
		WorktreeID: task.WorktreeID,
		Revision:   "abc1234",
		Dirty:      false,
		Valid:      true,
	}
	evidence := []InspectionEvidence{
		{ID: "ev-1", Kind: "test-pass", Passed: true, Summary: "all tests passed"},
	}
	return InspectionRequest{
		Task:         task,
		Execution:    exec,
		Worktree:     worktree,
		Evidence:     evidence,
		ChangedPaths: []string{"src/file.go"},
	}
}

func TestMergeAuthorityAcceptsGovernedTask(t *testing.T) {
	authority := MergeAuthority{Capability: MergeCapability}
	req := validInspectionRequest()
	result, err := authority.Inspect(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Accepted || !result.MergeReady || result.AcceptedTask.Status != TaskSuccess {
		t.Fatalf("result = %#v", result)
	}
}

func TestMergeAuthorityRejectsSelfReportedSuccess(t *testing.T) {
	authority := MergeAuthority{Capability: MergeCapability}
	req := validInspectionRequest()
	req.Execution.Status = TaskSuccess
	_, err := authority.Inspect(context.Background(), req)
	if !errors.Is(err, ErrSelfReportedSuccess) {
		t.Fatalf("expected self-reported success error, got %v", err)
	}
}

func TestMergeAuthorityRejectsDirtyAndInvalidWorktrees(t *testing.T) {
	authority := MergeAuthority{Capability: MergeCapability}
	req := validInspectionRequest()
	req.Worktree.Dirty = true
	if _, err := authority.Inspect(context.Background(), req); !errors.Is(err, ErrDirtyWorktree) {
		t.Fatalf("expected dirty worktree error, got %v", err)
	}

	req = validInspectionRequest()
	req.Worktree.Valid = false
	if _, err := authority.Inspect(context.Background(), req); !errors.Is(err, ErrInvalidWorktree) {
		t.Fatalf("expected invalid worktree error, got %v", err)
	}

	req = validInspectionRequest()
	req.Worktree.WorktreeID = "other-worktree"
	if _, err := authority.Inspect(context.Background(), req); !errors.Is(err, ErrInvalidWorktree) {
		t.Fatalf("expected mismatched worktree error, got %v", err)
	}
}

func TestMergeAuthorityRejectsMissingAndFailedEvidence(t *testing.T) {
	authority := MergeAuthority{Capability: MergeCapability}
	req := validInspectionRequest()
	req.Evidence = nil
	if _, err := authority.Inspect(context.Background(), req); !errors.Is(err, ErrMissingEvidence) {
		t.Fatalf("expected missing evidence error, got %v", err)
	}

	req = validInspectionRequest()
	req.Evidence = []InspectionEvidence{{ID: "ev-fail", Kind: "test-pass", Passed: false}}
	if _, err := authority.Inspect(context.Background(), req); !errors.Is(err, ErrInspectionRejected) {
		t.Fatalf("expected rejected evidence error, got %v", err)
	}
}

func TestMergeAuthorityRejectsTasksNotInAwaitingInspection(t *testing.T) {
	authority := MergeAuthority{Capability: MergeCapability}
	req := validInspectionRequest()
	req.Task.Status = TaskRunning
	if _, err := authority.Inspect(context.Background(), req); !errors.Is(err, ErrInspectionRejected) {
		t.Fatalf("expected inspection rejected error, got %v", err)
	}
}
