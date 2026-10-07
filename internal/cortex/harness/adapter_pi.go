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
	PiAdapterName    = "pi"
	PiAdapterVersion = "1.0.0"
	PiAdapterKind    = ToolKindPi
)

// Pi error classes wrapping standard harness errors.
// Error messages are sanitized and never leak paths, env vars, or secrets.
var (
	ErrPiExecutableNotFound = fmt.Errorf("%w: pi executable not found", ErrAdapterUnhealthy)
	ErrPiHandshakeFailed    = fmt.Errorf("%w: pi handshake failed", ErrAdapterUnhealthy)
	ErrPiProcessCrash       = fmt.Errorf("%w: pi process crashed", ErrAdapterFailure)
	ErrPiProcessFailed      = fmt.Errorf("%w: pi process failed", ErrAdapterFailure)
	ErrPiProtocolError      = fmt.Errorf("%w: pi protocol error", ErrAdapterFailure)
	ErrPiCapabilityMissing  = fmt.Errorf("%w: pi capability missing", ErrNoEligibleAdapter)
	ErrPiInvalidConfig      = fmt.Errorf("%w: pi configuration invalid", ErrInvalidContract)
)

// PiCapabilities returns the capabilities provided by the Pi engine.
func PiCapabilities() []ToolCapability {
	return []ToolCapability{
		CapabilityCoding,
		CapabilityAnalysis,
		CapabilityRefactor,
		CapabilityDebug,
		CapabilityReview,
		CapabilityTestGen,
		CapabilityDocGen,
	}
}

// PiExtendedDescriptor returns the routing descriptor for Pi engine adapter.
func PiExtendedDescriptor() ExtendedAdapterDescriptor {
	return ExtendedAdapterDescriptor{
		AdapterDescriptor: AdapterDescriptor{
			Name:          PiAdapterName,
			Kind:          ToolKindPi,
			Version:       PiAdapterVersion,
			Capabilities:  PiCapabilities(),
			SchemaVersion: HarnessContractVersion,
		},
		Identity: AdapterIdentity{
			Name:          PiAdapterName,
			InstanceID:    "pi-default",
			Kind:          ToolKindPi,
			Class:         EngineClassPi,
			Version:       PiAdapterVersion,
			SchemaVersion: HarnessContractVersion,
		},
		ResourceReq: DefaultResourceRequirements(EngineClassPi),
	}
}

// PiAdapterConfig controls the configuration of a Pi engine adapter.
type PiAdapterConfig struct {
	ExecutablePath string
	WorktreeRoot   string
	AllowedEnvVars []string
	MaxOutputBytes int64
	StartupTimeout time.Duration
	Sandbox        *ExecutionSandbox
	Runner         ProcessRunner
	OnEvidence     func(EvidenceRecord)
}

// PiAdapter implements EngineAdapter for the Pi engine class.
type PiAdapter struct {
	executablePath string
	worktreeRoot   string
	startupTimeout time.Duration
	sandbox        *ExecutionSandbox
	runner         ProcessRunner
	capabilities   []ToolCapability
	onEvidence     func(EvidenceRecord)
}

// NewPiAdapter creates a new Pi adapter with sandbox and lifecycle enforcement.
func NewPiAdapter(config PiAdapterConfig) (*PiAdapter, error) {
	if strings.TrimSpace(config.WorktreeRoot) == "" {
		return nil, fmt.Errorf("%w: worktree root is required", ErrPiInvalidConfig)
	}
	root, err := canonicalPath(config.WorktreeRoot)
	if err != nil {
		return nil, fmt.Errorf("%w: worktree root is invalid", ErrPiInvalidConfig)
	}
	info, err := os.Stat(root)
	if err != nil || !info.IsDir() {
		return nil, fmt.Errorf("%w: worktree root is unavailable", ErrPiInvalidConfig)
	}

	executable := strings.TrimSpace(config.ExecutablePath)
	runner := config.Runner

	if executable == "" {
		if runner == nil {
			found, err := findPiExecutable()
			if err != nil {
				return nil, err
			}
			executable = found
		} else {
			executable = "pi"
		}
	} else {
		// When runner is not a fake runner, verify executable existence
		if runner == nil {
			if _, lookErr := exec.LookPath(executable); lookErr != nil {
				if _, statErr := os.Stat(executable); statErr != nil {
					return nil, fmt.Errorf("%w: pi executable not found", ErrPiExecutableNotFound)
				}
			}
		}
	}

	if runner == nil {
		runner = &DefaultProcessRunner{}
	}

	sandbox := config.Sandbox
	if sandbox == nil {
		sbCfg := SandboxConfig{
			WorktreeRoot:   root,
			AllowedEnvVars: config.AllowedEnvVars,
		}
		if config.MaxOutputBytes > 0 {
			sbCfg.StdoutLimitBytes = config.MaxOutputBytes
			sbCfg.StderrLimitBytes = config.MaxOutputBytes
		}
		var sbErr error
		sandbox, sbErr = NewSandbox(sbCfg)
		if sbErr != nil {
			return nil, fmt.Errorf("%w: failed to initialize sandbox: %v", ErrPiInvalidConfig, sbErr)
		}
	}

	startupTimeout := config.StartupTimeout
	if startupTimeout <= 0 {
		startupTimeout = 10 * time.Second
	}

	return &PiAdapter{
		executablePath: executable,
		worktreeRoot:   root,
		startupTimeout: startupTimeout,
		sandbox:        sandbox,
		runner:         runner,
		capabilities:   PiCapabilities(),
		onEvidence:     config.OnEvidence,
	}, nil
}

func findPiExecutable() (string, error) {
	for _, name := range []string{"pi", "pi.exe"} {
		path, err := exec.LookPath(name)
		if err == nil {
			return path, nil
		}
	}
	return "", fmt.Errorf("%w: pi not found in PATH", ErrPiExecutableNotFound)
}

func (a *PiAdapter) Kind() ToolKind {
	return PiAdapterKind
}

func (a *PiAdapter) Capabilities() []ToolCapability {
	out := make([]ToolCapability, len(a.capabilities))
	copy(out, a.capabilities)
	return out
}

func (a *PiAdapter) Describe() AdapterDescriptor {
	return AdapterDescriptor{
		Name:          PiAdapterName,
		Kind:          PiAdapterKind,
		Version:       PiAdapterVersion,
		Capabilities:  a.Capabilities(),
		SchemaVersion: HarnessContractVersion,
	}
}

// Health checks adapter readiness via executable check and quick handshake.
func (a *PiAdapter) Health(ctx ExecutionContext) error {
	if ctx == nil {
		ctx = NewExecutionContext(context.Background(), "")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if a == nil || a.sandbox == nil {
		return fmt.Errorf("%w: adapter is uninitialized", ErrAdapterUnhealthy)
	}

	if a.worktreeRoot != "" {
		info, err := os.Stat(a.worktreeRoot)
		if err != nil || !info.IsDir() {
			return fmt.Errorf("%w: worktree root is unavailable", ErrAdapterUnhealthy)
		}
	}

	if _, ok := a.runner.(*DefaultProcessRunner); ok || a.runner == nil {
		if _, lookErr := exec.LookPath(a.executablePath); lookErr != nil {
			if _, statErr := os.Stat(a.executablePath); statErr != nil {
				return fmt.Errorf("%w: pi executable not found", ErrPiExecutableNotFound)
			}
		}
	}

	return a.performHandshake(ctx)
}

func (a *PiAdapter) performHandshake(ctx ExecutionContext) error {
	childCtx, cancel := context.WithTimeout(ctx, a.startupTimeout)
	defer cancel()

	res, err := a.runProcess(childCtx, a.executablePath, []string{"--handshake"}, nil, a.worktreeRoot)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, ErrTimeout) {
			return fmt.Errorf("%w: handshake timed out", ErrPiHandshakeFailed)
		}
		if errors.Is(err, context.Canceled) || errors.Is(err, ErrCanceled) {
			return ctx.Err()
		}
		return fmt.Errorf("%w: handshake execution failed", ErrPiHandshakeFailed)
	}

	trimmed := bytes.TrimSpace(res.Stdout)
	if len(trimmed) == 0 {
		return fmt.Errorf("%w: empty handshake output", ErrPiHandshakeFailed)
	}

	var handshake HandshakePayload
	if err := json.Unmarshal(trimmed, &handshake); err != nil || handshake.Protocol == "" {
		// Try parsing as full Message envelope. Pi may expose either the typed
		// handshake payload or the standard JSONL envelope during discovery.
		var msg Message
		if msgErr := json.Unmarshal(trimmed, &msg); msgErr == nil && msg.Type == MessageHandshake {
			if hsErr := json.Unmarshal(msg.Payload, &handshake); hsErr != nil {
				return fmt.Errorf("%w: invalid handshake message payload", ErrPiHandshakeFailed)
			}
		} else {
			return fmt.Errorf("%w: invalid handshake format", ErrPiHandshakeFailed)
		}
	}

	if handshake.Protocol != WorkerProtocolVersion {
		return fmt.Errorf("%w: unsupported protocol %q", ErrPiHandshakeFailed, handshake.Protocol)
	}

	return nil
}

// Execute runs a tool within the Pi engine sandbox and streams evidence.
func (a *PiAdapter) Execute(ctx ExecutionContext, envelope ExecutionEnvelope) (ToolResult, error) {
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

	timeout := envelope.Timeout
	if cfgTimeout := a.sandbox.Config().Timeout; timeout > cfgTimeout && cfgTimeout > 0 {
		timeout = cfgTimeout
	}
	childCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	_, _ = a.sandbox.RecordAudit(envelope, SandboxAuditEvent{
		Event:   "start",
		Command: a.executablePath,
		Args:    []string{"--worker"},
	})

	sandboxResult, runErr := a.runProcess(childCtx, a.executablePath, []string{"--worker"}, os.Environ(), ".")
	if runErr != nil {
		if errors.Is(runErr, ErrTimeout) || sandboxResult.TimedOut {
			return a.finishError(result, started, ErrTimeout)
		}
		if errors.Is(runErr, ErrCanceled) || sandboxResult.Canceled {
			return a.finishError(result, started, ErrCanceled)
		}
		return a.finishError(result, started, fmt.Errorf("%w: process crashed", ErrPiProcessCrash))
	}

	terminalPayload, evidenceIDs, err := a.consumeJSONL(envelope, sandboxResult.Stdout)
	if err != nil {
		return a.finishError(result, started, err)
	}

	result.EvidenceIDs = evidenceIDs
	result.Output = RedactBytes(sandboxResult.RedactedOut)

	switch terminalPayload.Status {
	case TerminalSuccess:
		result.Status = ToolStatusSuccess
	case TerminalCanceled:
		result.Status = ToolStatusCanceled
		return a.finishError(result, started, ErrCanceled)
	case TerminalFailure:
		result.Status = ToolStatusFailed
		if terminalPayload.Error != nil {
			result.Error = terminalPayload.Error
		}
		return a.finishError(result, started, fmt.Errorf("%w: worker reported failure", ErrPiProcessFailed))
	default:
		return a.finishError(result, started, fmt.Errorf("%w: unknown terminal status %q", ErrPiProtocolError, terminalPayload.Status))
	}

	completed := time.Now().UTC()
	result.CompletedAt = completed
	result.DurationMs = completed.Sub(started).Milliseconds()

	_, _ = a.sandbox.RecordAudit(envelope, SandboxAuditEvent{
		Event:      "complete",
		Command:    a.executablePath,
		ExitCode:   sandboxResult.ExitCode,
		DurationMs: result.DurationMs,
		Redacted:   true,
	})

	return result, nil
}

func (a *PiAdapter) validateEnvelope(ctx ExecutionContext, envelope ExecutionEnvelope) error {
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

func (a *PiAdapter) supports(toolName string) bool {
	norm := strings.ToLower(strings.TrimSpace(toolName))
	norm = strings.TrimPrefix(norm, "pi_")
	norm = strings.ReplaceAll(norm, "-", "_")
	switch norm {
	case "coding", "analysis", "refactor", "debug", "review", "test_gen", "doc_gen", "pi", "pi_task", "default":
		return true
	default:
		for _, cap := range a.capabilities {
			if string(cap) == norm {
				return true
			}
		}
		return false
	}
}

// MapExecutionEnvelopeToTask maps an ExecutionEnvelope to a process-safe TaskEnvelope.
// Private host boundaries (WorktreeRoot, PolicyDecision, Audit) are never leaked to the worker.
func MapExecutionEnvelopeToTask(envelope ExecutionEnvelope) TaskEnvelope {
	return TaskEnvelope{
		ContractVersion: HarnessContractVersion,
		ExecutionID:     envelope.ExecutionID,
		TaskID:          envelope.TaskID,
		WorktreeID:      envelope.WorktreeID,
		ProjectID:       envelope.ProjectID,
		ToolName:        envelope.ToolName,
		Input:           envelope.Input,
		TimeoutMs:       envelope.Timeout.Milliseconds(),
		TraceID:         envelope.TraceID,
	}
}

func (a *PiAdapter) runProcess(ctx context.Context, command string, args []string, env []string, workDir string) (SandboxResult, error) {
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
	res, runErr := a.runner.Run(ctx, command, args, filtered, canonicalWorkDir,
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

func (a *PiAdapter) consumeJSONL(envelope ExecutionEnvelope, stdout []byte) (*TerminalPayload, []string, error) {
	if len(bytes.TrimSpace(stdout)) == 0 {
		return nil, nil, fmt.Errorf("%w: no protocol messages emitted by worker", ErrPiProcessCrash)
	}

	decoder := NewDecoder(bytes.NewReader(stdout))
	var (
		evidenceIDs     []string
		terminalPayload *TerminalPayload
		handshakeSeen   bool
	)

	for {
		msg, err := decoder.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, nil, fmt.Errorf("%w: %v", ErrPiProtocolError, err)
		}

		switch msg.Type {
		case MessageHandshake:
			handshakeSeen = true

		case MessageCapabilities:
			// Handshake / capabilities discovery

		case MessageHealth:
			// Health report

		case MessageTask:
			// Task acknowledgement

		case MessageProgress:
			var p ProgressPayload
			if err := json.Unmarshal(msg.Payload, &p); err == nil {
				_, _ = a.sandbox.RecordAudit(envelope, SandboxAuditEvent{
					Event:   "progress",
					Command: p.Phase,
				})
			}

		case MessageEvidence:
			var ep EvidencePayload
			if err := json.Unmarshal(msg.Payload, &ep); err == nil {
				var rec EvidenceRecord
				if ep.Record != nil {
					rec = *ep.Record
				} else {
					var recErr error
					rec, recErr = NewEvidenceRecord(envelope.ExecutionID, envelope.TaskID, envelope.WorktreeID, ep.Kind, ep)
					if recErr != nil {
						return nil, nil, fmt.Errorf("%w: %v", ErrPiProtocolError, recErr)
					}
				}
				evidenceIDs = append(evidenceIDs, rec.ID)
				if a.onEvidence != nil {
					a.onEvidence(rec)
				}
			}

		case MessageDiagnostic:
			var dp DiagnosticPayload
			if err := json.Unmarshal(msg.Payload, &dp); err == nil {
				rec, recErr := NewEvidenceRecord(envelope.ExecutionID, envelope.TaskID, envelope.WorktreeID, EvidenceKindEngineDiagnostic, dp)
				if recErr == nil {
					evidenceIDs = append(evidenceIDs, rec.ID)
					if a.onEvidence != nil {
						a.onEvidence(rec)
					}
				}
			}

		case MessageHeartbeat:
			// Heartbeat received

		case MessageTerminal:
			var tp TerminalPayload
			if err := json.Unmarshal(msg.Payload, &tp); err != nil {
				return nil, nil, fmt.Errorf("%w: invalid terminal payload: %v", ErrPiProtocolError, err)
			}
			terminalPayload = &tp
			for _, id := range tp.EvidenceIDs {
				if !containsString(evidenceIDs, id) {
					evidenceIDs = append(evidenceIDs, id)
				}
			}

		case MessageShutdown:
			// Normal shutdown
		}
	}

	if !handshakeSeen {
		return nil, nil, fmt.Errorf("%w: worker exited without handshake", ErrPiProcessCrash)
	}
	if terminalPayload == nil {
		return nil, nil, fmt.Errorf("%w: worker exited without terminal status", ErrPiProcessCrash)
	}

	return terminalPayload, evidenceIDs, nil
}

func (a *PiAdapter) finishError(result ToolResult, started time.Time, err error) (ToolResult, error) {
	coded := ErrorFor(err)
	result.Error = &ToolError{Code: string(coded.Code), Message: coded.Message}
	result.Status = toolStatusFromError(coded)
	completed := time.Now().UTC()
	result.CompletedAt, result.DurationMs = completed, completed.Sub(started).Milliseconds()
	return result, err
}

var _ EngineAdapter = (*PiAdapter)(nil)
