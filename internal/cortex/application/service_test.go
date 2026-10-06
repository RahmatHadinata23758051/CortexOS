package application

import (
	"context"
	"errors"
	"testing"
)

func TestGetRuntimeSnapshotReturnsDeterministicLocalStatus(t *testing.T) {
	service := NewService()

	got, err := service.GetRuntimeSnapshot(context.Background(), SnapshotRequest{
		SchemaVersion: RuntimeSchemaVersion(),
	})
	if err != nil {
		t.Fatalf("GetRuntimeSnapshot returned error: %v", err)
	}

	want := RuntimeSnapshot{
		SchemaVersion: "cortexos.runtime.v1",
		Status:        "ready",
		Environment:   "local",
		Provider:      "disabled",
	}
	if got != want {
		t.Fatalf("GetRuntimeSnapshot = %#v, want %#v", got, want)
	}
}

func TestGetRuntimeSnapshotRejectsUnsupportedSchema(t *testing.T) {
	service := NewService()

	_, err := service.GetRuntimeSnapshot(context.Background(), SnapshotRequest{
		SchemaVersion: "unsupported",
	})
	if !errors.Is(err, ErrInvalidRequest) {
		t.Fatalf("error = %v, want ErrInvalidRequest", err)
	}
}

func TestGetRuntimeSnapshotHonorsCanceledContext(t *testing.T) {
	service := NewService()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := service.GetRuntimeSnapshot(ctx, SnapshotRequest{
		SchemaVersion: RuntimeSchemaVersion(),
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v, want context.Canceled", err)
	}
}
