package brain

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/RahmatHadinata23758051/CortexOS/internal/cortex/application"
)

var (
	ErrDispatcherNilProvider = errors.New("provider port is required")
	ErrDispatcherInvalidTask = errors.New("invalid task dispatch request")
)

// TaskDispatchRequest describes an execution unit sent to the Brain.
type TaskDispatchRequest struct {
	TaskID      string        `json:"taskId"`
	StaffID     string        `json:"staffId,omitempty"`
	SystemRole  string        `json:"systemRole,omitempty"`
	UserPrompt  string        `json:"userPrompt"`
	ContextDocs []string      `json:"contextDocs,omitempty"`
	Model       string        `json:"model,omitempty"`
	Config      SessionConfig `json:"config"`
	Stream      bool          `json:"stream"`
	// StreamHandler is called for each streaming token/tool chunk when Stream=true.
	// It is not serialized; it's a runtime callback.
	StreamHandler StreamHandler `json:"-"`
}

func (r TaskDispatchRequest) Validate() error {
	if strings.TrimSpace(r.TaskID) == "" {
		return fmt.Errorf("%w: taskId is required", ErrDispatcherInvalidTask)
	}
	if strings.TrimSpace(r.UserPrompt) == "" {
		return fmt.Errorf("%w: userPrompt is required", ErrDispatcherInvalidTask)
	}
	return r.Config.Validate()
}

// TaskDispatchResult is the outcome of a dispatched task execution.
type TaskDispatchResult struct {
	TaskID   string        `json:"taskId"`
	StaffID  string        `json:"staffId,omitempty"`
	State    State         `json:"state"`
	Output   string        `json:"output"`
	Steps    int           `json:"steps"`
	Tokens   int           `json:"tokens"`
	Duration time.Duration `json:"duration"`
	Error    string        `json:"error,omitempty"`
}

// Dispatcher orchestrates reasoning execution across brain.Session and application.ProviderPort.
type Dispatcher struct {
	provider application.ProviderPort
}

// NewDispatcher creates a Dispatcher with the given provider boundary.
func NewDispatcher(p application.ProviderPort) (*Dispatcher, error) {
	if p == nil {
		return nil, ErrDispatcherNilProvider
	}
	return &Dispatcher{provider: p}, nil
}

// Dispatch executes the task through the brain session lifecycle.
func (d *Dispatcher) Dispatch(ctx context.Context, req TaskDispatchRequest) (TaskDispatchResult, error) {
	if err := req.Validate(); err != nil {
		return TaskDispatchResult{}, err
	}
	if err := ctx.Err(); err != nil {
		return TaskDispatchResult{TaskID: req.TaskID, State: StateCanceled, Error: err.Error()}, err
	}

	session, err := NewSession(req.Config)
	if err != nil {
		return TaskDispatchResult{}, err
	}

	start := time.Now()

	// 1. Transition: Idle -> Planning
	if err := session.Transition(StatePlanning, "start task planning"); err != nil {
		return d.failResult(session, req, start, err), err
	}

	// Build messages
	messages := make([]application.ProviderMessage, 0, 4)
	if req.SystemRole != "" {
		messages = append(messages, application.ProviderMessage{
			Role:    "system",
			Content: req.SystemRole,
		})
	}
	// Append context docs if present
	if len(req.ContextDocs) > 0 {
		var b strings.Builder
		b.WriteString("Context retrieved for this task:\n")
		for i, doc := range req.ContextDocs {
			fmt.Fprintf(&b, "[%d] %s\n", i+1, doc)
		}
		messages = append(messages, application.ProviderMessage{
			Role:    "system",
			Content: b.String(),
		})
	}
	messages = append(messages, application.ProviderMessage{
		Role:    "user",
		Content: req.UserPrompt,
	})

	// 2. Transition: Planning -> Executing
	if err := session.Transition(StateExecuting, "dispatch to provider"); err != nil {
		return d.failResult(session, req, start, err), err
	}

	if err := session.BeginStep(); err != nil {
		return d.failResult(session, req, start, err), err
	}

	maxTokens := req.Config.MaxTokens
	var maxTokensPtr *int
	if maxTokens > 0 {
		maxTokensPtr = &maxTokens
	}

	provReq := application.ProviderCompleteRequest{
		Model:     req.Model,
		Messages:  messages,
		MaxTokens: maxTokensPtr,
	}

	if req.Stream {
		return d.dispatchStreaming(ctx, session, req, provReq, start)
	}

	// Non-streaming completion
	reader, err := d.provider.Complete(ctx, provReq)
	if err != nil {
		d.transitionProviderError(session, err)
		return d.failResult(session, req, start, err), err
	}
	defer reader.Close()

	output, tokenCount, parseErr := parseProviderResponse(reader)
	if parseErr != nil {
		_ = session.Transition(StateFailed, parseErr.Error())
		return d.failResult(session, req, start, parseErr), parseErr
	}

	if tokenCount > 0 {
		_ = session.AddTokens(tokenCount)
	}

	if err := session.Transition(StateCompleted, "completion received"); err != nil {
		return d.failResult(session, req, start, err), err
	}

	steps, tokens, _ := session.Stats()
	return TaskDispatchResult{
		TaskID:   req.TaskID,
		StaffID:  req.StaffID,
		State:    session.State(),
		Output:   output,
		Steps:    steps,
		Tokens:   tokens,
		Duration: time.Since(start),
	}, nil
}

func (d *Dispatcher) dispatchStreaming(ctx context.Context, session *Session, req TaskDispatchRequest, provReq application.ProviderCompleteRequest, start time.Time) (TaskDispatchResult, error) {
	if err := session.Transition(StateStreaming, "start token stream"); err != nil {
		return d.failResult(session, req, start, err), err
	}

	reader, err := d.provider.Stream(ctx, provReq)
	if err != nil {
		d.transitionProviderError(session, err)
		return d.failResult(session, req, start, err), err
	}
	defer reader.Close()

	output, consumeErr := ConsumeStream(ctx, req.TaskID, reader, req.StreamHandler)
	if consumeErr != nil {
		d.transitionProviderError(session, consumeErr)
		return d.failResult(session, req, start, consumeErr), consumeErr
	}
	if err := session.Transition(StateCompleted, "stream finished"); err != nil {
		return d.failResult(session, req, start, err), err
	}

	steps, tokens, _ := session.Stats()
	return TaskDispatchResult{
		TaskID:   req.TaskID,
		StaffID:  req.StaffID,
		State:    session.State(),
		Output:   output,
		Steps:    steps,
		Tokens:   tokens,
		Duration: time.Since(start),
	}, nil
}

func (d *Dispatcher) failResult(s *Session, req TaskDispatchRequest, start time.Time, err error) TaskDispatchResult {
	steps, tokens, _ := s.Stats()
	state := s.State()
	return TaskDispatchResult{
		TaskID:   req.TaskID,
		StaffID:  req.StaffID,
		State:    state,
		Steps:    steps,
		Tokens:   tokens,
		Duration: time.Since(start),
		Error:    err.Error(),
	}
}

func (d *Dispatcher) transitionProviderError(s *Session, err error) {
	if s == nil || err == nil {
		return
	}
	if errors.Is(err, context.Canceled) {
		_ = s.Transition(StateCanceled, "request canceled")
		return
	}
	_ = s.Transition(StateFailed, err.Error())
}

// parseProviderResponse parses the standard OpenAI Chat Completion response payload.
func parseProviderResponse(r io.Reader) (content string, tokens int, err error) {
	var resp struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
		Usage struct {
			TotalTokens int `json:"total_tokens"`
		} `json:"usage"`
	}
	if err := json.NewDecoder(r).Decode(&resp); err != nil {
		return "", 0, fmt.Errorf("failed to parse provider response: %w", err)
	}
	if len(resp.Choices) > 0 {
		content = resp.Choices[0].Message.Content
	}
	return content, resp.Usage.TotalTokens, nil
}
