package orchestra

import (
	"time"

	"github.com/RahmatHadinata23758051/CortexOS/internal/cortex/staff"
)

// AssignmentProvenance records the logical Staff assignment and its evidence
// lineage. It contains no process identity or authority.
type AssignmentProvenance struct {
	AssignmentID string    `json:"assignmentId,omitempty"`
	Source       string    `json:"source,omitempty"`
	AssignedAt   time.Time `json:"assignedAt,omitempty"`
	TaskID       string    `json:"taskId,omitempty"`
	ExecutionID  string    `json:"executionId,omitempty"`
	TraceID      string    `json:"traceId,omitempty"`
}

// AdvisoryMemory is bounded context supplied for execution guidance only.
type AdvisoryMemory struct {
	ID         string               `json:"id"`
	Kind       string               `json:"kind"`
	Content    string               `json:"content"`
	Source     string               `json:"source,omitempty"`
	Provenance AssignmentProvenance `json:"provenance,omitempty"`
}

const ContractVersion = "cortexos.orchestra.v1"

type TaskID string
type ExecutionID string
type EventID string

type TaskStatus string

const (
	TaskDraft              TaskStatus = "draft"
	TaskReady              TaskStatus = "ready"
	TaskRunning            TaskStatus = "running"
	TaskAwaitingInspection TaskStatus = "awaitingInspection"
	TaskSuccess            TaskStatus = "success"
	TaskFailed             TaskStatus = "failed"
	TaskCanceled           TaskStatus = "canceled"
)

type Task struct {
	ID                 TaskID        `json:"id"`
	WorkspaceID        string        `json:"workspaceId,omitempty"`
	ProjectID          string        `json:"projectId"`
	WorktreeID         string        `json:"worktreeId"`
	AssignedStaffID    staff.StaffID `json:"assignedStaffId,omitempty"`
	Title              string        `json:"title"`
	AcceptanceCriteria []string      `json:"acceptanceCriteria"`
	Dependencies       []TaskID      `json:"dependencies"`
	Status             TaskStatus    `json:"status"`
	AttemptCount       int           `json:"attemptCount"`
	MaxAttempts        int           `json:"maxAttempts"`
	CreatedAt          time.Time     `json:"createdAt"`
	UpdatedAt          time.Time     `json:"updatedAt"`
	SchemaVersion      string        `json:"schemaVersion"`
}

type Execution struct {
	ID          ExecutionID `json:"id"`
	TaskID      TaskID      `json:"taskId"`
	Attempt     int         `json:"attempt"`
	Status      TaskStatus  `json:"status"`
	StartedAt   time.Time   `json:"startedAt"`
	FinishedAt  *time.Time  `json:"finishedAt,omitempty"`
	EvidenceIDs []string    `json:"evidenceIds"`
}

type EventType string

const (
	EventTaskCreated        EventType = "task.created"
	EventTaskValidated      EventType = "task.validated"
	EventTaskDispatched     EventType = "task.dispatched"
	EventEvidenceCollected  EventType = "evidence.collected"
	EventInspectionAccepted EventType = "inspection.accepted"
	EventInspectionRejected EventType = "inspection.rejected"
	EventTaskCanceled       EventType = "task.canceled"
	EventTaskRetryRequested EventType = "task.retryRequested"
	EventExecutionFailed    EventType = "execution.failed"
)

type TaskEvent struct {
	ID          EventID     `json:"id"`
	TaskID      TaskID      `json:"taskId"`
	ExecutionID ExecutionID `json:"executionId,omitempty"`
	Sequence    int64       `json:"sequence"`
	Type        EventType   `json:"type"`
	From        TaskStatus  `json:"from,omitempty"`
	To          TaskStatus  `json:"to,omitempty"`
	Message     string      `json:"message,omitempty"`
	EvidenceIDs []string    `json:"evidenceIds,omitempty"`
	OccurredAt  time.Time   `json:"occurredAt"`
}

type Transition string

const (
	TransitionValidate        Transition = "validate"
	TransitionDispatch        Transition = "dispatch"
	TransitionCollectEvidence Transition = "collectEvidence"
	TransitionInspectAccepted Transition = "inspectAccepted"
	TransitionInspectRejected Transition = "inspectRejected"
	TransitionCancel          Transition = "cancel"
	TransitionRetry           Transition = "retry"
	TransitionExecutionFailed Transition = "executionFailed"
)
