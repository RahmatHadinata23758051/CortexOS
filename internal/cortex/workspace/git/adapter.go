package git

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/RahmatHadinata23758051/CortexOS/internal/cortex/workspace"
)

var branchNamePattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._/-]{0,99}$`)

var (
	errInvalidRepository = workspace.NewError(workspace.ErrNotGitRepository, "path is not a Git repository")
	errDirtyWorktree     = workspace.NewError(workspace.ErrConflict, "worktree has uncommitted changes")
	errActiveWorktree    = workspace.NewError(workspace.ErrConflict, "worktree has an active reference")
)

type Adapter struct {
	mu             sync.RWMutex
	runner         CommandRunner
	projects       workspace.ProjectRegistry
	worktreeRoot   string
	worktrees      map[workspace.WorktreeID]workspace.Worktree
	worktreeByPath map[string]workspace.WorktreeID
}

func New() *Adapter {
	return &Adapter{
		runner:         defaultRunner(),
		worktrees:      make(map[workspace.WorktreeID]workspace.Worktree),
		worktreeByPath: make(map[string]workspace.WorktreeID),
	}
}

func NewWithRunner(runner CommandRunner) (*Adapter, error) {
	if runner == nil {
		return nil, errors.New("git runner is required")
	}
	adapter := New()
	adapter.runner = runner
	return adapter, nil
}

func NewWithRegistry(registry workspace.ProjectRegistry, runner CommandRunner, worktreeRoot string) (*Adapter, error) {
	if registry == nil {
		return nil, errors.New("project registry is required")
	}
	if runner == nil {
		runner = defaultRunner()
	}
	canonicalRoot, err := workspace.CanonicalRoot(worktreeRoot)
	if err != nil {
		return nil, fmt.Errorf("configure Git worktree root: %w", err)
	}
	adapter := New()
	adapter.runner = runner
	adapter.projects = registry
	adapter.worktreeRoot = canonicalRoot
	return adapter, nil
}

type repositoryFacts struct {
	Root   string
	Branch string
}

func (a *Adapter) InspectRepository(ctx context.Context, root string) (repositoryFacts, error) {
	if err := requireContext(ctx); err != nil {
		return repositoryFacts{}, err
	}
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
			message := strings.ToLower(strings.TrimSpace(string(stderr)))
			if strings.Contains(message, "detached") || strings.Contains(message, "symbolic ref") {
				return "HEAD", nil
			}
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
	case strings.Contains(message, "already exists"), strings.Contains(message, "is already checked out"), strings.Contains(message, "already registered"):
		return workspace.WrapError(workspace.ErrConflict, "Git worktree conflicts with existing state", cause)
	case strings.Contains(message, "is dirty"), strings.Contains(message, "contains modified or untracked files"):
		return workspace.WrapError(workspace.ErrConflict, "Git worktree has uncommitted changes", cause)
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

func validateRevision(revision string) error {
	if revision == "" || strings.HasPrefix(revision, "-") || strings.ContainsAny(revision, "\r\n\x00") {
		return workspace.NewError(workspace.ErrInvalidRequest, "revision is invalid")
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

func (a *Adapter) InspectWorktree(ctx context.Context, id workspace.WorktreeID) (workspace.Worktree, error) {
	if err := requireContext(ctx); err != nil {
		return workspace.Worktree{}, err
	}
	a.mu.RLock()
	worktree, ok := a.worktrees[id]
	a.mu.RUnlock()
	if !ok {
		return workspace.Worktree{}, workspace.NewError(workspace.ErrNotFound, "worktree does not exist")
	}
	return a.inspectPath(ctx, worktree)
}

func (a *Adapter) CreateWorktree(ctx context.Context, worktree workspace.Worktree) (workspace.Worktree, error) {
	if err := requireContext(ctx); err != nil {
		return workspace.Worktree{}, err
	}
	if a.projects == nil || a.worktreeRoot == "" {
		return workspace.Worktree{}, workspace.NewError(workspace.ErrInvalidRequest, "Git adapter is not configured with project registry and worktree root")
	}
	if worktree.ID == "" || worktree.ProjectID == "" || worktree.Path == "" {
		return workspace.Worktree{}, workspace.NewError(workspace.ErrInvalidRequest, "worktree id, project id, and path are required")
	}
	if err := validateBranch(worktree.Branch); err != nil {
		return workspace.Worktree{}, err
	}
	if worktree.Revision != "" {
		if err := validateRevision(worktree.Revision); err != nil {
			return workspace.Worktree{}, err
		}
	}
	project, err := a.projects.GetProject(ctx, worktree.ProjectID)
	if err != nil {
		return workspace.Worktree{}, err
	}
	facts, err := a.InspectRepository(ctx, project.RepositoryRoot)
	if err != nil {
		return workspace.Worktree{}, err
	}
	if err := os.MkdirAll(filepath.Join(a.worktreeRoot, string(worktree.ProjectID)), 0o755); err != nil {
		return workspace.Worktree{}, workspace.WrapError(workspace.ErrStorageUnavailable, "worktree root cannot be prepared", err)
	}
	target, err := a.approvedTarget(worktree.ProjectID, worktree.Path)
	if err != nil {
		return workspace.Worktree{}, err
	}
	if _, err := os.Stat(target); err == nil {
		return workspace.Worktree{}, workspace.NewError(workspace.ErrConflict, "worktree path already exists")
	} else if !errors.Is(err, os.ErrNotExist) {
		return workspace.Worktree{}, workspace.WrapError(workspace.ErrPathDenied, "worktree path cannot be inspected", err)
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return workspace.Worktree{}, workspace.WrapError(workspace.ErrStorageUnavailable, "worktree parent cannot be created", err)
	}
	base := project.DefaultBranch
	if worktree.Revision != "" {
		base = worktree.Revision
	}
	if base == "" {
		base = "HEAD"
	}
	_, stderr, err := a.run(ctx, "-C", facts.Root, "worktree", "add", "-b", worktree.Branch, target, base)
	if err != nil {
		return workspace.Worktree{}, classifyGitError(stderr, err)
	}
	created, err := a.inspectPath(ctx, workspace.Worktree{
		ID:        worktree.ID,
		ProjectID: worktree.ProjectID,
		Path:      target,
		Branch:    worktree.Branch,
		Status:    workspace.WorktreeStatusActive,
	})
	if err != nil {
		return workspace.Worktree{}, workspace.WrapError(workspace.ErrInternal, "created worktree could not be verified", err)
	}
	created.CreatedAt = time.Now().UTC()
	created.UpdatedAt = created.CreatedAt
	created.Status = workspace.WorktreeStatusActive
	a.remember(created)
	return created, nil
}

func (a *Adapter) ListWorktrees(ctx context.Context, projectID workspace.ProjectID) ([]workspace.Worktree, error) {
	if err := requireContext(ctx); err != nil {
		return nil, err
	}
	if a.projects == nil {
		return nil, workspace.NewError(workspace.ErrInvalidRequest, "Git adapter is not configured with project registry")
	}
	project, err := a.projects.GetProject(ctx, projectID)
	if err != nil {
		return nil, err
	}
	facts, err := a.InspectRepository(ctx, project.RepositoryRoot)
	if err != nil {
		return nil, err
	}
	stdout, stderr, err := a.run(ctx, "-C", facts.Root, "worktree", "list", "--porcelain")
	if err != nil {
		return nil, classifyGitError(stderr, err)
	}
	entries := parseWorktreeList(string(stdout))
	result := make([]workspace.Worktree, 0, len(entries))
	for _, entry := range entries {
		if samePath(entry.Path, facts.Root) {
			continue
		}
		if _, err := a.approvedTarget(projectID, entry.Path); err != nil {
			return nil, err
		}
		id := a.idForPath(entry.Path)
		item := workspace.Worktree{ID: id, ProjectID: projectID, Path: entry.Path, Branch: entry.Branch, Revision: entry.Revision, Status: workspace.WorktreeStatusActive}
		item, err = a.inspectPath(ctx, item)
		if err != nil {
			return nil, err
		}
		a.remember(item)
		result = append(result, item)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].ID < result[j].ID })
	return result, nil
}

func (a *Adapter) RemoveWorktree(ctx context.Context, id workspace.WorktreeID) error {
	if err := requireContext(ctx); err != nil {
		return err
	}
	if a.projects == nil {
		return workspace.NewError(workspace.ErrInvalidRequest, "Git adapter is not configured with project registry")
	}
	a.mu.RLock()
	worktree, ok := a.worktrees[id]
	a.mu.RUnlock()
	if !ok {
		return workspace.NewError(workspace.ErrNotFound, "worktree does not exist")
	}
	project, err := a.projects.GetProject(ctx, worktree.ProjectID)
	if err != nil {
		return err
	}
	facts, err := a.InspectRepository(ctx, project.RepositoryRoot)
	if err != nil {
		return err
	}
	if _, err := a.approvedTarget(worktree.ProjectID, worktree.Path); err != nil {
		return err
	}
	observed, err := a.inspectPath(ctx, worktree)
	if err != nil {
		return err
	}
	if observed.Dirty {
		return errDirtyWorktree
	}
	if observed.ActiveReference != "" {
		return errActiveWorktree
	}
	_, stderr, err := a.run(ctx, "-C", facts.Root, "worktree", "remove", worktree.Path)
	if err != nil {
		return classifyGitError(stderr, err)
	}
	a.mu.Lock()
	delete(a.worktrees, id)
	delete(a.worktreeByPath, filepath.Clean(worktree.Path))
	a.mu.Unlock()
	return nil
}

func (a *Adapter) inspectPath(ctx context.Context, worktree workspace.Worktree) (workspace.Worktree, error) {
	path, err := workspace.CanonicalRoot(worktree.Path)
	if err != nil {
		return workspace.Worktree{}, err
	}
	stdout, stderr, err := a.run(ctx, "-C", path, "rev-parse", "--show-toplevel")
	if err != nil {
		return workspace.Worktree{}, classifyGitError(stderr, err)
	}
	actualRoot, err := workspace.CanonicalRoot(strings.TrimSpace(string(stdout)))
	if err != nil || !samePath(actualRoot, path) {
		return workspace.Worktree{}, errInvalidRepository
	}
	branch, err := a.currentBranch(ctx, path)
	if err != nil {
		return workspace.Worktree{}, err
	}
	head, stderr, err := a.run(ctx, "-C", path, "rev-parse", "HEAD")
	if err != nil {
		return workspace.Worktree{}, classifyGitError(stderr, err)
	}
	status, stderr, err := a.run(ctx, "-C", path, "status", "--porcelain=v1", "--untracked-files=all")
	if err != nil {
		return workspace.Worktree{}, classifyGitError(stderr, err)
	}
	worktree.Path = actualRoot
	worktree.Branch = branch
	worktree.Revision = strings.TrimSpace(string(head))
	worktree.Dirty = strings.TrimSpace(string(status)) != ""
	worktree.UpdatedAt = time.Now().UTC()
	if worktree.Status == "" {
		worktree.Status = workspace.WorktreeStatusActive
	}
	return worktree, nil
}

func (a *Adapter) approvedTarget(projectID workspace.ProjectID, target string) (string, error) {
	if a.worktreeRoot == "" {
		return "", workspace.NewError(workspace.ErrInvalidRequest, "worktree root is not configured")
	}
	projectRoot, err := workspace.ContainedPath(a.worktreeRoot, string(projectID))
	if err != nil {
		return "", err
	}
	canonicalTarget, err := workspace.CanonicalRoot(target)
	if err != nil {
		return "", err
	}
	relative, err := filepath.Rel(projectRoot, canonicalTarget)
	if err != nil || relative == "." || filepath.IsAbs(relative) || strings.HasPrefix(relative, ".."+string(filepath.Separator)) || relative == ".." {
		return "", workspace.NewError(workspace.ErrPathDenied, "worktree path is outside the approved worktree root")
	}
	if err := verifyExistingContainment(projectRoot, canonicalTarget); err != nil {
		return "", err
	}
	return canonicalTarget, nil
}

func verifyExistingContainment(root, target string) error {
	rootResolved, err := filepath.EvalSymlinks(root)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return workspace.WrapError(workspace.ErrPathDenied, "worktree root link cannot be resolved", err)
	}
	if err != nil {
		rootResolved = filepath.Clean(root)
	}
	ancestor := target
	for {
		resolved, resolveErr := filepath.EvalSymlinks(ancestor)
		if resolveErr == nil {
			if !samePath(rootResolved, resolved) && !pathWithin(rootResolved, resolved) {
				return workspace.NewError(workspace.ErrPathDenied, "worktree path resolves outside the approved root")
			}
			return nil
		}
		if !errors.Is(resolveErr, os.ErrNotExist) {
			return workspace.WrapError(workspace.ErrPathDenied, "worktree path link cannot be resolved", resolveErr)
		}
		parent := filepath.Dir(ancestor)
		if parent == ancestor {
			return nil
		}
		ancestor = parent
	}
}

func pathWithin(root, candidate string) bool {
	relative, err := filepath.Rel(root, candidate)
	return err == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator)) && !filepath.IsAbs(relative)
}

func (a *Adapter) remember(worktree workspace.Worktree) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.worktrees[worktree.ID] = worktree
	a.worktreeByPath[filepath.Clean(worktree.Path)] = worktree.ID
}

func (a *Adapter) idForPath(path string) workspace.WorktreeID {
	cleanPath := filepath.Clean(path)
	a.mu.RLock()
	if id, ok := a.worktreeByPath[cleanPath]; ok {
		a.mu.RUnlock()
		return id
	}
	a.mu.RUnlock()
	hash := sha256.Sum256([]byte(strings.ToLower(cleanPath)))
	return workspace.WorktreeID("wt-" + hex.EncodeToString(hash[:8]))
}

func parseWorktreeList(output string) []struct {
	Path     string
	Revision string
	Branch   string
} {
	var result []struct {
		Path     string
		Revision string
		Branch   string
	}
	var current *struct {
		Path     string
		Revision string
		Branch   string
	}
	flush := func() {
		if current != nil && current.Path != "" {
			result = append(result, *current)
		}
		current = nil
	}
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSuffix(line, "\r")
		switch {
		case strings.HasPrefix(line, "worktree "):
			flush()
			current = &struct {
				Path     string
				Revision string
				Branch   string
			}{Path: strings.TrimSpace(strings.TrimPrefix(line, "worktree "))}
		case current != nil && strings.HasPrefix(line, "HEAD "):
			current.Revision = strings.TrimSpace(strings.TrimPrefix(line, "HEAD "))
		case current != nil && strings.HasPrefix(line, "branch "):
			current.Branch = strings.TrimPrefix(line, "branch refs/heads/")
		}
	}
	flush()
	return result
}

func requireContext(ctx context.Context) error {
	if ctx == nil {
		return workspace.NewError(workspace.ErrInvalidRequest, "context is required")
	}
	if err := ctx.Err(); err != nil {
		return workspace.CanceledError(err)
	}
	return nil
}
