package knowledge

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

func capturedItem(t *testing.T, svc *Service) Item {
	t.Helper()
	item, err := svc.Capture(context.Background(), CaptureRequest{
		WorkspaceID: "ws-main",
		ProjectID:   "proj-core",
		Title:       "Deployment decision",
		Content:     "Use the governed deployment workflow.",
		Kind:        KindDecision,
		Tags:        []string{"release"},
		Provenance: Provenance{
			SourceKind:  "worker_output",
			SourceURI:   "task/task-1",
			Author:      "staff-implementer",
			TaskID:      "task-1",
			ExecutionID: "execution-1",
			CapturedAt:  time.Now().UTC(),
		},
	})
	if err != nil {
		t.Fatalf("capture failed: %v", err)
	}
	return item
}

func TestCaptureCreatesUnvalidatedDraft(t *testing.T) {
	store := NewMemoryStore()
	svc, err := NewService(store)
	if err != nil {
		t.Fatalf("NewService failed: %v", err)
	}

	item := capturedItem(t, svc)
	if item.Lifecycle != LifecycleDraft {
		t.Fatalf("expected draft lifecycle, got %s", item.Lifecycle)
	}
	if item.ValidationStatus != StatusUnvalidated {
		t.Fatalf("expected unvalidated status, got %s", item.ValidationStatus)
	}
	if item.ContentHash != HashContent(item.Content) {
		t.Fatalf("content hash was not canonicalized")
	}
	if item.Provenance.SourceKind != "worker_output" || item.Provenance.ExecutionID != "execution-1" {
		t.Fatalf("provenance was not preserved: %+v", item.Provenance)
	}
}

func TestUnvalidatedKnowledgeCannotBecomeActiveViaStore(t *testing.T) {
	store := NewMemoryStore()
	now := time.Now().UTC()
	item := Item{
		ID: "kno-untrusted", WorkspaceID: "ws", ProjectID: "proj", Title: "Claim", Content: "claim",
		Kind: KindReference, Lifecycle: LifecycleActive, ValidationStatus: StatusUnvalidated,
		ContentHash: HashContent("claim"), Version: 1, CreatedAt: now, UpdatedAt: now,
		SchemaVersion: ContractVersion,
		Provenance:    Provenance{SourceKind: "worker_output", Author: "worker", CapturedAt: now},
	}
	if _, err := store.Save(context.Background(), item); err == nil || !IsUntrustedClaim(err) {
		t.Fatalf("expected untrusted claim rejection, got %v", err)
	}
}

func TestValidatePromotesAndRecordsAuthority(t *testing.T) {
	store := NewMemoryStore()
	svc, _ := NewService(store)
	item := capturedItem(t, svc)

	validated, err := svc.Validate(context.Background(), item.ID, "inspector", "evidence matched acceptance criteria")
	if err != nil {
		t.Fatalf("Validate failed: %v", err)
	}
	if validated.Lifecycle != LifecycleActive || validated.ValidationStatus != StatusVerified {
		t.Fatalf("unexpected validated state: %+v", validated)
	}
	if validated.Provenance.ValidatedBy != "inspector" || validated.Provenance.ValidationReason == "" || validated.Provenance.ValidatedAt == nil {
		t.Fatalf("validation provenance missing: %+v", validated.Provenance)
	}
}

func TestRejectPreventsActivePromotion(t *testing.T) {
	store := NewMemoryStore()
	svc, _ := NewService(store)
	item := capturedItem(t, svc)

	rejected, err := svc.Reject(context.Background(), item.ID, "inspector", "unsupported claim")
	if err != nil {
		t.Fatalf("Reject failed: %v", err)
	}
	if rejected.ValidationStatus != StatusRejected || rejected.Lifecycle != LifecycleDraft {
		t.Fatalf("unexpected rejected state: %+v", rejected)
	}
	// A rejected item cannot be promoted to Active - validation should fail
	if _, err := svc.Validate(context.Background(), item.ID, "inspector", "retry"); err == nil || !IsInvalidRequest(err) {
		t.Fatalf("expected rejected item promotion to be blocked, got %v", err)
	}
}

func TestUpdateResetsValidationAndIncrementsVersion(t *testing.T) {
	store := NewMemoryStore()
	svc, _ := NewService(store)
	item := capturedItem(t, svc)
	item, err := svc.Validate(context.Background(), item.ID, "orchestra", "approved")
	if err != nil {
		t.Fatalf("Validate failed: %v", err)
	}

	updated, err := svc.Update(context.Background(), UpdateRequest{
		ID:      item.ID,
		Content: "Updated deployment decision.",
		Provenance: Provenance{
			SourceKind: "user_input",
			Author:     "maintainer",
		},
	})
	if err != nil {
		t.Fatalf("Update failed: %v", err)
	}
	if updated.Version != item.Version+1 || updated.Lifecycle != LifecycleDraft || updated.ValidationStatus != StatusUnvalidated {
		t.Fatalf("update did not reset governance state: %+v", updated)
	}
	if updated.ContentHash != HashContent(updated.Content) || updated.Provenance.Author != "maintainer" {
		t.Fatalf("update metadata was not applied: %+v", updated)
	}
}

func TestCorrectionPreservesFullProvenance(t *testing.T) {
	store := NewMemoryStore()
	svc, _ := NewService(store)
	item := capturedItem(t, svc)

	corrected, err := svc.Correct(context.Background(), CorrectionRequest{
		ID:      item.ID,
		Content: "Corrected decision.",
		Reason:  "review found an obsolete statement",
		Author:  "maintainer",
		License: "Apache-2.0",
		Provenance: Provenance{
			SourceKind: "correction",
			SourceURI:  "review/rev-1",
		},
	})
	if err != nil {
		t.Fatalf("Correct failed: %v", err)
	}
	if corrected.Version != 2 || corrected.ContentHash != HashContent(corrected.Content) {
		t.Fatalf("correction version/hash invalid: %+v", corrected)
	}
	if corrected.Provenance.Author != "maintainer" || corrected.Provenance.License != "Apache-2.0" {
		t.Fatalf("correction attribution was not preserved: %+v", corrected.Provenance)
	}
	if !strings.Contains(corrected.Provenance.ValidationReason, "obsolete statement") {
		t.Fatalf("correction reason missing: %q", corrected.Provenance.ValidationReason)
	}
}

func TestDeleteTombstonesAndListScopes(t *testing.T) {
	store := NewMemoryStore()
	svc, _ := NewService(store)
	first := capturedItem(t, svc)
	second, err := svc.Capture(context.Background(), CaptureRequest{
		WorkspaceID: "ws-other", ProjectID: "proj-other", Title: "Other", Content: "other", Kind: KindReference,
		Provenance: Provenance{SourceKind: "vault_note", Author: "user", CapturedAt: time.Now().UTC()},
	})
	if err != nil {
		t.Fatalf("second capture failed: %v", err)
	}

	workspace := WorkspaceID("ws-main")
	items, err := svc.List(context.Background(), Filter{WorkspaceID: &workspace})
	if err != nil || len(items) != 1 || items[0].ID != first.ID {
		t.Fatalf("workspace filter failed: items=%v err=%v", items, err)
	}
	if err := store.Delete(context.Background(), first.ID); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}
	deleted, err := svc.Get(context.Background(), first.ID)
	if err != nil {
		t.Fatalf("tombstone should remain readable: %v", err)
	}
	if deleted.Lifecycle != LifecycleTombstone {
		t.Fatalf("expected tombstone lifecycle, got %s", deleted.Lifecycle)
	}
	active := true
	items, err = svc.List(context.Background(), Filter{ActiveOnly: active})
	if err != nil {
		t.Fatalf("active list failed: %v", err)
	}
	for _, got := range items {
		if got.ID == first.ID || got.ID != second.ID {
			t.Fatalf("tombstoned item leaked from active list: %v", items)
		}
	}
}

func TestExpiryPruneAndCancellation(t *testing.T) {
	store := NewMemoryStore()
	svc, _ := NewService(store)
	expiry := time.Now().UTC().Add(10 * time.Millisecond)
	item, err := svc.Capture(context.Background(), CaptureRequest{
		WorkspaceID: "ws", ProjectID: "proj", Title: "Expiring", Content: "old", Kind: KindReference,
		ExpiresAt:  &expiry,
		Provenance: Provenance{SourceKind: "vault_note", Author: "user", CapturedAt: time.Now().UTC()},
	})
	if err != nil {
		t.Fatalf("capture failed: %v", err)
	}
	time.Sleep(20 * time.Millisecond)
	count, err := svc.PruneExpired(context.Background())
	if err != nil || count != 1 {
		t.Fatalf("expected one expired item, count=%d err=%v", count, err)
	}
	got, _ := svc.Get(context.Background(), item.ID)
	if got.Lifecycle != LifecycleExpired {
		t.Fatalf("expected expired lifecycle, got %s", got.Lifecycle)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = svc.List(ctx, Filter{})
	if err == nil || !errors.Is(err, context.Canceled) || !IsCanceled(err) {
		t.Fatalf("expected cancellation, got %v", err)
	}
}

func TestItemRedactsSensitiveContent(t *testing.T) {
	item := Item{Content: "email user@example.com api_key=abc123 Bearer token_xyz"}
	redacted := item.RedactedContent()
	for _, token := range []string{"user@example.com", "abc123", "token_xyz"} {
		if strings.Contains(redacted, token) {
			t.Fatalf("sensitive token leaked in redacted content: %q", redacted)
		}
	}
	if !strings.Contains(redacted, "[EMAIL]") || !strings.Contains(redacted, "[REDACTED]") {
		t.Fatalf("redaction markers missing: %q", redacted)
	}
}
