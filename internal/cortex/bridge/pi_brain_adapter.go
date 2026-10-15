// Pi CLI brain adapter.
//
// The Pi CLI is the model that answers and coordinates. This adapter exposes it
// through the existing application.BrainPort boundary so Orchestra's plan
// manager can talk to the model instead of a hardcoded reply. Each dispatch is
// a read-only, tool-less Pi turn: no files, no commands, no project context.
package bridge

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/RahmatHadinata23758051/CortexOS/internal/cortex/application"
	"github.com/RahmatHadinata23758051/CortexOS/internal/cortex/harness"
)

// PiBrainAdapter implements application.BrainPort with the real Pi CLI.
type PiBrainAdapter struct {
	runner     *harness.PiCLIAdapter
	systemRole string
	timeout    time.Duration
}

// NewPiBrainAdapter wraps a configured Pi CLI adapter as the Orchestra brain.
func NewPiBrainAdapter(runner *harness.PiCLIAdapter, systemRole string, timeout time.Duration) (application.BrainPort, error) {
	if runner == nil {
		return nil, errors.New("pi cli runner is required")
	}
	return &PiBrainAdapter{runner: runner, systemRole: systemRole, timeout: timeout}, nil
}

// Name reports the provider name for status.
func (a *PiBrainAdapter) Name() string { return "pi-cli-brain" }

// Dispatch runs one bounded Pi turn and returns the model's answer.
func (a *PiBrainAdapter) Dispatch(ctx context.Context, req application.BrainDispatchRequest) (application.BrainDispatchResult, error) {
	taskID := req.TaskID
	if taskID == "" {
		taskID = fmt.Sprintf("pi-brain-%d", time.Now().UnixNano())
	}
	timeout := a.timeout
	if req.Config != nil && req.Config.Timeout > 0 {
		timeout = req.Config.Timeout
	}
	out, err := a.runner.Complete(ctx, a.systemRole, req.UserPrompt, timeout)
	if err != nil {
		return application.BrainDispatchResult{
			TaskID: taskID, StaffID: req.StaffID, State: "failed",
			Error: err.Error(), SchemaVersion: application.BrainBridgeContract,
		}, err
	}
	return application.BrainDispatchResult{
		TaskID: taskID, StaffID: req.StaffID, State: "completed",
		Output: out, Steps: 1, SchemaVersion: application.BrainBridgeContract,
	}, nil
}

// DispatchStream emits the single completed answer as one chunk.
func (a *PiBrainAdapter) DispatchStream(ctx context.Context, req application.BrainDispatchRequest, handler application.StreamChunkHandler) (application.BrainDispatchResult, error) {
	result, err := a.Dispatch(ctx, req)
	if handler != nil && result.Output != "" {
		handler(application.StreamChunk{
			TaskID: result.TaskID, Type: "token", Data: result.Output,
			Sequence: 1, Timestamp: time.Now(),
		})
	}
	return result, err
}
