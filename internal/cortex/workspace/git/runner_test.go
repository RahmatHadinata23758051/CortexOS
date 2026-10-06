package git

import (
	"context"
	"testing"
)

type recordingRunner struct {
	args []string
}

func (r *recordingRunner) Run(_ context.Context, args ...string) ([]byte, []byte, error) {
	r.args = append([]string(nil), args...)
	return nil, nil, nil
}

func TestRunnerUsesArgumentVector(t *testing.T) {
	t.Parallel()

	runner := &recordingRunner{}
	if _, _, err := runner.Run(context.Background(), "status", "--porcelain"); err != nil {
		t.Fatal(err)
	}
	if len(runner.args) != 2 || runner.args[0] != "status" || runner.args[1] != "--porcelain" {
		t.Fatalf("args = %#v", runner.args)
	}
}

func TestExecRunnerHonorsCancellation(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, _, err := (execRunner{}).Run(ctx, "version")
	if err == nil {
		t.Fatal("expected canceled command error")
	}
}
