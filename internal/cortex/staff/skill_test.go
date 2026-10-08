package staff

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/RahmatHadinata23758051/CortexOS/internal/cortex/harness"
)

func validStaffDefinition() Definition {
	now := time.Now().UTC()
	return Definition{
		Identity: Identity{
			ID:   "staff-tester",
			Name: "Tester Staff",
		},
		Role:         RoleImplementer,
		Capabilities: []Capability{"coding", "test_gen", "analysis", "refactor"},
		Permissions: PermissionSet{
			{
				ID:       "perm-code",
				Action:   "code:execute",
				Resource: "workspace:default",
				Effect:   EffectAllow,
				Priority: 10,
			},
		},
		Skills: []SkillReference{
			{ID: "coding-assistant", Version: "1.0.0"},
		},
		Workspace: WorkspaceAssignment{
			WorkspaceID: "ws-primary",
			ProjectID:   "proj-cortex",
		},
		Lifecycle:     LifecycleActive,
		Availability:  AvailabilityAvailable,
		CreatedAt:     now,
		UpdatedAt:     now,
		SchemaVersion: ContractVersion,
	}
}

func testSkill(id SkillID, version string) Skill {
	now := time.Now().UTC()
	return Skill{
		ID:          id,
		Name:        "Test Skill",
		Description: "A skill for testing",
		Version:     version,
		Applicability: Applicability{
			AllowedRoles:         []Role{RoleImplementer},
			AllowedWorkspaces:    []WorkspaceID{"ws-primary"},
			RequiredCapabilities: []Capability{"coding"},
		},
		PromptTemplates: PromptTemplates{
			System: "System instruction for coding.",
			User:   "Context: {{.Context}}\nData: {{.Data}}",
		},
		Capabilities:    []Capability{"coding"},
		RequiresAsk:     false,
		MaxContextChars: 1000,
		SourceURI:       "skills/test-skill",
		Author:          "cortex-admin",
		CreatedAt:       now,
		UpdatedAt:       now,
		SchemaVersion:   SkillContractVersion,
	}
}

func TestSkillValidation(t *testing.T) {
	s := testSkill("skill-valid", "1.0.0")
	if err := s.Validate(); err != nil {
		t.Fatalf("expected valid skill, got: %v", err)
	}

	// Missing ID
	s2 := s
	s2.ID = ""
	if err := s2.Validate(); err == nil || !IsInvalidRequest(err) {
		t.Fatalf("expected invalid request for empty ID, got: %v", err)
	}

	// Unsupported schema version fails closed
	s3 := s
	s3.SchemaVersion = "cortexos.skill.v999"
	if err := s3.Validate(); err == nil || !IsUnsupportedVersion(err) {
		t.Fatalf("expected unsupported version error, got: %v", err)
	}

	// Missing capabilities fails closed
	s4 := s
	s4.Capabilities = nil
	if err := s4.Validate(); err == nil || !IsInvalidRequest(err) {
		t.Fatalf("expected invalid request for nil capabilities, got: %v", err)
	}

	// Negative maxContextChars fails
	s5 := s
	s5.MaxContextChars = -5
	if err := s5.Validate(); err == nil || !IsInvalidRequest(err) {
		t.Fatalf("expected invalid request for negative maxContextChars, got: %v", err)
	}

	// Missing templates fails
	s6 := s
	s6.PromptTemplates = PromptTemplates{}
	if err := s6.Validate(); err == nil || !IsInvalidRequest(err) {
		t.Fatalf("expected invalid request for missing prompt templates, got: %v", err)
	}
}

func TestSkillApplicability(t *testing.T) {
	def := validStaffDefinition()
	s := testSkill("skill-app", "1.0.0")

	// Base matches
	if !s.Applicability.Allows(def) {
		t.Fatalf("expected skill to be applicable to staff")
	}

	// Role mismatch
	sMismatchRole := s
	sMismatchRole.Applicability.AllowedRoles = []Role{RoleReviewer}
	if sMismatchRole.Applicability.Allows(def) {
		t.Fatalf("expected skill to disallow staff with mismatched role")
	}

	// Workspace mismatch
	sMismatchWs := s
	sMismatchWs.Applicability.AllowedWorkspaces = []WorkspaceID{"ws-other"}
	if sMismatchWs.Applicability.Allows(def) {
		t.Fatalf("expected skill to disallow staff with mismatched workspace")
	}

	// Excluded workspace
	sExcludedWs := s
	sExcludedWs.Applicability.ExcludedWorkspaces = []WorkspaceID{"ws-primary"}
	if sExcludedWs.Applicability.Allows(def) {
		t.Fatalf("expected skill to disallow staff in excluded workspace")
	}

	// Missing required capability
	sMissingCap := s
	sMissingCap.Applicability.RequiredCapabilities = []Capability{"hardware_access"}
	if sMissingCap.Applicability.Allows(def) {
		t.Fatalf("expected skill to disallow staff without required capability")
	}
}

func TestPrepareInjectionRedactionAndBounding(t *testing.T) {
	def := validStaffDefinition()
	s := testSkill("skill-sanitize", "1.0.0")
	s.PromptTemplates.System = "Secret key: api_key=topsecret123; Bearer token-xyz; email=alice@example.com."
	s.PromptTemplates.User = "Data: {{.Context}}"
	s.MaxContextChars = 40

	contextData := "Sensitive data with password=hunter2 and admin@company.org"
	injection, err := PrepareInjection(s, def, contextData)
	if err != nil {
		t.Fatalf("PrepareInjection failed: %v", err)
	}

	// Check secrets redacted
	if strings.Contains(injection.Resolved.SystemPrompt, "topsecret123") {
		t.Errorf("system prompt leaked api_key")
	}
	if strings.Contains(injection.Resolved.SystemPrompt, "token-xyz") {
		t.Errorf("system prompt leaked bearer token")
	}
	if strings.Contains(injection.Resolved.SystemPrompt, "alice@example.com") {
		t.Errorf("system prompt leaked email")
	}
	if strings.Contains(injection.Resolved.UserPrompt, "hunter2") {
		t.Errorf("user prompt leaked password")
	}
	if strings.Contains(injection.Resolved.UserPrompt, "admin@company.org") {
		t.Errorf("user prompt leaked email")
	}

	// Check bounding
	if len(injection.Resolved.UserPrompt) > s.MaxContextChars {
		t.Errorf("user prompt length %d exceeds MaxContextChars %d", len(injection.Resolved.UserPrompt), s.MaxContextChars)
	}
	if len(injection.Resolved.SystemPrompt) > s.MaxContextChars/2 {
		t.Errorf("system prompt length %d exceeds MaxContextChars/2 %d", len(injection.Resolved.SystemPrompt), s.MaxContextChars/2)
	}
}

func TestPreventCapabilityEscalation(t *testing.T) {
	def := validStaffDefinition()
	// Def has: coding, test_gen, analysis
	// Create skill requesting root_access capability
	escalationSkill := testSkill("skill-escalate", "1.0.0")
	escalationSkill.Capabilities = []Capability{"coding", "root_access"}
	escalationSkill.Applicability.RequiredCapabilities = []Capability{"coding"}

	store := NewSkillStore()
	if _, err := store.Save(context.Background(), escalationSkill); err != nil {
		t.Fatalf("store.Save failed: %v", err)
	}

	envelope := &harness.ExecutionEnvelope{
		ContractVersion: harness.HarnessContractVersion,
		ExecutionID:     "exec-test-1",
		TaskID:          "task-test-1",
		WorktreeID:      "wt-1",
		ProjectID:       "proj-1",
		ToolName:        "code_edit",
		Input:           json.RawMessage(`{}`),
		PolicyDecision: harness.PolicyDecision{
			Allowed: true,
			Effect:  harness.EffectAllow,
			Reason:  "policy allow",
		},
		Audit: harness.AuditMetadata{
			Actor: "staff-tester",
		},
	}

	// Attempt to inject skill with unauthorized capabilities
	err := InjectSkillIntoEnvelope(envelope, def, store, "some context")
	if err == nil {
		t.Fatalf("expected capability escalation to be blocked, but succeeded")
	}
	if !IsPermissionDenied(err) {
		t.Fatalf("expected ErrPermissionDenied, got: %v", err)
	}
	if envelope.SkillInjection != nil {
		t.Fatalf("skill injection must remain nil when escalation is blocked")
	}
	// Verify policy decision was NOT modified
	if !envelope.PolicyDecision.Allowed || envelope.PolicyDecision.Effect != harness.EffectAllow {
		t.Fatalf("envelope policy decision was corrupted: %+v", envelope.PolicyDecision)
	}
}

func TestInjectSkillIntoEnvelopeSuccess(t *testing.T) {
	def := validStaffDefinition()
	store := NewSkillStore()
	if err := SeedSkillStore(context.Background(), store); err != nil {
		t.Fatalf("SeedSkillStore failed: %v", err)
	}

	envelope := &harness.ExecutionEnvelope{
		ContractVersion: harness.HarnessContractVersion,
		ExecutionID:     "exec-success-1",
		TaskID:          "task-success-1",
		WorktreeID:      "wt-1",
		ProjectID:       "proj-1",
		ToolName:        "coding",
		Input:           json.RawMessage(`{}`),
		PolicyDecision: harness.PolicyDecision{
			Allowed: true,
			Effect:  harness.EffectAllow,
			Reason:  "policy allow",
		},
		Audit: harness.AuditMetadata{
			Actor: "staff-tester",
		},
	}

	err := InjectSkillIntoEnvelope(envelope, def, store, "function calculateTotal()")
	if err != nil {
		t.Fatalf("InjectSkillIntoEnvelope failed: %v", err)
	}

	if envelope.SkillInjection == nil {
		t.Fatalf("expected skill injection to be populated")
	}
	if envelope.SkillInjection.SkillID != "coding-assistant" {
		t.Errorf("expected skillId coding-assistant, got: %s", envelope.SkillInjection.SkillID)
	}
	if !strings.Contains(envelope.SkillInjection.UserPrompt, "function calculateTotal()") {
		t.Errorf("user prompt does not contain injected context")
	}
	// Policy decision remains unchanged
	if !envelope.PolicyDecision.Allowed {
		t.Errorf("policy decision was altered unexpectedly")
	}
}

func TestInjectSkillPathTraversalBlocked(t *testing.T) {
	def := validStaffDefinition()
	maliciousSkill := testSkill("skill-malicious", "1.0.0")
	maliciousSkill.SourceURI = "../../../etc/passwd"

	store := NewSkillStore()
	if _, err := store.Save(context.Background(), maliciousSkill); err != nil {
		t.Fatalf("store.Save failed: %v", err)
	}

	envelope := &harness.ExecutionEnvelope{
		ContractVersion: harness.HarnessContractVersion,
		ExecutionID:     "exec-test-traversal",
		TaskID:          "task-1",
		WorktreeID:      "wt-1",
		ProjectID:       "proj-1",
		ToolName:        "code_edit",
		Input:           json.RawMessage(`{}`),
	}

	err := InjectSkillIntoEnvelope(envelope, def, store, "context")
	if err == nil || !IsInvalidRequest(err) {
		t.Fatalf("expected path traversal in source URI to be rejected, got: %v", err)
	}
	if envelope.SkillInjection != nil {
		t.Fatalf("malicious skill was injected despite path traversal")
	}
}

func TestSkillStoreCRUD(t *testing.T) {
	store := NewSkillStore()
	ctx := context.Background()

	s1 := testSkill("skill-crud", "1.0.0")
	s2 := testSkill("skill-crud", "1.1.0")

	if _, err := store.Save(ctx, s1); err != nil {
		t.Fatalf("Save s1 failed: %v", err)
	}
	if _, err := store.Save(ctx, s2); err != nil {
		t.Fatalf("Save s2 failed: %v", err)
	}

	// Get specific version
	got, err := store.Get(ctx, "skill-crud", "1.0.0")
	if err != nil || got.Version != "1.0.0" {
		t.Fatalf("Get 1.0.0 failed: got=%v, err=%v", got, err)
	}

	// Get latest version
	latest, err := store.Get(ctx, "skill-crud", "latest")
	if err != nil || latest.Version != "1.1.0" {
		t.Fatalf("Get latest failed: got=%v, err=%v", latest, err)
	}

	// List all
	all, err := store.List(ctx)
	if err != nil || len(all) != 2 {
		t.Fatalf("List failed: all=%v, err=%v", all, err)
	}

	// Not found
	_, err = store.Get(ctx, "nonexistent", "latest")
	if err == nil || !IsNotFound(err) {
		t.Fatalf("expected not found for nonexistent skill, got: %v", err)
	}
}
