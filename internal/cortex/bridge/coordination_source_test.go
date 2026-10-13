package bridge

import (
	"context"
	"testing"

	"github.com/RahmatHadinata23758051/CortexOS/internal/cortex/application"
	"github.com/RahmatHadinata23758051/CortexOS/internal/cortex/orchestra"
	"github.com/RahmatHadinata23758051/CortexOS/internal/cortex/staff"
)

type coordinationOrchestraFake struct {
	tasks []orchestra.Task
}

func (f coordinationOrchestraFake) ListTasks(context.Context) ([]orchestra.Task, error) {
	return f.tasks, nil
}
func (coordinationOrchestraFake) GetTask(context.Context, orchestra.TaskID) (orchestra.Task, error) {
	return orchestra.Task{}, nil
}
func (coordinationOrchestraFake) ListEvents(context.Context, orchestra.TaskID) ([]orchestra.TaskEvent, error) {
	return nil, nil
}
func (coordinationOrchestraFake) CancelTask(context.Context, orchestra.TaskID) error { return nil }
func (coordinationOrchestraFake) RetryTask(context.Context, orchestra.TaskID) error  { return nil }
func (coordinationOrchestraFake) MergeTask(context.Context, orchestra.TaskID, string) (orchestra.Task, error) {
	return orchestra.Task{}, nil
}
func (coordinationOrchestraFake) CreateTask(context.Context, orchestra.Task) (orchestra.Task, error) {
	return orchestra.Task{}, nil
}

type coordinationStaffFake struct {
	items []application.StaffSummary
}

func (f coordinationStaffFake) GetStaff(context.Context, staff.StaffID) (application.StaffSummary, error) {
	return application.StaffSummary{}, nil
}
func (f coordinationStaffFake) ListStaff(context.Context, application.StaffListFilter) ([]application.StaffSummary, error) {
	return f.items, nil
}
func (f coordinationStaffFake) ListStaffByWorkspace(context.Context, staff.WorkspaceID) ([]application.StaffSummary, error) {
	return f.items, nil
}
func (coordinationStaffFake) GetStaffCapabilities(context.Context) ([]application.StaffCapabilitySummary, error) {
	return nil, nil
}
func (coordinationStaffFake) GetStaffAssignment(context.Context, staff.StaffID) (application.StaffAssignmentSummary, error) {
	return application.StaffAssignmentSummary{}, nil
}

func TestCoordinationSourceAdapterReadsAuthoritativePorts(t *testing.T) {
	task := orchestra.Task{ID: "task-1", ProjectID: "project-1", Title: "Read facts", Status: orchestra.TaskReady}
	staffItem := application.StaffSummary{ID: "staff-1", Name: "Planner"}
	adapter, err := NewCoordinationSourceAdapter(CoordinationSourceConfig{
		Orchestra: coordinationOrchestraFake{tasks: []orchestra.Task{task}},
		Staff:     coordinationStaffFake{items: []application.StaffSummary{staffItem}},
	})
	if err != nil {
		t.Fatalf("NewCoordinationSourceAdapter: %v", err)
	}
	gotTasks, err := adapter.ListTasks(context.Background())
	if err != nil || len(gotTasks) != 1 || gotTasks[0].ID != task.ID {
		t.Fatalf("ListTasks = %#v, %v", gotTasks, err)
	}
	gotStaff, err := adapter.ListStaff(context.Background())
	if err != nil || len(gotStaff) != 1 || gotStaff[0].ID != staffItem.ID {
		t.Fatalf("ListStaff = %#v, %v", gotStaff, err)
	}
}

func TestCoordinationSourceAdapterAllowsMissingStaffPort(t *testing.T) {
	adapter, err := NewCoordinationSourceAdapter(CoordinationSourceConfig{
		Orchestra: coordinationOrchestraFake{},
	})
	if err != nil {
		t.Fatalf("NewCoordinationSourceAdapter: %v", err)
	}
	items, err := adapter.ListStaff(context.Background())
	if err != nil || len(items) != 0 {
		t.Fatalf("ListStaff = %#v, %v; want empty without error", items, err)
	}
}
