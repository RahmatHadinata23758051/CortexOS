// Coordination source adapter.
//
// This adapter implements application.CoordinationSource by reading authoritative
// Orchestra task state and safe Staff summaries through their application ports.
// It performs no mutation and returns only validated domain records; the
// coordination projection validates and sanitizes again before bridge exposure.
package bridge

import (
	"context"

	"github.com/RahmatHadinata23758051/CortexOS/internal/cortex/application"
	"github.com/RahmatHadinata23758051/CortexOS/internal/cortex/orchestra"
)

// CoordinationSourceConfig configures the coordination read source.
type CoordinationSourceConfig struct {
	Orchestra application.OrchestraPort
	Staff     application.StaffPort
}

// CoordinationSourceAdapter projects authoritative Orchestra and Staff state.
type CoordinationSourceAdapter struct {
	orchestra application.OrchestraPort
	staff     application.StaffPort
}

// NewCoordinationSourceAdapter constructs the source and validates its config.
func NewCoordinationSourceAdapter(cfg CoordinationSourceConfig) (*CoordinationSourceAdapter, error) {
	if cfg.Orchestra == nil {
		return nil, application.ErrCoordinationInvalidRequest
	}
	return &CoordinationSourceAdapter{orchestra: cfg.Orchestra, staff: cfg.Staff}, nil
}

// ListTasks returns authoritative Orchestra tasks.
func (a *CoordinationSourceAdapter) ListTasks(ctx context.Context) ([]orchestra.Task, error) {
	return a.orchestra.ListTasks(ctx)
}

// ListStaff returns safe Staff summaries. When no Staff port is wired, an empty
// set is returned so coordination degrades without failing the snapshot.
func (a *CoordinationSourceAdapter) ListStaff(ctx context.Context) ([]application.StaffSummary, error) {
	if a.staff == nil {
		return nil, nil
	}
	return a.staff.ListStaff(ctx, application.StaffListFilter{})
}
