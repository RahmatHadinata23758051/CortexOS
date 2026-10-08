package application

import (
	"context"
	"errors"
	"fmt"

	"github.com/RahmatHadinata23758051/CortexOS/internal/cortex/workspace"
)

const runtimeSchemaVersion = "cortexos.runtime.v1"

var ErrInvalidRequest = errors.New("invalid runtime snapshot request")

// SnapshotRequest identifies the version of the bridge contract requested by a client.
type SnapshotRequest struct {
	SchemaVersion string `json:"schemaVersion"`
}

// RuntimeSnapshot is the deterministic read-only response exposed by the first bridge slice.
type RuntimeSnapshot struct {
	SchemaVersion string `json:"schemaVersion"`
	Status        string `json:"status"`
	Environment   string `json:"environment"`
	Provider      string `json:"provider"`
}

// Service owns application behavior and does not depend on Wails or frontend packages.
type Service struct {
	workspace *workspace.Service
	orchestra OrchestraPort
	harness   HarnessPort
	staff     StaffPort
	knowledge KnowledgePort
}

// NewService creates the minimal Phase 1 application service.
func NewService() *Service {
	return &Service{}
}

// NewServiceWithWorkspace adds the optional Workspace application boundary
// without coupling the application package to Wails or frontend DTOs.
func NewServiceWithWorkspace(workspaceService *workspace.Service) *Service {
	return &Service{workspace: workspaceService}
}

// NewServiceWithOrchestra adds the optional Orchestra application boundary.
func NewServiceWithOrchestra(orchestraService OrchestraPort) *Service {
	return &Service{orchestra: orchestraService}
}

// NewServiceWithWorkspaceAndOrchestra adds both boundaries.
func NewServiceWithWorkspaceAndOrchestra(workspaceService *workspace.Service, orchestraService OrchestraPort) *Service {
	return &Service{workspace: workspaceService, orchestra: orchestraService}
}

// NewServiceWithHarness adds the optional Harness observability boundary.
func NewServiceWithHarness(harnessService HarnessPort) *Service {
	return &Service{harness: harnessService}
}

// NewServiceWithAll combines Workspace, Orchestra, and Harness application boundaries.
func NewServiceWithAll(workspaceService *workspace.Service, orchestraService OrchestraPort, harnessService HarnessPort) *Service {
	return &Service{workspace: workspaceService, orchestra: orchestraService, harness: harnessService}
}

// NewServiceWithStaff adds the optional Staff observability boundary.
func NewServiceWithStaff(staffService StaffPort) *Service {
	return &Service{staff: staffService}
}

// NewServiceWithKnowledge adds the optional Knowledge observability boundary.
func NewServiceWithKnowledge(knowledgeService KnowledgePort) *Service {
	return &Service{knowledge: knowledgeService}
}

// NewServiceWithFull combines all available application boundaries.
func NewServiceWithFull(workspaceService *workspace.Service, orchestraService OrchestraPort, harnessService HarnessPort, staffService StaffPort, knowledgeService KnowledgePort) *Service {
	return &Service{workspace: workspaceService, orchestra: orchestraService, harness: harnessService, staff: staffService, knowledge: knowledgeService}
}

// GetRuntimeSnapshot returns a deterministic local runtime status.
func (s *Service) GetRuntimeSnapshot(ctx context.Context, request SnapshotRequest) (RuntimeSnapshot, error) {
	if err := ctx.Err(); err != nil {
		return RuntimeSnapshot{}, err
	}
	if request.SchemaVersion != runtimeSchemaVersion {
		return RuntimeSnapshot{}, fmt.Errorf("%w: schema version %q is not supported", ErrInvalidRequest, request.SchemaVersion)
	}

	return RuntimeSnapshot{
		SchemaVersion: runtimeSchemaVersion,
		Status:        "ready",
		Environment:   "local",
		Provider:      "disabled",
	}, nil
}

// RuntimeSchemaVersion returns the supported bridge schema identifier.
func RuntimeSchemaVersion() string {
	return runtimeSchemaVersion
}
