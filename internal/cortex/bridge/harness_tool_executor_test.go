package bridge

import (
	"context"
	"errors"
	"testing"

	"github.com/RahmatHadinata23758051/CortexOS/internal/cortex/application"
	"github.com/RahmatHadinata23758051/CortexOS/internal/cortex/harness"
)

type fakeBroker struct {
	lastReq   harness.ToolRequest
	res       harness.ToolResult
	err       error
	execCalls int
}

func (b *fakeBroker) Register(harness.ToolDefinition) error { return nil }
func (b *fakeBroker) Unregister(string) error               { return nil }
func (b *fakeBroker) Get(string) (harness.ToolDefinition, bool) {
	return harness.ToolDefinition{}, false
}
func (b *fakeBroker) List() []harness.ToolDefinition                { return nil }
func (b *fakeBroker) ListCapabilities() []harness.CapabilityBinding { return nil }
func (b *fakeBroker) Policy() harness.PolicyEngine                  { return nil }
func (b *fakeBroker) Execute(_ harness.ExecutionContext, r harness.ToolRequest) (harness.ToolResult, error) {
	b.execCalls++
	b.lastReq = r
	if b.err != nil {
		return harness.ToolResult{}, b.err
	}
	res := b.res
	if res.ExecutionID == "" {
		res.ExecutionID = r.ExecutionID
	}
	if res.Status == "" {
		res.Status = harness.ToolStatusSuccess
	}
	return res, nil
}

func TestHarnessToolExecutorRequiresBroker(t *testing.T) {
	t.Parallel()
	if _, err := NewHarnessToolExecutor(HarnessToolExecutorConfig{}); err == nil {
		t.Fatalf("expected error for nil broker")
	}
}

func TestHarnessToolExecutorSuccess(t *testing.T) {
	t.Parallel()
	broker := &fakeBroker{
		res: harness.ToolResult{
			Status:      harness.ToolStatusSuccess,
			EvidenceIDs: []string{"ev-1"},
		},
	}
	exec, err := NewHarnessToolExecutor(HarnessToolExecutorConfig{Broker: broker})
	if err != nil {
		t.Fatalf("NewHarnessToolExecutor: %v", err)
	}

	result, err := exec.ExecuteApprovedTool(context.Background(), application.ToolExecutionRequest{
		ProposalID:    "prop-1",
		CorrelationID: "corr-1",
		TaskID:        "task-1",
		WorktreeID:    "wt-1",
		ToolName:      "file_read",
		Capability:    "file_read",
		SchemaVersion: application.ToolProposalSchemaVersion,
	})
	if err != nil {
		t.Fatalf("ExecuteApprovedTool: %v", err)
	}
	if result.Status != application.ToolApprovalExecuted {
		t.Fatalf("status = %q, want executed", result.Status)
	}
	if len(result.EvidenceIDs) != 1 || result.EvidenceIDs[0] != "ev-1" {
		t.Fatalf("evidence = %#v", result.EvidenceIDs)
	}
	if broker.execCalls != 1 || broker.lastReq.ToolName != "file_read" {
		t.Fatalf("unexpected broker request: %#v", broker.lastReq)
	}
}

func TestHarnessToolExecutorBrokerFailure(t *testing.T) {
	t.Parallel()
	broker := &fakeBroker{
		err: errors.New("sandbox violation C:\\sensitive"),
	}
	exec, err := NewHarnessToolExecutor(HarnessToolExecutorConfig{Broker: broker})
	if err != nil {
		t.Fatalf("NewHarnessToolExecutor: %v", err)
	}

	result, err := exec.ExecuteApprovedTool(context.Background(), application.ToolExecutionRequest{
		ProposalID:    "prop-1",
		CorrelationID: "corr-1",
		TaskID:        "task-1",
		WorktreeID:    "wt-1",
		ToolName:      "file_read",
		Capability:    "file_read",
		SchemaVersion: application.ToolProposalSchemaVersion,
	})
	if err == nil {
		t.Fatalf("expected error from broker failure")
	}
	if result.Status != application.ToolApprovalFailed {
		t.Fatalf("status = %q, want failed", result.Status)
	}
}
