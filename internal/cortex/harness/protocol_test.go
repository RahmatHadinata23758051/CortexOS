package harness

import (
	"bytes"
	"errors"
	"io"
	"os"
	"strings"
	"testing"
	"time"
)

func mustReadFixture(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func TestProtocolFixtureRoundTripAndLifecycle(t *testing.T) {
	data := mustReadFixture(t, "testdata/worker_lifecycle.jsonl")
	decoder := NewDecoder(bytes.NewReader(data))
	var seen []MessageType
	for {
		message, err := decoder.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			t.Fatalf("decode fixture: %v", err)
		}
		seen = append(seen, message.Type)
	}
	want := []MessageType{
		MessageHandshake, MessageCapabilities, MessageHealth, MessageTask,
		MessageProgress, MessageEvidence, MessageDiagnostic, MessageHeartbeat,
		MessageTerminal, MessageShutdown,
	}
	if len(seen) != len(want) {
		t.Fatalf("message count = %d, want %d (%v)", len(seen), len(want), seen)
	}
	for i := range want {
		if seen[i] != want[i] {
			t.Fatalf("message %d = %q, want %q", i, seen[i], want[i])
		}
	}
	if decoder.State() != StateClosed {
		t.Fatalf("state = %q, want closed", decoder.State())
	}
}

func TestProtocolEncodeDecode(t *testing.T) {
	message, err := NewMessage(MessageProgress, "exec-1", 3, ProgressPayload{Phase: "editing", Percent: 40})
	if err != nil {
		t.Fatal(err)
	}
	line, err := EncodeLine(message)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := DecodeLine(line)
	if err != nil {
		t.Fatal(err)
	}
	if decoded.Type != message.Type || decoded.CorrelationID != message.CorrelationID || decoded.Sequence != message.Sequence {
		t.Fatalf("decoded envelope differs: %#v", decoded)
	}
}

func TestProtocolRejectsMalformedUnknownDuplicateAndOversized(t *testing.T) {
	tests := []struct {
		name string
		line []byte
		want error
	}{
		{"malformed", []byte(`{"version":`), ErrProtocolMalformed},
		{"unknown type", []byte(`{"version":"cortexos.worker.v1","type":"bogus","correlationId":"c","sequence":1,"payload":{}}`), ErrProtocolUnknownMessage},
		{"unknown envelope field", []byte(`{"version":"cortexos.worker.v1","type":"heartbeat","correlationId":"c","sequence":1,"payload":{"healthy":true,"at":"2026-10-07T00:00:00Z"},"extra":true}`), ErrProtocolMalformed},
		{"duplicate envelope key", []byte(`{"version":"cortexos.worker.v1","type":"heartbeat","correlationId":"c","sequence":1,"sequence":2,"payload":{"healthy":true,"at":"2026-10-07T00:00:00Z"}}`), ErrProtocolMalformed},
		{"duplicate payload key", []byte(`{"version":"cortexos.worker.v1","type":"heartbeat","correlationId":"c","sequence":1,"payload":{"healthy":true,"healthy":false,"at":"2026-10-07T00:00:00Z"}}`), ErrProtocolMalformed},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := DecodeLine(tt.line)
			if !errors.Is(err, tt.want) {
				t.Fatalf("error = %v, want errors.Is(..., %v)", err, tt.want)
			}
		})
	}
	oversized := bytes.Repeat([]byte("x"), MaxProtocolLineBytes+1)
	if _, err := DecodeLine(oversized); !errors.Is(err, ErrProtocolLineTooLarge) {
		t.Fatalf("oversized error = %v", err)
	}
}

func TestProtocolValidatorRejectsOutOfOrderDuplicateAndInvalidLifecycle(t *testing.T) {
	v := NewValidator()
	handshake := mustMessage(t, MessageHandshake, 1, HandshakePayload{WorkerID: "w1", WorkerVersion: "1", Protocol: WorkerProtocolVersion})
	if err := v.Accept(handshake); err != nil {
		t.Fatal(err)
	}
	if err := v.Accept(handshake); !errors.Is(err, ErrProtocolSequence) {
		t.Fatalf("duplicate sequence error = %v", err)
	}
	wrongSequence := mustMessage(t, MessageHealth, 3, HealthPayload{Healthy: true, Status: "ready"})
	if err := v.Accept(wrongSequence); !errors.Is(err, ErrProtocolSequence) {
		t.Fatalf("out of order error = %v", err)
	}
	if err := v.Accept(mustMessage(t, MessageTask, 2, testTask())); err != nil {
		t.Fatal(err)
	}
	if err := v.Accept(mustMessage(t, MessageTask, 3, testTask())); !errors.Is(err, ErrProtocolDuplicate) {
		t.Fatalf("second task error = %v", err)
	}
}

func TestProtocolCancellationTimeoutAndCrashAreObservable(t *testing.T) {
	v := NewValidator()
	acceptStartup(t, v)
	if err := v.Accept(mustMessage(t, MessageTask, 2, testTask())); err != nil {
		t.Fatal(err)
	}
	if err := v.Accept(mustMessage(t, MessageCancel, 3, CancelPayload{Reason: "caller canceled"})); err != nil {
		t.Fatal(err)
	}
	terminal := TerminalPayload{Status: TerminalCanceled, Message: "canceled", Error: &ToolError{Code: "harness.canceled", Message: "execution canceled"}}
	if err := v.Accept(mustMessage(t, MessageTerminal, 4, terminal)); err != nil {
		t.Fatal(err)
	}
	if v.State() != StateTerminated {
		t.Fatalf("state after cancellation = %q", v.State())
	}

	crashed := NewDecoder(strings.NewReader(string(mustLine(t, MessageHandshake, 1, HandshakePayload{WorkerID: "w", WorkerVersion: "1", Protocol: WorkerProtocolVersion})) + string(mustLine(t, MessageTask, 2, testTask()))))
	for {
		_, err := crashed.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
	}
	if crashed.State() != StateRunning {
		t.Fatalf("crashed worker state = %q, want running observation before process EOF", crashed.State())
	}
}

func TestProtocolConversionsRemainUntrusted(t *testing.T) {
	task := testTask()
	result := TerminalToToolResult(task, TerminalPayload{Status: TerminalSuccess, EvidenceIDs: []string{"ev-1"}})
	if result.Status != ToolStatusSuccess || !result.Redacted || len(result.EvidenceIDs) != 1 {
		t.Fatalf("unexpected result: %#v", result)
	}
	envelope := TaskEnvelopeToExecutionEnvelope(task, `C:\\private\\worktree`, PolicyDecision{Allowed: true}, AuditMetadata{Actor: "worker"})
	if envelope.WorktreeRoot == "" || envelope.Timeout <= 0 || envelope.ContractVersion != HarnessContractVersion {
		t.Fatalf("unexpected execution envelope: %#v", envelope)
	}
}

func TestProtocolCancellationLifecycle(t *testing.T) {
	data := mustReadFixture(t, "testdata/worker_cancellation.jsonl")
	decoder := NewDecoder(bytes.NewReader(data))
	var seen []MessageType
	for {
		m, err := decoder.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		seen = append(seen, m.Type)
	}
	want := []MessageType{MessageHandshake, MessageTask, MessageCancel, MessageTerminal, MessageShutdown}
	if !equalMessageTypes(seen, want) {
		t.Fatalf("seen = %v, want %v", seen, want)
	}
	if decoder.State() != StateClosed {
		t.Fatalf("state = %q", decoder.State())
	}
}

func TestProtocolFailureLifecycle(t *testing.T) {
	data := mustReadFixture(t, "testdata/worker_failure.jsonl")
	decoder := NewDecoder(bytes.NewReader(data))
	for {
		_, err := decoder.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
	}
	if decoder.State() != StateClosed {
		t.Fatalf("state = %q", decoder.State())
	}
}

func equalMessageTypes(a, b []MessageType) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestProtocolHeartbeatLoss(t *testing.T) {
	v := NewValidator()
	acceptStartup(t, v)
	at := time.Date(2026, 10, 7, 12, 0, 0, 0, time.UTC)
	if err := v.Accept(mustMessage(t, MessageHeartbeat, 2, HeartbeatPayload{Healthy: true, At: at})); err != nil {
		t.Fatal(err)
	}
	if !v.HeartbeatHealthy(at.Add(2*time.Second), 5*time.Second) {
		t.Fatal("heartbeat should be healthy")
	}
	if v.HeartbeatHealthy(at.Add(6*time.Second), 5*time.Second) {
		t.Fatal("heartbeat should be considered lost")
	}
	if age, ok := v.HeartbeatAge(at.Add(6 * time.Second)); !ok || age != 6*time.Second {
		t.Fatalf("heartbeat age = %v, present = %v", age, ok)
	}
}

func FuzzDecodeLineNeverPanics(f *testing.F) {
	f.Add([]byte(`{"version":"cortexos.worker.v1","type":"heartbeat","correlationId":"c","sequence":1,"payload":{"healthy":true,"at":"2026-10-07T00:00:00Z"}}`))
	f.Add([]byte(`{"version":"cortexos.worker.v1","type":"task","payload":`))
	f.Fuzz(func(t *testing.T, data []byte) {
		_, _ = DecodeLine(data)
	})
}

func acceptStartup(t *testing.T, v *Validator) {
	t.Helper()
	if err := v.Accept(mustMessage(t, MessageHandshake, 1, HandshakePayload{WorkerID: "w1", WorkerVersion: "1", Protocol: WorkerProtocolVersion})); err != nil {
		t.Fatal(err)
	}
}

func testTask() TaskEnvelope {
	return TaskEnvelope{
		ContractVersion: HarnessContractVersion,
		ExecutionID:     "exec-1",
		TaskID:          "task-1",
		WorktreeID:      "worktree-1",
		ProjectID:       "project-1",
		ToolName:        "test",
		Input:           []byte(`{"command":"go test ./..."}`),
		TimeoutMs:       5000,
	}
}

func mustMessage(t *testing.T, typ MessageType, sequence uint64, payload any) Message {
	t.Helper()
	message, err := NewMessage(typ, "exec-1", sequence, payload)
	if err != nil {
		t.Fatal(err)
	}
	return message
}

func mustLine(t *testing.T, typ MessageType, sequence uint64, payload any) []byte {
	t.Helper()
	line, err := EncodeLine(mustMessage(t, typ, sequence, payload))
	if err != nil {
		t.Fatal(err)
	}
	return line
}
