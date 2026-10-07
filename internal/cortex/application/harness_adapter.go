package application

import (
	"context"
	"fmt"
	"sort"

	"github.com/RahmatHadinata23758051/CortexOS/internal/cortex/harness"
	harnesssqlite "github.com/RahmatHadinata23758051/CortexOS/internal/cortex/harness/sqlite"
)

// HarnessAdapter implements HarnessPort using a SQLite store.
type HarnessAdapter struct {
	store  *harnesssqlite.Store
	broker harness.ToolBroker
}

// NewHarnessAdapter creates an adapter backed by a durable store and optional broker.
func NewHarnessAdapter(store *harnesssqlite.Store, broker harness.ToolBroker) *HarnessAdapter {
	return &HarnessAdapter{store: store, broker: broker}
}

// ListCapabilities returns capability bindings from the broker if present, otherwise the store.
func (a *HarnessAdapter) ListCapabilities(ctx context.Context) ([]HarnessCapabilityRecord, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if a == nil {
		return nil, harness.ErrStoreUnavailable
	}
	if a.broker != nil {
		caps := a.broker.ListCapabilities()
		result := make([]HarnessCapabilityRecord, 0, len(caps))
		for _, c := range caps {
			result = append(result, HarnessCapabilityRecord{
				Capability:    string(c.Capability),
				Description:   c.Description,
				RequiresAsk:   c.RequiresAsk,
				DefaultEffect: string(c.DefaultEffect),
			})
		}
		return result, nil
	}
	if a.store == nil {
		return nil, harness.ErrStoreUnavailable
	}
	caps, err := a.store.ListCapabilities(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]HarnessCapabilityRecord, 0, len(caps))
	for _, c := range caps {
		result = append(result, HarnessCapabilityRecord{
			Capability:    string(c.Capability),
			Description:   c.Description,
			RequiresAsk:   c.RequiresAsk,
			DefaultEffect: string(c.DefaultEffect),
		})
	}
	return result, nil
}

// ListWorkers returns worker health records from the store.
func (a *HarnessAdapter) ListWorkers(ctx context.Context) ([]HarnessWorkerRecord, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if a == nil || a.store == nil {
		return nil, harness.ErrStoreUnavailable
	}
	workers, err := a.store.ListWorkers(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]HarnessWorkerRecord, 0, len(workers))
	for _, w := range workers {
		result = append(result, HarnessWorkerRecord{ID: w.ID, InstanceID: w.InstanceID, Kind: string(w.Kind), Class: string(w.Class), Version: w.Version, WorkerSchema: w.SchemaVersion, OwnerPID: w.OwnerPID, OwnerProcessID: w.OwnerProcessID, RegisteredAt: w.RegisteredAt, LastHeartbeatAt: w.LastHeartbeatAt, Status: w.Status, Metadata: w.Metadata})
	}
	return result, nil
}

// ListActiveExecutions returns active execution records from the store.
func (a *HarnessAdapter) ListActiveExecutions(ctx context.Context) ([]HarnessExecutionRecord, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if a == nil || a.store == nil {
		return nil, harness.ErrStoreUnavailable
	}
	execs, err := a.store.ListActiveExecutions(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]HarnessExecutionRecord, 0, len(execs))
	for _, e := range execs {
		result = append(result, HarnessExecutionRecord{ID: e.ID, TaskID: e.TaskID, EngineClass: string(e.EngineClass), Priority: fmt.Sprintf("%d", e.Priority), MemoryMB: e.MemoryMB, CPUPriority: e.CPUPriority, TraceID: e.TraceID, WorkerID: e.OwnerWorkerID, CreatedAt: e.CreatedAt, WorktreePath: "", ProcessHandle: nil, RawInput: nil})
	}
	return result, nil
}

// ListEvidence returns redacted evidence records for an execution.
func (a *HarnessAdapter) ListEvidence(ctx context.Context, executionID string) ([]HarnessEvidenceRecord, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if executionID == "" {
		return nil, harness.ErrInvalidContract
	}
	if a == nil || a.store == nil {
		return nil, harness.ErrStoreUnavailable
	}
	evidence, err := a.store.ListEvidence(ctx, executionID)
	if err != nil {
		return nil, err
	}
	result := make([]HarnessEvidenceRecord, 0, len(evidence))
	for _, e := range evidence {
		audit := map[string]any{}
		_ = unmarshalAudit(e.Audit, &audit)
		result = append(result, HarnessEvidenceRecord{ID: e.ID, ExecutionID: e.ExecutionID, TaskID: e.TaskID, WorktreeID: e.WorktreeID, Kind: string(e.Kind), Digest: e.Digest, RedactedPayload: e.RedactedPayload, RawProvider: nil, Audit: audit, CollectedAt: e.CollectedAt})
	}
	sort.Slice(result, func(i, j int) bool { return result[i].ID < result[j].ID })
	return result, nil
}

func unmarshalAudit(audit harness.AuditMetadata, target *map[string]any) error {
	if audit.TraceID != "" {
		(*target)["traceId"] = audit.TraceID
	}
	if audit.Actor != "" {
		(*target)["actor"] = audit.Actor
	}
	if audit.Adapter != "" {
		(*target)["adapter"] = audit.Adapter
	}
	if audit.PolicyRuleID != "" {
		(*target)["policyRuleId"] = audit.PolicyRuleID
	}
	if audit.CorrelationID != "" {
		(*target)["correlationId"] = audit.CorrelationID
	}
	return nil
}
