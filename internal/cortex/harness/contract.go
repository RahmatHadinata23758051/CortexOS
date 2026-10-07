package harness

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

// HarnessContractVersion is the stable contract version for harness DTOs.
const HarnessContractVersion = "cortexos.harness.v1"

// ToolKind represents the category of a tool.
type ToolKind string

const (
	ToolKindNative ToolKind = "native" // Deterministic Go-implemented tools
	ToolKindPi     ToolKind = "pi"     // Pi engine (general coding)
	ToolKindOMP    ToolKind = "omp"    // OMP engine (specialist/recovery)
)

// ToolCapability represents a named capability a tool provides.
type ToolCapability string

func (c ToolCapability) String() string { return string(c) }

const (
	CapabilityShell          ToolCapability = "shell"
	CapabilityFileRead       ToolCapability = "file_read"
	CapabilityFileWrite      ToolCapability = "file_write"
	CapabilityFileEdit       ToolCapability = "file_edit"
	CapabilityGit            ToolCapability = "git"
	CapabilitySearch         ToolCapability = "search"
	CapabilityTaskPlan       ToolCapability = "task_plan"
	CapabilityCodeNavigation ToolCapability = "code_navigation"
	CapabilityTestRun        ToolCapability = "test_run"
	CapabilityLint           ToolCapability = "lint"
	CapabilityBuild          ToolCapability = "build"
	CapabilityCoding         ToolCapability = "coding"
	CapabilityAnalysis       ToolCapability = "analysis"
	CapabilityRefactor       ToolCapability = "refactor"
	CapabilityDebug          ToolCapability = "debug"
	CapabilityReview         ToolCapability = "review"
	CapabilityTestGen        ToolCapability = "test_gen"
	CapabilityDocGen         ToolCapability = "doc_gen"

	// OMP specialist capabilities
	CapabilitySpecialist         ToolCapability = "specialist"
	CapabilityRecovery           ToolCapability = "recovery"
	CapabilitySecurityAudit      ToolCapability = "security_audit"
	CapabilityDeepDebug          ToolCapability = "deep_debug"
	CapabilityArchitectureReview ToolCapability = "architecture_review"
)

// ToolDefinition describes a registerable tool.
type ToolDefinition struct {
	Name         string           `json:"name"`
	Kind         ToolKind         `json:"kind"`
	Description  string           `json:"description"`
	Capabilities []ToolCapability `json:"capabilities"`
	InputSchema  json.RawMessage  `json:"inputSchema"`
	OutputSchema json.RawMessage  `json:"outputSchema"`
	Timeout      time.Duration    `json:"timeout"`
	RequiresAsk  bool             `json:"requiresAsk"`
	Version      string           `json:"version"`
}

// Validate checks the tool definition for required fields and sane values.
func (d ToolDefinition) Validate() error {
	if d.Name == "" {
		return fmt.Errorf("%w: tool name is required", ErrInvalidContract)
	}
	if d.Kind == "" {
		return fmt.Errorf("%w: tool kind is required", ErrInvalidContract)
	}
	if !isValidKind(d.Kind) {
		return fmt.Errorf("%w: invalid tool kind: %q", ErrInvalidContract, d.Kind)
	}
	if len(d.Capabilities) == 0 {
		return fmt.Errorf("%w: at least one capability is required", ErrInvalidContract)
	}
	if d.Timeout <= 0 {
		return fmt.Errorf("%w: tool timeout must be positive", ErrInvalidContract)
	}
	if d.Version == "" {
		return fmt.Errorf("%w: tool version is required", ErrInvalidContract)
	}
	return nil
}

func isValidKind(k ToolKind) bool {
	return k == ToolKindNative || k == ToolKindPi || k == ToolKindOMP
}

// ToolRequest is the execution envelope for a tool invocation.
// Path-free, shell-free, secret-free. All paths must be worktree-relative.
type ToolRequest struct {
	ContractVersion string          `json:"contractVersion"`
	ExecutionID     string          `json:"executionId"`
	TaskID          string          `json:"taskId"`
	WorktreeID      string          `json:"worktreeId"`
	ToolName        string          `json:"toolName"`
	Action          string          `json:"action,omitempty"`
	Resource        string          `json:"resource,omitempty"`
	Input           json.RawMessage `json:"input"`
	Timeout         time.Duration   `json:"timeout"`
	Audit           AuditMetadata   `json:"audit,omitempty"`
	TraceID         string          `json:"traceId,omitempty"`
}

// Validate checks the request for required fields.
func (r ToolRequest) Validate() error {
	if r.ContractVersion != HarnessContractVersion {
		return fmt.Errorf("%w: unsupported contract version: %q", ErrInvalidContract, r.ContractVersion)
	}
	if r.ExecutionID == "" {
		return fmt.Errorf("%w: executionId is required", ErrInvalidContract)
	}
	if r.TaskID == "" {
		return fmt.Errorf("%w: taskId is required", ErrInvalidContract)
	}
	if r.WorktreeID == "" {
		return fmt.Errorf("%w: worktreeId is required", ErrInvalidContract)
	}
	if r.ToolName == "" {
		return fmt.Errorf("%w: toolName is required", ErrInvalidContract)
	}
	if r.Timeout <= 0 {
		return fmt.Errorf("%w: timeout must be positive", ErrInvalidContract)
	}
	return nil
}

// ToolResult is the structured outcome of a tool execution.
// Harness output is UNTRUSTED EVIDENCE — Inspector/Orchestra is the sole success authority.
type ToolResult struct {
	ContractVersion string          `json:"contractVersion"`
	ExecutionID     string          `json:"executionId"`
	TaskID          string          `json:"taskId"`
	ToolName        string          `json:"toolName"`
	Status          ToolStatus      `json:"status"`
	Output          json.RawMessage `json:"output,omitempty"`
	Error           *ToolError      `json:"error,omitempty"`
	EvidenceIDs     []string        `json:"evidenceIds,omitempty"`
	StartedAt       time.Time       `json:"startedAt"`
	CompletedAt     time.Time       `json:"completedAt"`
	DurationMs      int64           `json:"durationMs"`
	Redacted        bool            `json:"redacted"`
}

// ToolStatus represents the execution status of a tool.
type ToolStatus string

const (
	ToolStatusSuccess      ToolStatus = "success"
	ToolStatusFailed       ToolStatus = "failed"
	ToolStatusTimeout      ToolStatus = "timeout"
	ToolStatusCanceled     ToolStatus = "canceled"
	ToolStatusPolicyDenied ToolStatus = "policy_denied"
	ToolStatusSandboxError ToolStatus = "sandbox_error"
	ToolStatusAdapterError ToolStatus = "adapter_error"
)

// ToolError is a structured, typed error from tool execution.
// These are stable, redacted error codes safe for cross-boundary transport.
type ToolError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Details string `json:"details,omitempty"` // Redacted, safe for logging
}

func (e *ToolError) Error() string {
	if e == nil {
		return "<nil>"
	}
	if e.Details != "" {
		return e.Code + ": " + e.Message + " (" + e.Details + ")"
	}
	return e.Code + ": " + e.Message
}

// IsRetriable returns true if the error class suggests a retry may succeed.
func (e *ToolError) IsRetriable() bool {
	if e == nil {
		return false
	}
	switch e.Code {
	case "harness.timeout", "harness.canceled", "harness.sandbox_unavailable":
		return true
	default:
		return false
	}
}

// ExecutionEnvelope is the complete execution context passed to an engine adapter.
// It carries task, worktree, policy decision, and timeout/cancellation context.
type ExecutionEnvelope struct {
	ContractVersion string          `json:"contractVersion"`
	ExecutionID     string          `json:"executionId"`
	TaskID          string          `json:"taskId"`
	WorktreeID      string          `json:"worktreeId"`
	ProjectID       string          `json:"projectId"`
	WorktreeRoot    string          `json:"worktreeRoot"` // Absolute root for sandbox; never exposed to frontend
	ToolName        string          `json:"toolName"`
	Input           json.RawMessage `json:"input"`
	PolicyDecision  PolicyDecision  `json:"policyDecision"`
	Timeout         time.Duration   `json:"timeout"`
	Audit           AuditMetadata   `json:"audit"`
	TraceID         string          `json:"traceId,omitempty"`
}

// PolicyDecision carries the policy evaluation outcome for this execution.
type PolicyDecision struct {
	Allowed     bool   `json:"allowed"`
	RuleID      string `json:"ruleId,omitempty"`
	Effect      Effect `json:"effect"`
	Reason      string `json:"reason"`
	RequiresAsk bool   `json:"requiresAsk"`
}

// EngineAdapter is the stable interface for execution engines (native, Pi, OMP).
// Implementations MUST NOT leak engine-specific types across this boundary.
// All input/output flows through JSON schemas defined in ToolDefinition.
type EngineAdapter interface {
	// Kind returns the engine kind this adapter implements.
	Kind() ToolKind

	// Capabilities returns the capabilities this engine provides.
	Capabilities() []ToolCapability

	// Execute runs a tool within the engine's sandbox.
	// The envelope contains all context needed; the adapter MUST NOT
	// access ambient state (filesystem outside worktree, env, etc).
	Execute(ctx ExecutionContext, envelope ExecutionEnvelope) (ToolResult, error)

	// Health checks adapter readiness. Returns nil if healthy.
	Health(ctx ExecutionContext) error

	// Describe returns metadata about the adapter for discovery.
	Describe() AdapterDescriptor
}

// AdapterDescriptor provides static metadata about an engine adapter.
type AdapterDescriptor struct {
	Name          string           `json:"name"`
	Kind          ToolKind         `json:"kind"`
	Version       string           `json:"version"`
	Capabilities  []ToolCapability `json:"capabilities"`
	SchemaVersion string           `json:"schemaVersion"`
}

// ExecutionContext is the context passed to engine adapters.
// It provides cancellation, timeout, and tracing without leaking Go context.
type ExecutionContext interface {
	context.Context

	// TraceID returns the distributed trace ID for this execution.
	TraceID() string
}

// ContextAdapter wraps a standard context.Context with a trace ID.
type ContextAdapter struct {
	context.Context
	ID string
}

// NewExecutionContext adapts a standard context to the harness execution contract.
func NewExecutionContext(ctx context.Context, traceID string) ExecutionContext {
	if ctx == nil {
		ctx = context.Background()
	}
	return ContextAdapter{Context: ctx, ID: traceID}
}

func (c ContextAdapter) TraceID() string { return c.ID }

// ToolBroker is the interface for registering, checking, and executing tools.
// It enforces policy, sandboxing, and evidence collection.
type ToolBroker interface {
	// Register adds a tool definition to the broker.
	// Returns an error if the definition is invalid or a tool with the same name exists.
	Register(def ToolDefinition) error

	// Unregister removes a tool by name.
	Unregister(name string) error

	// Get retrieves a tool definition by name.
	Get(name string) (ToolDefinition, bool)

	// List returns all registered tool definitions.
	List() []ToolDefinition

	// Execute requests tool execution through the broker.
	// The broker checks policy, selects the appropriate engine adapter,
	// executes within sandbox, and returns a ToolResult with evidence.
	// The returned ToolResult is UNTRUSTED EVIDENCE — Inspector/Orchestra
	// is the sole authority for task success.
	Execute(ctx ExecutionContext, request ToolRequest) (ToolResult, error)

	// Policy returns the current policy engine for evaluation.
	Policy() PolicyEngine
}

// PolicyEngine evaluates permission requests against the loaded policy.
type PolicyEngine interface {
	// Evaluate checks if an action on a resource is allowed.
	Evaluate(request PolicyRequest) (PolicyDecision, error)

	// LoadPolicy replaces the current policy with a new one.
	LoadPolicy(policy Policy) error

	// CurrentPolicy returns the currently loaded policy.
	CurrentPolicy() Policy
}

// AuditMetadata records provenance without storing raw process output or secrets.
type AuditMetadata struct {
	Actor         string `json:"actor,omitempty"`
	Adapter       string `json:"adapter,omitempty"`
	PolicyRuleID  string `json:"policyRuleId,omitempty"`
	TraceID       string `json:"traceId,omitempty"`
	CorrelationID string `json:"correlationId,omitempty"`
}

// Sandbox is the execution isolation boundary used by a broker or adapter.
// Implementations must reject worktree escapes before starting side effects.
type Sandbox interface {
	Validate(ctx ExecutionContext, envelope ExecutionEnvelope) error
}

// PolicyRequest is a permission check request.
type PolicyRequest struct {
	Action   string `json:"action"`
	Resource string `json:"resource"`
	ToolName string `json:"toolName,omitempty"`
}
