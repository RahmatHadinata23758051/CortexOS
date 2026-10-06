package platform

import (
	"testing"

	"github.com/RahmatHadinata23758051/CortexOS/internal/cortex/application"
)

func TestBridgeReturnsApplicationSnapshot(t *testing.T) {
	bridge := NewBridge(application.NewService())

	got, err := bridge.GetRuntimeSnapshot(application.SnapshotRequest{
		SchemaVersion: application.RuntimeSchemaVersion(),
	})
	if err != nil {
		t.Fatalf("GetRuntimeSnapshot returned error: %v", err)
	}
	if got.Status != "ready" || got.Provider != "disabled" {
		t.Fatalf("GetRuntimeSnapshot = %#v, want ready/disabled", got)
	}
}
