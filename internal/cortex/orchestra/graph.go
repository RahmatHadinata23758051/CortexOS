package orchestra

import (
	"errors"
	"fmt"
	"sort"
)

const MaxPlanTasks = 256

var (
	ErrInvalidGraph      = errors.New("orchestra: invalid task graph")
	ErrMissingDependency = errors.New("orchestra: missing task dependency")
	ErrDependencyCycle   = errors.New("orchestra: task dependency cycle")
	ErrPlanTooLarge      = errors.New("orchestra: task graph exceeds limit")
)

// ValidateGraph validates task identity, task contracts, dependency references,
// and acyclicity without mutating the caller's slice.
func ValidateGraph(tasks []Task) error {
	if len(tasks) == 0 {
		return fmt.Errorf("%w: at least one task is required", ErrInvalidGraph)
	}
	if len(tasks) > MaxPlanTasks {
		return fmt.Errorf("%w: got %d tasks, limit is %d", ErrPlanTooLarge, len(tasks), MaxPlanTasks)
	}

	byID := make(map[TaskID]Task, len(tasks))
	for _, task := range tasks {
		if err := ValidateTask(task); err != nil {
			return err
		}
		if _, exists := byID[task.ID]; exists {
			return fmt.Errorf("%w: duplicate task id %q", ErrInvalidGraph, task.ID)
		}
		byID[task.ID] = task
	}

	for _, task := range tasks {
		seen := make(map[TaskID]struct{}, len(task.Dependencies))
		for _, dependencyID := range task.Dependencies {
			if dependencyID == task.ID {
				return fmt.Errorf("%w: task %q depends on itself", ErrDependencyCycle, task.ID)
			}
			if _, exists := byID[dependencyID]; !exists {
				return fmt.Errorf("%w: task %q depends on %q", ErrMissingDependency, task.ID, dependencyID)
			}
			if _, duplicate := seen[dependencyID]; duplicate {
				return fmt.Errorf("%w: task %q lists dependency %q more than once", ErrInvalidGraph, task.ID, dependencyID)
			}
			seen[dependencyID] = struct{}{}
		}
	}

	visiting := make(map[TaskID]bool, len(tasks))
	visited := make(map[TaskID]bool, len(tasks))
	var visit func(TaskID) error
	visit = func(id TaskID) error {
		if visiting[id] {
			return fmt.Errorf("%w: task %q", ErrDependencyCycle, id)
		}
		if visited[id] {
			return nil
		}
		visiting[id] = true
		for _, dependencyID := range byID[id].Dependencies {
			if err := visit(dependencyID); err != nil {
				return err
			}
		}
		visiting[id] = false
		visited[id] = true
		return nil
	}
	for id := range byID {
		if err := visit(id); err != nil {
			return err
		}
	}
	return nil
}

// ReadyTasks returns draft tasks whose dependencies have reached success. The
// result is sorted by stable task ID so scheduler behavior is reproducible.
func ReadyTasks(tasks []Task) ([]Task, error) {
	if err := ValidateGraph(tasks); err != nil {
		return nil, err
	}
	byID := make(map[TaskID]Task, len(tasks))
	for _, task := range tasks {
		byID[task.ID] = task
	}
	ready := make([]Task, 0, len(tasks))
	for _, task := range tasks {
		if task.Status != "" && task.Status != TaskDraft && task.Status != TaskReady {
			continue
		}
		allSuccessful := true
		for _, dependencyID := range task.Dependencies {
			if byID[dependencyID].Status != TaskSuccess {
				allSuccessful = false
				break
			}
		}
		if allSuccessful {
			if task.Status == "" {
				task.Status = TaskDraft
			}
			ready = append(ready, task)
		}
	}
	sort.Slice(ready, func(i, j int) bool { return ready[i].ID < ready[j].ID })
	return ready, nil
}
