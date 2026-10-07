package platform

import (
	"context"
	"testing"

	"github.com/RahmatHadinata23758051/CortexOS/internal/cortex/application"
)

type harnessPortStub struct{}

func (harnessPortStub) ListCapabilities(context.Context) ([]application.HarnessCapabilityRecord, error) {
	return []application.HarnessCapabilityRecord{{Capability: "shell"}}, nil
}
func (harnessPortStub) ListWorkers(context.Context) ([]application.HarnessWorkerRecord, error) {
	return []application.HarnessWorkerRecord{{ID: "w-1", Status: "healthy"}}, nil
}
func (harnessPortStub) ListActiveExecutions(context.Context) ([]application.HarnessExecutionRecord, error) {
	return []application.HarnessExecutionRecord{{ID: "e-1", TaskID: "t-1"}}, nil
}
func (harnessPortStub) ListEvidence(context.Context, string) ([]application.HarnessEvidenceRecord, error) {
	return []application.HarnessEvidenceRecord{{ID: "ev-1", ExecutionID: "e-1"}}, nil
}

func TestBridgeHarnessCapabilities(t *testing.T) {
	bridge := NewBridge(application.NewServiceWithHarness(harnessPortStub{}))
	res, err := bridge.ListHarnessCapabilities(application.HarnessCapabilitiesRequest{SchemaVersion: application.HarnessBridgeSchemaVersion})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(res) != 1 || res[0].Capability != "shell" {
		t.Fatalf("unexpected result: %+v", res)
	}
}

func TestBridgeHarnessWorkers(t *testing.T) {
	bridge := NewBridge(application.NewServiceWithHarness(harnessPortStub{}))
	res, err := bridge.ListHarnessWorkers(application.HarnessWorkersRequest{SchemaVersion: application.HarnessBridgeSchemaVersion})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(res) != 1 || res[0].ID != "w-1" {
		t.Fatalf("unexpected result: %+v", res)
	}
}

func TestBridgeHarnessActiveExecutions(t *testing.T) {
	bridge := NewBridge(application.NewServiceWithHarness(harnessPortStub{}))
	res, err := bridge.ListActiveHarnessExecutions(application.HarnessActiveExecutionsRequest{SchemaVersion: application.HarnessBridgeSchemaVersion})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(res) != 1 || res[0].ID != "e-1" {
		t.Fatalf("unexpected result: %+v", res)
	}
}

func TestBridgeHarnessEvidence(t *testing.T) {
	bridge := NewBridge(application.NewServiceWithHarness(harnessPortStub{}))
	res, err := bridge.GetHarnessEvidence(application.HarnessEvidenceRequest{SchemaVersion: application.HarnessBridgeSchemaVersion, ExecutionID: "e-1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(res) != 1 || res[0].ID != "ev-1" {
		t.Fatalf("unexpected result: %+v", res)
	}
}

func TestBridgeHarnessContextCancellation(t *testing.T) {
	bridge := NewBridge(application.NewServiceWithHarness(harnessPortStub{}))
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	bridge = bridge.WithContext(ctx)
	_, err := bridge.ListHarnessWorkers(application.HarnessWorkersRequest{SchemaVersion: application.HarnessBridgeSchemaVersion})
	if err == nil {
		t.Fatal("expected cancellation error")
	}
	if got := err.Error(); got != "harness.canceled: harness request canceled" {
		t.Fatalf("unexpected cancellation error: %q", got)
	}
}
