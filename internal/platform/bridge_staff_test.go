package platform

import (
	"context"
	"testing"
	"time"

	"github.com/RahmatHadinata23758051/CortexOS/internal/cortex/application"
	"github.com/RahmatHadinata23758051/CortexOS/internal/cortex/staff"
)

type bridgeMockStaffPort struct{}

func (b *bridgeMockStaffPort) GetStaff(ctx context.Context, id staff.StaffID) (application.StaffSummary, error) {
	return application.StaffSummary{
		ID:            id,
		Name:          "Bridge Staff",
		Role:          staff.RoleImplementer,
		Capabilities:  []staff.Capability{"coding"},
		WorkspaceID:   "ws-bridge",
		Lifecycle:     staff.LifecycleActive,
		Availability:  staff.AvailabilityAvailable,
		SchemaVersion: application.StaffBridgeSchemaVersion,
		CreatedAt:     time.Now().UTC(),
		UpdatedAt:     time.Now().UTC(),
	}, nil
}

func (b *bridgeMockStaffPort) ListStaff(ctx context.Context, filter application.StaffListFilter) ([]application.StaffSummary, error) {
	s, _ := b.GetStaff(ctx, "staff-bridge-1")
	return []application.StaffSummary{s}, nil
}

func (b *bridgeMockStaffPort) ListStaffByWorkspace(ctx context.Context, workspaceID staff.WorkspaceID) ([]application.StaffSummary, error) {
	return b.ListStaff(ctx, application.StaffListFilter{})
}

func (b *bridgeMockStaffPort) GetStaffCapabilities(ctx context.Context) ([]application.StaffCapabilitySummary, error) {
	return []application.StaffCapabilitySummary{
		{
			Capability:    "coding",
			RequiredTools: []string{"coding"},
			SchemaVersion: application.StaffBridgeSchemaVersion,
		},
	}, nil
}

func (b *bridgeMockStaffPort) GetStaffAssignment(ctx context.Context, id staff.StaffID) (application.StaffAssignmentSummary, error) {
	return application.StaffAssignmentSummary{
		StaffID:       id,
		WorkspaceID:   "ws-bridge",
		SchemaVersion: application.StaffBridgeSchemaVersion,
	}, nil
}

type bridgeMockKnowledgePort struct{}

func (b *bridgeMockKnowledgePort) ListKnowledgeSources(ctx context.Context, projectID string) ([]application.KnowledgeSourceSummary, error) {
	return []application.KnowledgeSourceSummary{
		{
			ID:            "know-1",
			ProjectID:     projectID,
			Title:         "Doc 1",
			RelativePath:  "docs/doc1.md",
			SchemaVersion: application.KnowledgeBridgeSchemaVersion,
		},
	}, nil
}

func (b *bridgeMockKnowledgePort) QueryKnowledge(ctx context.Context, projectID, query string, limit int) ([]application.KnowledgeQueryResult, error) {
	return []application.KnowledgeQueryResult{
		{
			DocumentID:    "know-1",
			Title:         "Doc 1",
			RelativePath:  "docs/doc1.md",
			Snippet:       "query match",
			Score:         0.9,
			SchemaVersion: application.KnowledgeBridgeSchemaVersion,
		},
	}, nil
}

func TestBridgeStaffAndKnowledgeMethods(t *testing.T) {
	svc := application.NewServiceWithFull(nil, nil, nil, &bridgeMockStaffPort{}, &bridgeMockKnowledgePort{})
	bridge := NewBridge(svc)

	// Staff Summary
	summary, err := bridge.GetStaffSummary(application.StaffRequest{
		SchemaVersion: application.StaffBridgeSchemaVersion,
		StaffID:       "staff-bridge-1",
	})
	if err != nil {
		t.Fatalf("GetStaffSummary failed: %v", err)
	}
	if summary.ID != "staff-bridge-1" {
		t.Errorf("expected staff-bridge-1, got %s", summary.ID)
	}

	// Staff List
	list, err := bridge.ListStaffSummaries(application.StaffListRequest{
		SchemaVersion: application.StaffBridgeSchemaVersion,
	})
	if err != nil {
		t.Fatalf("ListStaffSummaries failed: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("expected 1 staff item, got %d", len(list))
	}

	// Staff by workspace
	wsList, err := bridge.ListStaffByWorkspace(application.StaffByWorkspaceRequest{
		SchemaVersion: application.StaffBridgeSchemaVersion,
		WorkspaceID:   "ws-bridge",
	})
	if err != nil {
		t.Fatalf("ListStaffByWorkspace failed: %v", err)
	}
	if len(wsList) != 1 {
		t.Fatalf("expected 1 staff item in workspace, got %d", len(wsList))
	}

	// Staff Capabilities
	caps, err := bridge.ListStaffCapabilities(application.StaffCapabilitiesRequest{
		SchemaVersion: application.StaffBridgeSchemaVersion,
	})
	if err != nil {
		t.Fatalf("ListStaffCapabilities failed: %v", err)
	}
	if len(caps) != 1 {
		t.Fatalf("expected 1 capability, got %d", len(caps))
	}

	// Staff Assignment
	assign, err := bridge.GetStaffAssignment(application.StaffAssignmentRequest{
		SchemaVersion: application.StaffBridgeSchemaVersion,
		StaffID:       "staff-bridge-1",
	})
	if err != nil {
		t.Fatalf("GetStaffAssignment failed: %v", err)
	}
	if assign.WorkspaceID != "ws-bridge" {
		t.Errorf("expected ws-bridge, got %s", assign.WorkspaceID)
	}

	// Knowledge sources
	sources, err := bridge.ListKnowledgeSources(application.KnowledgeSourcesRequest{
		SchemaVersion: application.KnowledgeBridgeSchemaVersion,
		ProjectID:     "proj-bridge",
	})
	if err != nil {
		t.Fatalf("ListKnowledgeSources failed: %v", err)
	}
	if len(sources) != 1 {
		t.Fatalf("expected 1 knowledge source, got %d", len(sources))
	}

	// Knowledge query
	results, err := bridge.QueryKnowledge(application.KnowledgeQueryRequest{
		SchemaVersion: application.KnowledgeBridgeSchemaVersion,
		ProjectID:     "proj-bridge",
		Query:         "match",
	})
	if err != nil {
		t.Fatalf("QueryKnowledge failed: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
}
