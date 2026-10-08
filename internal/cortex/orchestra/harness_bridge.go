package orchestra

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/RahmatHadinata23758051/CortexOS/internal/cortex/harness"
)

// HarnessBridge implements the Executor interface by connecting the Orchestra
// Dispatcher to the Harness Tool Broker through typed ports.
//
// It propagates task, execution, worktree, and policy context to the broker,
// streams evidence back to Orchestra events via TaskStore.AppendEvent, and
// coordinates terminal outcomes under completion/cancellation races.
type HarnessBridge struct {
	mu              sync.Mutex
	broker          harness.ToolBroker
	store           TaskStore
	worktreeRoot    string
	clock           func() time.Time
	lastEvidence    map[ExecutionID][]string
	contextProvider EnvelopeProvider
}

// EnvelopeProvider prepares governed Staff context without granting authority.
type EnvelopeProvider interface {
	Prepare(context.Context, Task, Execution) (ExecutionEnvelope, error)
}

// NewHarnessBridge creates a new bridge connecting Orchestra to Harness.
func NewHarnessBridge(broker harness.ToolBroker, store TaskStore, worktreeRoot string) *HarnessBridge {
	return &HarnessBridge{
		broker:       broker,
		store:        store,
		worktreeRoot: worktreeRoot,
		clock:        time.Now,
		lastEvidence: make(map[ExecutionID][]string),
	}
}

// SetContextProvider adds governed advisory context to dispatch envelopes.
func (h *HarnessBridge) SetContextProvider(provider EnvelopeProvider) { h.contextProvider = provider }

// PrepareExecution lets the bridge supply governed context while preserving the
// default envelope shape for callers that do not configure a provider.
func (h *HarnessBridge) PrepareExecution(ctx context.Context, task Task, execution Execution) (ExecutionEnvelope, error) {
	envelope := ExecutionEnvelope{TaskID: task.ID, ProjectID: task.ProjectID, WorktreeID: task.WorktreeID, Attempt: execution.Attempt, Title: task.Title, TraceID: string(execution.ID)}
	if h.contextProvider != nil {
		prepared, err := h.contextProvider.Prepare(ctx, task, execution)
		if err != nil {
			return ExecutionEnvelope{}, err
		}
		return prepared, nil
	}
	return envelope, nil
}

// LastEvidenceIDs retrieves the evidence IDs recorded for an execution.
func (h *HarnessBridge) LastEvidenceIDs(id ExecutionID) []string {
	h.mu.Lock()
	defer h.mu.Unlock()
	return append([]string(nil), h.lastEvidence[id]...)
}

// Execute implements the Executor interface.
// It dispatches an execution envelope to the Harness broker, streams evidence,
// and returns the tool result. Context cancellation is propagated to the broker,
// sandbox, adapter, and child processes.
func (h *HarnessBridge) Execute(ctx context.Context, envelope ExecutionEnvelope) (execErr error) {
	defer func() {
		if r := recover(); r != nil {
			execErr = fmt.Errorf("harness panic: %v", r)
		}
	}()

	if err := ctx.Err(); err != nil {
		return err
	}

	// Retrieve the full task to get acceptance criteria and other context
	task, err := h.store.GetTask(ctx, envelope.TaskID)
	if err != nil {
		return fmt.Errorf("failed to get task: %w", err)
	}

	// Retrieve the execution record
	execution, err := h.store.GetExecution(ctx, ExecutionID(fmt.Sprintf("%s-%d", task.ID, envelope.Attempt)))
	if err != nil {
		return fmt.Errorf("failed to get execution: %w", err)
	}

	// Build the Harness ToolRequest from the Orchestra envelope
	toolName := h.selectToolForTask(task)
	timeout := 120 * time.Second
	if def, ok := h.broker.Get(toolName); ok && def.Timeout > 0 {
		timeout = def.Timeout
	}

	traceID := envelope.TraceID
	if traceID == "" {
		traceID = fmt.Sprintf("orch-%s-%d", task.ID, envelope.Attempt)
	}
	request := harness.ToolRequest{
		ContractVersion: harness.HarnessContractVersion,
		ExecutionID:     string(execution.ID),
		TaskID:          string(task.ID),
		ProjectID:       task.ProjectID,
		WorktreeID:      envelope.WorktreeID,
		ToolName:        toolName,
		Input:           h.buildInput(task, envelope),
		Timeout:         timeout,
		Audit: harness.AuditMetadata{
			Actor:         "orchestra-dispatcher",
			TraceID:       traceID,
			CorrelationID: string(execution.ID),
		},
		TraceID:          traceID,
		ExecutionContext: h.executionContext(envelope, execution, traceID),
		SkillInjection:   envelope.Skill,
	}
	if len(envelope.Memory) > 0 {
		var mems []harness.AdvisoryMemoryContext
		for _, m := range envelope.Memory {
			mems = append(mems, harness.AdvisoryMemoryContext{
				ID:      m.ID,
				Kind:    m.Kind,
				Content: m.Content,
				Source:  m.Source,
				Provenance: harness.AssignmentProvenance{
					TaskID:      m.Provenance.TaskID,
					ExecutionID: m.Provenance.ExecutionID,
					TraceID:     m.Provenance.TraceID,
				},
			})
		}
		request.MemoryContext = mems
	}

	// Create execution context with trace ID
	execCtx := harness.NewExecutionContext(ctx, request.TraceID)

	// Execute through the broker
	result, brokerErr := h.broker.Execute(execCtx, request)

	// Stream evidence back to Orchestra events regardless of outcome (using un-canceled context)
	evidenceCtx := context.WithoutCancel(ctx)
	if err := h.streamEvidence(evidenceCtx, task, execution, result); err != nil {
		if brokerErr == nil {
			return fmt.Errorf("evidence streaming failed: %w", err)
		}
	}

	// Return the broker execution error if any
	if brokerErr != nil {
		return brokerErr
	}

	return nil
}

func (h *HarnessBridge) executionContext(envelope ExecutionEnvelope, execution Execution, traceID string) *harness.ExecutionContextMetadata {
	metadata := &harness.ExecutionContextMetadata{
		WorkspaceID: envelope.WorkspaceID,
		ProjectID:   envelope.ProjectID,
		WorktreeID:  envelope.WorktreeID,
		Provenance:  harness.AssignmentProvenance{TaskID: string(execution.TaskID), ExecutionID: string(execution.ID), TraceID: traceID},
	}
	if envelope.Staff != nil {
		metadata.StaffID = string(envelope.Staff.ID)
		metadata.StaffRole = string(envelope.Staff.Role)
		if metadata.WorkspaceID == "" {
			metadata.WorkspaceID = string(envelope.Staff.Workspace.WorkspaceID)
		}
		if metadata.ProjectID == "" {
			metadata.ProjectID = string(envelope.Staff.Workspace.ProjectID)
		}
	}
	metadata.Assignment = harness.AssignmentMetadata{
		AssignmentID: envelope.Assignment.AssignmentID,
		Source:       envelope.Assignment.Source,
		AssignedAt:   envelope.Assignment.AssignedAt,
	}
	return metadata
}

// streamEvidence writes Harness evidence records as Orchestra TaskEvents.
func (h *HarnessBridge) streamEvidence(ctx context.Context, task Task, execution Execution, result harness.ToolResult) error {
	// Get evidence records from the broker for this execution when available.
	var evidenceRecords []harness.EvidenceRecord
	if auditBroker, ok := h.broker.(interface {
		AuditEventsForExecution(string) []harness.EvidenceRecord
	}); ok {
		evidenceRecords = auditBroker.AuditEventsForExecution(string(execution.ID))
	}
	if len(evidenceRecords) == 0 && len(result.EvidenceIDs) == 0 {
		return nil // No evidence to stream
	}

	var evidenceIDs []string

	// Add evidence IDs from the tool result
	evidenceIDs = append(evidenceIDs, result.EvidenceIDs...)

	// Add evidence IDs from broker audit log
	for _, rec := range evidenceRecords {
		evidenceIDs = append(evidenceIDs, rec.ID)
	}

	if len(evidenceIDs) == 0 {
		return nil
	}
	uniqueEvidenceIDs := make([]string, 0, len(evidenceIDs))
	seen := make(map[string]struct{}, len(evidenceIDs))
	for _, id := range evidenceIDs {
		if id == "" {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		uniqueEvidenceIDs = append(uniqueEvidenceIDs, id)
	}
	if len(uniqueEvidenceIDs) == 0 {
		return nil
	}
	h.mu.Lock()
	h.lastEvidence[execution.ID] = append([]string(nil), uniqueEvidenceIDs...)
	h.mu.Unlock()

	// Determine event type based on tool result status
	eventType := EventEvidenceCollected
	if result.Status == harness.ToolStatusPolicyDenied {
		eventType = EventExecutionFailed
	} else if result.Status == harness.ToolStatusTimeout {
		eventType = EventExecutionFailed
	} else if result.Status == harness.ToolStatusCanceled {
		eventType = EventTaskCanceled
	} else if result.Status != harness.ToolStatusSuccess {
		eventType = EventExecutionFailed
	}

	// Append event to Orchestra
	return h.store.AppendEvent(ctx, TaskEvent{
		TaskID:      task.ID,
		ExecutionID: execution.ID,
		Type:        eventType,
		From:        task.Status,
		To:          task.Status, // Status doesn't change here; Complete() handles transition
		EvidenceIDs: uniqueEvidenceIDs,
		Message:     fmt.Sprintf("Harness tool %s completed with status: %s", result.ToolName, result.Status),
		OccurredAt:  h.clock(),
	})
}

// selectToolForTask selects the appropriate tool based on task criteria.
// This is a simple heuristic; in practice, task metadata would indicate the tool.
func (h *HarnessBridge) selectToolForTask(task Task) string {
	// For now, default to a generic coding tool
	// In production, this would be determined by task type/acceptance criteria
	if len(task.AcceptanceCriteria) > 0 {
		for _, criterion := range task.AcceptanceCriteria {
			if contains(criterion, "test") {
				return "test_runner"
			}
			if contains(criterion, "build") {
				return "build_runner"
			}
			if contains(criterion, "lint") {
				return "lint_runner"
			}
		}
	}
	return "coding_agent"
}

// buildInput constructs the tool input JSON from the task and envelope.
func (h *HarnessBridge) buildInput(task Task, envelope ExecutionEnvelope) json.RawMessage {
	input := map[string]any{
		"title":              task.Title,
		"acceptanceCriteria": task.AcceptanceCriteria,
		"worktreeRoot":       h.worktreeRoot,
		"attempt":            envelope.Attempt,
	}
	raw, _ := json.Marshal(input)
	return raw
}

// contains checks if a string contains a substring (case-insensitive).
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && findSubstring(s, substr))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// HarnessBridgeError classifies bridge-level errors for retry decisions.
type HarnessBridgeError struct {
	Code      string
	Message   string
	Retriable bool
}

func (e *HarnessBridgeError) Error() string {
	if e == nil {
		return "<nil>"
	}
	return e.Code + ": " + e.Message
}

// IsRetriable returns true if the error suggests a retry may succeed.
func (e *HarnessBridgeError) IsRetriable() bool {
	return e != nil && e.Retriable
}

// ClassifyError converts a Harness tool error into a retryable classification.
func ClassifyError(err error) FailureClass {
	if err == nil {
		return FailureNonRetryable
	}

	var harnessErr *harness.ToolError
	if errors.As(err, &harnessErr) && harnessErr != nil {
		if harnessErr.IsRetriable() {
			return FailureRetryable
		}
		return FailureNonRetryable
	}
	var codedErr *harness.CodedError
	if errors.As(err, &codedErr) && codedErr != nil {
		if codedErr.Retryable {
			return FailureRetryable
		}
		return FailureNonRetryable
	}

	var bridgeErr *HarnessBridgeError
	if errors.As(err, &bridgeErr) && bridgeErr != nil {
		if bridgeErr.IsRetriable() {
			return FailureRetryable
		}
		return FailureNonRetryable
	}

	// Context cancellation and deadline are retryable
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return FailureRetryable
	}

	// Default to non-retryable for unknown errors
	return FailureNonRetryable
}
