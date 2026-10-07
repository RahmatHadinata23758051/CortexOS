package harness

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

// FakeEngine is a deterministic test fake for the EngineAdapter interface.
// It allows simulating successes, failures, timeouts, cancellations, and health.
type FakeEngine struct {
	mu           sync.Mutex
	kind         ToolKind
	capabilities []ToolCapability
	executeFunc  func(ctx ExecutionContext, env ExecutionEnvelope) (ToolResult, error)
	healthErr    error
	calls        []ExecutionEnvelope
}

func NewFakeEngine(kind ToolKind, caps ...ToolCapability) *FakeEngine {
	return &FakeEngine{
		kind:         kind,
		capabilities: caps,
	}
}

func (f *FakeEngine) Kind() ToolKind { return f.kind }

func (f *FakeEngine) Capabilities() []ToolCapability { return f.capabilities }

func (f *FakeEngine) Describe() AdapterDescriptor {
	return AdapterDescriptor{
		Name:          fmt.Sprintf("fake-%s-engine", f.kind),
		Kind:          f.kind,
		Version:       "1.0.0-fake",
		Capabilities:  f.capabilities,
		SchemaVersion: HarnessContractVersion,
	}
}

func (f *FakeEngine) Health(ctx ExecutionContext) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.healthErr
}

func (f *FakeEngine) SetHealthError(err error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.healthErr = err
}

func (f *FakeEngine) SetExecuteFunc(fn func(ctx ExecutionContext, env ExecutionEnvelope) (ToolResult, error)) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.executeFunc = fn
}

func (f *FakeEngine) Calls() []ExecutionEnvelope {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]ExecutionEnvelope, len(f.calls))
	copy(out, f.calls)
	return out
}

func (f *FakeEngine) Execute(ctx ExecutionContext, env ExecutionEnvelope) (ToolResult, error) {
	f.mu.Lock()
	f.calls = append(f.calls, env)
	fn := f.executeFunc
	f.mu.Unlock()

	if fn != nil {
		return fn(ctx, env)
	}

	// Default behavior: echo back success with a fake evidence record
	ev, err := NewEvidenceRecord(env.ExecutionID, env.TaskID, env.WorktreeID, EvidenceKindCommandExecution, map[string]string{
		"tool": env.ToolName,
		"echo": "ok",
	})
	if err != nil {
		return ToolResult{}, err
	}

	return ToolResult{
		ContractVersion: HarnessContractVersion,
		ExecutionID:     env.ExecutionID,
		TaskID:          env.TaskID,
		ToolName:        env.ToolName,
		Status:          ToolStatusSuccess,
		Output:          json.RawMessage(`{"status":"executed"}`),
		EvidenceIDs:     []string{ev.ID},
		StartedAt:       time.Now().Add(-10 * time.Millisecond),
		CompletedAt:     time.Now(),
		DurationMs:      10,
		Redacted:        true,
	}, nil
}
