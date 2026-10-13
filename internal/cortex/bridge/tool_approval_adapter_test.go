package bridge

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/RahmatHadinata23758051/CortexOS/internal/cortex/application"
)

type fakeExecutor struct {
	mu       sync.Mutex
	calls    int
	lastReq  application.ToolExecutionRequest
	result   application.ToolExecutionResult
	err      error
	blocking chan struct{}
}

func (f *fakeExecutor) ExecuteApprovedTool(ctx context.Context, req application.ToolExecutionRequest) (application.ToolExecutionResult, error) {
	f.mu.Lock()
	f.calls++
	f.lastReq = req
	f.mu.Unlock()
	if f.blocking != nil {
		select {
		case <-f.blocking:
		case <-ctx.Done():
			return application.ToolExecutionResult{}, ctx.Err()
		}
	}
	if f.err != nil {
		return application.ToolExecutionResult{}, f.err
	}
	res := f.result
	if res.ExecutionID == "" {
		res.ExecutionID = "exec-1"
	}
	if res.Status == "" {
		res.Status = application.ToolApprovalExecuted
	}
	res.SchemaVersion = application.ToolProposalSchemaVersion
	return res, nil
}

func (f *fakeExecutor) callCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.calls
}

func newTestApprovalAdapter(t *testing.T, exec application.ToolExecutionPort) application.ToolApprovalPort {
	t.Helper()
	adapter, err := NewToolApprovalAdapter(ToolApprovalAdapterConfig{
		Allowlist: map[string]string{
			"file_read":  "file_read",
			"file_write": "file_write",
			"git":        "git",
		},
		Executor: exec,
	})
	if err != nil {
		t.Fatalf("NewToolApprovalAdapter: %v", err)
	}
	return adapter
}

func sampleProposal() application.ToolProposal {
	return application.ToolProposal{
		ProposalID:          "prop-1",
		CorrelationID:       "corr-1",
		TaskID:              "task-1",
		WorktreeID:          "wt-1",
		ToolName:            "file_read",
		Capability:          "file_read",
		RedactedArgsSummary: "Read internal/config/config.go",
		ArgByteSize:         64,
		ProposedAt:          time.Unix(1000, 0).UTC(),
		SchemaVersion:       application.ToolProposalSchemaVersion,
	}
}

func sampleDecision(decision string) application.ToolApprovalDecision {
	return application.ToolApprovalDecision{
		ProposalID:     "prop-1",
		Decision:       decision,
		Actor:          "operator-1",
		Reason:         "Reviewed in cockpit",
		IdempotencyKey: "idem-1",
		DecidedAt:      time.Unix(2000, 0).UTC(),
		SchemaVersion:  application.ToolDecisionSchemaVersion,
	}
}

func TestToolApprovalAdapterRequiresConfig(t *testing.T) {
	t.Parallel()
	if _, err := NewToolApprovalAdapter(ToolApprovalAdapterConfig{Executor: &fakeExecutor{}}); err == nil {
		t.Fatalf("expected error for empty allowlist")
	}
	if _, err := NewToolApprovalAdapter(ToolApprovalAdapterConfig{Allowlist: map[string]string{"a": "a"}}); err == nil {
		t.Fatalf("expected error for nil executor")
	}
}

func TestToolApprovalAdapterSubmitAndApprove(t *testing.T) {
	t.Parallel()
	exec := &fakeExecutor{}
	adapter := newTestApprovalAdapter(t, exec)

	submitted, err := adapter.SubmitProposal(context.Background(), sampleProposal())
	if err != nil {
		t.Fatalf("SubmitProposal: %v", err)
	}
	if submitted.Status != application.ToolApprovalPending {
		t.Fatalf("status = %q, want pending", submitted.Status)
	}

	result, err := adapter.DecideProposal(context.Background(), sampleDecision("approve"))
	if err != nil {
		t.Fatalf("DecideProposal(approve): %v", err)
	}
	if result.Status != application.ToolApprovalExecuted {
		t.Fatalf("status = %q, want executed", result.Status)
	}
	if exec.callCount() != 1 {
		t.Fatalf("executor calls = %d, want 1", exec.callCount())
	}
	if exec.lastReq.WorktreeID != "wt-1" || exec.lastReq.Capability != "file_read" {
		t.Fatalf("unexpected exec request: %#v", exec.lastReq)
	}

	record, err := adapter.GetProposal(context.Background(), "prop-1")
	if err != nil {
		t.Fatalf("GetProposal: %v", err)
	}
	if record.Status != application.ToolApprovalExecuted || record.Actor != "operator-1" {
		t.Fatalf("record = %#v", record)
	}
}

func TestToolApprovalAdapterDenyDoesNotExecute(t *testing.T) {
	t.Parallel()
	exec := &fakeExecutor{}
	adapter := newTestApprovalAdapter(t, exec)
	if _, err := adapter.SubmitProposal(context.Background(), sampleProposal()); err != nil {
		t.Fatalf("SubmitProposal: %v", err)
	}
	result, err := adapter.DecideProposal(context.Background(), sampleDecision("deny"))
	if err != nil {
		t.Fatalf("DecideProposal(deny): %v", err)
	}
	if result.Status != application.ToolApprovalDenied {
		t.Fatalf("status = %q, want denied", result.Status)
	}
	if exec.callCount() != 0 {
		t.Fatalf("executor calls = %d, want 0 for denied proposal", exec.callCount())
	}
}

func TestToolApprovalAdapterUnknownToolRejected(t *testing.T) {
	t.Parallel()
	adapter := newTestApprovalAdapter(t, &fakeExecutor{})
	p := sampleProposal()
	p.ToolName = "shell_exec"
	p.Capability = "shell"
	if _, err := adapter.SubmitProposal(context.Background(), p); err == nil {
		t.Fatalf("expected error for non-allowlisted tool")
	}
}

func TestToolApprovalAdapterCapabilityMismatchRejected(t *testing.T) {
	t.Parallel()
	adapter := newTestApprovalAdapter(t, &fakeExecutor{})
	p := sampleProposal()
	p.Capability = "git" // allowlisted tool is file_read->file_read
	if _, err := adapter.SubmitProposal(context.Background(), p); err == nil {
		t.Fatalf("expected error for capability mismatch")
	}
}

func TestToolApprovalAdapterDuplicateSubmissionIsIdempotent(t *testing.T) {
	t.Parallel()
	adapter := newTestApprovalAdapter(t, &fakeExecutor{})
	first, err := adapter.SubmitProposal(context.Background(), sampleProposal())
	if err != nil {
		t.Fatalf("SubmitProposal: %v", err)
	}
	second, err := adapter.SubmitProposal(context.Background(), sampleProposal())
	if err != nil {
		t.Fatalf("SubmitProposal(dup): %v", err)
	}
	if first.ProposalID != second.ProposalID || second.Status != application.ToolApprovalPending {
		t.Fatalf("duplicate submission changed state: %#v vs %#v", first, second)
	}
}

func TestToolApprovalAdapterDuplicateDecisionIsIdempotent(t *testing.T) {
	t.Parallel()
	exec := &fakeExecutor{}
	adapter := newTestApprovalAdapter(t, exec)
	if _, err := adapter.SubmitProposal(context.Background(), sampleProposal()); err != nil {
		t.Fatalf("SubmitProposal: %v", err)
	}
	decision := sampleDecision("approve")
	if _, err := adapter.DecideProposal(context.Background(), decision); err != nil {
		t.Fatalf("DecideProposal: %v", err)
	}
	if _, err := adapter.DecideProposal(context.Background(), decision); err != nil {
		t.Fatalf("DecideProposal(dup): %v", err)
	}
	if exec.callCount() != 1 {
		t.Fatalf("executor calls = %d, want 1 after duplicate decision", exec.callCount())
	}
}

func TestToolApprovalAdapterConflictingDecisionRejected(t *testing.T) {
	t.Parallel()
	exec := &fakeExecutor{}
	adapter := newTestApprovalAdapter(t, exec)
	if _, err := adapter.SubmitProposal(context.Background(), sampleProposal()); err != nil {
		t.Fatalf("SubmitProposal: %v", err)
	}
	if _, err := adapter.DecideProposal(context.Background(), sampleDecision("deny")); err != nil {
		t.Fatalf("DecideProposal(deny): %v", err)
	}
	conflict := sampleDecision("approve")
	conflict.IdempotencyKey = "idem-2"
	if _, err := adapter.DecideProposal(context.Background(), conflict); !errors.Is(err, application.ErrProposalAlreadyDecided) {
		t.Fatalf("expected ErrProposalAlreadyDecided, got %v", err)
	}
	if exec.callCount() != 0 {
		t.Fatalf("executor calls = %d, want 0 after conflicting decision", exec.callCount())
	}
}

func TestToolApprovalAdapterUnknownProposalRejected(t *testing.T) {
	t.Parallel()
	adapter := newTestApprovalAdapter(t, &fakeExecutor{})
	if _, err := adapter.DecideProposal(context.Background(), sampleDecision("approve")); !errors.Is(err, application.ErrProposalNotFound) {
		t.Fatalf("expected ErrProposalNotFound, got %v", err)
	}
}

func TestToolApprovalAdapterCancellationFailsClosed(t *testing.T) {
	t.Parallel()
	exec := &fakeExecutor{blocking: make(chan struct{})}
	adapter := newTestApprovalAdapter(t, exec)
	if _, err := adapter.SubmitProposal(context.Background(), sampleProposal()); err != nil {
		t.Fatalf("SubmitProposal: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(10 * time.Millisecond)
		cancel()
	}()
	_, err := adapter.DecideProposal(ctx, sampleDecision("approve"))
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
	record, err := adapter.GetProposal(context.Background(), "prop-1")
	if err != nil {
		t.Fatalf("GetProposal: %v", err)
	}
	if record.Status != application.ToolApprovalCanceled {
		t.Fatalf("status = %q, want canceled", record.Status)
	}
}

func TestToolApprovalAdapterExecutorErrorIsSanitized(t *testing.T) {
	t.Parallel()
	secretErr := errors.New("adapter failed: raw output C:\\Users\\admin\\secret and token sk-abc123")
	exec := &fakeExecutor{err: secretErr}
	adapter := newTestApprovalAdapter(t, exec)
	if _, err := adapter.SubmitProposal(context.Background(), sampleProposal()); err != nil {
		t.Fatalf("SubmitProposal: %v", err)
	}
	_, err := adapter.DecideProposal(context.Background(), sampleDecision("approve"))
	if err == nil {
		t.Fatalf("expected error")
	}
	msg := err.Error()
	for _, leak := range []string{"sk-abc123", `C:\Users`, "raw output"} {
		if strings.Contains(msg, leak) {
			t.Fatalf("sanitized error leaked %q: %s", leak, msg)
		}
	}
}

func TestToolApprovalAdapterListPending(t *testing.T) {
	t.Parallel()
	adapter := newTestApprovalAdapter(t, &fakeExecutor{})
	p1 := sampleProposal()
	p2 := sampleProposal()
	p2.ProposalID = "prop-2"
	p2.CorrelationID = "corr-2"
	if _, err := adapter.SubmitProposal(context.Background(), p1); err != nil {
		t.Fatalf("SubmitProposal p1: %v", err)
	}
	if _, err := adapter.SubmitProposal(context.Background(), p2); err != nil {
		t.Fatalf("SubmitProposal p2: %v", err)
	}
	pending, err := adapter.ListPendingProposals(context.Background())
	if err != nil {
		t.Fatalf("ListPendingProposals: %v", err)
	}
	if len(pending) != 2 {
		t.Fatalf("pending = %d, want 2", len(pending))
	}
	if _, err := adapter.DecideProposal(context.Background(), sampleDecision("deny")); err != nil {
		t.Fatalf("DecideProposal: %v", err)
	}
	pending, err = adapter.ListPendingProposals(context.Background())
	if err != nil {
		t.Fatalf("ListPendingProposals: %v", err)
	}
	if len(pending) != 1 || pending[0].ProposalID != "prop-2" {
		t.Fatalf("pending after decision = %#v", pending)
	}
}

func TestToolApprovalAdapterRejectsUnsafeProposal(t *testing.T) {
	t.Parallel()
	adapter := newTestApprovalAdapter(t, &fakeExecutor{})
	p := sampleProposal()
	p.RedactedArgsSummary = "Read file at C:\\Users\\admin\\.ssh\\id_rsa"
	if _, err := adapter.SubmitProposal(context.Background(), p); err == nil {
		t.Fatalf("expected error for absolute path in summary")
	}
}
