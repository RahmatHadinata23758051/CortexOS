package harness

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// PiCLIAdapter drives the real `pi` CLI as a Harness coding engine.
//
// The legacy PiAdapter speaks the harness JSONL worker protocol
// (`pi --handshake` / `pi --worker`). The installed pi CLI (>= 1.2) does not
// implement those flags; it exposes a non-interactive JSON event mode
// (`pi --print --mode json`). This adapter is the boundary for that real CLI,
// so Pi can be selected as a coding engine without shipping a protocol shim.
//
// All execution flows through the shared ExecutionSandbox: worktree-root
// validation, bounded/redacted output, environment filtering, and process-group
// cancellation. The prompt is supplied as one argv element (never a shell), so
// shell metacharacters are inert; it is length/NUL bounded instead of being run
// through the sandbox's shell-oriented argument validator.
const (
	PiCLIAdapterName    = "pi-cli"
	PiCLIAdapterVersion = "1.0.0"

	piCLIMaxPromptBytes = 64 * 1024
	piCLIMaxSummaryLen  = 4096
)

var (
	ErrPiCLIExecutableNotFound = fmt.Errorf("%w: pi-cli executable not found", ErrAdapterUnhealthy)
	ErrPiCLIProcessCrash       = fmt.Errorf("%w: pi-cli process crashed", ErrAdapterFailure)
	ErrPiCLIProcessFailed      = fmt.Errorf("%w: pi-cli process failed", ErrAdapterFailure)
	ErrPiCLIProtocolError      = fmt.Errorf("%w: pi-cli protocol error", ErrAdapterFailure)
	ErrPiCLIInvalidConfig      = fmt.Errorf("%w: pi-cli configuration invalid", ErrInvalidContract)
)

// PiCLIAdapterConfig controls construction of the Pi CLI adapter.
type PiCLIAdapterConfig struct {
	ExecutablePath string
	WorktreeRoot   string
	AllowedEnvVars []string
	MaxOutputBytes int64
	MaxPromptBytes int
	Sandbox        *ExecutionSandbox
	Runner         ProcessRunner
	OnEvidence     func(EvidenceRecord)
}

// PiCLIAdapter implements EngineAdapter for the Pi CLI engine.
type PiCLIAdapter struct {
	executablePath string
	worktreeRoot   string
	timeout        time.Duration
	maxPromptBytes int
	sandbox        *ExecutionSandbox
	runner         ProcessRunner
	onEvidence     func(EvidenceRecord)
	capabilities   []ToolCapability
}

func NewPiCLIAdapter(config PiCLIAdapterConfig) (*PiCLIAdapter, error) {
	if strings.TrimSpace(config.WorktreeRoot) == "" {
		return nil, fmt.Errorf("%w: worktree root is required", ErrPiCLIInvalidConfig)
	}
	root, err := canonicalPath(config.WorktreeRoot)
	if err != nil {
		return nil, fmt.Errorf("%w: worktree root is invalid", ErrPiCLIInvalidConfig)
	}
	if info, statErr := os.Stat(root); statErr != nil || !info.IsDir() {
		return nil, fmt.Errorf("%w: worktree root is unavailable", ErrPiCLIInvalidConfig)
	}

	executable := strings.TrimSpace(config.ExecutablePath)
	runner := config.Runner
	if executable == "" {
		if runner == nil {
			found, findErr := findPiCLIExecutable()
			if findErr != nil {
				return nil, findErr
			}
			executable = found
		} else {
			executable = "pi"
		}
	} else if runner == nil {
		if _, lookErr := exec.LookPath(executable); lookErr != nil {
			if _, statErr := os.Stat(executable); statErr != nil {
				return nil, fmt.Errorf("%w: pi executable not found", ErrPiCLIExecutableNotFound)
			}
		}
	}
	if runner == nil {
		runner = &DefaultProcessRunner{}
	}

	sandbox := config.Sandbox
	if sandbox == nil {
		sbCfg := SandboxConfig{WorktreeRoot: root, AllowedEnvVars: config.AllowedEnvVars}
		if config.MaxOutputBytes > 0 {
			sbCfg.StdoutLimitBytes = config.MaxOutputBytes
			sbCfg.StderrLimitBytes = config.MaxOutputBytes
		}
		sandbox, err = NewSandbox(sbCfg)
		if err != nil {
			return nil, fmt.Errorf("%w: failed to initialize sandbox: %v", ErrPiCLIInvalidConfig, err)
		}
	}

	maxPrompt := config.MaxPromptBytes
	if maxPrompt <= 0 {
		maxPrompt = piCLIMaxPromptBytes
	}

	return &PiCLIAdapter{
		executablePath: executable,
		worktreeRoot:   root,
		timeout:        sandbox.Config().Timeout,
		maxPromptBytes: maxPrompt,
		sandbox:        sandbox,
		runner:         runner,
		onEvidence:     config.OnEvidence,
		capabilities:   PiCapabilities(),
	}, nil
}

// findPiCLIExecutable locates the installed pi CLI. Besides PATH it checks the
// standard pi install layout (~/.pi/agent/bin), because the Windows launcher
// (pi.cmd) is commonly not on PATH.
// ponytail: fixed well-known locations, not a general search. Add config-driven
// paths if another install layout appears.
func findPiCLIExecutable() (string, error) {
	if path, err := findPiExecutable(); err == nil {
		return path, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("%w: pi not found in PATH", ErrPiCLIExecutableNotFound)
	}
	for _, name := range []string{"pi", "pi.cmd", "pi.exe"} {
		candidate := filepath.Join(home, ".pi", "agent", "bin", name)
		if info, statErr := os.Stat(candidate); statErr == nil && !info.IsDir() {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("%w: pi not found in PATH or ~/.pi/agent/bin", ErrPiCLIExecutableNotFound)
}

func (a *PiCLIAdapter) Kind() ToolKind { return PiAdapterKind }

func (a *PiCLIAdapter) Capabilities() []ToolCapability {
	out := make([]ToolCapability, len(a.capabilities))
	copy(out, a.capabilities)
	return out
}

func (a *PiCLIAdapter) Describe() AdapterDescriptor {
	return AdapterDescriptor{
		Name:          PiCLIAdapterName,
		Kind:          PiAdapterKind,
		Version:       PiCLIAdapterVersion,
		Capabilities:  a.Capabilities(),
		SchemaVersion: HarnessContractVersion,
	}
}

// Health is an offline check: it never invokes the model and therefore does not
// depend on provider authentication. A broken/missing credential surfaces at
// execution time as a failed task with diagnostic evidence.
func (a *PiCLIAdapter) Health(ctx ExecutionContext) error {
	if ctx == nil {
		ctx = NewExecutionContext(context.Background(), "")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if a == nil || a.sandbox == nil {
		return fmt.Errorf("%w: adapter is uninitialized", ErrAdapterUnhealthy)
	}
	if info, err := os.Stat(a.worktreeRoot); err != nil || !info.IsDir() {
		return fmt.Errorf("%w: worktree root is unavailable", ErrAdapterUnhealthy)
	}
	if _, ok := a.runner.(*DefaultProcessRunner); ok || a.runner == nil {
		if _, lookErr := exec.LookPath(a.executablePath); lookErr != nil {
			if _, statErr := os.Stat(a.executablePath); statErr != nil {
				return fmt.Errorf("%w: pi executable not found", ErrPiCLIExecutableNotFound)
			}
		}
	}
	return nil
}

// Execute runs `pi --print --mode json --no-session -- <prompt>` in the worktree
// and maps its JSON event stream to a ToolResult plus command-execution
// evidence. Pi output is UNTRUSTED EVIDENCE; Orchestra remains the success
// authority.
func (a *PiCLIAdapter) Execute(ctx ExecutionContext, envelope ExecutionEnvelope) (ToolResult, error) {
	started := time.Now().UTC()
	result := ToolResult{
		ContractVersion: HarnessContractVersion,
		ExecutionID:     envelope.ExecutionID,
		TaskID:          envelope.TaskID,
		ToolName:        envelope.ToolName,
		Status:          ToolStatusFailed,
		Redacted:        true,
		StartedAt:       started,
	}
	if ctx == nil {
		ctx = NewExecutionContext(context.Background(), envelope.TraceID)
	}
	if err := a.validateEnvelope(ctx, envelope); err != nil {
		return a.finishError(result, started, err)
	}
	if err := ctx.Err(); err != nil {
		if errors.Is(err, context.Canceled) {
			return a.finishError(result, started, ErrCanceled)
		}
		if errors.Is(err, context.DeadlineExceeded) {
			return a.finishError(result, started, ErrTimeout)
		}
		return a.finishError(result, started, err)
	}

	prompt := piCLIPrompt(envelope.Input)
	if strings.TrimSpace(prompt) == "" {
		return a.finishError(result, started, fmt.Errorf("%w: prompt is empty", ErrInvalidContract))
	}
	if len(prompt) > a.maxPromptBytes || strings.IndexByte(prompt, 0) >= 0 {
		return a.finishError(result, started, fmt.Errorf("%w: prompt exceeds limits", ErrPiCLIInvalidConfig))
	}

	timeout := envelope.Timeout
	if a.timeout > 0 && timeout > a.timeout {
		timeout = a.timeout
	}
	childCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	sandboxResult, runErr := a.run(childCtx, []string{"--print", "--mode", "json", "--no-session", "--"}, prompt, os.Environ())
	if errors.Is(runErr, ErrTimeout) || sandboxResult.TimedOut {
		return a.finishError(result, started, ErrTimeout)
	}
	if errors.Is(runErr, ErrCanceled) || sandboxResult.Canceled {
		return a.finishError(result, started, ErrCanceled)
	}

	outcome, parseErr := parsePiCLIStream(sandboxResult.Stdout)
	if parseErr != nil {
		if runErr != nil {
			return a.finishError(result, started, fmt.Errorf("%w: process exited without a usable event stream", ErrPiCLIProcessCrash))
		}
		return a.finishError(result, started, parseErr)
	}

	rec, recErr := NewEvidenceRecord(envelope.ExecutionID, envelope.TaskID, envelope.WorktreeID, EvidenceKindCommandExecution, piCLIEvidence{
		ExitCode: sandboxResult.ExitCode,
		Summary:  truncate(outcome.Summary, piCLIMaxSummaryLen),
		Error:    outcome.ErrorMessage,
	})
	if recErr == nil {
		result.EvidenceIDs = []string{rec.ID}
		if a.onEvidence != nil {
			a.onEvidence(rec)
		}
	}

	output, _ := json.Marshal(piCLIResultOutput{
		ExitCode: sandboxResult.ExitCode,
		Summary:  truncate(outcome.Summary, piCLIMaxSummaryLen),
		Error:    outcome.ErrorMessage,
	})
	result.Output = RedactBytes(output)

	if outcome.ErrorMessage != "" || sandboxResult.ExitCode != 0 {
		return a.finishError(result, started, fmt.Errorf("%w: pi reported %s", ErrPiCLIProcessFailed,
			firstNonEmpty(outcome.ErrorMessage, fmt.Sprintf("exit code %d", sandboxResult.ExitCode))))
	}

	completed := time.Now().UTC()
	result.Status = ToolStatusSuccess
	result.CompletedAt = completed
	result.DurationMs = completed.Sub(started).Milliseconds()
	return result, nil
}

// run validates the fixed flags through the sandbox, then passes the prompt as a
// final argv element. The prompt is deliberately NOT routed through
// ValidateArgs: that validator rejects shell syntax and ".." which are inert for
// an argv-only exec but common in task text.
func (a *PiCLIAdapter) run(ctx context.Context, args []string, prompt string, env []string) (SandboxResult, error) {
	if a.runner == nil {
		a.runner = &DefaultProcessRunner{}
	}
	config := a.sandbox.Config()
	workDir, err := a.sandbox.ValidatePath(".")
	if err != nil {
		return SandboxResult{}, err
	}
	if err := a.sandbox.ValidateArgs(args, nil); err != nil {
		return SandboxResult{}, err
	}
	if env == nil {
		env = os.Environ()
	}
	filtered, err := a.sandbox.FilterEnvironment(env)
	if err != nil {
		return SandboxResult{}, err
	}
	full := append(append([]string(nil), args...), prompt)
	res, runErr := a.runner.Run(ctx, a.executablePath, full, filtered, workDir,
		config.StdoutLimitBytes, config.StderrLimitBytes, config.CancellationGracePeriod)
	res.RedactedOut = redactOutput(res.Stdout, config.RedactionConfig)
	res.RedactedErr = redactOutput(res.Stderr, config.RedactionConfig)
	if errors.Is(ctx.Err(), context.DeadlineExceeded) || res.TimedOut {
		res.TimedOut = true
		return res, ErrTimeout
	}
	if (errors.Is(ctx.Err(), context.Canceled) || res.Canceled) && !res.TimedOut {
		res.Canceled = true
		return res, ErrCanceled
	}
	if runErr != nil {
		return res, runErr
	}
	return res, nil
}

func (a *PiCLIAdapter) validateEnvelope(ctx ExecutionContext, envelope ExecutionEnvelope) error {
	if envelope.ContractVersion != HarnessContractVersion {
		return fmt.Errorf("%w: unsupported execution contract version", ErrInvalidContract)
	}
	if envelope.ExecutionID == "" || envelope.TaskID == "" || envelope.WorktreeID == "" || envelope.ToolName == "" {
		return fmt.Errorf("%w: execution identifiers are required", ErrInvalidContract)
	}
	if envelope.WorktreeRoot == "" {
		return ErrWorktreeViolation
	}
	root, err := canonicalPath(envelope.WorktreeRoot)
	if err != nil || !samePath(root, a.worktreeRoot) {
		return ErrWorktreeViolation
	}
	if envelope.Timeout <= 0 {
		return fmt.Errorf("%w: timeout must be positive", ErrInvalidContract)
	}
	if !envelope.PolicyDecision.Allowed {
		return ErrPolicyDenied
	}
	if !a.supports(envelope.ToolName) {
		return fmt.Errorf("%w: %s", ErrPiCapabilityMissing, envelope.ToolName)
	}
	if a.sandbox != nil {
		if err := a.sandbox.Validate(ctx, envelope); err != nil {
			return err
		}
	}
	return nil
}

func (a *PiCLIAdapter) supports(toolName string) bool {
	norm := strings.ToLower(strings.TrimSpace(toolName))
	norm = strings.TrimPrefix(norm, "pi_")
	norm = strings.TrimSuffix(norm, "_agent")
	norm = strings.ReplaceAll(norm, "-", "_")
	switch norm {
	case "coding", "analysis", "refactor", "debug", "review", "test_gen", "doc_gen", "pi", "pi_task", "default":
		return true
	}
	for _, cap := range a.capabilities {
		if string(cap) == norm {
			return true
		}
	}
	return false
}

func (a *PiCLIAdapter) finishError(result ToolResult, started time.Time, err error) (ToolResult, error) {
	coded := ErrorFor(err)
	result.Error = &ToolError{Code: string(coded.Code), Message: coded.Message}
	result.Status = toolStatusFromError(coded)
	completed := time.Now().UTC()
	result.CompletedAt, result.DurationMs = completed, completed.Sub(started).Milliseconds()
	return result, err
}

// piCLITaskInput covers the Orchestra bridge input shape and a direct prompt.
type piCLITaskInput struct {
	Prompt             string   `json:"prompt"`
	Title              string   `json:"title"`
	AcceptanceCriteria []string `json:"acceptanceCriteria"`
}

func piCLIPrompt(raw json.RawMessage) string {
	var in piCLITaskInput
	if len(bytes.TrimSpace(raw)) > 0 {
		_ = json.Unmarshal(raw, &in)
	}
	if strings.TrimSpace(in.Prompt) != "" {
		return strings.TrimSpace(in.Prompt)
	}
	var b strings.Builder
	b.WriteString(strings.TrimSpace(in.Title))
	if len(in.AcceptanceCriteria) > 0 {
		b.WriteString("\n\nAcceptance criteria:\n")
		for _, c := range in.AcceptanceCriteria {
			b.WriteString("- ")
			b.WriteString(strings.TrimSpace(c))
			b.WriteString("\n")
		}
	}
	return strings.TrimSpace(b.String())
}

type piCLIResultOutput struct {
	ExitCode int    `json:"exitCode"`
	Summary  string `json:"summary,omitempty"`
	Error    string `json:"error,omitempty"`
}

type piCLIEvidence struct {
	ExitCode int    `json:"exitCode"`
	Summary  string `json:"summary,omitempty"`
	Error    string `json:"error,omitempty"`
}

// piCLIEvent is the minimal shape of a `pi --mode json` stream line.
type piCLIEvent struct {
	Type    string        `json:"type"`
	Message *piCLIMessage `json:"message"`
}

type piCLIMessage struct {
	Role         string          `json:"role"`
	Content      json.RawMessage `json:"content"`
	ErrorMessage string          `json:"errorMessage"`
}

type piCLIOutcome struct {
	Summary      string
	ErrorMessage string
}

// parsePiCLIStream reads newline-delimited JSON events, capturing the last
// assistant text and the first model/provider error message.
func parsePiCLIStream(stdout []byte) (piCLIOutcome, error) {
	if len(bytes.TrimSpace(stdout)) == 0 {
		return piCLIOutcome{}, fmt.Errorf("%w: pi emitted no output", ErrPiCLIProtocolError)
	}
	var (
		outcome  piCLIOutcome
		sawEvent bool
	)
	for _, line := range bytes.Split(stdout, []byte("\n")) {
		line = bytes.TrimSpace(line)
		if len(line) == 0 {
			continue
		}
		var event piCLIEvent
		if err := json.Unmarshal(line, &event); err != nil {
			continue // tolerate non-JSON banner lines
		}
		if event.Message == nil {
			continue
		}
		sawEvent = true
		if event.Message.ErrorMessage != "" && outcome.ErrorMessage == "" {
			outcome.ErrorMessage = event.Message.ErrorMessage
		}
		if event.Message.Role == "assistant" {
			if text := piCLIContentText(event.Message.Content); text != "" {
				outcome.Summary = text
			}
		}
	}
	if !sawEvent {
		return piCLIOutcome{}, fmt.Errorf("%w: pi emitted no parseable events", ErrPiCLIProtocolError)
	}
	return outcome, nil
}

// piCLIContentText extracts text from either a string or an array of parts.
func piCLIContentText(raw json.RawMessage) string {
	if len(bytes.TrimSpace(raw)) == 0 {
		return ""
	}
	var asString string
	if err := json.Unmarshal(raw, &asString); err == nil {
		return asString
	}
	var parts []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	}
	if err := json.Unmarshal(raw, &parts); err != nil {
		return ""
	}
	var b strings.Builder
	for _, p := range parts {
		if p.Type == "text" && p.Text != "" {
			if b.Len() > 0 {
				b.WriteString("\n")
			}
			b.WriteString(p.Text)
		}
	}
	return b.String()
}

func truncate(s string, max int) string {
	if max <= 0 || len(s) <= max {
		return s
	}
	return s[:max]
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

var _ EngineAdapter = (*PiCLIAdapter)(nil)
