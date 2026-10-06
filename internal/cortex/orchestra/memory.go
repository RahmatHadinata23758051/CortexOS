package orchestra

import (
	"context"
	"errors"
	"sort"
	"sync"
)

var ErrStoreUnavailable = errors.New("orchestra: task store is unavailable")

type TaskStore interface {
	SaveTask(context.Context, Task) error
	GetTask(context.Context, TaskID) (Task, error)
	ListTasks(context.Context) ([]Task, error)
	AppendEvent(context.Context, TaskEvent) error
	ListEvents(context.Context, TaskID) ([]TaskEvent, error)
}

type MemoryTaskStore struct {
	mu     sync.RWMutex
	tasks  map[TaskID]Task
	events map[TaskID][]TaskEvent
	seq    int64
}

func NewMemoryTaskStore() *MemoryTaskStore {
	return &MemoryTaskStore{tasks: make(map[TaskID]Task), events: make(map[TaskID][]TaskEvent)}
}

func (s *MemoryTaskStore) SaveTask(ctx context.Context, task Task) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := ValidateTask(task); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.tasks[task.ID] = task
	return nil
}

func (s *MemoryTaskStore) GetTask(ctx context.Context, id TaskID) (Task, error) {
	if err := ctx.Err(); err != nil {
		return Task{}, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	task, exists := s.tasks[id]
	if !exists {
		return Task{}, ErrNotFound
	}
	return task, nil
}

func (s *MemoryTaskStore) ListTasks(ctx context.Context) ([]Task, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	tasks := make([]Task, 0, len(s.tasks))
	for _, task := range s.tasks {
		tasks = append(tasks, task)
	}
	sort.Slice(tasks, func(i, j int) bool { return tasks[i].ID < tasks[j].ID })
	return tasks, nil
}

func (s *MemoryTaskStore) AppendEvent(ctx context.Context, event TaskEvent) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.seq++
	event.Sequence = s.seq
	s.events[event.TaskID] = append(s.events[event.TaskID], event)
	return nil
}

func (s *MemoryTaskStore) ListEvents(ctx context.Context, id TaskID) ([]TaskEvent, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	events := append([]TaskEvent(nil), s.events[id]...)
	sort.Slice(events, func(i, j int) bool { return events[i].Sequence < events[j].Sequence })
	return events, nil
}

var ErrNotFound = errors.New("orchestra: task not found")
