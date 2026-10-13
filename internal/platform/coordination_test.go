package platform

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/RahmatHadinata23758051/CortexOS/internal/cortex/application"
)

type platformCoordinationPort struct {
	snapshot application.CoordinationSnapshot
}

func (p *platformCoordinationPort) GetCoordinationSnapshot(_ context.Context, _ application.CoordinationSnapshotRequest) (application.CoordinationSnapshot, error) {
	return p.snapshot, nil
}

func sampleCoordinationSnapshot() application.CoordinationSnapshot {
	return application.CoordinationSnapshot{
		SchemaVersion: application.CoordinationSchemaVersion,
		ProjectID:     "proj-a",
		ActivePhase:   1,
		TotalPhases:   2,
		Tasks: []application.CoordinationTask{
			{ID: "task-root", ProjectID: "proj-a", Title: "Plan sprint", Status: "success", Phase: 0, Progress: 100},
			{ID: "task-build", ProjectID: "proj-a", Title: "Build assets", Status: "running", Phase: 1, Progress: 50, AssignedStaffID: "staff-dev"},
		},
		Assignments: []application.StaffAssignmentStatus{
			{StaffID: "staff-dev", Name: "Dev", Role: "implementer", State: "running"},
		},
		Metrics:     application.CoordinationMetrics{TotalTasks: 2, RunningTasks: 1, CompletedTasks: 1, TotalStaffCount: 1, ActiveStaffCount: 1},
		GeneratedAt: time.Unix(1, 0).UTC(),
	}
}

func TestBridgeGetCoordinationSnapshotEmitsChanged(t *testing.T) {
	service := application.NewService()
	service.SetCoordination(&platformCoordinationPort{snapshot: sampleCoordinationSnapshot()})
	b := NewBridge(service)

	var names []string
	var payloads []any
	unsubscribe := b.RegisterEventHook(func(name string, payload any) {
		names = append(names, name)
		payloads = append(payloads, payload)
	})
	defer unsubscribe()

	snapshot, err := b.GetCoordinationSnapshot(application.CoordinationSnapshotRequest{
		SchemaVersion: application.CoordinationSchemaVersion,
		ProjectID:     "proj-a",
	})
	if err != nil {
		t.Fatalf("GetCoordinationSnapshot: %v", err)
	}
	if len(snapshot.Tasks) != 2 {
		t.Fatalf("expected 2 tasks, got %d", len(snapshot.Tasks))
	}
	if len(names) != 1 || names[0] != EventCoordinationChanged {
		t.Fatalf("events = %#v, want coordination:changed", names)
	}
	for _, payload := range payloads {
		encoded := strings.TrimSpace(string(mustJSONCoordination(t, payload)))
		for _, forbidden := range []string{`C:\\Users`, "/home/", "/etc/", "apiKey", "processHandle", "providerOutput"} {
			if strings.Contains(encoded, forbidden) {
				t.Fatalf("coordination payload leaked %q: %s", forbidden, encoded)
			}
		}
	}
}

func TestBridgeEmitCoordinationEventMapsTypes(t *testing.T) {
	b := NewBridge(application.NewService())
	cases := map[string]string{
		"assigned":   EventCoordinationAssigned,
		"reassigned": EventCoordinationReassigned,
		"progress":   EventCoordinationProgress,
		"blocked":    EventCoordinationBlocked,
		"completed":  EventCoordinationCompleted,
		"canceled":   EventCoordinationCanceled,
	}
	for eventType, wantName := range cases {
		var gotName string
		unsubscribe := b.RegisterEventHook(func(name string, _ any) { gotName = name })
		payload := application.CoordinationEventPayload{
			SchemaVersion: application.CoordinationSchemaVersion,
			EventType:     eventType,
			TaskID:        "task-1",
			Status:        "running",
			Timestamp:     time.Unix(1, 0).UTC(),
		}
		if err := b.EmitCoordinationEvent(payload); err != nil {
			t.Fatalf("EmitCoordinationEvent(%s): %v", eventType, err)
		}
		unsubscribe()
		if gotName != wantName {
			t.Fatalf("event %s mapped to %q, want %q", eventType, gotName, wantName)
		}
	}
}

func TestBridgeEmitCoordinationEventRejectsMalformedPayload(t *testing.T) {
	b := NewBridge(application.NewService())
	called := false
	unsubscribe := b.RegisterEventHook(func(string, any) { called = true })
	defer unsubscribe()

	// Unsupported schema version must fail closed without emission.
	err := b.EmitCoordinationEvent(application.CoordinationEventPayload{
		SchemaVersion: "bad.version",
		EventType:     "assigned",
		TaskID:        "task-1",
	})
	if err == nil {
		t.Fatalf("expected validation error for unsupported schema")
	}
	if called {
		t.Fatalf("malformed payload must not emit an event")
	}

	// Unknown event type validates but maps to no event name.
	if err := b.EmitCoordinationEvent(application.CoordinationEventPayload{
		SchemaVersion: application.CoordinationSchemaVersion,
		EventType:     "unknown",
		TaskID:        "task-1",
	}); err != nil {
		t.Fatalf("unexpected error for unknown type: %v", err)
	}
	if called {
		t.Fatalf("unknown event type must not emit an event")
	}
}

func TestBridgeCoordinationUnwiredFailsClosed(t *testing.T) {
	b := NewBridge(application.NewService())
	if _, err := b.GetCoordinationSnapshot(application.CoordinationSnapshotRequest{
		SchemaVersion: application.CoordinationSchemaVersion,
	}); err == nil {
		t.Fatalf("expected error when coordination is not configured")
	}
}

func mustJSONCoordination(t *testing.T, value any) []byte {
	t.Helper()
	encoded, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	return encoded
}
