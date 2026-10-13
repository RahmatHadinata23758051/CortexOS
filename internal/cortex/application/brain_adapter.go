package application

import (
	"context"
	"sync"
	"time"
)

// FakeBrainAdapter is a test double for BrainPort.
type FakeBrainAdapter struct {
	name    string
	results map[string]BrainDispatchResult
	streams map[string][]StreamChunk
	mu      sync.Mutex
}

func NewFakeBrainAdapter(name string) *FakeBrainAdapter {
	return &FakeBrainAdapter{
		name:    name,
		results: make(map[string]BrainDispatchResult),
		streams: make(map[string][]StreamChunk),
	}
}

func (f *FakeBrainAdapter) Name() string { return f.name }

func (f *FakeBrainAdapter) Dispatch(ctx context.Context, req BrainDispatchRequest) (BrainDispatchResult, error) {
	if err := ctx.Err(); err != nil {
		return BrainDispatchResult{}, err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if r, ok := f.results[req.TaskID]; ok {
		return r, nil
	}
	return BrainDispatchResult{
		TaskID:        req.TaskID,
		StaffID:       req.StaffID,
		State:         "completed",
		Output:        "fake response for " + req.UserPrompt,
		Steps:         1,
		Tokens:        10,
		Duration:      time.Millisecond,
		SchemaVersion: BrainBridgeContract,
	}, nil
}

func (f *FakeBrainAdapter) DispatchStream(ctx context.Context, req BrainDispatchRequest, handler StreamChunkHandler) (BrainDispatchResult, error) {
	f.mu.Lock()
	chunks := f.streams[req.TaskID]
	f.mu.Unlock()

	if len(chunks) == 0 {
		chunks = []StreamChunk{
			{TaskID: req.TaskID, Type: "token", Data: "fake ", Sequence: 1, Timestamp: time.Now()},
			{TaskID: req.TaskID, Type: "token", Data: "streaming ", Sequence: 2, Timestamp: time.Now()},
			{TaskID: req.TaskID, Type: "token", Data: "response", Sequence: 3, Timestamp: time.Now()},
			{TaskID: req.TaskID, Type: "done", Data: "", Sequence: 4, Timestamp: time.Now()},
		}
	}

	for _, c := range chunks {
		select {
		case <-ctx.Done():
			return BrainDispatchResult{}, ctx.Err()
		default:
		}
		if handler != nil {
			handler(c)
		}
	}

	return BrainDispatchResult{
		TaskID:        req.TaskID,
		StaffID:       req.StaffID,
		State:         "completed",
		Output:        "fake streaming response",
		Steps:         1,
		Tokens:        10,
		Duration:      time.Millisecond,
		SchemaVersion: BrainBridgeContract,
	}, nil
}

// SetResult configures a canned response for a task ID (for testing).
func (f *FakeBrainAdapter) SetResult(taskID string, result BrainDispatchResult) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.results[taskID] = result
}

// SetStream configures canned streaming chunks for a task ID (for testing).
func (f *FakeBrainAdapter) SetStream(taskID string, chunks []StreamChunk) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.streams[taskID] = chunks
}
