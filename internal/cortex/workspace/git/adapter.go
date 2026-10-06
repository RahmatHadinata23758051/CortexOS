package git

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/RahmatHadinata23758051/CortexOS/internal/cortex/workspace"
)

var branchNamePattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._/-]{0,99}$`)

var (
	errInvalidRepository = workspace.NewError(workspace.ErrNotGitRepository, "path is not a Git repository")
	errDirtyWorktree     = workspace.NewError(workspace.ErrConflict, "worktree has uncommitted changes")
	errActiveWorktree    = workspace.NewError(workspace.ErrConflict, "worktree has an active reference")
)

type Adapter struct {
	runner CommandRunner
}

func New() *Adapter { return &Adapter{runner: defaultRunner()} }

func NewWithRunner(runner CommandRunner) (*Adapter, error) {
	if runner == nil {
		return nil, errors.New("git runner is required")
	}
	return &Adapter{runner: runner}, nil
}

type repositoryFacts struct {
	Root   string
	Branch string
}

func (a *Adapter) InspectRepository(ctx context.Context, root string) (repositoryFacts, error) {
	canonicalRoot, err := workspace.CanonicalRoot(root)
	if err != nil {
		return repositoryFacts{}, err
	}
	stdout, stderr, err := a.run(ctx, "-C", canonicalRoot, "rev-parse", "--show-toplevel")
	if err != nil {
		return repositoryFacts{}, classifyGitError(stderr, err)
	}
	actualRoot, err := workspace.CanonicalRoot(strings.TrimSpace(string(stdout)))
	if err != nil || !samePath(actualRoot, canonicalRoot) {
		return repositoryFacts{}, errInvalidRepository
	}
	branch, err := a.currentBranch(ctx, canonicalRoot)
	if err != nil {
		return repositoryFacts{}, err
	}
	return repositoryFacts{Root: actualRoot, Branch: branch}, nil
}

func (a *Adapter) currentBranch(ctx context.Context, root string) (string, error) {
	stdout, stderr, err := a.run(ctx, "-C", root, "symbolic-ref", "--quiet", "--short", "HEAD")
	if err != nil {
		if strings.TrimSpace(string(stderr)) != "" {
			return "", classifyGitError(stderr, err)
		}
		return "HEAD", nil
	}
	branch := strings.TrimSpace(string(stdout))
	if branch == "" {
		return "HEAD", nil
	}
	return branch, nil
}

func (a *Adapter) run(ctx context.Context, args ...string) ([]byte, []byte, error) {
	if ctx == nil {
		return nil, nil, workspace.NewError(workspace.ErrInvalidRequest, "context is required")
	}
	if err := ctx.Err(); err != nil {
		return nil, nil, workspace.CanceledError(err)
	}
	stdout, stderr, err := a.runner.Run(ctx, args...)
	if err != nil && (errors.Is(ctx.Err(), context.Canceled) || errors.Is(ctx.Err(), context.DeadlineExceeded)) {
		return nil, stderr, workspace.CanceledError(ctx.Err())
	}
	return stdout, stderr, err
}

func classifyGitError(stderr []byte, cause error) error {
	message := strings.ToLower(strings.TrimSpace(string(stderr)))
	switch {
	case strings.Contains(message, "not a git repository"):
		return workspace.WrapError(workspace.ErrNotGitRepository, "path is not a Git repository", cause)
	case strings.Contains(message, "already exists"), strings.Contains(message, "is already checked out"):
		return workspace.WrapError(workspace.ErrConflict, "Git worktree conflicts with existing state", cause)
	default:
		return workspace.WrapError(workspace.ErrInternal, "Git operation failed", cause)
	}
}

func validateBranch(branch string) error {
	if branch == "" || branch == "HEAD" || !branchNamePattern.MatchString(branch) ||
		strings.Contains(branch, "..") || strings.HasSuffix(branch, ".") || strings.HasSuffix(branch, "/") ||
		strings.Contains(branch, "@{") {
		return workspace.NewError(workspace.ErrInvalidRequest, "branch name is invalid")
	}
	return nil
}

func samePath(left, right string) bool {
	if filepath.VolumeName(left) != filepath.VolumeName(right) {
		return false
	}
	if filepath.Separator == '\\' {
		return strings.EqualFold(filepath.Clean(left), filepath.Clean(right))
	}
	return filepath.Clean(left) == filepath.Clean(right)
}

func formatGitFailure(operation string, stderr []byte, cause error) error {
	if cause == nil {
		cause = errors.New("unknown git failure")
	}
	if strings.TrimSpace(string(stderr)) == "" {
		return workspace.WrapError(workspace.ErrInternal, fmt.Sprintf("Git %s failed", operation), cause)
	}
	return workspace.WrapError(workspace.ErrInternal, fmt.Sprintf("Git %s failed", operation), cause)
}
