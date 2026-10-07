package harness

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"sync"
	"time"
)

// CapabilityBinding associates a ToolCapability with a policy binding and constraints.
type CapabilityBinding struct {
	Capability    ToolCapability `json:"capability"`
	Description   string         `json:"description,omitempty"`
	RequiresAsk   bool           `json:"requiresAsk,omitempty"`
	DefaultEffect Effect         `json:"defaultEffect,omitempty"`
}

// Approval records explicit authorization for a pending ask decision.
type Approval struct {
	ExecutionID string    `json:"executionId"`
	ToolName    string    `json:"toolName,omitempty"`
	Action      string    `json:"action,omitempty"`
	Approver    string    `json:"approver"`
	Reason      string    `json:"reason,omitempty"`
	GrantedAt   time.Time `json:"grantedAt"`
}

type approvalKey struct{}

// WithApproval attaches an explicit approval to a context.
func WithApproval(ctx context.Context, approval Approval) context.Context {
	return context.WithValue(ctx, approvalKey{}, approval)
}

// DefaultCapabilities returns the set of standard harness tool capabilities.
func DefaultCapabilities() []CapabilityBinding {
	return []CapabilityBinding{
		{Capability: CapabilityShell, Description: "Execute commands in isolated worktree"},
		{Capability: CapabilityFileRead, Description: "Read files within worktree boundary"},
		{Capability: CapabilityFileWrite, Description: "Write files within worktree boundary"},
		{Capability: CapabilityFileEdit, Description: "Edit files within worktree boundary"},
		{Capability: CapabilityGit, Description: "Execute git operations on worktree"},
		{Capability: CapabilitySearch, Description: "Search content within worktree"},
		{Capability: CapabilityTaskPlan, Description: "Task and plan coordination"},
		{Capability: CapabilityCodeNavigation, Description: "Navigate symbols and references"},
		{Capability: CapabilityTestRun, Description: "Run tests in isolated sandbox"},
		{Capability: CapabilityLint, Description: "Run linters in isolated sandbox"},
		{Capability: CapabilityBuild, Description: "Run build commands in isolated sandbox"},
		{Capability: CapabilityCoding, Description: "Code generation and modification"},
		{Capability: CapabilityAnalysis, Description: "Code analysis and understanding"},
		{Capability: CapabilityRefactor, Description: "Code refactoring and restructuring"},
		{Capability: CapabilityDebug, Description: "Bug diagnosis and fixing"},
		{Capability: CapabilityReview, Description: "Code review and quality audit"},
		{Capability: CapabilityTestGen, Description: "Automated test generation"},
		{Capability: CapabilityDocGen, Description: "Documentation generation"},
	}
}

// Broker is the default implementation of ToolBroker.
// It acts as the mandatory execution gate: policy enforcement, engine routing,
// sandboxing, and immutable audit evidence collection. No direct process execution
// occurs outside the broker.
type Broker struct {
	mu           sync.RWMutex
	tools        map[string]ToolDefinition
	capabilities map[ToolCapability]CapabilityBinding
	engines      map[ToolKind]EngineAdapter // Legacy: single engine per kind
	router       *BasicRouter               // Capability-based routing
	policyEngine PolicyEngine
	clock        func() time.Time
	auditLog     []EvidenceRecord
	approvals    map[string]Approval
	sandbox      *ExecutionSandbox
}

// NewBroker creates a new tool broker with the given policy engine.
func NewBroker(policyEngine PolicyEngine) *Broker {
	b := &Broker{
		tools:        make(map[string]ToolDefinition),
		capabilities: make(map[ToolCapability]CapabilityBinding),
		engines:      make(map[ToolKind]EngineAdapter),
		router:       NewBasicRouter(DefaultRoutingPolicy()),
		policyEngine: policyEngine,
		clock:        time.Now,
		approvals:    make(map[string]Approval),
	}
	for _, capBinding := range DefaultCapabilities() {
		b.capabilities[capBinding.Capability] = capBinding
	}
	return b
}

// RegisterCapability registers or updates a capability binding.
func (b *Broker) RegisterCapability(binding CapabilityBinding) error {
	if binding.Capability == "" {
		return fmt.Errorf("%w: capability name is required", ErrInvalidContract)
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	b.capabilities[binding.Capability] = binding
	return nil
}

// Capability retrieves a registered capability binding by name.
func (b *Broker) Capability(cap ToolCapability) (CapabilityBinding, bool) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	binding, ok := b.capabilities[cap]
	return binding, ok
}

// ListCapabilities returns all registered capability bindings.
func (b *Broker) ListCapabilities() []CapabilityBinding {
	b.mu.RLock()
	defer b.mu.RUnlock()
	out := make([]CapabilityBinding, 0, len(b.capabilities))
	for _, binding := range b.capabilities {
		out = append(out, binding)
	}
	sort.Slice(out, func(i, j int) bool { return string(out[i].Capability) < string(out[j].Capability) })
	return out
}

// SetSandbox configures an optional execution sandbox for process isolation.
func (b *Broker) SetSandbox(sandbox *ExecutionSandbox) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.sandbox = sandbox
}

// Sandbox returns the configured execution sandbox, if any.
func (b *Broker) Sandbox() *ExecutionSandbox {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.sandbox
}

// Approve registers an explicit approval on the broker for a specific execution ID.
func (b *Broker) Approve(approval Approval) error {
	if approval.ExecutionID == "" {
		return fmt.Errorf("%w: executionId is required for approval", ErrInvalidContract)
	}
	if approval.Approver == "" {
		return fmt.Errorf("%w: approver is required for approval", ErrInvalidContract)
	}
	if approval.GrantedAt.IsZero() {
		approval.GrantedAt = b.clock()
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	b.approvals[approval.ExecutionID] = approval
	return nil
}

// AuditEvents returns a copy of all immutable audit events collected by the broker.
func (b *Broker) AuditEvents() []EvidenceRecord {
	b.mu.RLock()
	defer b.mu.RUnlock()
	out := make([]EvidenceRecord, len(b.auditLog))
	copy(out, b.auditLog)
	return out
}

// AuditEventsForExecution returns immutable audit events for a specific execution ID.
func (b *Broker) AuditEventsForExecution(executionID string) []EvidenceRecord {
	b.mu.RLock()
	defer b.mu.RUnlock()
	var out []EvidenceRecord
	for _, ev := range b.auditLog {
		if ev.ExecutionID == executionID {
			out = append(out, ev)
		}
	}
	return out
}

// Register implements ToolBroker.Register.
// Validates tool definition and binds declared capabilities to the broker's capability registry.
// Fails closed if any declared capability is unknown or unregistered.
func (b *Broker) Register(def ToolDefinition) error {
	if err := def.Validate(); err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidContract, err)
	}
	b.mu.Lock()
	defer b.mu.Unlock()

	for _, cap := range def.Capabilities {
		if cap == "" {
			return fmt.Errorf("%w: empty capability in tool definition", ErrInvalidContract)
		}
		if _, exists := b.capabilities[cap]; !exists {
			return fmt.Errorf("%w: unregistered capability %q", ErrInvalidContract, cap)
		}
	}

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
	identity := AdapterIdentity{
		Name:          desc.Name,
		InstanceID:    generateInstanceID(desc.Name),
		Kind:          engine.Kind(),
		Class:         engineClassFromToolKind(engine.Kind()),
		Version:       desc.Version,
		SchemaVersion: desc.SchemaVersion,
	}
	if err := b.router.Register(engine, ExtendedAdapterDescriptor{
		AdapterDescriptor: desc,
		Identity:          identity,
		ResourceReq:       DefaultResourceRequirements(identity.Class),
	}); err != nil {
		return err
	}
	return b.registerLegacyEngine(engine)
}

// registerLegacyEngine maintains backward compatibility with the existing single-engine-per-kind model.
func (b *Broker) registerLegacyEngine(engine EngineAdapter) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	desc := engine.Describe()
	if desc.Name == "" || desc.Kind == "" {
		return fmt.Errorf("%w: engine descriptor missing name or kind", ErrInvalidContract)
	}
	if _, exists := b.engines[engine.Kind()]; exists {
		return fmt.Errorf("%w: engine kind %q already registered", ErrDuplicateTool, engine.Kind())
	}
	b.engines[engine.Kind()] = engine
	return nil
}

// Engine retrieves an engine adapter by kind using the legacy single-engine model.
func (b *Broker) Engine(kind ToolKind) (EngineAdapter, bool) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	e, ok := b.engines[kind]
	return e, ok
}

// Execute implements ToolBroker.Execute as the mandatory execution gate.
// It validates requests, enforces capability and policy constraints (fail-closed),
// routes to registered engine adapters, and collects immutable audit evidence.
// The result is UNTRUSTED EVIDENCE; Inspector/Orchestra is the sole success authority.
func (b *Broker) Execute(ctx ExecutionContext, request ToolRequest) (ToolResult, error) {
	if err := request.Validate(); err != nil {
		coded := ErrorFor(err)
		evID := b.recordAudit(request, EvidenceKindPolicyDecision, "denied", "malformed request", nil, coded)
		return brokerFailureResult(request, ToolStatusFailed, coded, evID), coded
	}
	if ctx == nil {
		ctx = NewExecutionContext(context.Background(), request.TraceID)
	}

	// Look up tool definition
	b.mu.RLock()
	def, ok := b.tools[request.ToolName]
	b.mu.RUnlock()
	if !ok {
		err := fmt.Errorf("%w: %q", ErrToolNotFound, request.ToolName)
		coded := ErrorFor(err)
		evID := b.recordAudit(request, EvidenceKindPolicyDecision, "denied", "unknown tool", nil, coded)
		return brokerFailureResult(request, ToolStatusFailed, coded, evID), coded
	}

	// Validate requested action against declared capabilities (bypass prevention)
	var targetCap ToolCapability
	if request.Action != "" {
		targetCap = ToolCapability(request.Action)
		b.mu.RLock()
		_, capExists := b.capabilities[targetCap]
		b.mu.RUnlock()
		if !capExists {
			err := fmt.Errorf("%w: invalid capability %q", ErrInvalidContract, targetCap)
			coded := ErrorFor(err)
			evID := b.recordAudit(request, EvidenceKindPolicyDecision, "denied", "invalid capability", nil, coded)
			return brokerFailureResult(request, ToolStatusPolicyDenied, coded, evID), coded
		}

		toolHasCap := false
		for _, c := range def.Capabilities {
			if c == targetCap {
				toolHasCap = true
				break
			}
		}
		if !toolHasCap {
			err := fmt.Errorf("%w: tool %q does not provide capability %q", ErrPolicyDenied, def.Name, targetCap)
			coded := ErrorFor(err)
			evID := b.recordAudit(request, EvidenceKindPolicyDecision, "denied", "capability bypass attempt: undeclared capability", nil, coded)
			return brokerFailureResult(request, ToolStatusPolicyDenied, coded, evID), coded
		}
	} else {
		targetCap = def.Capabilities[0]
	}

	// Look up capability binding
	b.mu.RLock()
	capBinding := b.capabilities[targetCap]
	b.mu.RUnlock()

	// Evaluate policy
	if b.policyEngine == nil {
		err := fmt.Errorf("%w: policy engine is not configured", ErrPolicyDenied)
		coded := ErrorFor(err)
		evID := b.recordAudit(request, EvidenceKindPolicyDecision, "denied", "policy engine not configured", nil, coded)
		return brokerFailureResult(request, ToolStatusPolicyDenied, coded, evID), coded
	}

	action := targetCap.String()
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
		coded := ErrorFor(err)
		evID := b.recordAudit(request, EvidenceKindPolicyDecision, "denied", "policy evaluation error", nil, coded)
		return brokerFailureResult(request, ToolStatusPolicyDenied, coded, evID), coded
	}

	// Apply capability binding default if policy did not match an explicit rule
	if decision.RuleID == "" && capBinding.DefaultEffect != "" {
		decision.Effect = capBinding.DefaultEffect
		decision.Allowed = capBinding.DefaultEffect == EffectAllow
		decision.RequiresAsk = capBinding.DefaultEffect == EffectAsk
		decision.Reason = "capability default policy binding"
	}

	// Tool-level or capability-level requiresAsk takes precedence over allow (no silent allow)
	if (def.RequiresAsk || capBinding.RequiresAsk) && decision.Allowed {
		decision.Allowed = false
		decision.RequiresAsk = true
		decision.Effect = EffectAsk
		decision.Reason = "tool capability requires explicit approval"
	}

	// Policy allow/ask/deny enforcement
	if !decision.Allowed {
		if decision.RequiresAsk {
			// Explicit ask approval boundary
			approval, approved := b.checkApproval(ctx, request.ExecutionID, request.ToolName, action)
			if !approved {
				coded := ErrorFor(ErrPolicyAsk)
				evID := b.recordAudit(request, EvidenceKindPolicyDecision, "denied", "policy requires approval; no explicit approval grant", &decision, coded)
				return brokerFailureResult(request, ToolStatusPolicyDenied, coded, evID), coded
			}
			decision.Allowed = true
			decision.RequiresAsk = false
			decision.Effect = EffectAllow
			decision.Reason = fmt.Sprintf("approved by %s", approval.Approver)
			b.recordAudit(request, EvidenceKindPolicyDecision, "approved", "explicit approval granted at boundary", &decision, nil)
		} else {
			coded := ErrorFor(ErrPolicyDenied)
			evID := b.recordAudit(request, EvidenceKindPolicyDecision, "denied", "policy denied", &decision, coded)
			return brokerFailureResult(request, ToolStatusPolicyDenied, coded, evID), coded
		}
	} else {
		b.recordAudit(request, EvidenceKindPolicyDecision, "approved", "policy allowed", &decision, nil)
	}

	// Select engine via capability-based router
	selection, err := b.router.Find(ctx, def.Capabilities)
	if err != nil {
		if errors.Is(err, ErrNoEligibleAdapter) {
			err = fmt.Errorf("%w: no adapter for tool kind %q", ErrAdapterFailure, def.Kind)
		}
		coded := ErrorFor(err)
		evID := b.recordAudit(request, EvidenceKindCommandExecution, "failed", "no eligible adapter", &decision, coded)
		return brokerFailureResult(request, ToolStatusAdapterError, coded, evID), coded
	}
	engine := selection.Adapter
	b.mu.RLock()
	sandbox := b.sandbox
	b.mu.RUnlock()

	// Build envelope with deterministic matched-rule metadata (no secrets).
	// WorktreeRoot is provided by the sandbox when configured; adapters
	// must not expose host paths to workers.
	worktreeRoot := ""
	if sandbox != nil {
		worktreeRoot = sandbox.WorktreeRoot()
	}
	envelope := ExecutionEnvelope{
		ContractVersion: HarnessContractVersion,
		ExecutionID:     request.ExecutionID,
		TaskID:          request.TaskID,
		WorktreeID:      request.WorktreeID,
		ProjectID:       request.WorktreeID, // Use WorktreeID as ProjectID fallback; app bridge sets correctly
		WorktreeRoot:    worktreeRoot,
		ToolName:        request.ToolName,
		Input:           request.Input,
		PolicyDecision:  decision,
		Timeout:         request.Timeout,
		Audit: AuditMetadata{
			Actor:         request.Audit.Actor,
			Adapter:       selection.Descriptor.Identity.Name,
			PolicyRuleID:  decision.RuleID,
			TraceID:       request.TraceID,
			CorrelationID: selection.TieBreakKey,
		},
		TraceID: request.TraceID,
	}

	execCtx, cancel := context.WithTimeout(ctx, request.Timeout)
	defer cancel()
	execCtx = &contextWithTrace{Context: execCtx, traceID: request.TraceID}

	startedAt := b.clock()
	var result ToolResult
	var execErr error
	if sandbox != nil {
		result, execErr = sandbox.ExecuteAdapter(NewExecutionContext(execCtx, request.TraceID), engine, envelope)
	} else {
		result, execErr = engine.Execute(NewExecutionContext(execCtx, request.TraceID), envelope)
	}
	completedAt := b.clock()

	if execErr != nil {
		coded := ErrorFor(execErr)
		status := toolStatusFromError(coded)
		var evID string
		if status == ToolStatusCanceled || status == ToolStatusTimeout {
			evID = b.recordAudit(request, EvidenceKindCommandExecution, "canceled", coded.Message, &decision, coded)
		} else {
			evID = b.recordAudit(request, EvidenceKindCommandExecution, "failed", coded.Message, &decision, coded)
		}
		var evidenceIDs []string
		if len(result.EvidenceIDs) > 0 {
			evidenceIDs = result.EvidenceIDs
		} else if evID != "" {
			evidenceIDs = []string{evID}
		}
		result = ToolResult{
			ContractVersion: HarnessContractVersion,
			ExecutionID:     request.ExecutionID,
			TaskID:          request.TaskID,
			ToolName:        request.ToolName,
			Status:          status,
			Error:           &ToolError{Code: string(coded.Code), Message: coded.Message},
			EvidenceIDs:     evidenceIDs,
			StartedAt:       startedAt,
			CompletedAt:     completedAt,
			DurationMs:      completedAt.Sub(startedAt).Milliseconds(),
			Redacted:        true,
		}
		return result, coded
	}

	// Enforce contract version and redaction on result
	if result.ContractVersion != HarnessContractVersion {
		result.ContractVersion = HarnessContractVersion
	}
	result.StartedAt = startedAt
	result.CompletedAt = completedAt
	result.DurationMs = completedAt.Sub(startedAt).Milliseconds()
	result.Redacted = true

	evID := b.recordAudit(request, EvidenceKindCommandExecution, "completed", "tool execution successful", &decision, nil)
	if len(result.EvidenceIDs) == 0 && evID != "" {
		result.EvidenceIDs = []string{evID}
	}
	return result, nil
}

// Policy implements ToolBroker.Policy.
func (b *Broker) Policy() PolicyEngine { return b.policyEngine }

func (b *Broker) checkApproval(ctx context.Context, executionID, toolName, action string) (Approval, bool) {
	if ctx != nil {
		if val := ctx.Value(approvalKey{}); val != nil {
			if app, ok := val.(Approval); ok {
				if app.ExecutionID == "" || app.ExecutionID == executionID {
					if app.ToolName == "" || app.ToolName == toolName {
						if app.Action == "" || app.Action == action {
							return app, true
						}
					}
				}
			}
		}
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	app, ok := b.approvals[executionID]
	if ok {
		if (app.ToolName == "" || app.ToolName == toolName) && (app.Action == "" || app.Action == action) {
			delete(b.approvals, executionID)
			return app, true
		}
	}
	return Approval{}, false
}

func brokerFailureResult(request ToolRequest, status ToolStatus, coded *CodedError, evidenceID string) ToolResult {
	result := ToolResult{
		ContractVersion: HarnessContractVersion,
		ExecutionID:     request.ExecutionID,
		TaskID:          request.TaskID,
		ToolName:        request.ToolName,
		Status:          status,
		Error:           &ToolError{Code: string(coded.Code), Message: coded.Message},
		Redacted:        true,
	}
	if evidenceID != "" {
		result.EvidenceIDs = []string{evidenceID}
	}
	return result
}

func (b *Broker) recordAudit(req ToolRequest, kind EvidenceKind, event string, reason string, decision *PolicyDecision, coded *CodedError) string {
	execID := req.ExecutionID
	if execID == "" {
		execID = "exec-unknown"
	}
	taskID := req.TaskID
	if taskID == "" {
		taskID = "task-unknown"
	}
	worktreeID := req.WorktreeID
	if worktreeID == "" {
		worktreeID = "wt-unknown"
	}

	payload := map[string]any{
		"event":       event,
		"reason":      reason,
		"toolName":    req.ToolName,
		"action":      req.Action,
		"traceId":     req.TraceID,
		"executionId": req.ExecutionID,
		"taskId":      req.TaskID,
		"worktreeId":  req.WorktreeID,
	}
	ruleID := ""
	if decision != nil {
		ruleID = decision.RuleID
		payload["ruleId"] = decision.RuleID
		payload["effect"] = string(decision.Effect)
		payload["policyReason"] = decision.Reason
	}
	if coded != nil {
		payload["errorCode"] = string(coded.Code)
		payload["errorMessage"] = coded.Message
	}

	record, err := NewEvidenceRecord(execID, taskID, worktreeID, kind, payload)
	if err != nil {
		return ""
	}
	record.Audit = AuditMetadata{
		Actor:        req.Audit.Actor,
		PolicyRuleID: ruleID,
		TraceID:      req.TraceID,
	}

	b.mu.Lock()
	b.auditLog = append(b.auditLog, record)
	b.mu.Unlock()
	return record.ID
}

func toolStatusFromError(e *CodedError) ToolStatus {
	switch e.Code {
	case ErrorCodePolicyDenied:
		return ToolStatusPolicyDenied
	case ErrorCodePolicyAsk:
		return ToolStatusPolicyDenied
	case ErrorCodeTimeout:
		return ToolStatusTimeout
	case ErrorCodeCanceled:
		return ToolStatusCanceled
	case ErrorCodeSandbox:
		return ToolStatusSandboxError
	case ErrorCodeWorktreeViolation:
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

func (c *contextWithTrace) TraceID() string { return c.traceID }

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
