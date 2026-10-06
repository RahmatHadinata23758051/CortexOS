package git

import (
	"context"
	"errors"
	"testing"

	"github.com/RahmatHadinata23758051/CortexOS/internal/cortex/workspace"
)

type scriptedRunner struct {
	steps []scriptedStep
}

type scriptedStep struct {
	stdout string
	stderr string
	err    error
}

func (r *scriptedRunner) Run(_ context.Context, _ ...string) ([]byte, []byte, error) {
	if len(r.steps) == 0 {
		return nil, nil, errors.New("unexpected Git call")
	}
	step := r.steps[0]
	r.steps = r.steps[1:]
	return []byte(step.stdout), []byte(step.stderr), step.err
}

func TestInspectRepositoryParsesFacts(t *testing.T) {
	t.Parallel()

	runner := &scriptedRunner{steps: []scriptedStep{
		{stdout: "C:\\workspace\\repo\n"},
		{stdout: "feature/demo\n"},
	}}
	adapter, err := NewWithRunner(runner)
	if err != nil {
		t.Fatal(err)
	}
	facts, err := adapter.InspectRepository(context.Background(), `C:\workspace\repo`)
	if err != nil {
		t.Fatal(err)
	}
	if facts.Branch != "feature/demo" {
		t.Fatalf("branch = %q", facts.Branch)
	}
}

func TestInspectRepositoryRejectsNonGitOutput(t *testing.T) {
	t.Parallel()

	runner := &scriptedRunner{steps: []scriptedStep{{stderr: "fatal: not a git repository\n", err: errors.New("exit status 128")}}}
	adapter, err := NewWithRunner(runner)
	if err != nil {
		t.Fatal(err)
	}
	_, err = adapter.InspectRepository(context.Background(), `C:\workspace\repo`)
	if workspace.ErrorCodeOf(err) != workspace.ErrNotGitRepository {
		t.Fatalf("error code = %q, want %q", workspace.ErrorCodeOf(err), workspace.ErrNotGitRepository)
	}
}

func TestValidateBranchRejectsUnsafeNames(t *testing.T) {
	t.Parallel()

	for _, branch := range []string{"", "HEAD", "../escape", "feature..bad", "feature/", "feature@{1}", "feature name"} {
		if workspace.ErrorCodeOf(validateBranch(branch)) != workspace.ErrInvalidRequest {
			t.Errorf("branch %q was accepted", branch)
		}
	}
	if err := validateBranch("feature/safe-1"); err != nil {
		t.Fatalf("safe branch rejected: %v", err)
	}
}
