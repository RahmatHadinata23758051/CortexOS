package harness

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// Broker is the default implementation of ToolBroker.
// It manages tool registration, policy checks, engine routing, and execution with evidence collection.
type Broker struct {
	mu           sync.RWMutex
	tools        map[string]ToolDefinition
	engines      map[ToolKind]EngineAdapter
	policyEngine PolicyEngine
	clock        func() time.Time
}

// NewBroker creates a new tool broker with the given policy engine.
func NewBroker(policyEngine PolicyEngine) *Broker {
	return &Broker{
		tools:        make(map[string]ToolDefinition),
		engines:      make(map[ToolKind]EngineAdapter),
		policyEngine: policyEngine,
		clock:        time.Now,
	}
}

// Register implements ToolBroker.Register.
func (b *Broker) Register(def ToolDefinition) error {
	if err := def.Validate(); err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidContract, err)
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if _, exists := b.tools[def.Name]; exists {
		return fmt.Errorf("%w: %q", ErrDuplicateTool, def.Name)
	}
	b.tools[def.Name] = def
	return nil
}

// Unregister implements ToolBroker.Unregister.
func (b *Broker) Unregister(name string) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if _, exists := b.tools[name]; !exists {
		return fmt.Errorf("%w: %q", ErrToolNotFound, name)
	}
	delete(b.tools, name)
	return nil
}

// Get implements ToolBroker.Get.
func (b *Broker) Get(name string) (ToolDefinition, bool) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	def, ok := b.tools[name]
	return def, ok
}

// List implements ToolBroker.List.
func (b *Broker) List() []ToolDefinition {
	b.mu.RLock()
	defer b.mu.RUnlock()
	out := make([]ToolDefinition, 0, len(b.tools))
	for _, def := range b.tools {
		out = append(out, def)
	}
	return out
}

// RegisterEngine registers an engine adapter for a tool kind.
func (b *Broker) RegisterEngine(engine EngineAdapter) error {
	if engine == nil {
		return fmt.Errorf("%w: engine is nil", ErrInvalidContract)
	}
	desc := engine.Describe()
	if desc.Name == "" || desc.Kind == "" {
		return fmt.Errorf("%w: engine descriptor missing name or kind", ErrInvalidContract)
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if _, exists := b.engines[engine.Kind()]; exists {
		return fmt.Errorf("%w: engine kind %q already registered", ErrDuplicateTool, engine.Kind())
	}
	b.engines[engine.Kind()] = engine
	return nil
}

// Engine retrieves an engine adapter by kind.
func (b *Broker) Engine(kind ToolKind) (EngineAdapter, bool) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	e, ok := b.engines[kind]
	return e, ok
}

// Execute implements ToolBroker.Execute.
// It checks policy, selects the engine, executes with timeout/cancellation,
// and returns a ToolResult with collected evidence.
// The result is UNTRUSTED EVIDENCE; Inspector/Orchestra is the sole success authority.
func (b *Broker) Execute(ctx ExecutionContext, request ToolRequest) (ToolResult, error) {
	if err := request.Validate(); err != nil {
		return ToolResult{}, ErrorFor(err)
	}
	if ctx == nil {
		ctx = NewExecutionContext(context.Background(), request.TraceID)
	}

	// Look up tool definition
	b.mu.RLock()
	def, ok := b.tools[request.ToolName]
	b.mu.RUnlock()
	if !ok {
		return ToolResult{}, ErrorFor(fmt.Errorf("%w: %q", ErrToolNotFound, request.ToolName))
	}

	// Evaluate policy
	if b.policyEngine == nil {
		return ToolResult{}, ErrorFor(fmt.Errorf("%w: policy engine is not configured", ErrPolicyDenied))
	}
	action := def.Capabilities[0].String()
	if request.Action != "" {
		action = request.Action
	}
	resource := request.WorktreeID
	if request.Resource != "" {
		resource = request.Resource
	}
	decision, err := b.policyEngine.Evaluate(PolicyRequest{
		Action:   action,
		Resource: resource,
		ToolName: request.ToolName,
	})
	if err != nil {
		return ToolResult{}, ErrorFor(err)
	}
	if !decision.Allowed {
		if decision.RequiresAsk {
			return ToolResult{}, ErrorFor(ErrPolicyAsk)
		}
		return ToolResult{}, ErrorFor(ErrPolicyDenied)
	}

	// Select engine
	b.mu.RLock()
	engine, ok := b.engines[def.Kind]
	b.mu.RUnlock()
	if !ok {
		return ToolResult{}, ErrorFor(fmt.Errorf("%w: no engine for kind %q", ErrAdapterFailure, def.Kind))
	}

	// Build envelope
	envelope := ExecutionEnvelope{
		ContractVersion: HarnessContractVersion,
		ExecutionID:     request.ExecutionID,
		TaskID:          request.TaskID,
		WorktreeID:      request.WorktreeID,
		ToolName:        request.ToolName,
		Input:           request.Input,
		PolicyDecision:  decision,
		Timeout:         request.Timeout,
		Audit: AuditMetadata{
			Adapter:      engine.Describe().Name,
			PolicyRuleID: decision.RuleID,
			TraceID:      request.TraceID,
		},
		TraceID: request.TraceID,
	}

	// Execute with context timeout
	execCtx, cancel := context.WithTimeout(ctx, request.Timeout)
	defer cancel()
	execCtx = &contextWithTrace{Context: execCtx, traceID: request.TraceID}

	startedAt := b.clock()
	result, execErr := engine.Execute(NewExecutionContext(execCtx, request.TraceID), envelope)
	completedAt := b.clock()

	if execErr != nil {
		coded := ErrorFor(execErr)
		result = ToolResult{
			ContractVersion: HarnessContractVersion,
			ExecutionID:     request.ExecutionID,
			TaskID:          request.TaskID,
			ToolName:        request.ToolName,
			Status:          toolStatusFromError(coded),
			Error:           &ToolError{Code: string(coded.Code), Message: coded.Message},
			StartedAt:       startedAt,
			CompletedAt:     completedAt,
			DurationMs:      completedAt.Sub(startedAt).Milliseconds(),
			Redacted:        true,
		}
		return result, coded
	}

	// Enforce contract version on result
	if result.ContractVersion != HarnessContractVersion {
		result.ContractVersion = HarnessContractVersion
	}
	result.StartedAt = startedAt
	result.CompletedAt = completedAt
	result.DurationMs = completedAt.Sub(startedAt).Milliseconds()
	result.Redacted = true

	return result, nil
}

// Policy implements ToolBroker.Policy.
func (b *Broker) Policy() PolicyEngine { return b.policyEngine }

func toolStatusFromError(e *CodedError) ToolStatus {
	switch e.Code {
	case ErrorCodePolicyDenied:
		return ToolStatusPolicyDenied
	case ErrorCodeTimeout:
		return ToolStatusTimeout
	case ErrorCodeCanceled:
		return ToolStatusCanceled
	case ErrorCodeSandbox:
		return ToolStatusSandboxError
	case ErrorCodeAdapterFailure:
		return ToolStatusAdapterError
	default:
		return ToolStatusFailed
	}
}

// contextWithTrace wraps a context with a trace ID.
type contextWithTrace struct {
	context.Context
	traceID string
}

// CancellationPolicy defines how the broker handles cancellation.
type CancellationPolicy struct {
	GracePeriod time.Duration `json:"gracePeriod"`
	ForceKill   bool          `json:"forceKill"`
}

// TimeoutPolicy defines execution timeout behavior.
type TimeoutPolicy struct {
	Default   time.Duration `json:"default"`
	Max       time.Duration `json:"max"`
	Extension time.Duration `json:"extension"`
}

// DefaultTimeoutPolicy returns a sane default timeout policy.
func DefaultTimeoutPolicy() TimeoutPolicy {
	return TimeoutPolicy{
		Default:   120 * time.Second,
		Max:       600 * time.Second,
		Extension: 30 * time.Second,
	}
}

// DefaultCancellationPolicy returns a sane default cancellation policy.
func DefaultCancellationPolicy() CancellationPolicy {
	return CancellationPolicy{
		GracePeriod: 5 * time.Second,
		ForceKill:   true,
	}
}

// ExecutionConfig holds execution-time configuration.
type ExecutionConfig struct {
	Timeout      TimeoutPolicy      `json:"timeout"`
	Cancellation CancellationPolicy `json:"cancellation"`
	Redaction    RedactionConfig    `json:"redaction"`
}

// RedactionConfig controls what gets redacted in evidence.
type RedactionConfig struct {
	Secrets       bool `json:"secrets"`
	AbsolutePaths bool `json:"absolutePaths"`
	EnvVars       bool `json:"envVars"`
	CmdLine       bool `json:"cmdLine"`
}

// DefaultExecutionConfig returns a safe default configuration.
func DefaultExecutionConfig() ExecutionConfig {
	return ExecutionConfig{
		Timeout:      DefaultTimeoutPolicy(),
		Cancellation: DefaultCancellationPolicy(),
		Redaction: RedactionConfig{
			Secrets:       true,
			AbsolutePaths: true,
			EnvVars:       true,
			CmdLine:       true,
		},
	}
}
