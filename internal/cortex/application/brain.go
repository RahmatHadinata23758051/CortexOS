package application

import (
	"context"
	"time"
)

// BrainPort is the application boundary for the Brain runtime.
// It isolates the UI/bridge from provider, persistence, and process details.
type BrainPort interface {
	// Dispatch executes a brain task and returns the final result.
	// The context controls the overall deadline for the dispatch.
	Dispatch(ctx context.Context, req BrainDispatchRequest) (BrainDispatchResult, error)

	// DispatchStream executes a brain task and emits streaming events
	// via the provided handler. The handler must not block for long.
	DispatchStream(ctx context.Context, req BrainDispatchRequest, handler StreamChunkHandler) (BrainDispatchResult, error)

	// Name returns the provider name for status reporting.
	Name() string
}

// BrainDispatchRequest carries the input for a brain dispatch.
type BrainDispatchRequest struct {
	// TaskID is a unique identifier for this dispatch (for tracing/correlation).
	TaskID string

	// UserPrompt is the raw user instruction.
	UserPrompt string

	// StaffID optional: routes to a specific staff system prompt.
	StaffID string

	// Config optional: overrides session budgets (timeout, step limit, token limit).
	Config *SessionConfig

	// SchemaVersion identifies the bridge contract version.
	SchemaVersion string
}

// BrainDispatchResult carries the output of a completed brain dispatch.
type BrainDispatchResult struct {
	TaskID        string
	StaffID       string
	State         string // one of: idle, planning, executing, streaming, tool_calling, completed, failed, canceled
	Output        string
	Steps         int
	Tokens        int
	Duration      time.Duration
	Error         string
	SchemaVersion string
}

// StreamChunkHandler processes a single streaming chunk from the brain.
// Implementations must be fast and non-blocking; heavy work should be delegated.
type StreamChunkHandler func(chunk StreamChunk)

// StreamChunk represents a token/state delta during streaming.
type StreamChunk struct {
	// TaskID correlates the chunk to its dispatch.
	TaskID string

	// Type: "token" | "tool_call" | "state_change" | "done" | "error"
	Type string

	// Data holds the payload: token text, tool call JSON, state name, or error message.
	Data string

	// Sequence is a monotonically increasing number for ordering.
	Sequence int

	// Timestamp when the chunk was produced.
	Timestamp time.Time
}

// SessionConfig mirrors brain.SessionConfig for the bridge boundary.
type SessionConfig struct {
	// Timeout is the maximum wall-clock time for the entire session.
	Timeout time.Duration

	// StepLimit is the maximum number of reasoning steps.
	StepLimit int

	// MaxTokens is the maximum total tokens (prompt + completion).
	MaxTokens int
}

// BrainBridgeContract identifies the schema contract for the brain bridge.
const BrainBridgeContract = "cortexos.brain.bridge.v1"
