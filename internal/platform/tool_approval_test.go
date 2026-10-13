package platform

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/RahmatHadinata23758051/CortexOS/internal/cortex/application"
)

type platformApprovalPort struct {
	proposal application.ToolProposalResult
	decision application.ToolApprovalResult
	record   application.ToolProposalRecord
}

func (p *platformApprovalPort) SubmitProposal(_ context.Context, proposal application.ToolProposal) (application.ToolProposalResult, error) {
	p.proposal = application.ToolProposalResult{
		ProposalID:    proposal.ProposalID,
		Status:        application.ToolApprovalPending,
		SubmittedAt:   time.Unix(1, 0).UTC(),
		SchemaVersion: application.ToolProposalSchemaVersion,
	}
	return p.proposal, nil
}

func (p *platformApprovalPort) DecideProposal(_ context.Context, decision application.ToolApprovalDecision) (application.ToolApprovalResult, error) {
	status := application.ToolApprovalDenied
	if decision.Decision == "approve" {
		status = application.ToolApprovalExecuted
	}
	p.decision = application.ToolApprovalResult{
		ProposalID:    decision.ProposalID,
		Status:        status,
		Actor:         decision.Actor,
		DecidedAt:     time.Unix(2, 0).UTC(),
		SchemaVersion: application.ToolDecisionSchemaVersion,
	}
	return p.decision, nil
}

func (p *platformApprovalPort) GetProposal(context.Context, string) (application.ToolProposalRecord, error) {
	return p.record, nil
}

func (p *platformApprovalPort) ListPendingProposals(context.Context) ([]application.ToolProposalRecord, error) {
	return []application.ToolProposalRecord{p.record}, nil
}

func TestBridgeToolApprovalLifecycleEvents(t *testing.T) {
	port := &platformApprovalPort{}
	service := application.NewService()
	service.SetToolApproval(port)
	b := NewBridge(service)

	var names []string
	var payloads []any
	unsubscribe := b.RegisterEventHook(func(name string, payload any) {
		names = append(names, name)
		payloads = append(payloads, payload)
	})
	defer unsubscribe()

	proposal := application.ToolProposal{
		ProposalID:          "prop-platform",
		CorrelationID:       "corr-platform",
		TaskID:              "task-platform",
		WorktreeID:          "wt-platform",
		ToolName:            "file_read",
		Capability:          "file_read",
		RedactedArgsSummary: "Read relative config file",
		ArgByteSize:         32,
		SchemaVersion:       application.ToolProposalSchemaVersion,
	}
	if _, err := b.ProposeToolCall(proposal); err != nil {
		t.Fatalf("ProposeToolCall: %v", err)
	}
	if _, err := b.DecideToolCall(application.ToolApprovalDecision{
		ProposalID:     proposal.ProposalID,
		Decision:       "deny",
		Actor:          "operator-platform",
		IdempotencyKey: "idem-platform",
		SchemaVersion:  application.ToolDecisionSchemaVersion,
	}); err != nil {
		t.Fatalf("DecideToolCall: %v", err)
	}

	if len(names) != 2 || names[0] != EventToolProposed || names[1] != EventToolDenied {
		t.Fatalf("events = %#v, want proposed/denied", names)
	}
	for _, payload := range payloads {
		encoded := strings.TrimSpace(string(mustJSON(t, payload)))
		for _, forbidden := range []string{"Read relative config file", "rawArgs", "apiKey", "processHandle", `C:\\Users`, "/home/"} {
			if strings.Contains(encoded, forbidden) {
				t.Fatalf("event payload leaked %q: %s", forbidden, encoded)
			}
		}
	}
}

func TestBridgeToolApprovalExecutedEvent(t *testing.T) {
	port := &platformApprovalPort{}
	service := application.NewService()
	service.SetToolApproval(port)
	b := NewBridge(service)

	var event string
	unsubscribe := b.RegisterEventHook(func(name string, _ any) { event = name })
	defer unsubscribe()

	result, err := b.DecideToolCall(application.ToolApprovalDecision{
		ProposalID:     "prop-platform",
		Decision:       "approve",
		Actor:          "operator-platform",
		IdempotencyKey: "idem-platform-approve",
		SchemaVersion:  application.ToolDecisionSchemaVersion,
	})
	if err != nil {
		t.Fatalf("DecideToolCall: %v", err)
	}
	if result.Status != application.ToolApprovalExecuted || event != EventToolExecuted {
		t.Fatalf("result/event = %#v/%q, want executed/%q", result, event, EventToolExecuted)
	}
}

func mustJSON(t *testing.T, value any) []byte {
	t.Helper()
	encoded, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("marshal event payload: %v", err)
	}
	return encoded
}
