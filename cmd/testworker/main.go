package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/RahmatHadinata23758051/CortexOS/internal/cortex/harness"
)

const (
	WorkerID      = "test-worker-1"
	WorkerVersion = "1.0.0-test"
)

type worker struct {
	mu      sync.Mutex
	stdout  *os.File
	stderr  *os.File
	seq     uint64
	corr    string
	taskEnv *harness.TaskEnvelope
}

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	w := &worker{
		stdout: os.Stdout,
		stderr: os.Stderr,
		seq:    0,
	}

	// Read task context from environment (set by test via AllowedEnvVars)
	w.taskEnv = w.taskEnvelopeFromEnv()

	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "--handshake":
			if err := w.handshake("handshake"); err != nil {
				fmt.Fprintf(w.stderr, "handshake failed: %v\n", err)
				os.Exit(1)
			}
			return
		case "--worker":
			if err := w.runWorker(ctx); err != nil {
				fmt.Fprintf(w.stderr, "worker failed: %v\n", err)
				os.Exit(1)
			}
			return
		}
	}

	fmt.Fprintln(w.stderr, "Usage: testworker [--handshake|--worker]")
	os.Exit(1)
}

func (w *worker) taskEnvelopeFromEnv() *harness.TaskEnvelope {
	execID := os.Getenv("CORTEXOS_EXECUTION_ID")
	taskID := os.Getenv("CORTEXOS_TASK_ID")
	worktreeID := os.Getenv("CORTEXOS_WORKTREE_ID")
	projectID := os.Getenv("CORTEXOS_PROJECT_ID")
	toolName := os.Getenv("CORTEXOS_TOOL_NAME")
	inputStr := strings.TrimSpace(os.Getenv("CORTEXOS_INPUT"))

	if execID == "" || taskID == "" || worktreeID == "" || projectID == "" || toolName == "" {
		return nil
	}

	var input json.RawMessage
	if inputStr != "" && json.Valid([]byte(inputStr)) {
		input = json.RawMessage(inputStr)
	} else {
		input = json.RawMessage(`{"prompt":"test"}`)
	}

	return &harness.TaskEnvelope{
		ContractVersion: harness.HarnessContractVersion,
		ExecutionID:     execID,
		TaskID:          taskID,
		WorktreeID:      worktreeID,
		ProjectID:       projectID,
		ToolName:        toolName,
		Input:           input,
		TimeoutMs:       30000,
		TraceID:         execID,
	}
}

func (w *worker) handshake(corr string) error {
	payload := harness.HandshakePayload{
		WorkerID:      WorkerID,
		WorkerVersion: WorkerVersion,
		Protocol:      harness.WorkerProtocolVersion,
	}
	msg, err := harness.NewMessage(harness.MessageHandshake, corr, w.nextSeq(), payload)
	if err != nil {
		return err
	}
	line, err := harness.EncodeLine(msg)
	if err != nil {
		return err
	}
	_, err = w.stdout.Write(line)
	return err
}

func (w *worker) runWorker(ctx context.Context) error {
	// If no task envelope from env, fail gracefully
	if w.taskEnv == nil {
		taskID := os.Getenv("CORTEXOS_TASK_ID")
		if taskID == "" {
			taskID = "unknown"
		}
		// Still need a corr for terminal
		w.corr = taskID
		return w.sendTerminal(harness.TerminalFailure, "missing task context env vars", nil)
	}

	// Set correlation ID to execution ID
	w.corr = w.taskEnv.ExecutionID

	// Emit handshake response
	if err := w.handshake(w.corr); err != nil {
		return err
	}

	// Emit task echo (correlation check)
	taskMsg, err := harness.NewMessage(harness.MessageTask, w.corr, w.nextSeq(), *w.taskEnv)
	if err != nil {
		return err
	}
	taskLine, err := harness.EncodeLine(taskMsg)
	if err != nil {
		return err
	}
	if _, err := w.stdout.Write(taskLine); err != nil {
		return err
	}

	// Simulate work: produce progress events
	progressPayload := harness.ProgressPayload{
		Phase:   "run",
		Percent: 25,
		Message: "initializing",
	}
	if err := w.sendMessage(harness.MessageProgress, progressPayload); err != nil {
		return err
	}

	// Simulate actual work
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(100 * time.Millisecond):
	}

	progressPayload = harness.ProgressPayload{
		Phase:   "run",
		Percent: 50,
		Message: "processing",
	}
	if err := w.sendMessage(harness.MessageProgress, progressPayload); err != nil {
		return err
	}

	// Produce evidence
	evidencePayload := harness.EvidencePayload{
		EvidenceID: fmt.Sprintf("ev-%s-%d", w.taskEnv.TaskID, time.Now().UnixNano()),
		Kind:       harness.EvidenceKindCommandExecution,
		Digest:     strings.Repeat("a", 64),
		Summary:    fmt.Sprintf("completed task %s with tool %s", w.taskEnv.TaskID, w.taskEnv.ToolName),
	}
	if err := w.sendMessage(harness.MessageEvidence, evidencePayload); err != nil {
		return err
	}

	// Terminal success
	terminalPayload := harness.TerminalPayload{
		Status:      harness.TerminalSuccess,
		Message:     "task completed successfully",
		EvidenceIDs: []string{evidencePayload.EvidenceID},
	}
	return w.sendMessage(harness.MessageTerminal, terminalPayload)
}

func (w *worker) sendMessage(msgType harness.MessageType, payload any) error {
	msg, err := harness.NewMessage(msgType, w.corr, w.nextSeq(), payload)
	if err != nil {
		return err
	}
	line, err := harness.EncodeLine(msg)
	if err != nil {
		return err
	}
	_, err = w.stdout.Write(line)
	return err
}

func (w *worker) sendTerminal(status harness.TerminalStatus, message string, evidenceIDs []string) error {
	payload := harness.TerminalPayload{
		Status:      status,
		Message:     message,
		EvidenceIDs: evidenceIDs,
	}
	return w.sendMessage(harness.MessageTerminal, payload)
}

func (w *worker) nextSeq() uint64 {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.seq++
	return w.seq
}
