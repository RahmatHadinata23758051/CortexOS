package application

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func validProposal() ToolProposal {
	return ToolProposal{
		ProposalID:          "prop-1234",
		CorrelationID:       "corr-5678",
		TaskID:              "task-main-1",
		WorktreeID:          "wt-alpha-0",
		ToolName:            "file_read",
		Capability:          "file_read",
		RedactedArgsSummary: "Read relative path internal/config/config.go",
		ArgByteSize:         128,
		ProposedAt:          time.Now().UTC(),
		SchemaVersion:       ToolProposalSchemaVersion,
	}
}

func validDecision() ToolApprovalDecision {
	return ToolApprovalDecision{
		ProposalID:     "prop-1234",
		Decision:       "approve",
		Actor:          "operator-alice",
		Reason:         "Approved for code inspection within worktree",
		IdempotencyKey: "idem-key-1",
		DecidedAt:      time.Now().UTC(),
		SchemaVersion:  ToolDecisionSchemaVersion,
	}
}

func TestToolProposalValidation(t *testing.T) {
	t.Parallel()

	t.Run("valid proposal succeeds", func(t *testing.T) {
		p := validProposal()
		if err := p.Validate(); err != nil {
			t.Fatalf("unexpected validation error: %v", err)
		}
	})

	t.Run("unsupported schema version fails", func(t *testing.T) {
		p := validProposal()
		p.SchemaVersion = "cortexos.tool-approval.proposal.v0"
		if err := p.Validate(); err == nil {
			t.Fatalf("expected error for unsupported schema version")
		}
	})

	t.Run("invalid proposalId fails", func(t *testing.T) {
		for _, bad := range []string{"", "prop/123", "prop with spaces", strings.Repeat("x", 65)} {
			p := validProposal()
			p.ProposalID = bad
			if err := p.Validate(); err == nil {
				t.Fatalf("expected error for proposalId %q", bad)
			}
		}
	})

	t.Run("invalid correlationId fails", func(t *testing.T) {
		p := validProposal()
		p.CorrelationID = ""
		if err := p.Validate(); err == nil {
			t.Fatalf("expected error for empty correlationId")
		}
	})

	t.Run("invalid taskId fails", func(t *testing.T) {
		p := validProposal()
		p.TaskID = "   "
		if err := p.Validate(); err == nil {
			t.Fatalf("expected error for blank taskId")
		}
	})

	t.Run("invalid toolName fails", func(t *testing.T) {
		for _, bad := range []string{"", "tool/exec", "run command", strings.Repeat("a", 65)} {
			p := validProposal()
			p.ToolName = bad
			if err := p.Validate(); err == nil {
				t.Fatalf("expected error for toolName %q", bad)
			}
		}
	})

	t.Run("excessive arg byte size fails", func(t *testing.T) {
		p := validProposal()
		p.ArgByteSize = 65537
		if err := p.Validate(); err == nil {
			t.Fatalf("expected error for argByteSize > 64KB")
		}
	})

	t.Run("excessive args summary length fails", func(t *testing.T) {
		p := validProposal()
		p.RedactedArgsSummary = strings.Repeat("a", 1001)
		if err := p.Validate(); err == nil {
			t.Fatalf("expected error for summary > 1000 chars")
		}
	})

	t.Run("args summary with secret token fails", func(t *testing.T) {
		for _, secret := range []string{"sk-123456789", "bearer eyJhbGci", "authorization: token", "-----BEGIN PRIVATE KEY-----"} {
			p := validProposal()
			p.RedactedArgsSummary = "Pass token: " + secret
			if err := p.Validate(); err == nil {
				t.Fatalf("expected error for secret token %q", secret)
			}
		}
	})

	t.Run("args summary with absolute path fails", func(t *testing.T) {
		for _, abs := range []string{`C:\Users\admin\Desktop`, `D:\data\keys`, `/etc/passwd`, `/home/user/.ssh`, `/Users/admin/pass`} {
			p := validProposal()
			p.RedactedArgsSummary = "Accessing " + abs
			if err := p.Validate(); err == nil {
				t.Fatalf("expected error for absolute path %q", abs)
			}
		}
	})
}

func TestToolApprovalDecisionValidation(t *testing.T) {
	t.Parallel()

	t.Run("valid decision succeeds", func(t *testing.T) {
		d := validDecision()
		if err := d.Validate(); err != nil {
			t.Fatalf("unexpected validation error: %v", err)
		}

		d.Decision = "deny"
		d.Reason = "Operation outside policy scope"
		if err := d.Validate(); err != nil {
			t.Fatalf("unexpected validation error for deny: %v", err)
		}
	})

	t.Run("unsupported schema version fails", func(t *testing.T) {
		d := validDecision()
		d.SchemaVersion = "cortexos.tool-approval.decision.v0"
		if err := d.Validate(); err == nil {
			t.Fatalf("expected error for unsupported schema version")
		}
	})

	t.Run("invalid decision value fails", func(t *testing.T) {
		for _, bad := range []string{"", "allow", "yes", "rejected", "APPROVED"} {
			d := validDecision()
			d.Decision = bad
			if err := d.Validate(); err == nil {
				t.Fatalf("expected error for decision %q", bad)
			}
		}
	})

	t.Run("empty or oversized actor fails", func(t *testing.T) {
		d := validDecision()
		d.Actor = ""
		if err := d.Validate(); err == nil {
			t.Fatalf("expected error for empty actor")
		}
		d.Actor = strings.Repeat("x", 129)
		if err := d.Validate(); err == nil {
			t.Fatalf("expected error for actor > 128 chars")
		}
	})

	t.Run("oversized reason fails", func(t *testing.T) {
		d := validDecision()
		d.Reason = strings.Repeat("r", 501)
		if err := d.Validate(); err == nil {
			t.Fatalf("expected error for reason > 500 chars")
		}
	})

	t.Run("reason with secret token fails", func(t *testing.T) {
		d := validDecision()
		d.Reason = "Secret token: ghp_abc123456"
		if err := d.Validate(); err == nil {
			t.Fatalf("expected error for secret token in reason")
		}
	})

	t.Run("reason with absolute path fails", func(t *testing.T) {
		d := validDecision()
		d.Reason = "Target file at C:\\Windows\\System32\\cmd.exe"
		if err := d.Validate(); err == nil {
			t.Fatalf("expected error for absolute path in reason")
		}
	})
}

func TestToolApprovalContractsContainNoForbiddenFields(t *testing.T) {
	t.Parallel()

	p := validProposal()
	encodedProposal, err := json.Marshal(p)
	if err != nil {
		t.Fatalf("marshal proposal error: %v", err)
	}

	d := validDecision()
	encodedDecision, err := json.Marshal(d)
	if err != nil {
		t.Fatalf("marshal decision error: %v", err)
	}

	for _, text := range []string{string(encodedProposal), string(encodedDecision)} {
		for _, forbidden := range []string{"apiKey", "processHandle", "password", "secret", "privateKey", `C:\`, `/Users/`, `/home/`} {
			if strings.Contains(text, forbidden) {
				t.Fatalf("contract contains forbidden keyword %q: %s", forbidden, text)
			}
		}
	}
}
