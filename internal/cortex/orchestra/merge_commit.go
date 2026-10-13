package orchestra

import (
	"context"
	"errors"
	"fmt"
)

var (
	ErrMergeConflict = errors.New("orchestra: merge conflict: task status changed concurrently")
)

// MergeCommit defines the all-or-nothing persistence payload for an authoritative
// Inspector merge decision (ADR-0005). All fields are persisted in one atomic transaction:
// 1. MergedTask transitions to TaskSuccess.
// 2. AcceptedEvent records the EventInspectionAccepted timeline entry.
// 3. UnblockedTasks transitions dependent tasks from TaskDraft to TaskReady.
// 4. ValidatedEvents records the EventTaskValidated timeline entries for each unblocked task.
type MergeCommit struct {
	MergedTask      Task
	AcceptedEvent   TaskEvent
	UnblockedTasks  []Task
	ValidatedEvents []TaskEvent
}

// Validate ensures the commit payload satisfies basic domain integrity before hitting storage.
func (c MergeCommit) Validate() error {
	if c.MergedTask.ID == "" {
		return fmt.Errorf("%w: merged task id is required", ErrInvalidTask)
	}
	if c.MergedTask.Status != TaskSuccess {
		return fmt.Errorf("%w: merged task status must be %s, got %s", ErrInvalidTask, TaskSuccess, c.MergedTask.Status)
	}
	if c.AcceptedEvent.TaskID != c.MergedTask.ID {
		return fmt.Errorf("%w: accepted event task id %q does not match merged task %q", ErrInvalidTask, c.AcceptedEvent.TaskID, c.MergedTask.ID)
	}
	if c.AcceptedEvent.Type != EventInspectionAccepted {
		return fmt.Errorf("%w: accepted event type must be %s, got %s", ErrInvalidTask, EventInspectionAccepted, c.AcceptedEvent.Type)
	}
	if len(c.UnblockedTasks) != len(c.ValidatedEvents) {
		return fmt.Errorf("%w: unblocked tasks count (%d) must match validated events count (%d)", ErrInvalidTask, len(c.UnblockedTasks), len(c.ValidatedEvents))
	}
	for i, t := range c.UnblockedTasks {
		if t.Status != TaskReady {
			return fmt.Errorf("%w: unblocked task %s status must be %s, got %s", ErrInvalidTask, t.ID, TaskReady, t.Status)
		}
		if c.ValidatedEvents[i].TaskID != t.ID {
			return fmt.Errorf("%w: validated event task id %q does not match unblocked task %q", ErrInvalidTask, c.ValidatedEvents[i].TaskID, t.ID)
		}
		if c.ValidatedEvents[i].Type != EventTaskValidated {
			return fmt.Errorf("%w: validated event type must be %s, got %s", ErrInvalidTask, EventTaskValidated, c.ValidatedEvents[i].Type)
		}
	}
	return nil
}

// MergeCommitter is an optional capability interface implemented by durable stores (like SQLite)
// that can execute a MergeCommit in a single atomic database transaction with compare-and-set semantics.
type MergeCommitter interface {
	CommitMerge(context.Context, MergeCommit) error
}
