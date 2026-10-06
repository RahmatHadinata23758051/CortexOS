package git

import (
	"bytes"
	"context"
	"errors"
	"os/exec"
)

// CommandRunner is the deliberately narrow process boundary used by the Git
// adapter. Implementations must execute the fixed Git executable without a
// shell and must return stdout/stderr separately for safe parsing.
type CommandRunner interface {
	Run(context.Context, ...string) (stdout []byte, stderr []byte, err error)
}

type execRunner struct{}

func (execRunner) Run(ctx context.Context, args ...string) ([]byte, []byte, error) {
	if ctx == nil {
		return nil, nil, errors.New("git command context is required")
	}
	command := exec.CommandContext(ctx, "git", args...)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = &stderr
	err := command.Run()
	return stdout.Bytes(), stderr.Bytes(), err
}

func defaultRunner() CommandRunner { return execRunner{} }
