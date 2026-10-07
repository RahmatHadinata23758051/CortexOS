package application

import (
	"context"
	"errors"
	"time"

	"github.com/RahmatHadinata23758051/CortexOS/internal/cortex/harness"
)

// HarnessBridgeSchemaVersion is the stable contract version for Harness observability DTOs.
const HarnessBridgeSchemaVersion = "cortexos.harness.bridge.v1"

var ErrHarnessInvalidRequest = errors.New("invalid harness bridge request")

// HarnessPort is the application boundary for Harness observability. Implementations
// may read from a broker, worker registry, or durable store, but callers receive only
// typed records that the application mapper can safely reduce to bridge DTOs.
// Context is passed unchanged to every operation and must be honored by implementations.
type HarnessPort interface {
	ListCapabilities(context.Context) ([]HarnessCapabilityRecord, error)
	ListWorkers(context.Context) ([]HarnessWorkerRecord, error)
	ListActiveExecutions(context.Context) ([]HarnessExecutionRecord, error)
	ListEvidence(context.Context, string) ([]HarnessEvidenceRecord, error)
}

// HarnessCapabilityRecord is an internal application record. Only the fields selected
// by the mapper are serialized across Wails.
type HarnessCapabilityRecord struct {
	Capability    string
	Description   string
	RequiresAsk   bool
	DefaultEffect string
}

// HarnessWorkerRecord contains internal worker state. Owner PID, process identity, and
// metadata are intentionally never copied to HarnessWorkerSummary.
type HarnessWorkerRecord struct {
	ID              string
	InstanceID      string
	Kind            string
	Class           string
	Version         string
	WorkerSchema    string
	OwnerPID        int
	OwnerProcessID  string
	RegisteredAt    time.Time
	LastHeartbeatAt time.Time
	Status          string
	Metadata        map[string]any
}

// HarnessExecutionRecord contains internal reservation state. The provider must keep
// any provider-specific handles and paths here; none are serialized by this package.
type HarnessExecutionRecord struct {
	ID            string
	TaskID        string
	EngineClass   string
	Priority      string
	MemoryMB      int
	CPUPriority   int
	TraceID       string
	WorkerID      string
	CreatedAt     time.Time
	WorktreePath  string
	ProcessHandle any
	RawInput      []byte
}

// HarnessEvidenceRecord contains raw internal evidence. Raw payload and audit data are
// deliberately omitted from the public summary.
type HarnessEvidenceRecord struct {
	ID              string
	ExecutionID     string
	TaskID          string
	WorktreeID      string
	Kind            string
	Digest          string
	RedactedPayload []byte
	RawProvider     []byte
	Audit           map[string]any
	CollectedAt     time.Time
}

// HarnessCapabilitySummary is the safe, serialized form of a capability binding.
type HarnessCapabilitySummary struct {
	Capability    string `json:"capability"`
	Description   string `json:"description,omitempty"`
	RequiresAsk   bool   `json:"requiresAsk,omitempty"`
	DefaultEffect string `json:"defaultEffect,omitempty"`
	SchemaVersion string `json:"schemaVersion"`
}

// HarnessWorkerSummary is the safe, serialized form of worker health. It does not
// expose PID, process handles, owner process ID, or raw metadata.
type HarnessWorkerSummary struct {
	ID              string    `json:"id"`
	InstanceID      string    `json:"instanceId"`
	Kind            string    `json:"kind"`
	Class           string    `json:"class"`
	Version         string    `json:"version"`
	WorkerSchema    string    `json:"workerSchema"`
	Status          string    `json:"status"`
	RegisteredAt    time.Time `json:"registeredAt"`
	LastHeartbeatAt time.Time `json:"lastHeartbeatAt"`
	SchemaVersion   string    `json:"schemaVersion"`
}

// HarnessExecutionSummary is the safe, serialized form of an active execution.
// Worktree paths, sandbox configuration, raw input, and process handles are excluded.
type HarnessExecutionSummary struct {
	ID            string    `json:"id"`
	TaskID        string    `json:"taskId"`
	EngineClass   string    `json:"engineClass"`
	Priority      string    `json:"priority"`
	MemoryMB      int       `json:"memoryMB"`
	CPUPriority   int       `json:"cpuPriority"`
	TraceID       string    `json:"traceId"`
	WorkerID      string    `json:"workerId"`
	CreatedAt     time.Time `json:"createdAt"`
	SchemaVersion string    `json:"schemaVersion"`
}

// HarnessEvidenceSummary is a metadata-only evidence summary. It does not expose raw
// provider output, payload, audit metadata, secrets, or filesystem paths.
type HarnessEvidenceSummary struct {
	ID            string    `json:"id"`
	ExecutionID   string    `json:"executionId"`
	TaskID        string    `json:"taskId"`
	WorktreeID    string    `json:"worktreeId"`
	Kind          string    `json:"kind"`
	Digest        string    `json:"digest"`
	CollectedAt   time.Time `json:"collectedAt"`
	SchemaVersion string    `json:"schemaVersion"`
}

type HarnessCapabilitiesRequest struct {
	SchemaVersion string `json:"schemaVersion"`
}
type HarnessWorkersRequest struct {
	SchemaVersion string `json:"schemaVersion"`
}
type HarnessActiveExecutionsRequest struct {
	SchemaVersion string `json:"schemaVersion"`
}
type HarnessEvidenceRequest struct {
	SchemaVersion string `json:"schemaVersion"`
	ExecutionID   string `json:"executionId"`
}

// HarnessBridgeError is the stable, generic error returned across the bridge.
type HarnessBridgeError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func (e *HarnessBridgeError) Error() string {
	if e == nil {
		return "<nil>"
	}
	return e.Code + ": " + e.Message
}

func (s *Service) ListHarnessCapabilities(ctx context.Context, request HarnessCapabilitiesRequest) ([]HarnessCapabilitySummary, error) {
	if err := validateHarnessRequest(ctx, request.SchemaVersion); err != nil {
		return nil, err
	}
	if s == nil || s.harness == nil {
		return nil, harnessBridgeError(harness.ErrStoreUnavailable)
	}
	records, err := s.harness.ListCapabilities(ctx)
	if err != nil {
		return nil, mapHarnessError(err)
	}
	result := make([]HarnessCapabilitySummary, 0, len(records))
	for _, record := range records {
		result = append(result, HarnessCapabilitySummary{Capability: record.Capability, Description: record.Description, RequiresAsk: record.RequiresAsk, DefaultEffect: record.DefaultEffect, SchemaVersion: HarnessBridgeSchemaVersion})
	}
	return result, nil
}

func (s *Service) ListHarnessWorkers(ctx context.Context, request HarnessWorkersRequest) ([]HarnessWorkerSummary, error) {
	if err := validateHarnessRequest(ctx, request.SchemaVersion); err != nil {
		return nil, err
	}
	if s == nil || s.harness == nil {
		return nil, harnessBridgeError(harness.ErrStoreUnavailable)
	}
	records, err := s.harness.ListWorkers(ctx)
	if err != nil {
		return nil, mapHarnessError(err)
	}
	result := make([]HarnessWorkerSummary, 0, len(records))
	for _, record := range records {
		result = append(result, HarnessWorkerSummary{ID: record.ID, InstanceID: record.InstanceID, Kind: record.Kind, Class: record.Class, Version: record.Version, WorkerSchema: record.WorkerSchema, Status: record.Status, RegisteredAt: record.RegisteredAt, LastHeartbeatAt: record.LastHeartbeatAt, SchemaVersion: HarnessBridgeSchemaVersion})
	}
	return result, nil
}

func (s *Service) ListActiveHarnessExecutions(ctx context.Context, request HarnessActiveExecutionsRequest) ([]HarnessExecutionSummary, error) {
	if err := validateHarnessRequest(ctx, request.SchemaVersion); err != nil {
		return nil, err
	}
	if s == nil || s.harness == nil {
		return nil, harnessBridgeError(harness.ErrStoreUnavailable)
	}
	records, err := s.harness.ListActiveExecutions(ctx)
	if err != nil {
		return nil, mapHarnessError(err)
	}
	result := make([]HarnessExecutionSummary, 0, len(records))
	for _, record := range records {
		result = append(result, HarnessExecutionSummary{ID: record.ID, TaskID: record.TaskID, EngineClass: record.EngineClass, Priority: record.Priority, MemoryMB: record.MemoryMB, CPUPriority: record.CPUPriority, TraceID: record.TraceID, WorkerID: record.WorkerID, CreatedAt: record.CreatedAt, SchemaVersion: HarnessBridgeSchemaVersion})
	}
	return result, nil
}

func (s *Service) GetHarnessEvidence(ctx context.Context, request HarnessEvidenceRequest) ([]HarnessEvidenceSummary, error) {
	if err := validateHarnessRequest(ctx, request.SchemaVersion); err != nil {
		return nil, err
	}
	if request.ExecutionID == "" {
		return nil, harnessBridgeError(harness.ErrInvalidContract)
	}
	if s == nil || s.harness == nil {
		return nil, harnessBridgeError(harness.ErrStoreUnavailable)
	}
	records, err := s.harness.ListEvidence(ctx, request.ExecutionID)
	if err != nil {
		return nil, mapHarnessError(err)
	}
	result := make([]HarnessEvidenceSummary, 0, len(records))
	for _, record := range records {
		result = append(result, HarnessEvidenceSummary{ID: record.ID, ExecutionID: record.ExecutionID, TaskID: record.TaskID, WorktreeID: record.WorktreeID, Kind: record.Kind, Digest: record.Digest, CollectedAt: record.CollectedAt, SchemaVersion: HarnessBridgeSchemaVersion})
	}
	return result, nil
}

func validateHarnessRequest(ctx context.Context, version string) error {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return mapHarnessError(err)
	}
	if version != HarnessBridgeSchemaVersion {
		return &HarnessBridgeError{Code: "harness.unsupported_version", Message: "harness bridge contract version is unsupported"}
	}
	return nil
}

func mapHarnessError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return &HarnessBridgeError{Code: "harness.canceled", Message: "harness request canceled"}
	}
	return harnessBridgeError(err)
}

func harnessBridgeError(err error) error {
	code := "harness.internal"
	switch {
	case errors.Is(err, harness.ErrNotFound):
		code = "harness.not_found"
	case errors.Is(err, harness.ErrInvalidContract):
		code = "harness.invalid_request"
	case errors.Is(err, harness.ErrStoreUnavailable):
		code = "harness.storage_unavailable"
	case errors.Is(err, harness.ErrOwnership):
		code = "harness.ownership"
	}
	messages := map[string]string{
		"harness.not_found":           "harness record was not found",
		"harness.invalid_request":     "harness request is invalid",
		"harness.storage_unavailable": "harness state is unavailable",
		"harness.ownership":           "harness operation not permitted",
		"harness.internal":            "harness request failed",
	}
	return &HarnessBridgeError{Code: code, Message: messages[code]}
}
