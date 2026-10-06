package application

import (
	"context"
	"errors"
	"sort"
	"time"

	"github.com/RahmatHadinata23758051/CortexOS/internal/cortex/orchestra"
)

const OrchestraBridgeSchemaVersion = "cortexos.orchestra.bridge.v1"

var ErrOrchestraInvalidRequest = errors.New("invalid orchestra bridge request")

// OrchestraPort is the Wails-independent application boundary for Orchestra
// observability and governed mutations. It intentionally exposes domain facts,
// not SQL, filesystem paths, process output, or engine handles.
type OrchestraPort interface {
	ListTasks(context.Context) ([]orchestra.Task, error)
	GetTask(context.Context, orchestra.TaskID) (orchestra.Task, error)
	ListEvents(context.Context, orchestra.TaskID) ([]orchestra.TaskEvent, error)
	CancelTask(context.Context, orchestra.TaskID) error
	RetryTask(context.Context, orchestra.TaskID) error
}

type OrchestraTaskListRequest struct {
	SchemaVersion string `json:"schemaVersion"`
	ProjectID     string `json:"projectId,omitempty"`
	Limit         int    `json:"limit"`
}

type OrchestraTaskRequest struct {
	SchemaVersion string `json:"schemaVersion"`
	TaskID        string `json:"taskId"`
}

type OrchestraTaskSummary struct {
	ID            string `json:"id"`
	ProjectID     string `json:"projectId"`
	Status        string `json:"status"`
	Attempt       int    `json:"attempt"`
	MaxAttempts   int    `json:"maxAttempts"`
	Progress      int    `json:"progress"`
	SchemaVersion string `json:"schemaVersion"`
}

type OrchestraEvidenceSummary struct {
	ID    string `json:"id"`
	Count int    `json:"count"`
}

type OrchestraTimelineEntry struct {
	Sequence    int64     `json:"sequence"`
	Type        string    `json:"type"`
	From        string    `json:"from,omitempty"`
	To          string    `json:"to,omitempty"`
	OccurredAt  time.Time `json:"occurredAt"`
	EvidenceIDs []string  `json:"evidenceIds,omitempty"`
}

type OrchestraTaskDetail struct {
	ID              string                     `json:"id"`
	ProjectID       string                     `json:"projectId"`
	Status          string                     `json:"status"`
	Attempt         int                        `json:"attempt"`
	MaxAttempts     int                        `json:"maxAttempts"`
	Progress        int                        `json:"progress"`
	AcceptanceCount int                        `json:"acceptanceCount"`
	DependencyCount int                        `json:"dependencyCount"`
	Evidence        []OrchestraEvidenceSummary `json:"evidence"`
	Timeline        []OrchestraTimelineEntry   `json:"timeline"`
	SchemaVersion   string                     `json:"schemaVersion"`
}

type OrchestraBridgeError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func (e *OrchestraBridgeError) Error() string {
	if e == nil {
		return "<nil>"
	}
	return e.Code + ": " + e.Message
}

func (s *Service) ListOrchestraTasks(ctx context.Context, request OrchestraTaskListRequest) ([]OrchestraTaskSummary, error) {
	if err := validateOrchestraSchema(request.SchemaVersion); err != nil {
		return nil, err
	}
	if request.Limit < 1 || request.Limit > 100 {
		return nil, orchestraBridgeError(orchestra.ErrInvalidTask)
	}
	if s == nil || s.orchestra == nil {
		return nil, orchestraBridgeError(orchestra.ErrStoreUnavailable)
	}
	tasks, err := s.orchestra.ListTasks(ctx)
	if err != nil {
		return nil, mapOrchestraError(err)
	}
	result := make([]OrchestraTaskSummary, 0, min(request.Limit, len(tasks)))
	for _, task := range tasks {
		if request.ProjectID != "" && task.ProjectID != request.ProjectID {
			continue
		}
		result = append(result, orchestraTaskSummary(task))
		if len(result) == request.Limit {
			break
		}
	}
	return result, nil
}

func (s *Service) GetOrchestraTask(ctx context.Context, request OrchestraTaskRequest) (OrchestraTaskDetail, error) {
	if err := validateOrchestraSchema(request.SchemaVersion); err != nil {
		return OrchestraTaskDetail{}, err
	}
	if request.TaskID == "" {
		return OrchestraTaskDetail{}, orchestraBridgeError(orchestra.ErrInvalidTask)
	}
	if s == nil || s.orchestra == nil {
		return OrchestraTaskDetail{}, orchestraBridgeError(orchestra.ErrStoreUnavailable)
	}
	task, err := s.orchestra.GetTask(ctx, orchestra.TaskID(request.TaskID))
	if err != nil {
		return OrchestraTaskDetail{}, mapOrchestraError(err)
	}
	events, err := s.orchestra.ListEvents(ctx, task.ID)
	if err != nil {
		return OrchestraTaskDetail{}, mapOrchestraError(err)
	}
	return orchestraTaskDetail(task, events), nil
}

func (s *Service) CancelOrchestraTask(ctx context.Context, request OrchestraTaskRequest) error {
	if err := validateOrchestraSchema(request.SchemaVersion); err != nil {
		return err
	}
	if request.TaskID == "" {
		return orchestraBridgeError(orchestra.ErrInvalidTask)
	}
	if s == nil || s.orchestra == nil {
		return orchestraBridgeError(orchestra.ErrStoreUnavailable)
	}
	return mapOrchestraError(s.orchestra.CancelTask(ctx, orchestra.TaskID(request.TaskID)))
}

func (s *Service) RetryOrchestraTask(ctx context.Context, request OrchestraTaskRequest) error {
	if err := validateOrchestraSchema(request.SchemaVersion); err != nil {
		return err
	}
	if request.TaskID == "" {
		return orchestraBridgeError(orchestra.ErrInvalidTask)
	}
	if s == nil || s.orchestra == nil {
		return orchestraBridgeError(orchestra.ErrStoreUnavailable)
	}
	return mapOrchestraError(s.orchestra.RetryTask(ctx, orchestra.TaskID(request.TaskID)))
}

func validateOrchestraSchema(version string) error {
	if version != OrchestraBridgeSchemaVersion {
		return &OrchestraBridgeError{Code: "orchestra.unsupported_version", Message: "orchestra bridge contract version is unsupported"}
	}
	return nil
}

func orchestraTaskSummary(task orchestra.Task) OrchestraTaskSummary {
	return OrchestraTaskSummary{
		ID: string(task.ID), ProjectID: task.ProjectID, Status: string(task.Status),
		Attempt: task.AttemptCount, MaxAttempts: task.MaxAttempts,
		Progress: orchestraProgress(task.Status), SchemaVersion: OrchestraBridgeSchemaVersion,
	}
}

func orchestraTaskDetail(task orchestra.Task, events []orchestra.TaskEvent) OrchestraTaskDetail {
	timeline := make([]OrchestraTimelineEntry, 0, len(events))
	evidenceCounts := make(map[string]int)
	for _, event := range events {
		ids := append([]string(nil), event.EvidenceIDs...)
		sort.Strings(ids)
		for _, id := range ids {
			if id != "" {
				evidenceCounts[id]++
			}
		}
		timeline = append(timeline, OrchestraTimelineEntry{
			Sequence: event.Sequence, Type: string(event.Type), From: string(event.From), To: string(event.To),
			OccurredAt: event.OccurredAt, EvidenceIDs: ids,
		})
	}
	evidence := make([]OrchestraEvidenceSummary, 0, len(evidenceCounts))
	for id, count := range evidenceCounts {
		evidence = append(evidence, OrchestraEvidenceSummary{ID: id, Count: count})
	}
	sort.Slice(evidence, func(i, j int) bool { return evidence[i].ID < evidence[j].ID })
	return OrchestraTaskDetail{
		ID: string(task.ID), ProjectID: task.ProjectID, Status: string(task.Status),
		Attempt: task.AttemptCount, MaxAttempts: task.MaxAttempts, Progress: orchestraProgress(task.Status),
		AcceptanceCount: len(task.AcceptanceCriteria), DependencyCount: len(task.Dependencies),
		Evidence: evidence, Timeline: timeline, SchemaVersion: OrchestraBridgeSchemaVersion,
	}
}

func orchestraProgress(status orchestra.TaskStatus) int {
	switch status {
	case orchestra.TaskDraft:
		return 0
	case orchestra.TaskReady:
		return 20
	case orchestra.TaskRunning:
		return 50
	case orchestra.TaskAwaitingInspection:
		return 80
	case orchestra.TaskSuccess:
		return 100
	case orchestra.TaskFailed, orchestra.TaskCanceled:
		return 100
	default:
		return 0
	}
}

func mapOrchestraError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return &OrchestraBridgeError{Code: "orchestra.canceled", Message: "orchestra request canceled"}
	}
	return orchestraBridgeError(err)
}

func orchestraBridgeError(err error) error {
	code := "orchestra.internal"
	switch {
	case errors.Is(err, orchestra.ErrNotFound):
		code = "orchestra.not_found"
	case errors.Is(err, orchestra.ErrInvalidTask), errors.Is(err, orchestra.ErrInvalidDispatch):
		code = "orchestra.invalid_request"
	case errors.Is(err, orchestra.ErrDuplicateDispatch):
		code = "orchestra.conflict"
	case errors.Is(err, orchestra.ErrStoreUnavailable):
		code = "orchestra.storage_unavailable"
	case errors.Is(err, orchestra.ErrTerminalTask):
		code = "orchestra.terminal"
	case errors.Is(err, orchestra.ErrDependencyCycle), errors.Is(err, orchestra.ErrMissingDependency), errors.Is(err, orchestra.ErrInvalidGraph):
		code = "orchestra.invalid_graph"
	}
	messages := map[string]string{
		"orchestra.not_found":           "orchestra task was not found",
		"orchestra.invalid_request":     "orchestra request is invalid",
		"orchestra.conflict":            "orchestra task is already active or changed",
		"orchestra.storage_unavailable": "orchestra state is unavailable",
		"orchestra.terminal":            "orchestra task is terminal",
		"orchestra.invalid_graph":       "orchestra task graph is invalid",
	}
	message := messages[code]
	if message == "" {
		message = "orchestra request failed"
	}
	return &OrchestraBridgeError{Code: code, Message: message}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
