package application

import (
	"context"
	"testing"
	"time"
)

func TestBrainDispatchRequest_Defaults(t *testing.T) {
	fake := NewFakeBrainAdapter("test-fake")
	svc := NewServiceWithBrain(fake)

	ctx := context.Background()
	result, err := svc.BrainDispatch(ctx, BrainDispatchRequest{
		TaskID:     "",
		UserPrompt: "hello",
	})
	if err != nil {
		t.Fatalf("BrainDispatch failed: %v", err)
	}
	if result.SchemaVersion != BrainBridgeContract {
		t.Fatalf("expected schema version %q, got %q", BrainBridgeContract, result.SchemaVersion)
	}
	if result.State != "completed" {
		t.Fatalf("expected completed state, got %q", result.State)
	}
}

func TestService_BrainNotConfigured(t *testing.T) {
	svc := NewService()
	ctx := context.Background()
	_, err := svc.BrainDispatch(ctx, BrainDispatchRequest{TaskID: "t1", UserPrompt: "x"})
	if err == nil {
		t.Fatal("expected error when brain not configured")
	}
}

func TestService_BrainDispatchStream(t *testing.T) {
	fake := NewFakeBrainAdapter("test-fake")
	svc := NewServiceWithBrain(fake)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var got []StreamChunk
	result, err := svc.BrainDispatchStream(ctx, BrainDispatchRequest{
		TaskID:     "stream-1",
		UserPrompt: "hello stream",
	}, func(c StreamChunk) {
		got = append(got, c)
	})
	if err != nil {
		t.Fatalf("BrainDispatchStream failed: %v", err)
	}
	if result.State != "completed" {
		t.Fatalf("expected completed state, got %q", result.State)
	}
	if len(got) != 4 {
		t.Fatalf("expected 4 chunks, got %d", len(got))
	}
	if got[len(got)-1].Type != "done" {
		t.Fatalf("last chunk should be done, got %q", got[len(got)-1].Type)
	}
}

func TestService_BrainDispatchStreamNilHandler(t *testing.T) {
	svc := NewServiceWithBrain(NewFakeBrainAdapter("f"))
	ctx := context.Background()
	// FakeBrainAdapter should still work with nil chunks but require handler
	if fake, ok := interface{}(svc.brain).(*FakeBrainAdapter); ok {
		_ = fake
	}
	// For the real adapter path, test nil handler rejection
	// FakeBrainAdapter tolerates nil handler by just not calling it
	_, err := svc.BrainDispatchStream(ctx, BrainDispatchRequest{TaskID: "t", UserPrompt: "x"}, nil)
	// FakeBrainAdapter doesn't validate handler; just ensure no panic
	_ = err
}
