package harness

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
)

// SandboxConfig controls the execution boundary. A zero limit or duration is
// replaced with the corresponding safe default by NewSandbox.
type SandboxConfig struct {
	WorktreeRoot            string
	AllowedEnvVars          []string
	StdoutLimitBytes        int64
	StderrLimitBytes        int64
	Timeout                 time.Duration
	CancellationGracePeriod time.Duration
	RedactionConfig         RedactionConfig
}

func DefaultSandboxConfig() SandboxConfig {
	return SandboxConfig{
		AllowedEnvVars:          []string{"PATH", "HOME", "USER", "USERPROFILE", "TEMP", "TMP", "LANG", "LC_ALL"},
		StdoutLimitBytes:        1024 * 1024,
		StderrLimitBytes:        512 * 1024,
		Timeout:                 120 * time.Second,
		CancellationGracePeriod: 5 * time.Second,
		RedactionConfig: RedactionConfig{
			Secrets: true, AbsolutePaths: true, EnvVars: true, CmdLine: true,
		},
	}
}

// ExecutionSandbox is the concrete sandbox implementation. It is intentionally
// engine-neutral: adapters remain responsible for interpreting their input,
// while this type owns validation, process execution, and output hygiene.
type ExecutionSandbox struct {
	mu     sync.RWMutex
	config SandboxConfig
	root   string
}

func NewSandbox(config SandboxConfig) (*ExecutionSandbox, error) {
	if config.WorktreeRoot == "" {
		return nil, fmt.Errorf("%w: worktree root is required", ErrSandbox)
	}
	root, err := canonicalPath(config.WorktreeRoot)
	if err != nil {
		return nil, fmt.Errorf("%w: invalid worktree root", ErrSandbox)
	}
	info, err := os.Stat(root)
	if err != nil || !info.IsDir() {
		return nil, fmt.Errorf("%w: worktree root is unavailable", ErrSandbox)
	}

	defaults := DefaultSandboxConfig()
	if config.StdoutLimitBytes <= 0 {
		config.StdoutLimitBytes = defaults.StdoutLimitBytes
	}
	if config.StderrLimitBytes <= 0 {
		config.StderrLimitBytes = defaults.StderrLimitBytes
	}
	if config.Timeout <= 0 {
		config.Timeout = defaults.Timeout
	}
	if config.CancellationGracePeriod <= 0 {
		config.CancellationGracePeriod = defaults.CancellationGracePeriod
	}
	if len(config.AllowedEnvVars) == 0 {
		config.AllowedEnvVars = defaults.AllowedEnvVars
	}
	for i := range config.AllowedEnvVars {
		config.AllowedEnvVars[i] = strings.ToUpper(strings.TrimSpace(config.AllowedEnvVars[i]))
	}
	return &ExecutionSandbox{config: config, root: root}, nil
}

func (s *ExecutionSandbox) WorktreeRoot() string { return s.root }

func (s *ExecutionSandbox) Config() SandboxConfig {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.config
}

// Validate implements the harness Sandbox contract. It performs all checks
// that can be made before an adapter is allowed to create side effects.
func (s *ExecutionSandbox) Validate(_ ExecutionContext, envelope ExecutionEnvelope) error {
	if envelope.ExecutionID == "" || envelope.TaskID == "" || envelope.WorktreeID == "" {
		return fmt.Errorf("%w: execution scope is incomplete", ErrInvalidContract)
	}
	if envelope.WorktreeRoot == "" {
		return fmt.Errorf("%w: worktree root is required", ErrWorktreeViolation)
	}
	root, err := canonicalPath(envelope.WorktreeRoot)
	if err != nil || !samePath(root, s.root) {
		return fmt.Errorf("%w: execution root is not assigned root", ErrWorktreeViolation)
	}
	if envelope.Timeout <= 0 {
		return fmt.Errorf("%w: timeout must be positive", ErrInvalidContract)
	}
	if !envelope.PolicyDecision.Allowed {
		return fmt.Errorf("%w: policy decision is not allowed", ErrPolicyDenied)
	}
	return nil
}

// ExecuteAdapter validates the envelope and invokes an adapter with a child
// context. Adapter output is bounded and redacted before it is returned.
func (s *ExecutionSandbox) ExecuteAdapter(ctx ExecutionContext, adapter EngineAdapter, envelope ExecutionEnvelope) (ToolResult, error) {
	if adapter == nil {
		return ToolResult{}, fmt.Errorf("%w: adapter is nil", ErrSandbox)
	}
	if err := s.Validate(ctx, envelope); err != nil {
		return ToolResult{}, err
	}
	if ctx == nil {
		ctx = NewExecutionContext(context.Background(), envelope.TraceID)
	}
	config := s.Config()
	timeout := envelope.Timeout
	if timeout > config.Timeout {
		timeout = config.Timeout
	}
	child, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	result, err := adapter.Execute(NewExecutionContext(child, envelope.TraceID), envelope)
	result.Output = redactAndLimitJSON(result.Output, config.StdoutLimitBytes, config.RedactionConfig)
	result.Redacted = true
	if err != nil {
		return result, err
	}
	return result, nil
}

// ValidatePath resolves path relative to the worktree and rejects absolute,
// traversal, and symlink paths which resolve outside the canonical root.
func (s *ExecutionSandbox) ValidatePath(path string) (string, error) {
	if path == "" || strings.IndexByte(path, 0) >= 0 {
		return "", fmt.Errorf("%w: invalid path", ErrWorktreeViolation)
	}
	if filepath.IsAbs(path) {
		// Absolute paths are accepted only when they are already inside root.
		// This is useful for adapter internals, but never broadens the boundary.
		path = filepath.Clean(path)
	} else {
		path = filepath.Join(s.root, filepath.FromSlash(path))
	}
	candidate, err := canonicalCandidatePath(path)
	if err != nil {
		return "", fmt.Errorf("%w: path cannot be resolved", ErrWorktreeViolation)
	}
	if !samePath(candidate, s.root) && !isContained(candidate, s.root) {
		return "", fmt.Errorf("%w: path is outside worktree", ErrWorktreeViolation)
	}
	return candidate, nil
}

func (s *ExecutionSandbox) ValidatePaths(paths []string) ([]string, error) {
	out := make([]string, len(paths))
	for i, path := range paths {
		var err error
		out[i], err = s.ValidatePath(path)
		if err != nil {
			return nil, err
		}
	}
	return out, nil
}

// ValidateArgs rejects shell syntax, traversal, and absolute paths outside the
// worktree. Adapters should pass argv, never a shell command line.
func (s *ExecutionSandbox) ValidateArgs(args []string, _ []string) error {
	for _, arg := range args {
		if containsShellMeta(arg) || strings.Contains(arg, "..") {
			return fmt.Errorf("%w: unsafe argument", ErrWorktreeViolation)
		}
		if filepath.IsAbs(arg) {
			if _, err := s.ValidatePath(arg); err != nil {
				return err
			}
		}
	}
	return nil
}

// FilterEnvironment passes only explicitly allowed, non-secret variables.
func (s *ExecutionSandbox) FilterEnvironment(env []string) ([]string, error) {
	s.mu.RLock()
	allowed := make(map[string]struct{}, len(s.config.AllowedEnvVars))
	for _, name := range s.config.AllowedEnvVars {
		allowed[strings.ToUpper(name)] = struct{}{}
	}
	s.mu.RUnlock()

	out := make([]string, 0, len(env))
	for _, entry := range env {
		name, _, ok := strings.Cut(entry, "=")
		if !ok {
			continue
		}
		name = strings.ToUpper(name)
		if _, ok := allowed[name]; !ok || isSecretVar(name) {
			continue
		}
		out = append(out, entry)
	}
	return out, nil
}

type SandboxResult struct {
	Stdout      []byte
	Stderr      []byte
	ExitCode    int
	TimedOut    bool
	Canceled    bool
	Truncated   bool
	RedactedOut []byte
	RedactedErr []byte
}

type ProcessRunner interface {
	Run(ctx context.Context, cmd string, args []string, env []string, workDir string,
		stdoutLimit, stderrLimit int64, gracePeriod time.Duration) (SandboxResult, error)
}

// DefaultProcessRunner uses os/exec. CommandContext terminates the process on
// cancellation; Wait always runs, so pipes and process resources are cleaned up.
type DefaultProcessRunner struct{}

func (d *DefaultProcessRunner) Run(ctx context.Context, command string, args []string, env []string, workDir string,
	stdoutLimit, stderrLimit int64, gracePeriod time.Duration) (SandboxResult, error) {
	cmd := exec.Command(command, args...)
	cmd.Dir = workDir
	cmd.Env = env
	cmd.SysProcAttr = processGroupAttr()
	var stdout, stderr limitedBuffer
	stdout.limit, stderr.limit = stdoutLimit, stderrLimit
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Start(); err != nil {
		return SandboxResult{}, err
	}

	wait := make(chan error, 1)
	go func() { wait <- cmd.Wait() }()
	var runErr error
	select {
	case runErr = <-wait:
	case <-ctx.Done():
		// Kill the complete process group, then wait so no child or pipe is left behind.
		terminateProcessGroup(cmd)
		select {
		case runErr = <-wait:
		case <-time.After(gracePeriod):
			terminateProcessGroup(cmd)
			runErr = <-wait
		}
	}

	result := SandboxResult{Stdout: stdout.Bytes(), Stderr: stderr.Bytes(), Truncated: stdout.truncated || stderr.truncated}
	if cmd.ProcessState != nil {
		result.ExitCode = cmd.ProcessState.ExitCode()
	}
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		result.TimedOut = true
	}
	if errors.Is(ctx.Err(), context.Canceled) {
		result.Canceled = true
	}
	return result, runErr
}

type limitedBuffer struct {
	bytes.Buffer
	limit     int64
	truncated bool
}

func (b *limitedBuffer) Write(p []byte) (int, error) {
	if b.limit <= 0 {
		b.truncated = len(p) > 0
		return len(p), nil
	}
	remaining := b.limit - int64(b.Len())
	if remaining <= 0 {
		b.truncated = len(p) > 0
		return len(p), nil
	}
	if int64(len(p)) > remaining {
		_, _ = b.Buffer.Write(p[:remaining])
		b.truncated = true
		return len(p), nil
	}
	return b.Buffer.Write(p)
}

func (s *ExecutionSandbox) ExecuteWithSandbox(ctx ExecutionContext, runner ProcessRunner,
	command string, args []string, env []string, workDir string) (SandboxResult, error) {
	if runner == nil {
		runner = &DefaultProcessRunner{}
	}
	if ctx == nil {
		ctx = NewExecutionContext(context.Background(), "")
	}
	config := s.Config()
	canonicalWorkDir, err := s.ValidatePath(workDir)
	if err != nil {
		return SandboxResult{}, err
	}
	if filepath.IsAbs(command) {
		if _, err := s.ValidatePath(command); err != nil {
			return SandboxResult{}, err
		}
	}
	if err := s.ValidateArgs(args, nil); err != nil {
		return SandboxResult{}, err
	}
	filtered, err := s.FilterEnvironment(env)
	if err != nil {
		return SandboxResult{}, err
	}
	child, cancel := context.WithTimeout(ctx, config.Timeout)
	defer cancel()
	result, runErr := runner.Run(child, command, args, filtered, canonicalWorkDir,
		config.StdoutLimitBytes, config.StderrLimitBytes, config.CancellationGracePeriod)
	result.RedactedOut = redactOutput(result.Stdout, config.RedactionConfig)
	result.RedactedErr = redactOutput(result.Stderr, config.RedactionConfig)
	if errors.Is(child.Err(), context.DeadlineExceeded) || result.TimedOut {
		result.TimedOut = true
		return result, ErrTimeout
	}
	if (errors.Is(child.Err(), context.Canceled) || result.Canceled) && !result.TimedOut {
		result.Canceled = true
		return result, ErrCanceled
	}
	if runErr != nil {
		if errors.Is(runErr, context.DeadlineExceeded) {
			result.TimedOut = true
			return result, ErrTimeout
		}
		if errors.Is(runErr, context.Canceled) {
			result.Canceled = true
			return result, ErrCanceled
		}
		return result, fmt.Errorf("%w: process execution failed", ErrSandbox)
	}
	return result, nil
}

func (s *ExecutionSandbox) RecordAudit(envelope ExecutionEnvelope, event SandboxAuditEvent) (EvidenceRecord, error) {
	event.Timestamp = time.Now().UTC()
	event.ExecutionID = envelope.ExecutionID
	event.TaskID = envelope.TaskID
	event.WorktreeID = envelope.WorktreeID
	return NewEvidenceRecord(envelope.ExecutionID, envelope.TaskID, envelope.WorktreeID, EvidenceKindSandboxAudit, event)
}

type SandboxAuditEvent struct {
	Event       string    `json:"event"`
	Path        string    `json:"path,omitempty"`
	Command     string    `json:"command,omitempty"`
	Args        []string  `json:"args,omitempty"`
	Truncated   bool      `json:"truncated,omitempty"`
	Redacted    bool      `json:"redacted,omitempty"`
	ExitCode    int       `json:"exitCode,omitempty"`
	DurationMs  int64     `json:"durationMs,omitempty"`
	Timestamp   time.Time `json:"timestamp"`
	ExecutionID string    `json:"executionId"`
	TaskID      string    `json:"taskId"`
	WorktreeID  string    `json:"worktreeId"`
}

func canonicalPath(path string) (string, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	return filepath.EvalSymlinks(abs)
}

// canonicalCandidatePath resolves a path for read operations. For write
// operations, callers must validate parent directories separately to prevent
// symlink swap attacks. The path is resolved if it exists; otherwise it is
// cleaned without following final symlinks to avoid races.
func canonicalCandidatePath(path string) (string, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	// Try to resolve existing symlinks. If the path doesn't exist, clean it
	// without following symlinks.
	if _, err := os.Stat(abs); err == nil {
		return filepath.EvalSymlinks(abs)
	}
	return filepath.Clean(abs), nil
}

func samePath(a, b string) bool {
	if runtime.GOOS == "windows" {
		return strings.EqualFold(filepath.Clean(a), filepath.Clean(b))
	}
	return filepath.Clean(a) == filepath.Clean(b)
}

func isContained(target, root string) bool {
	target, root = filepath.Clean(target), filepath.Clean(root)
	if runtime.GOOS == "windows" {
		target, root = strings.ToLower(target), strings.ToLower(root)
	}
	return strings.HasPrefix(target, root+string(filepath.Separator))
}

func containsShellMeta(s string) bool {
	return strings.ContainsAny(s, ";|&`$()<>\r\n")
}

func isSecretVar(name string) bool {
	name = strings.ToUpper(name)
	for _, part := range []string{"KEY", "SECRET", "TOKEN", "PASSWORD", "PASSWD", "AUTH", "BEARER", "CREDENTIAL", "PRIVATE", "CERT", "OAUTH", "JWT", "COOKIE"} {
		if strings.Contains(name, part) {
			return true
		}
	}
	return false
}

func redactOutput(value []byte, config RedactionConfig) []byte {
	if !config.Secrets && !config.AbsolutePaths {
		return append([]byte(nil), value...)
	}
	return []byte(RedactString(string(value)))
}

func redactAndLimitJSON(value []byte, limit int64, config RedactionConfig) []byte {
	if int64(len(value)) > limit {
		value = value[:limit]
	}
	return redactOutput(value, config)
}

var _ Sandbox = (*ExecutionSandbox)(nil)
var _ ProcessRunner = (*DefaultProcessRunner)(nil)
