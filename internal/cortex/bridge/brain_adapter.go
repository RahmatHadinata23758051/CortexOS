// Package bridge wires the brain runtime into the application boundary.
// It lives outside application and brain to avoid an import cycle while
// keeping the real dispatch logic typed and testable.
package bridge

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/RahmatHadinata23758051/CortexOS/internal/cortex/application"
	"github.com/RahmatHadinata23758051/CortexOS/internal/cortex/brain"
)

// RealBrainAdapter implements application.BrainPort using the brain runtime.
type RealBrainAdapter struct {
	dispatcher *brain.ResilientDispatcher
	name       string
}

// NewRealBrainAdapter creates an application adapter for the brain runtime.
// It wraps a ResilientDispatcher for retry/recovery and manages session lifecycle.
func NewRealBrainAdapter(dispatcher *brain.ResilientDispatcher) (application.BrainPort, error) {
	if dispatcher == nil {
		return nil, errors.New("dispatcher cannot be nil")
	}
	return &RealBrainAdapter{
		dispatcher: dispatcher,
		name:       dispatcher.ProviderName(),
	}, nil
}

// Name returns the provider name for status reporting.
func (a *RealBrainAdapter) Name() string {
	return a.name
}

// Dispatch executes a complete (non-streaming) brain task.
func (a *RealBrainAdapter) Dispatch(ctx context.Context, req application.BrainDispatchRequest) (application.BrainDispatchResult, error) {
	if req.TaskID == "" {
		req.TaskID = fmt.Sprintf("brain-%d", time.Now().UnixNano())
	}
	if req.SchemaVersion == "" {
		req.SchemaVersion = application.BrainBridgeContract
	}

	cfg := toBrainSessionConfig(req.Config)
	result, err := a.dispatcher.DispatchWithRetry(ctx, brain.TaskDispatchRequest{
		TaskID:     req.TaskID,
		UserPrompt: req.UserPrompt,
		StaffID:    req.StaffID,
		Config:     cfg,
	})
	return fromBrainDispatchResult(req.TaskID, req.StaffID, req.SchemaVersion, result, err), err
}

// DispatchStream executes a brain task with streaming token callbacks.
func (a *RealBrainAdapter) DispatchStream(ctx context.Context, req application.BrainDispatchRequest, handler application.StreamChunkHandler) (application.BrainDispatchResult, error) {
	if req.TaskID == "" {
		req.TaskID = fmt.Sprintf("brain-%d", time.Now().UnixNano())
	}
	if req.SchemaVersion == "" {
		req.SchemaVersion = application.BrainBridgeContract
	}

	if handler == nil {
		return application.BrainDispatchResult{}, errors.New("stream handler cannot be nil")
	}

	cfg := toBrainSessionConfig(req.Config)
	result, err := a.dispatcher.DispatchWithRetry(ctx, brain.TaskDispatchRequest{
		TaskID:        req.TaskID,
		UserPrompt:    req.UserPrompt,
		StaffID:       req.StaffID,
		Config:        cfg,
		Stream:        true,
		StreamHandler: makeStreamHandler(req.TaskID, handler),
	})
	return fromBrainDispatchResult(req.TaskID, req.StaffID, req.SchemaVersion, result, err), err
}

func toBrainSessionConfig(cfg *application.SessionConfig) brain.SessionConfig {
	if cfg == nil {
		return brain.SessionConfig{}
	}
	return brain.SessionConfig{
		Timeout:   cfg.Timeout,
		StepLimit: cfg.StepLimit,
		MaxTokens: cfg.MaxTokens,
	}
}

func makeStreamHandler(taskID string, handler application.StreamChunkHandler) brain.StreamHandler {
	return func(chunk brain.StreamChunk) error {
		chunkType := "token"
		data := chunk.Content
		if chunk.Done {
			chunkType = "done"
			data = ""
		} else if chunk.ToolCall != nil {
			chunkType = "tool_call"
			data = chunk.ToolCall.Name
		}

		handler(application.StreamChunk{
			TaskID:    taskID,
			Type:      chunkType,
			Data:      data,
			Sequence:  chunk.Sequence,
			Timestamp: time.Now(),
		})
		return nil
	}
}

func fromBrainDispatchResult(taskID, staffID, schemaVersion string, result brain.TaskDispatchResult, err error) application.BrainDispatchResult {
	r := application.BrainDispatchResult{
		TaskID:        taskID,
		StaffID:       staffID,
		State:         string(result.State),
		Output:        result.Output,
		Steps:         result.Steps,
		Tokens:        result.Tokens,
		Duration:      result.Duration,
		SchemaVersion: schemaVersion,
	}
	if err != nil {
		r.Error = err.Error()
	}
	return r
}
