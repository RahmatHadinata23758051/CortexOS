package harness

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/RahmatHadinata23758051/CortexOS/internal/cortex/workspace"
	workspacegit "github.com/RahmatHadinata23758051/CortexOS/internal/cortex/workspace/git"
)

const (
	NativeAdapterVersion = "1.0.0"
	NativeAdapterName    = "native"

	defaultNativeMaxBytes = 1 << 20
	defaultNativeMaxItems = 1000
)

// ErrNativeUnsupportedOperation is returned for operations which are not part
// of the native adapter's deliberately small, deterministic surface. In
// particular, native never interprets a shell command or falls back to one.
var ErrNativeUnsupportedOperation = errors.New("harness: native operation is unsupported")

// NativeOperationContext is passed to every workspace, Git, and validation
// port. Keeping the execution scope in a typed value prevents ports from
// silently using ambient state or a different worktree.
type NativeOperationContext struct {
	ExecutionID    string
	TaskID         string
	WorktreeID     string
	ProjectID      string
	WorktreeRoot   string
	ToolName       string
	PolicyDecision PolicyDecision
	Audit          AuditMetadata
}

func (c NativeOperationContext) validate() error {
	if c.ExecutionID == "" || c.TaskID == "" || c.WorktreeID == "" {
		return fmt.Errorf("%w: native execution scope is incomplete", ErrInvalidContract)
	}
	if c.WorktreeRoot == "" {
		return ErrWorktreeViolation
	}
	if !c.PolicyDecision.Allowed {
		return ErrPolicyDenied
	}
	return nil
}

// NativeWorkspacePort is the only filesystem boundary used by the adapter.
// Implementations must interpret paths as worktree-relative and must enforce
// the supplied operation context before doing I/O.
type NativeWorkspacePort interface {
	Inspect(context.Context, NativeOperationContext, string) (NativeFileInfo, error)
	ReadFile(context.Context, NativeOperationContext, string) ([]byte, error)
	WriteFile(context.Context, NativeOperationContext, string, []byte) error
	EditFile(context.Context, NativeOperationContext, string, string, string) error
	Search(context.Context, NativeOperationContext, string, string, int) ([]NativeSearchMatch, error)
}

// NativeGitPort is a controlled Git boundary. It exposes observations only;
// Git mutation is intentionally not a native operation.
type NativeGitPort interface {
	Status(context.Context, NativeOperationContext) (NativeGitStatus, error)
	Diff(context.Context, NativeOperationContext, string) (NativeGitDiff, error)
}

// NativeValidationPort is the broker/sandbox boundary for validation. A port
// receives a fixed validation kind, not an executable or shell command line.
type NativeValidationPort interface {
	Run(context.Context, NativeOperationContext, NativeValidationRequest) (NativeValidationResult, error)
}

type NativeFileInfo struct {
	Path    string `json:"path"`
	Size    int64  `json:"size"`
	Mode    string `json:"mode"`
	IsDir   bool   `json:"isDir"`
	ModTime string `json:"modTime,omitempty"`
}

type NativeSearchMatch struct {
	Path  string `json:"path"`
	Line  int    `json:"line"`
	Match string `json:"match"`
}

type NativeGitStatus struct {
	Branch   string           `json:"branch"`
	Revision string           `json:"revision"`
	Dirty    bool             `json:"dirty"`
	Entries  []NativeGitEntry `json:"entries"`
}

type NativeGitEntry struct {
	Path    string `json:"path"`
	Index   string `json:"index,omitempty"`
	Workdir string `json:"workdir,omitempty"`
}

type NativeGitDiff struct {
	Path      string `json:"path,omitempty"`
	Patch     string `json:"patch"`
	Truncated bool   `json:"truncated"`
}

type NativeValidationRequest struct {
	Kind string   `json:"kind"`
	Args []string `json:"args,omitempty"`
}

type NativeValidationResult struct {
	Kind      string `json:"kind"`
	Passed    bool   `json:"passed"`
	ExitCode  int    `json:"exitCode"`
	Stdout    string `json:"stdout,omitempty"`
	Stderr    string `json:"stderr,omitempty"`
	Truncated bool   `json:"truncated,omitempty"`
}

// NativeAdapterConfig wires the adapter to narrow ports. WorktreeRoot is
// required for the built-in ports and is never included in public output.
type NativeAdapterConfig struct {
	Workspace    NativeWorkspacePort
	Git          NativeGitPort
	Validation   NativeValidationPort
	WorktreeRoot string
	MaxBytes     int
	MaxItems     int
}

type NativeAdapter struct {
	workspace  NativeWorkspacePort
	git        NativeGitPort
	validation NativeValidationPort
	root       string
	maxBytes   int
	maxItems   int
}

// NewNativeAdapter creates a native adapter. Ports are explicit dependencies;
// there is no implicit shell or process fallback.
func NewNativeAdapter(config NativeAdapterConfig) (*NativeAdapter, error) {
	if config.Workspace == nil || config.Git == nil || config.Validation == nil {
		return nil, fmt.Errorf("%w: native workspace, Git, and validation ports are required", ErrInvalidContract)
	}
	if strings.TrimSpace(config.WorktreeRoot) == "" {
		return nil, fmt.Errorf("%w: native worktree root is required", ErrInvalidContract)
	}
	root, err := workspace.CanonicalRoot(config.WorktreeRoot)
	if err != nil {
		return nil, fmt.Errorf("%w: native worktree root is invalid", ErrInvalidContract)
	}
	if info, err := os.Stat(root); err != nil || !info.IsDir() {
		return nil, fmt.Errorf("%w: native worktree root is unavailable", ErrInvalidContract)
	}
	if config.MaxBytes <= 0 {
		config.MaxBytes = defaultNativeMaxBytes
	}
	if config.MaxItems <= 0 {
		config.MaxItems = defaultNativeMaxItems
	}
	return &NativeAdapter{workspace: config.Workspace, git: config.Git, validation: config.Validation,
		root: root, maxBytes: config.MaxBytes, maxItems: config.MaxItems}, nil
}

// NewNativeAdapterWithPorts is useful for unit tests and callers that already
// validated the root. It still requires a root in the supplied execution
// envelope, so it cannot broaden the filesystem boundary.
func NewNativeAdapterWithPorts(workspacePort NativeWorkspacePort, gitPort NativeGitPort, validationPort NativeValidationPort) *NativeAdapter {
	return &NativeAdapter{workspace: workspacePort, git: gitPort, validation: validationPort,
		maxBytes: defaultNativeMaxBytes, maxItems: defaultNativeMaxItems}
}

func (a *NativeAdapter) Kind() ToolKind { return ToolKindNative }

func (a *NativeAdapter) Capabilities() []ToolCapability {
	return []ToolCapability{CapabilityFileRead, CapabilityFileWrite, CapabilityFileEdit,
		CapabilityGit, CapabilitySearch, CapabilityTestRun, CapabilityLint, CapabilityBuild}
}

func (a *NativeAdapter) Describe() AdapterDescriptor {
	return AdapterDescriptor{Name: NativeAdapterName, Kind: ToolKindNative, Version: NativeAdapterVersion,
		Capabilities: a.Capabilities(), SchemaVersion: HarnessContractVersion}
}

func (a *NativeAdapter) Health(ctx ExecutionContext) error {
	if ctx == nil {
		ctx = NewExecutionContext(context.Background(), "")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if a == nil || a.workspace == nil || a.git == nil || a.validation == nil {
		return fmt.Errorf("%w: native ports are not configured", ErrAdapterUnhealthy)
	}
	if a.root != "" {
		info, err := os.Stat(a.root)
		if err != nil || !info.IsDir() {
			return fmt.Errorf("%w: native worktree root is unavailable", ErrAdapterUnhealthy)
		}
	}
	return nil
}

func (a *NativeAdapter) Execute(ctx ExecutionContext, envelope ExecutionEnvelope) (ToolResult, error) {
	started := time.Now().UTC()
	result := ToolResult{ContractVersion: HarnessContractVersion, ExecutionID: envelope.ExecutionID,
		TaskID: envelope.TaskID, ToolName: envelope.ToolName, Status: ToolStatusFailed, Redacted: true,
		StartedAt: started}
	if ctx == nil {
		ctx = NewExecutionContext(context.Background(), envelope.TraceID)
	}
	effectiveRoot, err := validateNativeEnvelope(envelope, a.root)
	if err != nil {
		return a.finishError(result, started, err)
	}
	if err := ctx.Err(); err != nil {
		return a.finishError(result, started, err)
	}
	operation, input, err := decodeNativeInput(envelope.ToolName, envelope.Input)
	if err != nil {
		return a.finishError(result, started, err)
	}
	if !a.supports(operation) {
		return a.finishError(result, started, fmt.Errorf("%w: %s", ErrNativeUnsupportedOperation, operation))
	}
	opCtx := NativeOperationContext{ExecutionID: envelope.ExecutionID, TaskID: envelope.TaskID,
		WorktreeID: envelope.WorktreeID, ProjectID: envelope.ProjectID, WorktreeRoot: effectiveRoot,
		ToolName: envelope.ToolName, PolicyDecision: envelope.PolicyDecision, Audit: envelope.Audit}
	if err := opCtx.validate(); err != nil {
		return a.finishError(result, started, err)
	}

	value, kind, err := a.executeOperation(ctx, opCtx, operation, input)
	if err != nil {
		return a.finishError(result, started, err)
	}
	// Evidence is generated from the bounded, redacted value—not from raw port
	// output—so the digest and public payload describe exactly the same bytes.
	bounded, err := a.boundJSON(value)
	if err != nil {
		return a.finishError(result, started, err)
	}
	evidence, err := NewEvidenceRecord(envelope.ExecutionID, envelope.TaskID, envelope.WorktreeID, kind, bounded)
	if err != nil {
		return a.finishError(result, started, err)
	}
	output, err := json.Marshal(nativeOutput{Operation: operation, Result: bounded, Evidence: evidence})
	if err != nil {
		return a.finishError(result, started, fmt.Errorf("%w: native output encoding failed", ErrAdapterFailure))
	}
	result.Status = ToolStatusSuccess
	result.Output = RedactBytes(output)
	result.EvidenceIDs = []string{evidence.ID}
	completed := time.Now().UTC()
	result.CompletedAt, result.DurationMs = completed, completed.Sub(started).Milliseconds()
	return result, nil
}

type nativeOutput struct {
	Operation string          `json:"operation"`
	Result    json.RawMessage `json:"result"`
	Evidence  EvidenceRecord  `json:"evidence"`
}

func validateNativeEnvelope(e ExecutionEnvelope, defaultRoot string) (string, error) {
	if e.ContractVersion != HarnessContractVersion {
		return "", fmt.Errorf("%w: unsupported execution contract", ErrInvalidContract)
	}
	if e.ExecutionID == "" || e.TaskID == "" || e.WorktreeID == "" || e.ToolName == "" {
		return "", fmt.Errorf("%w: native execution identifiers are required", ErrInvalidContract)
	}
	root := e.WorktreeRoot
	if root == "" {
		root = defaultRoot
	}
	if root == "" {
		return "", ErrWorktreeViolation
	}
	canonical, err := workspace.CanonicalRoot(root)
	if err != nil {
		return "", ErrWorktreeViolation
	}
	if defaultRoot != "" && !samePath(canonical, defaultRoot) {
		return "", ErrWorktreeViolation
	}
	if e.Timeout <= 0 {
		return "", fmt.Errorf("%w: native timeout is required", ErrInvalidContract)
	}
	if !e.PolicyDecision.Allowed {
		return "", ErrPolicyDenied
	}
	return canonical, nil
}

type nativeInput struct {
	Operation string   `json:"operation"`
	Path      string   `json:"path,omitempty"`
	Query     string   `json:"query,omitempty"`
	Content   string   `json:"content,omitempty"`
	Old       string   `json:"old,omitempty"`
	New       string   `json:"new,omitempty"`
	Args      []string `json:"args,omitempty"`
}

func decodeNativeInput(toolName string, raw json.RawMessage) (string, nativeInput, error) {
	var in nativeInput
	if len(raw) != 0 {
		if err := json.Unmarshal(raw, &in); err != nil {
			return "", in, fmt.Errorf("%w: native input is invalid JSON", ErrInvalidContract)
		}
	}
	operation := strings.ToLower(strings.TrimSpace(in.Operation))
	if operation == "" {
		operation = strings.ToLower(strings.TrimSpace(toolName))
	}
	operation = strings.ReplaceAll(operation, "-", "_")
	operation = strings.TrimPrefix(operation, "native_")
	switch operation {
	case "read", "read_file", "file_read":
		operation = "file_read"
	case "write", "write_file", "file_write":
		operation = "file_write"
	case "edit", "edit_file", "file_edit":
		operation = "file_edit"
	case "find", "search":
		operation = "search"
	case "status", "git_status":
		operation = "git_status"
	case "diff", "git_diff":
		operation = "git_diff"
	case "test", "test_run", "validate":
		operation = "test_run"
	case "go_vet", "vet", "lint":
		operation = "lint"
	case "go_build", "build":
		operation = "build"
	}
	return operation, in, nil
}

func (a *NativeAdapter) supports(operation string) bool {
	switch operation {
	case "file_read", "file_write", "file_edit", "search", "git_status", "git_diff", "test_run", "lint", "build":
		return true
	default:
		return false
	}
}

func (a *NativeAdapter) executeOperation(ctx context.Context, opCtx NativeOperationContext, operation string, in nativeInput) (any, EvidenceKind, error) {
	if err := ctx.Err(); err != nil {
		return nil, EvidenceKindEngineDiagnostic, err
	}
	switch operation {
	case "file_read":
		data, err := a.workspace.ReadFile(ctx, opCtx, in.Path)
		if err != nil {
			return nil, EvidenceKindEngineDiagnostic, err
		}
		if len(data) > a.maxBytes {
			return nil, EvidenceKindEngineDiagnostic, fmt.Errorf("%w: file result is bounded", ErrAdapterFailure)
		}
		return map[string]any{"path": in.Path, "content": string(data), "size": len(data)}, EvidenceKindEngineDiagnostic, nil
	case "file_write":
		if len(in.Content) > a.maxBytes {
			return nil, EvidenceKindFileMutation, fmt.Errorf("%w: file content is bounded", ErrAdapterFailure)
		}
		if err := a.workspace.WriteFile(ctx, opCtx, in.Path, []byte(in.Content)); err != nil {
			return nil, EvidenceKindFileMutation, err
		}
		return map[string]any{"path": in.Path, "size": len(in.Content), "written": true}, EvidenceKindFileMutation, nil
	case "file_edit":
		if len(in.Old) > a.maxBytes || len(in.New) > a.maxBytes {
			return nil, EvidenceKindFileMutation, fmt.Errorf("%w: edit is bounded", ErrAdapterFailure)
		}
		if err := a.workspace.EditFile(ctx, opCtx, in.Path, in.Old, in.New); err != nil {
			return nil, EvidenceKindFileMutation, err
		}
		return map[string]any{"path": in.Path, "edited": true}, EvidenceKindFileMutation, nil
	case "search":
		matches, err := a.workspace.Search(ctx, opCtx, in.Path, in.Query, a.maxItems)
		if err != nil {
			return nil, EvidenceKindEngineDiagnostic, err
		}
		if len(matches) > a.maxItems {
			matches = matches[:a.maxItems]
		}
		sort.SliceStable(matches, func(i, j int) bool {
			if matches[i].Path != matches[j].Path {
				return matches[i].Path < matches[j].Path
			}
			return matches[i].Line < matches[j].Line
		})
		return map[string]any{"query": in.Query, "matches": matches}, EvidenceKindEngineDiagnostic, nil
	case "git_status":
		status, err := a.git.Status(ctx, opCtx)
		if err != nil {
			return nil, EvidenceKindEngineDiagnostic, err
		}
		sort.SliceStable(status.Entries, func(i, j int) bool { return status.Entries[i].Path < status.Entries[j].Path })
		if len(status.Entries) > a.maxItems {
			status.Entries = status.Entries[:a.maxItems]
		}
		return status, EvidenceKindEngineDiagnostic, nil
	case "git_diff":
		diff, err := a.git.Diff(ctx, opCtx, in.Path)
		if err != nil {
			return nil, EvidenceKindEngineDiagnostic, err
		}
		if len(diff.Patch) > a.maxBytes {
			diff.Patch = diff.Patch[:a.maxBytes]
			diff.Truncated = true
		}
		return diff, EvidenceKindEngineDiagnostic, nil
	case "test_run", "lint", "build":
		validation, err := a.validation.Run(ctx, opCtx, NativeValidationRequest{Kind: operation, Args: boundedArgs(in.Args)})
		if err != nil {
			return nil, EvidenceKindCommandExecution, err
		}
		validation.Stdout = RedactString(limitString(validation.Stdout, a.maxBytes))
		validation.Stderr = RedactString(limitString(validation.Stderr, a.maxBytes))
		return validation, EvidenceKindCommandExecution, nil
	default:
		return nil, EvidenceKindEngineDiagnostic, fmt.Errorf("%w: %s", ErrNativeUnsupportedOperation, operation)
	}
}

func boundedArgs(args []string) []string {
	out := append([]string(nil), args...)
	for i := range out {
		out[i] = strings.TrimSpace(out[i])
	}
	return out
}

func limitString(value string, limit int) string {
	if limit <= 0 || len(value) <= limit {
		return value
	}
	return value[:limit]
}

func (a *NativeAdapter) boundJSON(value any) (json.RawMessage, error) {
	raw, err := json.Marshal(value)
	if err != nil {
		return nil, fmt.Errorf("%w: native result encoding failed", ErrAdapterFailure)
	}
	if len(raw) > a.maxBytes {
		return nil, fmt.Errorf("%w: native result is bounded", ErrAdapterFailure)
	}
	return RedactBytes(raw), nil
}

func (a *NativeAdapter) finishError(result ToolResult, started time.Time, err error) (ToolResult, error) {
	coded := ErrorFor(err)
	result.Error = &ToolError{Code: string(coded.Code), Message: coded.Message}
	result.Status = toolStatusFromError(coded)
	completed := time.Now().UTC()
	result.CompletedAt, result.DurationMs = completed, completed.Sub(started).Milliseconds()
	return result, err
}

// OSWorkspacePort is a reference implementation of NativeWorkspacePort. It is
// intentionally a port, not a general filesystem API: all paths are relative,
// and all calls are scoped by NativeOperationContext.WorktreeRoot.
type OSWorkspacePort struct {
	MaxBytes int
}

func (p OSWorkspacePort) path(ctx NativeOperationContext, relative string) (string, error) {
	root, err := workspace.CanonicalRoot(ctx.WorktreeRoot)
	if err != nil {
		return "", ErrWorktreeViolation
	}
	return workspace.ContainedPath(root, relative)
}
func (p OSWorkspacePort) Inspect(_ context.Context, ctx NativeOperationContext, relative string) (NativeFileInfo, error) {
	path, err := p.path(ctx, relative)
	if err != nil {
		return NativeFileInfo{}, err
	}
	info, err := os.Stat(path)
	if err != nil {
		return NativeFileInfo{}, err
	}
	return NativeFileInfo{Path: relative, Size: info.Size(), Mode: info.Mode().String(), IsDir: info.IsDir(), ModTime: info.ModTime().UTC().Format(time.RFC3339)}, nil
}
func (p OSWorkspacePort) ReadFile(_ context.Context, ctx NativeOperationContext, relative string) ([]byte, error) {
	path, err := p.path(ctx, relative)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	limit := p.MaxBytes
	if limit <= 0 {
		limit = defaultNativeMaxBytes
	}
	if len(data) > limit {
		return nil, fmt.Errorf("%w: file is bounded", ErrAdapterFailure)
	}
	return data, nil
}
func (p OSWorkspacePort) WriteFile(_ context.Context, ctx NativeOperationContext, relative string, data []byte) error {
	path, err := p.path(ctx, relative)
	if err != nil {
		return err
	}
	limit := p.MaxBytes
	if limit <= 0 {
		limit = defaultNativeMaxBytes
	}
	if len(data) > limit {
		return fmt.Errorf("%w: file is bounded", ErrAdapterFailure)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}
func (p OSWorkspacePort) EditFile(_ context.Context, ctx NativeOperationContext, relative, old, replacement string) error {
	data, err := p.ReadFile(context.Background(), ctx, relative)
	if err != nil {
		return err
	}
	if bytes.Count(data, []byte(old)) != 1 {
		return fmt.Errorf("%w: edit match must be unique", ErrInvalidContract)
	}
	return p.WriteFile(context.Background(), ctx, relative, bytes.Replace(data, []byte(old), []byte(replacement), 1))
}
func (p OSWorkspacePort) Search(_ context.Context, ctx NativeOperationContext, relative, query string, limit int) ([]NativeSearchMatch, error) {
	if query == "" {
		return nil, fmt.Errorf("%w: search query is required", ErrInvalidContract)
	}
	root, err := workspace.CanonicalRoot(ctx.WorktreeRoot)
	if err != nil {
		return nil, ErrWorktreeViolation
	}
	base := root
	if relative != "" {
		base, err = workspace.ContainedPath(root, relative)
		if err != nil {
			return nil, err
		}
	}
	matches := make([]NativeSearchMatch, 0)
	err = filepath.Walk(base, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if info.IsDir() || (limit > 0 && len(matches) >= limit) {
			return nil
		}
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			return nil
		}
		for lineNo, line := range strings.Split(string(data), "\n") {
			if strings.Contains(line, query) {
				rel, _ := filepath.Rel(root, path)
				matches = append(matches, NativeSearchMatch{Path: filepath.ToSlash(rel), Line: lineNo + 1, Match: limitString(line, 4096)})
				if limit > 0 && len(matches) >= limit {
					break
				}
			}
		}
		return nil
	})
	return matches, err
}

// OSGitPort is a fixed-argv Git port. It never invokes a shell and does not
// expose arbitrary Git arguments to the native adapter.
type OSGitPort struct {
	Runner   workspacegit.CommandRunner
	MaxBytes int
}

func (p OSGitPort) Status(ctx context.Context, op NativeOperationContext) (NativeGitStatus, error) {
	root, err := workspace.CanonicalRoot(op.WorktreeRoot)
	if err != nil {
		return NativeGitStatus{}, ErrWorktreeViolation
	}
	runner := p.Runner
	if runner == nil {
		return NativeGitStatus{}, fmt.Errorf("%w: Git port runner is required", ErrInvalidContract)
	}
	stdout, stderr, runErr := runner.Run(ctx, "-C", root, "status", "--porcelain=v1", "--branch", "--untracked-files=all")
	if runErr != nil {
		_ = stderr
		return NativeGitStatus{}, fmt.Errorf("%w: Git status failed", ErrAdapterFailure)
	}
	return parseNativeGitStatus(string(stdout)), nil
}
func (p OSGitPort) Diff(ctx context.Context, op NativeOperationContext, relative string) (NativeGitDiff, error) {
	root, err := workspace.CanonicalRoot(op.WorktreeRoot)
	if err != nil {
		return NativeGitDiff{}, ErrWorktreeViolation
	}
	runner := p.Runner
	if runner == nil {
		return NativeGitDiff{}, fmt.Errorf("%w: Git port runner is required", ErrInvalidContract)
	}
	args := []string{"-C", root, "diff", "--no-ext-diff", "--binary", "--"}
	if relative != "" {
		clean, pathErr := workspace.CanonicalRelative(relative)
		if pathErr != nil {
			return NativeGitDiff{}, pathErr
		}
		args = append(args, filepath.ToSlash(clean))
	}
	stdout, _, runErr := runner.Run(ctx, args...)
	if runErr != nil {
		return NativeGitDiff{}, fmt.Errorf("%w: Git diff failed", ErrAdapterFailure)
	}
	return NativeGitDiff{Path: relative, Patch: string(stdout)}, nil
}

func parseNativeGitStatus(output string) NativeGitStatus {
	status := NativeGitStatus{Entries: []NativeGitEntry{}}
	for _, line := range strings.Split(strings.ReplaceAll(output, "\r\n", "\n"), "\n") {
		if strings.HasPrefix(line, "## ") {
			status.Branch = strings.TrimPrefix(line, "## ")
			continue
		}
		if len(line) < 3 {
			continue
		}
		status.Entries = append(status.Entries, NativeGitEntry{Path: strings.TrimSpace(line[3:]), Index: string(line[0]), Workdir: string(line[1])})
	}
	status.Dirty = len(status.Entries) > 0
	return status
}

// NativeSandboxValidationPort maps validation kinds to fixed commands and uses
// ExecutionSandbox for path, timeout, environment, output, and cancellation
// enforcement. It deliberately has no arbitrary-command mode.
type NativeSandboxValidationPort struct{ Sandbox *ExecutionSandbox }

func (p NativeSandboxValidationPort) Run(ctx context.Context, op NativeOperationContext, request NativeValidationRequest) (NativeValidationResult, error) {
	if p.Sandbox == nil {
		return NativeValidationResult{}, fmt.Errorf("%w: validation sandbox is required", ErrInvalidContract)
	}
	var args []string
	switch request.Kind {
	case "test_run":
		args = []string{"test", "./..."}
	case "lint":
		args = []string{"vet", "./..."}
	case "build":
		args = []string{"build", "./..."}
	default:
		return NativeValidationResult{}, fmt.Errorf("%w: %s", ErrNativeUnsupportedOperation, request.Kind)
	}
	// Selectors are intentionally ignored: validation is a stable repository
	// check, rather than an arbitrary process execution surface.
	result, err := p.Sandbox.ExecuteWithSandbox(NewExecutionContext(ctx, op.Audit.TraceID), &DefaultProcessRunner{}, "go", args, nil, ".")
	out := NativeValidationResult{Kind: request.Kind, Passed: err == nil, ExitCode: result.ExitCode, Stdout: string(result.RedactedOut), Stderr: string(result.RedactedErr), Truncated: result.Truncated}
	if len(request.Args) > 0 {
		// Args field is intentionally omitted; request.Args are not used to
		// prevent arbitrary command execution through validation ports.
	}
	return out, err
}
