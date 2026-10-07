package harness

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sync"
	"time"
)

// WorkerProtocolVersion identifies the versioned wire contract used by a
// CortexOS worker. It is deliberately separate from HarnessContractVersion:
// the former is a process protocol, while the latter describes harness DTOs.
const WorkerProtocolVersion = "cortexos.worker.v1"

// MaxProtocolLineBytes is the maximum number of bytes in one JSONL record,
// excluding the line terminator. The limit is part of the safety boundary and
// must be applied before decoding untrusted worker output.
const MaxProtocolLineBytes = 1 << 20 // 1 MiB

// MaxProtocolMessages is the maximum number of JSONL messages a worker may
// emit in a single session. This prevents unbounded resource consumption from
// malicious or buggy workers. It is applied at the Decoder level.
const MaxProtocolMessages = 1024

var (
	ErrProtocolMalformed      = errors.New("harness: malformed worker protocol message")
	ErrProtocolLineTooLarge   = errors.New("harness: worker protocol line is too large")
	ErrProtocolUnknownMessage = errors.New("harness: unknown worker protocol message")
	ErrProtocolSequence       = errors.New("harness: invalid worker protocol sequence")
	ErrProtocolLifecycle      = errors.New("harness: invalid worker lifecycle transition")
	ErrProtocolDuplicate      = errors.New("harness: duplicate worker protocol message")
	ErrProtocolClosed         = errors.New("harness: worker protocol is closed")
	ErrProtocolHeartbeatLost  = errors.New("harness: worker heartbeat lost")
	ErrProtocolCorrelation    = errors.New("harness: worker protocol correlation mismatch")
)

// MessageType is the only discriminator accepted on the worker wire.
type MessageType string

const (
	MessageHandshake    MessageType = "handshake"
	MessageCapabilities MessageType = "capabilities"
	MessageHealth       MessageType = "health"
	MessageTask         MessageType = "task"
	MessageProgress     MessageType = "progress"
	MessageEvidence     MessageType = "evidence"
	MessageDiagnostic   MessageType = "diagnostic"
	MessageHeartbeat    MessageType = "heartbeat"
	MessageTerminal     MessageType = "terminal"
	MessageCancel       MessageType = "cancel"
	MessageShutdown     MessageType = "shutdown"
)

// Message is the common JSONL envelope. Payload is decoded according to Type.
// Worker output, including terminal success, is untrusted evidence; this
// protocol never grants a worker authority to mark an Orchestra task successful.
type Message struct {
	Version       string          `json:"version"`
	Type          MessageType     `json:"type"`
	CorrelationID string          `json:"correlationId"`
	Sequence      uint64          `json:"sequence"`
	Payload       json.RawMessage `json:"payload"`
}

// HandshakePayload starts a worker session.
type HandshakePayload struct {
	WorkerID      string `json:"workerId"`
	WorkerVersion string `json:"workerVersion"`
	Protocol      string `json:"protocol"`
}

func (p HandshakePayload) Validate() error {
	if p.WorkerID == "" || p.WorkerVersion == "" || p.Protocol == "" {
		return fmt.Errorf("%w: handshake worker id, version, and protocol are required", ErrInvalidContract)
	}
	return nil
}

// CapabilitiesPayload describes worker capabilities. Capabilities are
// informational and do not bypass ToolBroker policy.
type CapabilitiesPayload struct {
	Capabilities []ToolCapability `json:"capabilities"`
}

func (p CapabilitiesPayload) Validate() error {
	if len(p.Capabilities) == 0 {
		return fmt.Errorf("%w: at least one worker capability is required", ErrInvalidContract)
	}
	for _, capability := range p.Capabilities {
		if capability == "" {
			return fmt.Errorf("%w: worker capability must not be empty", ErrInvalidContract)
		}
	}
	return nil
}

// HealthPayload is a point-in-time readiness report.
type HealthPayload struct {
	Healthy bool   `json:"healthy"`
	Status  string `json:"status"`
	Detail  string `json:"detail,omitempty"`
}

func (p HealthPayload) Validate() error {
	if p.Status == "" {
		return fmt.Errorf("%w: health status is required", ErrInvalidContract)
	}
	return nil
}

// TaskEnvelope is the process-safe subset of ExecutionEnvelope. Input and
// policy decisions remain JSON data so engine-specific values cannot cross the
// harness boundary as Go types.
type TaskEnvelope struct {
	ContractVersion string          `json:"contractVersion"`
	ExecutionID     string          `json:"executionId"`
	TaskID          string          `json:"taskId"`
	WorktreeID      string          `json:"worktreeId"`
	ProjectID       string          `json:"projectId"`
	ToolName        string          `json:"toolName"`
	Input           json.RawMessage `json:"input"`
	TimeoutMs       int64           `json:"timeoutMs"`
	TraceID         string          `json:"traceId,omitempty"`
}

func (t TaskEnvelope) Validate() error {
	if t.ContractVersion != HarnessContractVersion {
		return fmt.Errorf("%w: unsupported task contract version %q", ErrInvalidContract, t.ContractVersion)
	}
	if t.ExecutionID == "" || t.TaskID == "" || t.WorktreeID == "" || t.ProjectID == "" || t.ToolName == "" {
		return fmt.Errorf("%w: task execution, task, worktree, project, and tool IDs are required", ErrInvalidContract)
	}
	if t.TimeoutMs <= 0 {
		return fmt.Errorf("%w: task timeout must be positive", ErrInvalidContract)
	}
	if len(t.Input) == 0 || !json.Valid(t.Input) {
		return fmt.Errorf("%w: task input must be valid JSON", ErrInvalidContract)
	}
	return nil
}

type ProgressPayload struct {
	Phase   string `json:"phase"`
	Percent int    `json:"percent"`
	Message string `json:"message,omitempty"`
}

func (p ProgressPayload) Validate() error {
	if p.Phase == "" || p.Percent < 0 || p.Percent > 100 {
		return fmt.Errorf("%w: progress phase and percent (0..100) are required", ErrInvalidContract)
	}
	return nil
}

// EvidencePayload is intentionally a reference plus redacted summary. It is
// not a success assertion and must be inspected by Orchestra/Inspector.
type EvidencePayload struct {
	EvidenceID string          `json:"evidenceId"`
	Kind       EvidenceKind    `json:"kind"`
	Digest     string          `json:"digest"`
	Summary    string          `json:"summary,omitempty"`
	Record     *EvidenceRecord `json:"record,omitempty"`
}

func (p EvidencePayload) Validate() error {
	if p.EvidenceID == "" || p.Kind == "" || p.Digest == "" {
		return fmt.Errorf("%w: evidence id, kind, and digest are required", ErrInvalidContract)
	}
	if len(p.Digest) != 64 {
		return fmt.Errorf("%w: evidence digest must be a SHA-256 hex string", ErrInvalidContract)
	}
	for _, character := range p.Digest {
		if !((character >= '0' && character <= '9') || (character >= 'a' && character <= 'f') || (character >= 'A' && character <= 'F')) {
			return fmt.Errorf("%w: evidence digest must be hexadecimal", ErrInvalidContract)
		}
	}
	return nil
}

type DiagnosticPayload struct {
	Level   string `json:"level"`
	Code    string `json:"code"`
	Message string `json:"message"`
	Detail  string `json:"detail,omitempty"`
}

func (p DiagnosticPayload) Validate() error {
	if p.Level == "" || p.Code == "" || p.Message == "" {
		return fmt.Errorf("%w: diagnostic level, code, and message are required", ErrInvalidContract)
	}
	switch p.Level {
	case "debug", "info", "warn", "error":
	default:
		return fmt.Errorf("%w: invalid diagnostic level %q", ErrInvalidContract, p.Level)
	}
	return nil
}

type HeartbeatPayload struct {
	Healthy bool      `json:"healthy"`
	At      time.Time `json:"at"`
}

func (p HeartbeatPayload) Validate() error {
	if p.At.IsZero() {
		return fmt.Errorf("%w: heartbeat timestamp is required", ErrInvalidContract)
	}
	return nil
}

// TerminalStatus represents the worker's claimed terminal state.
// Worker claims remain untrusted evidence; Inspector/Orchestra owns SUCCESS.
type TerminalStatus string

const (
	TerminalSuccess  TerminalStatus = "success"
	TerminalFailure  TerminalStatus = "failure"
	TerminalCanceled TerminalStatus = "canceled"
)

type TerminalPayload struct {
	Status      TerminalStatus `json:"status"`
	Message     string         `json:"message,omitempty"`
	Error       *ToolError     `json:"error,omitempty"`
	EvidenceIDs []string       `json:"evidenceIds,omitempty"`
}

func (p TerminalPayload) Validate() error {
	if p.Status != TerminalSuccess && p.Status != TerminalFailure && p.Status != TerminalCanceled {
		return fmt.Errorf("%w: invalid terminal status %q", ErrInvalidContract, p.Status)
	}
	return nil
}

// TerminalToToolResult converts a worker terminal claim to a harness result.
// The result remains untrusted evidence; Inspector/Orchestra must validate it.
func TerminalToToolResult(task TaskEnvelope, terminal TerminalPayload) ToolResult {
	status := ToolStatusFailed
	switch terminal.Status {
	case TerminalSuccess:
		status = ToolStatusSuccess
	case TerminalCanceled:
		status = ToolStatusCanceled
	}
	return ToolResult{
		ContractVersion: HarnessContractVersion,
		ExecutionID:     task.ExecutionID,
		TaskID:          task.TaskID,
		ToolName:        task.ToolName,
		Status:          status,
		Error:           terminal.Error,
		EvidenceIDs:     append([]string(nil), terminal.EvidenceIDs...),
		CompletedAt:     time.Now().UTC(),
		Redacted:        true,
	}
}

// TaskEnvelopeToExecutionEnvelope converts a protocol task into the harness
// envelope. The broker supplies the private worktree root and policy decision.
func TaskEnvelopeToExecutionEnvelope(task TaskEnvelope, worktreeRoot string, policy PolicyDecision, audit AuditMetadata) ExecutionEnvelope {
	return ExecutionEnvelope{
		ContractVersion: HarnessContractVersion,
		ExecutionID:     task.ExecutionID,
		TaskID:          task.TaskID,
		WorktreeID:      task.WorktreeID,
		ProjectID:       task.ProjectID,
		WorktreeRoot:    worktreeRoot,
		ToolName:        task.ToolName,
		Input:           task.Input,
		PolicyDecision:  policy,
		Timeout:         time.Duration(task.TimeoutMs) * time.Millisecond,
		Audit:           audit,
		TraceID:         task.TraceID,
	}
}

type CancelPayload struct {
	Reason string `json:"reason"`
}

func (p CancelPayload) Validate() error {
	if p.Reason == "" {
		return fmt.Errorf("%w: cancel reason is required", ErrInvalidContract)
	}
	return nil
}

type ShutdownPayload struct {
	Reason string `json:"reason,omitempty"`
}

func (p ShutdownPayload) Validate() error { return nil }

// ValidateHeartbeat returns ErrProtocolHeartbeatLost when the most recent
// heartbeat exceeds maxAge. It is useful to turn process silence into a
// deterministic adapter observation without claiming task success.
func (v *Validator) ValidateHeartbeat(now time.Time, maxAge time.Duration) error {
	if !v.HeartbeatHealthy(now, maxAge) {
		return ErrProtocolHeartbeatLost
	}
	return nil
}

// NewMessage creates and validates an envelope. Sequence numbers are assigned
// by the session/stream owner, not by worker payload constructors.
func NewMessage(typ MessageType, correlationID string, sequence uint64, payload any) (Message, error) {
	if !knownMessageType(typ) {
		return Message{}, fmt.Errorf("%w: %q", ErrProtocolUnknownMessage, typ)
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return Message{}, fmt.Errorf("%w: marshal payload: %v", ErrProtocolMalformed, err)
	}
	m := Message{Version: WorkerProtocolVersion, Type: typ, CorrelationID: correlationID, Sequence: sequence, Payload: raw}
	if err := ValidateMessage(m); err != nil {
		return Message{}, err
	}
	return m, nil
}

// ValidateMessage applies strict envelope and typed-payload validation without
// changing stream state.
func ValidateMessage(m Message) error {
	if m.Version != WorkerProtocolVersion || m.Type == "" || m.CorrelationID == "" || m.Sequence == 0 || len(m.Payload) == 0 {
		return fmt.Errorf("%w: missing or invalid envelope fields", ErrProtocolMalformed)
	}
	if !knownMessageType(m.Type) {
		return fmt.Errorf("%w: %q", ErrProtocolUnknownMessage, m.Type)
	}
	if len(m.Payload) > MaxProtocolLineBytes {
		return ErrProtocolLineTooLarge
	}
	if err := validateJSON(m.Payload); err != nil {
		return fmt.Errorf("%w: payload: %v", ErrProtocolMalformed, err)
	}
	var target interface{ Validate() error }
	switch m.Type {
	case MessageHandshake:
		target = new(HandshakePayload)
	case MessageCapabilities:
		target = new(CapabilitiesPayload)
	case MessageHealth:
		target = new(HealthPayload)
	case MessageTask:
		target = new(TaskEnvelope)
	case MessageProgress:
		target = new(ProgressPayload)
	case MessageEvidence:
		target = new(EvidencePayload)
	case MessageDiagnostic:
		target = new(DiagnosticPayload)
	case MessageHeartbeat:
		target = new(HeartbeatPayload)
	case MessageTerminal:
		target = new(TerminalPayload)
	case MessageCancel:
		target = new(CancelPayload)
	case MessageShutdown:
		target = new(ShutdownPayload)
	}
	if err := strictUnmarshal(m.Payload, target); err != nil {
		return fmt.Errorf("%w: payload for %s: %v", ErrProtocolMalformed, m.Type, err)
	}
	if err := target.Validate(); err != nil {
		return err
	}
	return nil
}

// DecodeLine strictly decodes one bounded JSONL record. It rejects duplicate
// JSON object keys as well as unknown fields in the envelope and payload.
func DecodeLine(line []byte) (Message, error) {
	if len(line) == 0 {
		return Message{}, fmt.Errorf("%w: empty line", ErrProtocolMalformed)
	}
	if line[len(line)-1] == '\n' {
		line = line[:len(line)-1]
		if len(line) > 0 && line[len(line)-1] == '\r' {
			line = line[:len(line)-1]
		}
	}
	if len(line) == 0 {
		return Message{}, fmt.Errorf("%w: empty line", ErrProtocolMalformed)
	}
	if len(line) > MaxProtocolLineBytes {
		return Message{}, ErrProtocolLineTooLarge
	}
	if err := validateJSON(line); err != nil {
		return Message{}, fmt.Errorf("%w: %v", ErrProtocolMalformed, err)
	}
	var m Message
	if err := strictUnmarshal(line, &m); err != nil {
		return Message{}, fmt.Errorf("%w: %v", ErrProtocolMalformed, err)
	}
	if err := ValidateMessage(m); err != nil {
		return Message{}, err
	}
	return m, nil
}

func EncodeLine(m Message) ([]byte, error) {
	if err := ValidateMessage(m); err != nil {
		return nil, err
	}
	line, err := json.Marshal(m)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrProtocolMalformed, err)
	}
	if len(line) > MaxProtocolLineBytes {
		return nil, ErrProtocolLineTooLarge
	}
	return append(line, '\n'), nil
}

// ProtocolState is the lifecycle state maintained by Validator.
type ProtocolState string

const (
	StateAwaitingHandshake ProtocolState = "awaiting_handshake"
	StateReady             ProtocolState = "ready"
	StateRunning           ProtocolState = "running"
	StateCancelRequested   ProtocolState = "cancel_requested"
	StateTerminated        ProtocolState = "terminated"
	StateClosed            ProtocolState = "closed"
)

// Validator enforces monotonically increasing sequence numbers and the worker
// lifecycle. It is safe for one reader and one writer only when externally
// synchronized; use Decoder/Encoder for concurrent stream plumbing.
type Validator struct {
	mu            sync.Mutex
	state         ProtocolState
	lastSeq       uint64
	correlation   string
	activeTask    string
	lastHeartbeat time.Time
}

func NewValidator() *Validator            { return &Validator{state: StateAwaitingHandshake} }
func (v *Validator) State() ProtocolState { v.mu.Lock(); defer v.mu.Unlock(); return v.state }

// HeartbeatHealthy reports whether a heartbeat has arrived within maxAge.
// A heartbeat is a liveness signal only; it never changes task authority.
func (v *Validator) HeartbeatHealthy(now time.Time, maxAge time.Duration) bool {
	v.mu.Lock()
	defer v.mu.Unlock()
	return !v.lastHeartbeat.IsZero() && maxAge > 0 && !now.Before(v.lastHeartbeat) && now.Sub(v.lastHeartbeat) <= maxAge
}

func (v *Validator) HeartbeatAge(now time.Time) (time.Duration, bool) {
	v.mu.Lock()
	defer v.mu.Unlock()
	if v.lastHeartbeat.IsZero() {
		return 0, false
	}
	if now.Before(v.lastHeartbeat) {
		return 0, true
	}
	return now.Sub(v.lastHeartbeat), true
}

func (v *Validator) Accept(m Message) error {
	v.mu.Lock()
	defer v.mu.Unlock()
	if err := ValidateMessage(m); err != nil {
		return err
	}
	if v.state == StateClosed {
		return ErrProtocolClosed
	}
	if m.Sequence != v.lastSeq+1 {
		return fmt.Errorf("%w: expected %d, got %d", ErrProtocolSequence, v.lastSeq+1, m.Sequence)
	}
	if v.state != StateAwaitingHandshake && m.CorrelationID != v.correlation {
		return fmt.Errorf("%w: expected %q, got %q", ErrProtocolCorrelation, v.correlation, m.CorrelationID)
	}
	if v.state == StateAwaitingHandshake {
		if m.Type != MessageHandshake {
			return fmt.Errorf("%w: handshake must be first", ErrProtocolLifecycle)
		}
		var p HandshakePayload
		if err := strictUnmarshal(m.Payload, &p); err != nil {
			return err
		}
		if p.Protocol != WorkerProtocolVersion {
			return fmt.Errorf("%w: unsupported worker protocol %q", ErrProtocolMalformed, p.Protocol)
		}
		v.correlation, v.state = m.CorrelationID, StateReady
	} else {
		if m.Type == MessageHandshake {
			return fmt.Errorf("%w: handshake is only allowed once", ErrProtocolDuplicate)
		}
		if m.Type == MessageShutdown {
			v.state = StateClosed
		} else if v.state == StateTerminated {
			return fmt.Errorf("%w: only shutdown is allowed after terminal", ErrProtocolLifecycle)
		} else if m.Type == MessageHeartbeat {
			var p HeartbeatPayload
			if err := strictUnmarshal(m.Payload, &p); err != nil {
				return err
			}
			v.lastHeartbeat = p.At
		} else if m.Type == MessageTask {
			if v.activeTask != "" {
				return fmt.Errorf("%w: task already active", ErrProtocolDuplicate)
			}
			if v.state != StateReady {
				return fmt.Errorf("%w: task delivery requires ready state, got %q", ErrProtocolLifecycle, v.state)
			}
			var p TaskEnvelope
			if err := strictUnmarshal(m.Payload, &p); err != nil {
				return err
			}
			v.activeTask, v.state = p.ExecutionID, StateRunning
		} else if m.Type == MessageProgress || m.Type == MessageEvidence {
			if v.state != StateRunning && v.state != StateCancelRequested {
				return fmt.Errorf("%w: %s requires an active running task", ErrProtocolLifecycle, m.Type)
			}
		} else if m.Type == MessageCancel {
			if v.state != StateRunning && v.state != StateCancelRequested {
				return fmt.Errorf("%w: cancellation requires an active task", ErrProtocolLifecycle)
			}
			v.state = StateCancelRequested
		} else if m.Type == MessageTerminal {
			if v.state != StateRunning && v.state != StateCancelRequested {
				return fmt.Errorf("%w: terminal requires an active task", ErrProtocolLifecycle)
			}
			v.state = StateTerminated
		}
	}
	v.lastSeq = m.Sequence
	return nil
}

// Decoder reads bounded records and applies lifecycle validation.
type Decoder struct {
	scanner   *bufio.Scanner
	validator *Validator
	messages  int
}

func NewDecoder(r io.Reader) *Decoder {
	s := bufio.NewScanner(r)
	s.Buffer(make([]byte, 4096), MaxProtocolLineBytes+1)
	return &Decoder{scanner: s, validator: NewValidator()}
}
func (d *Decoder) Next() (Message, error) {
	if d.messages >= MaxProtocolMessages {
		return Message{}, fmt.Errorf("%w: message count exceeds %d", ErrProtocolLineTooLarge, MaxProtocolMessages)
	}
	if !d.scanner.Scan() {
		if err := d.scanner.Err(); err != nil {
			return Message{}, ErrProtocolLineTooLarge
		}
		return Message{}, io.EOF
	}
	m, err := DecodeLine(d.scanner.Bytes())
	if err != nil {
		return Message{}, err
	}
	if err := d.validator.Accept(m); err != nil {
		return Message{}, err
	}
	d.messages++
	return m, nil
}
func (d *Decoder) State() ProtocolState { return d.validator.State() }

// Encoder writes validated records as one complete JSONL line.
type Encoder struct {
	mu sync.Mutex
	w  io.Writer
}

func NewEncoder(w io.Writer) *Encoder { return &Encoder{w: w} }
func (e *Encoder) Write(m Message) error {
	line, err := EncodeLine(m)
	if err != nil {
		return err
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	n, err := e.w.Write(line)
	if err != nil {
		return err
	}
	if n != len(line) {
		return io.ErrShortWrite
	}
	return nil
}

func knownMessageType(t MessageType) bool {
	switch t {
	case MessageHandshake, MessageCapabilities, MessageHealth, MessageTask, MessageProgress, MessageEvidence, MessageDiagnostic, MessageHeartbeat, MessageTerminal, MessageCancel, MessageShutdown:
		return true
	}
	return false
}

func strictUnmarshal(data []byte, dst any) error {
	d := json.NewDecoder(bytes.NewReader(data))
	d.DisallowUnknownFields()
	if err := d.Decode(dst); err != nil {
		return err
	}
	var extra any
	if err := d.Decode(&extra); err != io.EOF {
		if err == nil {
			return errors.New("multiple JSON values")
		}
		return err
	}
	return nil
}

// validateJSON parses every object and rejects duplicate keys. The standard
// library's Unmarshal intentionally accepts duplicate keys, which is unsafe at
// a process boundary because later keys silently overwrite earlier values.
func validateJSON(data []byte) error {
	d := json.NewDecoder(bytes.NewReader(data))
	d.UseNumber()
	if err := consumeJSONValue(d); err != nil {
		return err
	}
	var extra any
	if err := d.Decode(&extra); err != io.EOF {
		if err == nil {
			return errors.New("multiple JSON values")
		}
		return err
	}
	return nil
}
func consumeJSONValue(d *json.Decoder) error {
	t, err := d.Token()
	if err != nil {
		return err
	}
	switch x := t.(type) {
	case json.Delim:
		switch x {
		case '{':
			seen := map[string]struct{}{}
			for d.More() {
				key, err := d.Token()
				if err != nil {
					return err
				}
				ks, ok := key.(string)
				if !ok {
					return errors.New("object key is not a string")
				}
				if _, exists := seen[ks]; exists {
					return fmt.Errorf("duplicate object key %q", ks)
				}
				seen[ks] = struct{}{}
				if err := consumeJSONValue(d); err != nil {
					return err
				}
			}
			end, err := d.Token()
			if err != nil || end != json.Delim('}') {
				return errors.New("unterminated object")
			}
		case '[':
			for d.More() {
				if err := consumeJSONValue(d); err != nil {
					return err
				}
			}
			end, err := d.Token()
			if err != nil || end != json.Delim(']') {
				return errors.New("unterminated array")
			}
		default:
			return errors.New("unexpected delimiter")
		}
	}
	return nil
}
