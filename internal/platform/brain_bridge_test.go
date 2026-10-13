package platform

import (
	"testing"

	"github.com/RahmatHadinata23758051/CortexOS/internal/cortex/application"
)

func TestBridgeDispatchBrainTaskEmitsState(t *testing.T) {
	service := application.NewServiceWithBrain(application.NewFakeBrainAdapter("fake"))
	bridge := NewBridge(service)

	var eventName string
	var payload any
	unsubscribe := bridge.RegisterEventHook(func(name string, value any) {
		eventName = name
		payload = value
	})
	defer unsubscribe()

	result, err := bridge.DispatchBrainTask(application.BrainDispatchRequest{
		TaskID:     "bridge-1",
		UserPrompt: "hello",
	})
	if err != nil {
		t.Fatalf("DispatchBrainTask returned error: %v", err)
	}
	if result.State != "completed" {
		t.Fatalf("result state = %q, want completed", result.State)
	}
	if eventName != EventBrainStateChanged {
		t.Fatalf("event = %q, want %q", eventName, EventBrainStateChanged)
	}
	if payload == nil {
		t.Fatal("expected state event payload")
	}
}

func TestBridgeDispatchBrainTaskStreamEmitsChunksAndFinalState(t *testing.T) {
	service := application.NewServiceWithBrain(application.NewFakeBrainAdapter("fake"))
	bridge := NewBridge(service)

	var events []string
	unsubscribe := bridge.RegisterEventHook(func(name string, payload any) {
		events = append(events, name)
	})
	defer unsubscribe()

	result, err := bridge.DispatchBrainTaskStream(application.BrainDispatchRequest{
		TaskID:     "bridge-stream-1",
		UserPrompt: "stream hello",
	})
	if err != nil {
		t.Fatalf("DispatchBrainTaskStream returned error: %v", err)
	}
	if result.State != "completed" {
		t.Fatalf("result state = %q, want completed", result.State)
	}
	if len(events) != 5 {
		t.Fatalf("event count = %d, want 5 chunks + final state", len(events))
	}
	for i := 0; i < 4; i++ {
		if events[i] != EventBrainStreamChunk {
			t.Fatalf("event %d = %q, want stream chunk", i, events[i])
		}
	}
	if events[4] != EventBrainStateChanged {
		t.Fatalf("final event = %q, want state changed", events[4])
	}
}

func TestBridgeDispatchBrainTaskDisabled(t *testing.T) {
	bridge := NewBridge(application.NewService())
	if _, err := bridge.DispatchBrainTask(application.BrainDispatchRequest{
		TaskID:     "disabled-1",
		UserPrompt: "hello",
	}); err == nil {
		t.Fatal("expected error when brain is disabled")
	}
}
