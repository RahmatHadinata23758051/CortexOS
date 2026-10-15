package application

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/RahmatHadinata23758051/CortexOS/internal/cortex/orchestra"
)

const WorkerLifecycleSchemaVersion = "cortexos.worker-lifecycle.v1"

var (
	ErrWorkerInvalidRequest = errors.New("worker lifecycle: invalid request")
	ErrWorkerNotFound       = errors.New("worker lifecycle: worker not found")
	ErrWorkerInvalidState   = errors.New("worker lifecycle: invalid state transition")
)

type WorkerLifecycleState string

const (
	WorkerSpawned          WorkerLifecycleState = "spawned"
	WorkerRunning          WorkerLifecycleState = "running"
	WorkerAwaitingApproval WorkerLifecycleState = "awaiting_approval"
	WorkerReviewing        WorkerLifecycleState = "reviewing"
	WorkerAccepted         WorkerLifecycleState = "accepted"
	WorkerUnspawned        WorkerLifecycleState = "unspawned"
	WorkerFailed           WorkerLifecycleState = "failed"
	WorkerBlocked          WorkerLifecycleState = "blocked"
)

type WorkerCard struct {
	WorkerID      string               `json:"workerId"`
	TaskID        string               `json:"taskId"`
	ProjectID     string               `json:"projectId"`
	CallSign      string               `json:"callSign,omitempty"`
	Role          string               `json:"role,omitempty"`
	State         WorkerLifecycleState `json:"state"`
	Reason        string               `json:"reason,omitempty"`
	SpawnedAt     time.Time            `json:"spawnedAt"`
	UpdatedAt     time.Time            `json:"updatedAt"`
	UnspawnedAt   *time.Time           `json:"unspawnedAt,omitempty"`
	SchemaVersion string               `json:"schemaVersion"`
}

type WorkerLifecycleEvent struct {
	EventID       string               `json:"eventId"`
	WorkerID      string               `json:"workerId"`
	TaskID        string               `json:"taskId"`
	ProjectID     string               `json:"projectId"`
	From          WorkerLifecycleState `json:"from,omitempty"`
	To            WorkerLifecycleState `json:"to"`
	Reason        string               `json:"reason,omitempty"`
	OccurredAt    time.Time            `json:"occurredAt"`
	SchemaVersion string               `json:"schemaVersion"`
}

type WorkerCardsRequest struct {
	SchemaVersion  string `json:"schemaVersion"`
	ProjectID      string `json:"projectId,omitempty"`
	IncludeHistory bool   `json:"includeHistory,omitempty"`
}

type WorkerCardsResult struct {
	SchemaVersion string                 `json:"schemaVersion"`
	ProjectID     string                 `json:"projectId,omitempty"`
	Workers       []WorkerCard           `json:"workers"`
	Events        []WorkerLifecycleEvent `json:"events"`
	GeneratedAt   time.Time              `json:"generatedAt"`
}

type WorkerTransitionRequest struct {
	WorkerID      string               `json:"workerId"`
	State         WorkerLifecycleState `json:"state"`
	Reason        string               `json:"reason,omitempty"`
	SchemaVersion string               `json:"schemaVersion"`
}

type WorkerTransitionResult struct {
	Worker        WorkerCard           `json:"worker"`
	Event         WorkerLifecycleEvent `json:"event"`
	SchemaVersion string               `json:"schemaVersion"`
}

type WorkerLifecyclePort interface {
	Sync(context.Context, WorkerCardsRequest) (WorkerCardsResult, error)
	Transition(context.Context, string, WorkerLifecycleState, string) (WorkerCard, WorkerLifecycleEvent, error)
}

// WorkerLifecycleManager is a bounded session projection over authoritative
// Orchestra task state. Task state remains the source of truth; this projection
// owns only ephemeral worker-card identity and lifecycle history.
// ponytail: ephemeral worker cards are held in session memory (max 500 lifecycle events). Upgrade path: serialize to sqlite table orchestra_worker_cards for cross-session recovery.
type WorkerLifecycleManager struct {
	mu      sync.RWMutex
	tasks   OrchestraPort
	workers map[string]WorkerCard
	events  []WorkerLifecycleEvent
	now     func() time.Time
	seq     uint64
}

func NewWorkerLifecycleManager(tasks OrchestraPort) *WorkerLifecycleManager {
	return &WorkerLifecycleManager{tasks: tasks, workers: make(map[string]WorkerCard), now: time.Now}
}

func (m *WorkerLifecycleManager) Sync(ctx context.Context, request WorkerCardsRequest) (WorkerCardsResult, error) {
	if request.SchemaVersion != WorkerLifecycleSchemaVersion {
		return WorkerCardsResult{}, fmt.Errorf("%w: unsupported schema version", ErrWorkerInvalidRequest)
	}
	if err := ctx.Err(); err != nil {
		return WorkerCardsResult{}, err
	}
	if m == nil || m.tasks == nil {
		return WorkerCardsResult{}, ErrWorkerInvalidRequest
	}
	tasks, err := m.tasks.ListTasks(ctx)
	if err != nil {
		return WorkerCardsResult{}, err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, task := range tasks {
		if request.ProjectID != "" && task.ProjectID != request.ProjectID {
			continue
		}
		if task.AssignedStaffID == "" {
			continue
		}
		workerID := workerIDForTask(task.ID)
		card, exists := m.workers[workerID]
		if !exists {
			card = WorkerCard{
				WorkerID: workerID, TaskID: string(task.ID), ProjectID: task.ProjectID,
				CallSign: string(task.AssignedStaffID), State: WorkerSpawned,
				SpawnedAt: m.now().UTC(), UpdatedAt: m.now().UTC(), SchemaVersion: WorkerLifecycleSchemaVersion,
			}
			m.workers[workerID] = card
			m.appendEventLocked(card, "", WorkerSpawned, "task dispatched")
		}
		target := workerStateForTask(task.Status)
		if target != "" && target != card.State {
			if canSyncWorkerState(card.State, target) {
				m.transitionLocked(&card, target, "synchronized from Orchestra task")
				// A successful task has passed the authoritative merge gate;
				// accepted workers are ephemeral and leave the active canvas.
				if target == WorkerAccepted {
					m.transitionLocked(&card, WorkerUnspawned, "task accepted by Orchestra")
				}
				m.workers[workerID] = card
			}
		}
	}
	return m.resultLocked(request), nil
}

func (m *WorkerLifecycleManager) Transition(ctx context.Context, workerID string, target WorkerLifecycleState, reason string) (WorkerCard, WorkerLifecycleEvent, error) {
	if err := ctx.Err(); err != nil {
		return WorkerCard{}, WorkerLifecycleEvent{}, err
	}
	if strings.TrimSpace(workerID) == "" || !validWorkerState(target) {
		return WorkerCard{}, WorkerLifecycleEvent{}, ErrWorkerInvalidRequest
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	card, ok := m.workers[workerID]
	if !ok {
		return WorkerCard{}, WorkerLifecycleEvent{}, ErrWorkerNotFound
	}
	if card.State == target {
		return card, WorkerLifecycleEvent{}, nil
	}
	if !canTransitionWorker(card.State, target) {
		return WorkerCard{}, WorkerLifecycleEvent{}, fmt.Errorf("%w: %s -> %s", ErrWorkerInvalidState, card.State, target)
	}
	event := m.transitionLocked(&card, target, reason)
	m.workers[workerID] = card
	return card, event, nil
}

func (m *WorkerLifecycleManager) resultLocked(request WorkerCardsRequest) WorkerCardsResult {
	workers := make([]WorkerCard, 0, len(m.workers))
	for _, card := range m.workers {
		if request.ProjectID != "" && card.ProjectID != request.ProjectID {
			continue
		}
		if !request.IncludeHistory && card.State == WorkerUnspawned {
			continue
		}
		workers = append(workers, card)
	}
	sort.Slice(workers, func(i, j int) bool { return workers[i].WorkerID < workers[j].WorkerID })
	events := make([]WorkerLifecycleEvent, 0, len(m.events))
	for _, event := range m.events {
		if request.ProjectID == "" || event.ProjectID == request.ProjectID {
			events = append(events, event)
		}
	}
	return WorkerCardsResult{SchemaVersion: WorkerLifecycleSchemaVersion, ProjectID: request.ProjectID, Workers: workers, Events: events, GeneratedAt: m.now().UTC()}
}

func (m *WorkerLifecycleManager) transitionLocked(card *WorkerCard, target WorkerLifecycleState, reason string) WorkerLifecycleEvent {
	from := card.State
	now := m.now().UTC()
	card.State, card.Reason, card.UpdatedAt = target, reason, now
	if target == WorkerUnspawned {
		card.UnspawnedAt = &now
	}
	return m.appendEventLocked(*card, string(from), target, reason)
}

func (m *WorkerLifecycleManager) appendEventLocked(card WorkerCard, from string, target WorkerLifecycleState, reason string) WorkerLifecycleEvent {
	m.seq++
	event := WorkerLifecycleEvent{EventID: fmt.Sprintf("worker-event-%d", m.seq), WorkerID: card.WorkerID, TaskID: card.TaskID, ProjectID: card.ProjectID, From: WorkerLifecycleState(from), To: target, Reason: reason, OccurredAt: m.now().UTC(), SchemaVersion: WorkerLifecycleSchemaVersion}
	m.events = append(m.events, event)
	if len(m.events) > 500 {
		m.events = m.events[len(m.events)-500:]
	}
	return event
}

func workerIDForTask(taskID orchestra.TaskID) string { return "worker-" + string(taskID) }

func workerStateForTask(status orchestra.TaskStatus) WorkerLifecycleState {
	switch status {
	case orchestra.TaskDraft, orchestra.TaskReady:
		return WorkerSpawned
	case orchestra.TaskRunning:
		return WorkerRunning
	case orchestra.TaskAwaitingInspection:
		return WorkerReviewing
	case orchestra.TaskSuccess:
		return WorkerAccepted
	case orchestra.TaskFailed:
		return WorkerFailed
	default:
		return ""
	}
}

func validWorkerState(state WorkerLifecycleState) bool {
	switch state {
	case WorkerSpawned, WorkerRunning, WorkerAwaitingApproval, WorkerReviewing, WorkerAccepted, WorkerUnspawned, WorkerFailed, WorkerBlocked:
		return true
	default:
		return false
	}
}

func canSyncWorkerState(from, to WorkerLifecycleState) bool {
	if to == WorkerAccepted && from == WorkerSpawned {
		return true
	}
	return canTransitionWorker(from, to)
}

func canTransitionWorker(from, to WorkerLifecycleState) bool {
	if from == WorkerUnspawned {
		return false
	}
	switch to {
	case WorkerRunning:
		return from == WorkerSpawned || from == WorkerAwaitingApproval || from == WorkerBlocked
	case WorkerAwaitingApproval:
		return from == WorkerSpawned || from == WorkerRunning
	case WorkerReviewing:
		return from == WorkerRunning || from == WorkerAwaitingApproval
	case WorkerAccepted:
		return from == WorkerReviewing || from == WorkerRunning
	case WorkerUnspawned:
		return from == WorkerAccepted || from == WorkerFailed
	case WorkerFailed:
		return from == WorkerRunning || from == WorkerReviewing || from == WorkerBlocked
	case WorkerBlocked:
		return from == WorkerSpawned || from == WorkerRunning
	case WorkerSpawned:
		return false
	default:
		return false
	}
}
