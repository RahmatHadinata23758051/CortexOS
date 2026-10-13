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
	TaskID string `json:"taskId"`

	// UserPrompt is the raw user instruction.
	UserPrompt string `json:"userPrompt"`

	// StaffID optional: routes to a specific staff system prompt.
	StaffID string `json:"staffId,omitempty"`

	// Config optional: overrides session budgets (timeout, step limit, token limit).
	Config *SessionConfig `json:"config,omitempty"`

	// SchemaVersion identifies the bridge contract version.
	SchemaVersion string `json:"schemaVersion"`
}

// BrainDispatchResult carries the output of a completed brain dispatch.
type BrainDispatchResult struct {
	TaskID        string        `json:"taskId"`
	StaffID       string        `json:"staffId,omitempty"`
	State         string        `json:"state"` // one of: idle, planning, executing, streaming, tool_calling, completed, failed, canceled
	Output        string        `json:"output"`
	Steps         int           `json:"steps"`
	Tokens        int           `json:"tokens"`
	Duration      time.Duration `json:"duration"`
	Error         string        `json:"error,omitempty"`
	SchemaVersion string        `json:"schemaVersion"`
}

// StreamChunkHandler processes a single streaming chunk from the brain.
// Implementations must be fast and non-blocking; heavy work should be delegated.
type StreamChunkHandler func(chunk StreamChunk)

// StreamChunk represents a token/state delta during streaming.
type StreamChunk struct {
	// TaskID correlates the chunk to its dispatch.
	TaskID string `json:"taskId"`

	// Type: "token" | "tool_call" | "state_change" | "done" | "error"
	Type string `json:"type"`

	// Data holds the payload: token text, tool call JSON, state name, or error message.
	Data string `json:"data,omitempty"`

	// Sequence is a monotonically increasing number for ordering.
	Sequence int `json:"sequence"`

	// Timestamp when the chunk was produced.
	Timestamp time.Time `json:"timestamp"`
}

// SessionConfig mirrors brain.SessionConfig for the bridge boundary.
type SessionConfig struct {
	// Timeout is the maximum wall-clock time for the entire session.
	Timeout time.Duration `json:"timeout,omitempty"`

	// StepLimit is the maximum number of reasoning steps.
	StepLimit int `json:"stepLimit,omitempty"`

	// MaxTokens is the maximum total tokens (prompt + completion).
	MaxTokens int `json:"maxTokens,omitempty"`
}

// BrainBridgeContract identifies the schema contract for the brain bridge.
const BrainBridgeContract = "cortexos.brain.bridge.v1"
