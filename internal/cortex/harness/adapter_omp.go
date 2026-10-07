package harness

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"
)

const (
	OMPAdapterName    = "omp"
	OMPAdapterVersion = "1.0.0"
	OMPAdapterKind    = ToolKindOMP
	// Hard ceiling on escalation attempts to prevent unbounded process loops.
	// This is the absolute maximum number of process attempts (1 initial + retries).
	HardOMPMaxAttempts = 4
)

// Errors returned by the OMP adapter are deliberately based on stable harness
// errors.  Callers can therefore health-check OMP and fall back to another
// adapter without depending on process or provider details.
var (
	ErrOMPExecutableNotFound = fmt.Errorf("%w: omp executable not found", ErrAdapterUnhealthy)
	ErrOMPHandshakeFailed    = fmt.Errorf("%w: omp handshake failed", ErrAdapterUnhealthy)
	ErrOMPProcessCrash       = fmt.Errorf("%w: omp process crashed", ErrAdapterFailure)
	ErrOMPProcessFailed      = fmt.Errorf("%w: omp process failed", ErrAdapterFailure)
	ErrOMPProtocolError      = fmt.Errorf("%w: omp protocol error", ErrAdapterFailure)
	ErrOMPCapabilityMissing  = fmt.Errorf("%w: omp capability missing", ErrNoEligibleAdapter)
	ErrOMPInvalidConfig      = fmt.Errorf("%w: omp configuration invalid", ErrInvalidContract)
)

// OMPCapabilities are intentionally specialist-only.  OMP must not become a
// default/general-purpose route merely because it is installed.
func OMPCapabilities() []ToolCapability {
	return []ToolCapability{
		CapabilitySpecialist,
		CapabilityRecovery,
		CapabilitySecurityAudit,
		CapabilityDeepDebug,
		CapabilityArchitectureReview,
	}
}

func OMPExtendedDescriptor() ExtendedAdapterDescriptor {
	return ExtendedAdapterDescriptor{
		AdapterDescriptor: AdapterDescriptor{
			Name:          OMPAdapterName,
			Kind:          OMPAdapterKind,
			Version:       OMPAdapterVersion,
			Capabilities:  OMPCapabilities(),
			SchemaVersion: HarnessContractVersion,
		},
		Identity: AdapterIdentity{
			Name:          OMPAdapterName,
			InstanceID:    "omp-default",
			Kind:          OMPAdapterKind,
			Class:         EngineClassOMP,
			Version:       OMPAdapterVersion,
			SchemaVersion: HarnessContractVersion,
		},
		ResourceReq: DefaultResourceRequirements(EngineClassOMP),
	}
}

// OMPAdapterConfig configures the OMP process boundary.  MaxEscalations and
// RetryLimit are both hard caps: an execution can make at most
// 1 + min(MaxEscalations, RetryLimit) process attempts.  Zero means no retry.
type OMPAdapterConfig struct {
	ExecutablePath string
	WorktreeRoot   string
	AllowedEnvVars []string
	MaxOutputBytes int64
	StartupTimeout time.Duration
	MaxEscalations int
	RetryLimit     int
	Sandbox        *ExecutionSandbox
	Runner         ProcessRunner
	OnEvidence     func(EvidenceRecord)
}

type OMPAdapter struct {
	executablePath string
	worktreeRoot   string
	startupTimeout time.Duration
	maxEscalations int
	retryLimit     int
	sandbox        *ExecutionSandbox
	runner         ProcessRunner
	capabilities   []ToolCapability
	onEvidence     func(EvidenceRecord)
}

func NewOMPAdapter(config OMPAdapterConfig) (*OMPAdapter, error) {
	if strings.TrimSpace(config.WorktreeRoot) == "" {
		return nil, fmt.Errorf("%w: worktree root is required", ErrOMPInvalidConfig)
	}
	root, err := canonicalPath(config.WorktreeRoot)
	if err != nil {
		return nil, fmt.Errorf("%w: worktree root is invalid", ErrOMPInvalidConfig)
	}
	info, err := os.Stat(root)
	if err != nil || !info.IsDir() {
		return nil, fmt.Errorf("%w: worktree root is unavailable", ErrOMPInvalidConfig)
	}
	if config.MaxEscalations < 0 || config.RetryLimit < 0 {
		return nil, fmt.Errorf("%w: escalation and retry limits must not be negative", ErrOMPInvalidConfig)
	}

	executable := strings.TrimSpace(config.ExecutablePath)
	runner := config.Runner
	if executable == "" {
		if runner == nil {
			executable, err = findOMPExecutable()
			if err != nil {
				return nil, err
			}
		} else {
			executable = "omp"
		}
	} else if runner == nil {
		if _, lookErr := exec.LookPath(executable); lookErr != nil {
			if _, statErr := os.Stat(executable); statErr != nil {
				return nil, ErrOMPExecutableNotFound
			}
		}
	}
	if runner == nil {
		runner = &DefaultProcessRunner{}
	}

	sandbox := config.Sandbox
	if sandbox == nil {
		sbConfig := SandboxConfig{WorktreeRoot: root, AllowedEnvVars: config.AllowedEnvVars}
		if config.MaxOutputBytes > 0 {
			sbConfig.StdoutLimitBytes = config.MaxOutputBytes
			sbConfig.StderrLimitBytes = config.MaxOutputBytes
		}
		sandbox, err = NewSandbox(sbConfig)
		if err != nil {
			return nil, fmt.Errorf("%w: failed to initialize sandbox", ErrOMPInvalidConfig)
		}
	}

	startupTimeout := config.StartupTimeout
	if startupTimeout <= 0 {
		startupTimeout = 10 * time.Second
	}
	return &OMPAdapter{
		executablePath: executable,
		worktreeRoot:   root,
		startupTimeout: startupTimeout,
		maxEscalations: config.MaxEscalations,
		retryLimit:     config.RetryLimit,
		sandbox:        sandbox,
		runner:         runner,
		capabilities:   OMPCapabilities(),
		onEvidence:     config.OnEvidence,
	}, nil
}

func findOMPExecutable() (string, error) {
	for _, name := range []string{"omp", "omp.exe"} {
		if path, err := exec.LookPath(name); err == nil {
			return path, nil
		}
	}
	return "", ErrOMPExecutableNotFound
}

func (a *OMPAdapter) Kind() ToolKind { return OMPAdapterKind }

func (a *OMPAdapter) Capabilities() []ToolCapability {
	out := make([]ToolCapability, len(a.capabilities))
	copy(out, a.capabilities)
	return out
}

func (a *OMPAdapter) Describe() AdapterDescriptor {
	return AdapterDescriptor{
		Name:          OMPAdapterName,
		Kind:          OMPAdapterKind,
		Version:       OMPAdapterVersion,
		Capabilities:  a.Capabilities(),
		SchemaVersion: HarnessContractVersion,
	}
}

// Health is intentionally non-fatal for construction: an unavailable OMP is
// a normal deployment state and the router can select a fallback adapter.
func (a *OMPAdapter) Health(ctx ExecutionContext) error {
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
		if _, err := exec.LookPath(a.executablePath); err != nil {
			if _, statErr := os.Stat(a.executablePath); statErr != nil {
				return ErrOMPExecutableNotFound
			}
		}
	}
	return a.performHandshake(ctx)
}

func (a *OMPAdapter) performHandshake(ctx ExecutionContext) error {
	childCtx, cancel := context.WithTimeout(ctx, a.startupTimeout)
	defer cancel()
	res, err := a.runProcess(childCtx, a.executablePath, []string{"--handshake"}, nil, a.worktreeRoot)
	if err != nil {
		if errors.Is(err, ErrTimeout) || errors.Is(err, context.DeadlineExceeded) {
			return fmt.Errorf("%w: handshake timed out", ErrOMPHandshakeFailed)
		}
		if errors.Is(err, ErrCanceled) || errors.Is(err, context.Canceled) {
			return ctx.Err()
		}
		return fmt.Errorf("%w: handshake execution failed", ErrOMPHandshakeFailed)
	}
	trimmed := bytes.TrimSpace(res.Stdout)
	if len(trimmed) == 0 {
		return fmt.Errorf("%w: empty handshake output", ErrOMPHandshakeFailed)
	}
	var handshake HandshakePayload
	if json.Unmarshal(trimmed, &handshake) != nil || handshake.Protocol == "" {
		var msg Message
		if json.Unmarshal(trimmed, &msg) != nil || msg.Type != MessageHandshake || json.Unmarshal(msg.Payload, &handshake) != nil {
			return fmt.Errorf("%w: invalid handshake format", ErrOMPHandshakeFailed)
		}
	}
	if err := handshake.Validate(); err != nil || handshake.Protocol != WorkerProtocolVersion {
		return fmt.Errorf("%w: unsupported handshake", ErrOMPHandshakeFailed)
	}
	return nil
}

func (a *OMPAdapter) Execute(ctx ExecutionContext, envelope ExecutionEnvelope) (ToolResult, error) {
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

	taskEnvelope := MapExecutionEnvelopeToTask(envelope)
	if err := taskEnvelope.Validate(); err != nil {
		return a.finishError(result, started, err)
	}
	retries := 0
	if a.maxEscalations > 0 && a.retryLimit > 0 {
		retries = minOMPInt(a.maxEscalations, a.retryLimit)
	} else if a.maxEscalations > 0 {
		retries = a.maxEscalations
	} else if a.retryLimit > 0 {
		retries = a.retryLimit
	}
	if retries > HardOMPMaxAttempts-1 {
		retries = HardOMPMaxAttempts - 1
	}
	attempts := 1 + retries
	var lastErr error
	var evidenceIDs []string
	for attempt := 0; attempt < attempts; attempt++ {
		if err := ctx.Err(); err != nil {
			if errors.Is(err, context.Canceled) {
				return a.finishError(result, started, ErrCanceled)
			}
			return a.finishError(result, started, ErrTimeout)
		}
		timeout := envelope.Timeout
		if sandboxTimeout := a.sandbox.Config().Timeout; sandboxTimeout > 0 && timeout > sandboxTimeout {
			timeout = sandboxTimeout
		}
		childCtx, cancel := context.WithTimeout(ctx, timeout)
		_, _ = a.sandbox.RecordAudit(envelope, SandboxAuditEvent{Event: "start", Command: a.executablePath, Args: []string{"--worker", "--engine", "omp"}})
		sandboxResult, runErr := a.runProcess(childCtx, a.executablePath, []string{"--worker", "--engine", "omp"}, os.Environ(), ".")
		cancel()
		if runErr != nil {
			if errors.Is(runErr, ErrTimeout) || sandboxResult.TimedOut {
				return a.finishError(result, started, ErrTimeout)
			}
			if errors.Is(runErr, ErrCanceled) || sandboxResult.Canceled {
				return a.finishError(result, started, ErrCanceled)
			}
			lastErr = ErrOMPProcessCrash
			continue
		}

		terminal, ids, parseErr := a.consumeJSONL(envelope, sandboxResult.Stdout)
		evidenceIDs = appendUniqueOMP(evidenceIDs, ids...)
		if parseErr != nil {
			lastErr = parseErr
			continue
		}
		if terminal.Status == TerminalCanceled {
			return a.finishError(result, started, ErrCanceled)
		}
		if terminal.Status == TerminalFailure {
			lastErr = ErrOMPProcessFailed
			continue
		}
		result.EvidenceIDs = evidenceIDs
		if len(sandboxResult.RedactedOut) > 0 {
			result.Output = json.RawMessage(sandboxResult.RedactedOut)
		} else {
			result.Output = json.RawMessage(RedactBytes(sandboxResult.Stdout))
		}
		result.Status = ToolStatusSuccess
		completed := time.Now().UTC()
		result.CompletedAt, result.DurationMs = completed, completed.Sub(started).Milliseconds()
		_, _ = a.sandbox.RecordAudit(envelope, SandboxAuditEvent{Event: "complete", Command: a.executablePath, ExitCode: sandboxResult.ExitCode, DurationMs: result.DurationMs, Redacted: true})
		return result, nil
	}
	if lastErr == nil {
		lastErr = ErrOMPProcessCrash
	}
	result.EvidenceIDs = evidenceIDs
	return a.finishError(result, started, lastErr)
}

func minOMPInt(left, right int) int {
	if left < right {
		return left
	}
	return right
}

func appendUniqueOMP(dst []string, values ...string) []string {
	for _, value := range values {
		if value == "" || containsString(dst, value) {
			continue
		}
		dst = append(dst, value)
	}
	return dst
}

func (a *OMPAdapter) validateEnvelope(ctx ExecutionContext, envelope ExecutionEnvelope) error {
	if envelope.ContractVersion != HarnessContractVersion {
		return fmt.Errorf("%w: unsupported execution contract version", ErrInvalidContract)
	}
	if envelope.ExecutionID == "" || envelope.TaskID == "" || envelope.WorktreeID == "" || envelope.ToolName == "" {
		return fmt.Errorf("%w: execution identifiers are required", ErrInvalidContract)
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
		return fmt.Errorf("%w: %s", ErrOMPCapabilityMissing, envelope.ToolName)
	}
	if a.sandbox == nil {
		return fmt.Errorf("%w: sandbox is unavailable", ErrAdapterUnhealthy)
	}
	return a.sandbox.Validate(ctx, envelope)
}

func (a *OMPAdapter) supports(toolName string) bool {
	norm := strings.ToLower(strings.TrimSpace(toolName))
	norm = strings.TrimPrefix(norm, "omp_")
	norm = strings.ReplaceAll(norm, "-", "_")
	for _, capability := range a.capabilities {
		if string(capability) == norm {
			return true
		}
	}
	switch norm {
	case "omp", "omp_task", "default":
		return true
	default:
		return false
	}
}

func (a *OMPAdapter) runProcess(ctx context.Context, command string, args []string, env []string, workDir string) (SandboxResult, error) {
	if a.runner == nil {
		a.runner = &DefaultProcessRunner{}
	}
	config := a.sandbox.Config()
	canonicalWorkDir, err := a.sandbox.ValidatePath(workDir)
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
	res, runErr := a.runner.Run(ctx, command, args, filtered, canonicalWorkDir, config.StdoutLimitBytes, config.StderrLimitBytes, config.CancellationGracePeriod)
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

func (a *OMPAdapter) consumeJSONL(envelope ExecutionEnvelope, stdout []byte) (*TerminalPayload, []string, error) {
	if len(bytes.TrimSpace(stdout)) == 0 {
		return nil, nil, ErrOMPProcessCrash
	}
	decoder := NewDecoder(bytes.NewReader(stdout))
	var terminal *TerminalPayload
	var evidenceIDs []string
	for {
		message, err := decoder.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, nil, fmt.Errorf("%w: %v", ErrOMPProtocolError, err)
		}
		switch message.Type {
		case MessageHandshake:
			var payload HandshakePayload
			if err := json.Unmarshal(message.Payload, &payload); err != nil || payload.Protocol != WorkerProtocolVersion {
				return nil, nil, fmt.Errorf("%w: invalid handshake", ErrOMPProtocolError)
			}
		case MessageTask:
			var task TaskEnvelope
			if err := json.Unmarshal(message.Payload, &task); err != nil || task.ExecutionID != envelope.ExecutionID || task.TaskID != envelope.TaskID {
				return nil, nil, fmt.Errorf("%w: task correlation mismatch", ErrOMPProtocolError)
			}
		case MessageProgress, MessageHeartbeat, MessageCapabilities, MessageHealth, MessageShutdown:
			// These records are validated by Decoder and are retained as lifecycle evidence.
		case MessageEvidence:
			var payload EvidencePayload
			if err := json.Unmarshal(message.Payload, &payload); err != nil || payload.Validate() != nil {
				return nil, nil, fmt.Errorf("%w: invalid evidence", ErrOMPProtocolError)
			}
			if payload.Record != nil {
				record := *payload.Record
				if record.ID != payload.EvidenceID || record.Kind != payload.Kind || record.Digest != payload.Digest || record.ExecutionID != envelope.ExecutionID || record.TaskID != envelope.TaskID || record.WorktreeID != envelope.WorktreeID {
					return nil, nil, fmt.Errorf("%w: evidence correlation mismatch", ErrOMPProtocolError)
				}
			}
			evidenceIDs = appendUniqueOMP(evidenceIDs, payload.EvidenceID)
			if a.onEvidence != nil {
				record := EvidenceRecord{}
				if payload.Record != nil {
					record = *payload.Record
				} else {
					record, err = NewEvidenceRecord(envelope.ExecutionID, envelope.TaskID, envelope.WorktreeID, payload.Kind, payload)
					if err != nil {
						return nil, nil, fmt.Errorf("%w: invalid evidence record", ErrOMPProtocolError)
					}
				}
				a.onEvidence(record)
			}
		case MessageDiagnostic:
			var diagnostic DiagnosticPayload
			if err := json.Unmarshal(message.Payload, &diagnostic); err != nil {
				return nil, nil, fmt.Errorf("%w: invalid diagnostic", ErrOMPProtocolError)
			}
		case MessageTerminal:
			var payload TerminalPayload
			if err := json.Unmarshal(message.Payload, &payload); err != nil {
				return nil, nil, fmt.Errorf("%w: invalid terminal", ErrOMPProtocolError)
			}
			terminal = &payload
			evidenceIDs = appendUniqueOMP(evidenceIDs, payload.EvidenceIDs...)
		}
	}
	if terminal == nil {
		return nil, nil, ErrOMPProcessCrash
	}
	return terminal, evidenceIDs, nil
}

func (a *OMPAdapter) finishError(result ToolResult, started time.Time, err error) (ToolResult, error) {
	coded := ErrorFor(err)
	result.Error = &ToolError{Code: string(coded.Code), Message: coded.Message}
	result.Status = toolStatusFromError(coded)
	completed := time.Now().UTC()
	result.CompletedAt, result.DurationMs = completed, completed.Sub(started).Milliseconds()
	return result, err
}

var _ EngineAdapter = (*OMPAdapter)(nil)
