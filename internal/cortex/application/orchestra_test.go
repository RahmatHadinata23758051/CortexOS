package application

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/RahmatHadinata23758051/CortexOS/internal/cortex/orchestra"
)

type orchestraPortStub struct {
	tasks    []orchestra.Task
	events   map[orchestra.TaskID][]orchestra.TaskEvent
	err      error
	canceled orchestra.TaskID
	retried  orchestra.TaskID
	merged   orchestra.TaskID
}

func (s *orchestraPortStub) ListTasks(context.Context) ([]orchestra.Task, error) {
	return s.tasks, s.err
}
func (s *orchestraPortStub) GetTask(_ context.Context, id orchestra.TaskID) (orchestra.Task, error) {
	for _, task := range s.tasks {
		if task.ID == id {
			return task, nil
		}
	}
	return orchestra.Task{}, orchestra.ErrNotFound
}
func (s *orchestraPortStub) ListEvents(_ context.Context, id orchestra.TaskID) ([]orchestra.TaskEvent, error) {
	return s.events[id], s.err
}
func (s *orchestraPortStub) CancelTask(_ context.Context, id orchestra.TaskID) error {
	s.canceled = id
	return s.err
}
func (s *orchestraPortStub) RetryTask(_ context.Context, id orchestra.TaskID) error {
	s.retried = id
	return s.err
}
func (s *orchestraPortStub) MergeTask(_ context.Context, id orchestra.TaskID, _ string) (orchestra.Task, error) {
	s.merged = id
	if s.err != nil {
		return orchestra.Task{}, s.err
	}
	for _, task := range s.tasks {
		if task.ID == id {
			task.Status = orchestra.TaskSuccess
			return task, nil
		}
	}
	return orchestra.Task{}, orchestra.ErrNotFound
}

func TestOrchestraBridgeReturnsStablePathFreeDTOs(t *testing.T) {
	stub := &orchestraPortStub{
		tasks: []orchestra.Task{{
			ID: "task-1", ProjectID: "project-1", WorktreeID: "worktree-1", Title: "task",
			AcceptanceCriteria: []string{"criterion"}, Dependencies: []orchestra.TaskID{"task-0"},
			Status: orchestra.TaskAwaitingInspection, AttemptCount: 1, MaxAttempts: 2,
		}},
		events: map[orchestra.TaskID][]orchestra.TaskEvent{"task-1": {
			{Sequence: 2, Type: orchestra.EventEvidenceCollected, From: orchestra.TaskRunning, To: orchestra.TaskAwaitingInspection, OccurredAt: time.Date(2026, 10, 7, 1, 0, 0, 0, time.UTC), EvidenceIDs: []string{"evidence-b", "evidence-a"}},
		}},
	}
	service := NewServiceWithOrchestra(stub)
	list, err := service.ListOrchestraTasks(context.Background(), OrchestraTaskListRequest{SchemaVersion: OrchestraBridgeSchemaVersion, Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	wantList := []OrchestraTaskSummary{{ID: "task-1", ProjectID: "project-1", Status: string(orchestra.TaskAwaitingInspection), Attempt: 1, MaxAttempts: 2, Progress: 80, SchemaVersion: OrchestraBridgeSchemaVersion}}
	if !reflect.DeepEqual(list, wantList) {
		t.Fatalf("list = %#v, want %#v", list, wantList)
	}

	detail, err := service.GetOrchestraTask(context.Background(), OrchestraTaskRequest{SchemaVersion: OrchestraBridgeSchemaVersion, TaskID: "task-1"})
	if err != nil {
		t.Fatal(err)
	}
	if detail.ID != "task-1" || detail.Progress != 80 || len(detail.Timeline) != 1 || detail.Timeline[0].EvidenceIDs[0] != "evidence-a" {
		t.Fatalf("detail = %#v", detail)
	}
	encoded := detail.ID + detail.ProjectID + detail.Status + detail.Timeline[0].Type + detail.Evidence[0].ID
	if strings.Contains(encoded, `C:\`) || strings.Contains(encoded, "/") {
		t.Fatal("orchestra DTO contains a filesystem path")
	}
}

func TestOrchestraBridgeMutationsAndVersionErrors(t *testing.T) {
	stub := &orchestraPortStub{}
	service := NewServiceWithOrchestra(stub)
	request := OrchestraTaskRequest{SchemaVersion: OrchestraBridgeSchemaVersion, TaskID: "task-1"}
	if err := service.CancelOrchestraTask(context.Background(), request); err != nil {
		t.Fatal(err)
	}
	if stub.canceled != "task-1" {
		t.Fatalf("canceled task = %q", stub.canceled)
	}
	if err := service.RetryOrchestraTask(context.Background(), request); err != nil {
		t.Fatal(err)
	}
	if stub.retried != "task-1" {
		t.Fatalf("retried task = %q", stub.retried)
	}
	_, err := service.ListOrchestraTasks(context.Background(), OrchestraTaskListRequest{SchemaVersion: "wrong", Limit: 10})
	var bridgeErr *OrchestraBridgeError
	if !errors.As(err, &bridgeErr) || bridgeErr.Code != "orchestra.unsupported_version" || strings.Contains(bridgeErr.Error(), "wrong") {
		t.Fatalf("version error = %#v", err)
	}
}

func TestOrchestraBridgeMapsUnavailableAndInvalidRequests(t *testing.T) {
	service := NewService()
	_, err := service.ListOrchestraTasks(context.Background(), OrchestraTaskListRequest{SchemaVersion: OrchestraBridgeSchemaVersion, Limit: 10})
	var unavailable *OrchestraBridgeError
	if !errors.As(err, &unavailable) || unavailable.Code != "orchestra.storage_unavailable" {
		t.Fatalf("unavailable error = %#v", err)
	}

	service = NewServiceWithOrchestra(&orchestraPortStub{})
	_, err = service.ListOrchestraTasks(context.Background(), OrchestraTaskListRequest{SchemaVersion: OrchestraBridgeSchemaVersion, Limit: 0})
	var invalid *OrchestraBridgeError
	if !errors.As(err, &invalid) || invalid.Code != "orchestra.invalid_request" {
		t.Fatalf("invalid error = %#v", err)
	}
}

func TestOrchestraMergeBridgeReturnsAcceptedTask(t *testing.T) {
	stub := &orchestraPortStub{tasks: []orchestra.Task{{
		ID: "task-1", ProjectID: "project-1", WorktreeID: "worktree-1", Title: "task",
		AcceptanceCriteria: []string{"criterion"}, Status: orchestra.TaskAwaitingInspection,
		AttemptCount: 1, MaxAttempts: 2,
	}}}
	service := NewServiceWithOrchestra(stub)
	result, err := service.MergeOrchestraTask(context.Background(), OrchestraMergeRequest{
		SchemaVersion: OrchestraBridgeSchemaVersion, TaskID: "task-1", Note: "evidence verified",
	})
	if err != nil {
		t.Fatal(err)
	}
	if stub.merged != "task-1" {
		t.Fatalf("merged task = %q", stub.merged)
	}
	if result.TaskID != "task-1" || result.Status != string(orchestra.TaskSuccess) || result.SchemaVersion != OrchestraBridgeSchemaVersion {
		t.Fatalf("merge result = %#v", result)
	}
}

func TestOrchestraMergeBridgeFailsClosed(t *testing.T) {
	service := NewService()
	_, err := service.MergeOrchestraTask(context.Background(), OrchestraMergeRequest{SchemaVersion: OrchestraBridgeSchemaVersion, TaskID: "task-1"})
	var unavailable *OrchestraBridgeError
	if !errors.As(err, &unavailable) || unavailable.Code != "orchestra.storage_unavailable" {
		t.Fatalf("unwired merge error = %#v", err)
	}

	service = NewServiceWithOrchestra(&orchestraPortStub{err: orchestra.ErrInspectionRejected})
	_, err = service.MergeOrchestraTask(context.Background(), OrchestraMergeRequest{SchemaVersion: OrchestraBridgeSchemaVersion, TaskID: "task-1"})
	var rejected *OrchestraBridgeError
	if !errors.As(err, &rejected) || rejected.Code != "orchestra.inspection_rejected" {
		t.Fatalf("rejected merge error = %#v", err)
	}

	service = NewServiceWithOrchestra(&orchestraPortStub{})
	_, err = service.MergeOrchestraTask(context.Background(), OrchestraMergeRequest{SchemaVersion: OrchestraBridgeSchemaVersion})
	if !errors.As(err, &rejected) || rejected.Code != "orchestra.invalid_request" {
		t.Fatalf("empty task error = %#v", err)
	}

	_, err = service.MergeOrchestraTask(context.Background(), OrchestraMergeRequest{SchemaVersion: "wrong", TaskID: "task-1"})
	if !errors.As(err, &rejected) || rejected.Code != "orchestra.unsupported_version" {
		t.Fatalf("version error = %#v", err)
	}
}
