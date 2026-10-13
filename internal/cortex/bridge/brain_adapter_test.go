package bridge

import (
	"context"
	"testing"
	"time"

	"github.com/RahmatHadinata23758051/CortexOS/internal/cortex/application"
	"github.com/RahmatHadinata23758051/CortexOS/internal/cortex/brain"
	"github.com/RahmatHadinata23758051/CortexOS/internal/cortex/provider"
)

func TestRealBrainAdapter_Dispatch(t *testing.T) {
	fake := provider.NewFakeProvider("fake-provider")
	resilient, err := brain.NewResilientDispatcher(application.NewProviderAdapter(fake), brain.DefaultRetryPolicy())
	if err != nil {
		t.Fatal(err)
	}

	adapter, err := NewRealBrainAdapter(resilient)
	if err != nil {
		t.Fatal(err)
	}

	if adapter.Name() != "fake-provider" {
		t.Fatalf("adapter name = %q, want fake-provider", adapter.Name())
	}

	ctx := context.Background()
	result, err := adapter.Dispatch(ctx, application.BrainDispatchRequest{
		TaskID:     "adapter-task-1",
		UserPrompt: "test prompt",
	})
	if err != nil {
		t.Fatalf("Dispatch failed: %v", err)
	}
	if result.State != "completed" {
		t.Fatalf("result state = %q, want completed", result.State)
	}
	if result.SchemaVersion != application.BrainBridgeContract {
		t.Fatalf("schema = %q, want %q", result.SchemaVersion, application.BrainBridgeContract)
	}
}

func TestRealBrainAdapter_DispatchStream(t *testing.T) {
	fake := provider.NewFakeProvider("stream-fake")
	resilient, err := brain.NewResilientDispatcher(application.NewProviderAdapter(fake), brain.DefaultRetryPolicy())
	if err != nil {
		t.Fatal(err)
	}

	adapter, err := NewRealBrainAdapter(resilient)
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var chunks []application.StreamChunk
	result, err := adapter.DispatchStream(ctx, application.BrainDispatchRequest{
		TaskID:     "adapter-stream-1",
		UserPrompt: "stream prompt",
	}, func(c application.StreamChunk) {
		chunks = append(chunks, c)
	})
	if err != nil {
		t.Fatalf("DispatchStream failed: %v", err)
	}
	if result.State != "completed" {
		t.Fatalf("result state = %q, want completed", result.State)
	}
	if len(chunks) == 0 {
		t.Fatal("expected at least one stream chunk")
	}
	if chunks[len(chunks)-1].Type != "done" {
		t.Fatalf("expected last chunk done, got %q", chunks[len(chunks)-1].Type)
	}
}

func TestRealBrainAdapter_NilDispatcher(t *testing.T) {
	if _, err := NewRealBrainAdapter(nil); err == nil {
		t.Fatal("expected error for nil dispatcher")
	}
}

func TestRealBrainAdapter_NilHandler(t *testing.T) {
	fake := provider.NewFakeProvider("f")
	resilient, _ := brain.NewResilientDispatcher(application.NewProviderAdapter(fake), brain.DefaultRetryPolicy())
	adapter, _ := NewRealBrainAdapter(resilient)
	if _, err := adapter.DispatchStream(context.Background(), application.BrainDispatchRequest{TaskID: "t", UserPrompt: "u"}, nil); err == nil {
		t.Fatal("expected error for nil stream handler")
	}
}
