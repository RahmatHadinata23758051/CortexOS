package brain

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
)

var (
	ErrStreamCanceled  = errors.New("stream processing canceled")
	ErrStreamMalformed = errors.New("malformed stream chunk")
)

// StreamChunk is a single normalized token or tool call chunk from the LLM provider.
type StreamChunk struct {
	TaskID       string         `json:"taskId"`
	Sequence     int            `json:"sequence"`
	Content      string         `json:"content,omitempty"`
	ToolCall     *ToolCallChunk `json:"toolCall,omitempty"`
	FinishReason string         `json:"finishReason,omitempty"`
	Done         bool           `json:"done"`
}

// ToolCallChunk represents incremental tool call data during streaming.
type ToolCallChunk struct {
	Index     int    `json:"index"`
	ID        string `json:"id,omitempty"`
	Name      string `json:"name,omitempty"`
	Arguments string `json:"arguments,omitempty"`
}

// StreamHandler is invoked for each received stream chunk.
// Returning an error aborts stream reading immediately.
type StreamHandler func(StreamChunk) error

// ConsumeStream reads an SSE data stream and dispatches StreamChunks to the handler.
// It handles OpenAI-compatible SSE framing (data: {...}\n\n and data: [DONE]).
func ConsumeStream(ctx context.Context, taskID string, r io.Reader, handler StreamHandler) (string, error) {
	if r == nil {
		return "", errors.New("reader is nil")
	}
	if handler == nil {
		handler = func(StreamChunk) error { return nil }
	}

	scanner := bufio.NewScanner(r)
	// Bounded buffer for stream lines to prevent memory exhaustion
	const maxLineLength = 64 * 1024
	buf := make([]byte, maxLineLength)
	scanner.Buffer(buf, maxLineLength)

	var fullContent strings.Builder
	seq := 0

	for scanner.Scan() {
		if err := ctx.Err(); err != nil {
			return fullContent.String(), err
		}

		line := scanner.Bytes()
		line = bytes.TrimSpace(line)

		// Skip empty lines or SSE comments
		if len(line) == 0 || line[0] == ':' {
			continue
		}

		// SSE payload must start with "data:"
		if !bytes.HasPrefix(line, []byte("data:")) {
			continue
		}

		payload := bytes.TrimSpace(line[5:])
		if len(payload) == 0 {
			continue
		}

		// Check for standard stream termination: "data: [DONE]"
		if bytes.Equal(payload, []byte("[DONE]")) {
			seq++
			_ = handler(StreamChunk{
				TaskID:   taskID,
				Sequence: seq,
				Done:     true,
			})
			return fullContent.String(), nil
		}

		// Parse JSON chunk
		var raw struct {
			Choices []struct {
				Delta struct {
					Content   string `json:"content"`
					ToolCalls []struct {
						Index    int    `json:"index"`
						ID       string `json:"id"`
						Function struct {
							Name      string `json:"name"`
							Arguments string `json:"arguments"`
						} `json:"function"`
					} `json:"tool_calls"`
				} `json:"delta"`
				FinishReason *string `json:"finish_reason"`
			} `json:"choices"`
		}

		if err := json.Unmarshal(payload, &raw); err != nil {
			// Skip unrecognized control frames instead of hard-crashing
			continue
		}

		if len(raw.Choices) == 0 {
			continue
		}

		choice := raw.Choices[0]
		seq++
		chunk := StreamChunk{
			TaskID:   taskID,
			Sequence: seq,
			Content:  choice.Delta.Content,
		}

		if choice.FinishReason != nil {
			chunk.FinishReason = *choice.FinishReason
		}

		// Handle tool call chunks
		if len(choice.Delta.ToolCalls) > 0 {
			tc := choice.Delta.ToolCalls[0]
			chunk.ToolCall = &ToolCallChunk{
				Index:     tc.Index,
				ID:        tc.ID,
				Name:      tc.Function.Name,
				Arguments: tc.Function.Arguments,
			}
		}

		if chunk.Content != "" {
			fullContent.WriteString(chunk.Content)
		}

		if err := handler(chunk); err != nil {
			return fullContent.String(), fmt.Errorf("stream handler aborted: %w", err)
		}
	}

	if err := scanner.Err(); err != nil {
		if errors.Is(err, context.Canceled) || ctx.Err() != nil {
			return fullContent.String(), ctx.Err()
		}
		return fullContent.String(), fmt.Errorf("scan stream: %w", err)
	}

	// Emit final done chunk if not received explicitly
	seq++
	_ = handler(StreamChunk{
		TaskID:   taskID,
		Sequence: seq,
		Done:     true,
	})

	return fullContent.String(), nil
}
