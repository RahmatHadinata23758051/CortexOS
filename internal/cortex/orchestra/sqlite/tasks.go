package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/RahmatHadinata23758051/CortexOS/internal/cortex/orchestra"
)

var _ orchestra.TaskStore = (*Store)(nil)

func (s *Store) SaveTask(ctx context.Context, task orchestra.Task) error {
	if s == nil || s.db == nil {
		return orchestra.ErrStoreUnavailable
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := orchestra.ValidateTask(task); err != nil {
		return err
	}
	if task.SchemaVersion == "" {
		task.SchemaVersion = orchestra.ContractVersion
	}
	if task.CreatedAt.IsZero() {
		task.CreatedAt = time.Now().UTC()
	}
	if task.UpdatedAt.IsZero() {
		task.UpdatedAt = task.CreatedAt
	}
	criteriaJSON, err := json.Marshal(task.AcceptanceCriteria)
	if err != nil {
		return fmt.Errorf("marshal acceptance criteria: %w", err)
	}
	depsJSON, err := json.Marshal(task.Dependencies)
	if err != nil {
		return fmt.Errorf("marshal dependencies: %w", err)
	}

	query := `INSERT INTO orchestra_tasks (
		id, project_id, worktree_id, title, acceptance_criteria, dependencies,
		status, attempt_count, max_attempts, created_at, updated_at, schema_version
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	ON CONFLICT(id) DO UPDATE SET
		project_id = excluded.project_id,
		worktree_id = excluded.worktree_id,
		title = excluded.title,
		acceptance_criteria = excluded.acceptance_criteria,
		dependencies = excluded.dependencies,
		status = excluded.status,
		attempt_count = excluded.attempt_count,
		max_attempts = excluded.max_attempts,
		updated_at = excluded.updated_at,
		schema_version = excluded.schema_version;`

	_, err = s.db.ExecContext(ctx, query,
		string(task.ID),
		task.ProjectID,
		task.WorktreeID,
		task.Title,
		string(criteriaJSON),
		string(depsJSON),
		string(task.Status),
		task.AttemptCount,
		task.MaxAttempts,
		task.CreatedAt.Format(time.RFC3339Nano),
		task.UpdatedAt.Format(time.RFC3339Nano),
		task.SchemaVersion,
	)
	if err != nil {
		return fmt.Errorf("save task %q: %w", task.ID, err)
	}
	return nil
}

func (s *Store) GetTask(ctx context.Context, id orchestra.TaskID) (orchestra.Task, error) {
	if s == nil || s.db == nil {
		return orchestra.Task{}, orchestra.ErrStoreUnavailable
	}
	if err := ctx.Err(); err != nil {
		return orchestra.Task{}, err
	}
	if id == "" {
		return orchestra.Task{}, orchestra.ErrNotFound
	}
	query := `SELECT id, project_id, worktree_id, title, acceptance_criteria, dependencies,
		status, attempt_count, max_attempts, created_at, updated_at, schema_version
		FROM orchestra_tasks WHERE id = ?`

	row := s.db.QueryRowContext(ctx, query, string(id))
	var task orchestra.Task
	var rawID, criteriaStr, depsStr, statusStr, createdAtStr, updatedAtStr string

	err := row.Scan(
		&rawID,
		&task.ProjectID,
		&task.WorktreeID,
		&task.Title,
		&criteriaStr,
		&depsStr,
		&statusStr,
		&task.AttemptCount,
		&task.MaxAttempts,
		&createdAtStr,
		&updatedAtStr,
		&task.SchemaVersion,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return orchestra.Task{}, orchestra.ErrNotFound
		}
		return orchestra.Task{}, fmt.Errorf("get task %q: %w", id, err)
	}
	task.ID = orchestra.TaskID(rawID)
	task.Status = orchestra.TaskStatus(statusStr)
	task.CreatedAt, _ = time.Parse(time.RFC3339Nano, createdAtStr)
	task.UpdatedAt, _ = time.Parse(time.RFC3339Nano, updatedAtStr)
	if err := json.Unmarshal([]byte(criteriaStr), &task.AcceptanceCriteria); err != nil {
		task.AcceptanceCriteria = []string{}
	}
	if err := json.Unmarshal([]byte(depsStr), &task.Dependencies); err != nil {
		task.Dependencies = []orchestra.TaskID{}
	}
	return task, nil
}

func (s *Store) ListTasks(ctx context.Context) ([]orchestra.Task, error) {
	if s == nil || s.db == nil {
		return nil, orchestra.ErrStoreUnavailable
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	query := `SELECT id, project_id, worktree_id, title, acceptance_criteria, dependencies,
		status, attempt_count, max_attempts, created_at, updated_at, schema_version
		FROM orchestra_tasks ORDER BY id ASC`

	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("list tasks: %w", err)
	}
	defer rows.Close()

	var tasks []orchestra.Task
	for rows.Next() {
		var task orchestra.Task
		var rawID, criteriaStr, depsStr, statusStr, createdAtStr, updatedAtStr string
		err := rows.Scan(
			&rawID,
			&task.ProjectID,
			&task.WorktreeID,
			&task.Title,
			&criteriaStr,
			&depsStr,
			&statusStr,
			&task.AttemptCount,
			&task.MaxAttempts,
			&createdAtStr,
			&updatedAtStr,
			&task.SchemaVersion,
		)
		if err != nil {
			return nil, fmt.Errorf("scan task: %w", err)
		}
		task.ID = orchestra.TaskID(rawID)
		task.Status = orchestra.TaskStatus(statusStr)
		task.CreatedAt, _ = time.Parse(time.RFC3339Nano, createdAtStr)
		task.UpdatedAt, _ = time.Parse(time.RFC3339Nano, updatedAtStr)
		if err := json.Unmarshal([]byte(criteriaStr), &task.AcceptanceCriteria); err != nil {
			task.AcceptanceCriteria = []string{}
		}
		if err := json.Unmarshal([]byte(depsStr), &task.Dependencies); err != nil {
			task.Dependencies = []orchestra.TaskID{}
		}
		tasks = append(tasks, task)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate tasks: %w", err)
	}
	sort.Slice(tasks, func(i, j int) bool { return tasks[i].ID < tasks[j].ID })
	return tasks, nil
}

func (s *Store) AppendEvent(ctx context.Context, event orchestra.TaskEvent) error {
	if s == nil || s.db == nil {
		return orchestra.ErrStoreUnavailable
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if event.TaskID == "" {
		return fmt.Errorf("task event: task id is required")
	}
	if event.OccurredAt.IsZero() {
		event.OccurredAt = time.Now().UTC()
	}
	if event.ID == "" {
		event.ID = orchestra.EventID(fmt.Sprintf("%s-%d-%d", event.TaskID, time.Now().UnixNano(), event.Sequence))
	}
	evidenceJSON, err := json.Marshal(event.EvidenceIDs)
	if err != nil {
		return fmt.Errorf("marshal event evidence ids: %w", err)
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin append event: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if event.Sequence <= 0 {
		var maxSeq int64
		err := tx.QueryRowContext(ctx, `SELECT COALESCE(MAX(sequence), 0) FROM orchestra_events WHERE task_id = ?`, string(event.TaskID)).Scan(&maxSeq)
		if err != nil {
			return fmt.Errorf("query max sequence: %w", err)
		}
		event.Sequence = maxSeq + 1
	}

	query := `INSERT INTO orchestra_events (
		id, task_id, execution_id, sequence, type, from_status, to_status, message, evidence_ids, occurred_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

	_, err = tx.ExecContext(ctx, query,
		string(event.ID),
		string(event.TaskID),
		nullableString(string(event.ExecutionID)),
		event.Sequence,
		string(event.Type),
		string(event.From),
		string(event.To),
		event.Message,
		string(evidenceJSON),
		event.OccurredAt.Format(time.RFC3339Nano),
	)
	if err != nil {
		return fmt.Errorf("insert task event: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit task event: %w", err)
	}
	return nil
}

func (s *Store) ListEvents(ctx context.Context, id orchestra.TaskID) ([]orchestra.TaskEvent, error) {
	if s == nil || s.db == nil {
		return nil, orchestra.ErrStoreUnavailable
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	query := `SELECT id, task_id, execution_id, sequence, type, from_status, to_status, message, evidence_ids, occurred_at
		FROM orchestra_events WHERE task_id = ? ORDER BY sequence ASC`

	rows, err := s.db.QueryContext(ctx, query, string(id))
	if err != nil {
		return nil, fmt.Errorf("list events for task %q: %w", id, err)
	}
	defer rows.Close()

	var events []orchestra.TaskEvent
	for rows.Next() {
		var event orchestra.TaskEvent
		var rawID, taskIDStr, execIDStr, typeStr, fromStr, toStr, evidenceStr, occurredAtStr string
		err := rows.Scan(
			&rawID,
			&taskIDStr,
			&execIDStr,
			&event.Sequence,
			&typeStr,
			&fromStr,
			&toStr,
			&event.Message,
			&evidenceStr,
			&occurredAtStr,
		)
		if err != nil {
			return nil, fmt.Errorf("scan event: %w", err)
		}
		event.ID = orchestra.EventID(rawID)
		event.TaskID = orchestra.TaskID(taskIDStr)
		event.ExecutionID = orchestra.ExecutionID(execIDStr)
		event.Type = orchestra.EventType(typeStr)
		event.From = orchestra.TaskStatus(fromStr)
		event.To = orchestra.TaskStatus(toStr)
		event.OccurredAt, _ = time.Parse(time.RFC3339Nano, occurredAtStr)
		if err := json.Unmarshal([]byte(evidenceStr), &event.EvidenceIDs); err != nil {
			event.EvidenceIDs = []string{}
		}
		events = append(events, event)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate events: %w", err)
	}
	return events, nil
}

func (s *Store) SaveExecution(ctx context.Context, exec orchestra.Execution) error {
	if s == nil || s.db == nil {
		return orchestra.ErrStoreUnavailable
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if exec.ID == "" || exec.TaskID == "" {
		return fmt.Errorf("execution id and task id are required")
	}
	if exec.StartedAt.IsZero() {
		exec.StartedAt = time.Now().UTC()
	}
	evidenceJSON, err := json.Marshal(exec.EvidenceIDs)
	if err != nil {
		return fmt.Errorf("marshal execution evidence ids: %w", err)
	}
	var finishedAtStr *string
	if exec.FinishedAt != nil {
		formatted := exec.FinishedAt.Format(time.RFC3339Nano)
		finishedAtStr = &formatted
	}

	query := `INSERT INTO orchestra_executions (
		id, task_id, attempt, status, started_at, finished_at, evidence_ids
	) VALUES (?, ?, ?, ?, ?, ?, ?)
	ON CONFLICT(id) DO UPDATE SET
		status = excluded.status,
		finished_at = excluded.finished_at,
		evidence_ids = excluded.evidence_ids;`

	_, err = s.db.ExecContext(ctx, query,
		string(exec.ID),
		string(exec.TaskID),
		exec.Attempt,
		string(exec.Status),
		exec.StartedAt.Format(time.RFC3339Nano),
		finishedAtStr,
		string(evidenceJSON),
	)
	if err != nil {
		return fmt.Errorf("save execution %q: %w", exec.ID, err)
	}
	return nil
}

func (s *Store) GetExecution(ctx context.Context, id orchestra.ExecutionID) (orchestra.Execution, error) {
	if s == nil || s.db == nil {
		return orchestra.Execution{}, orchestra.ErrStoreUnavailable
	}
	if err := ctx.Err(); err != nil {
		return orchestra.Execution{}, err
	}
	query := `SELECT id, task_id, attempt, status, started_at, finished_at, evidence_ids
		FROM orchestra_executions WHERE id = ?`

	row := s.db.QueryRowContext(ctx, query, string(id))
	var exec orchestra.Execution
	var rawID, taskIDStr, statusStr, startedAtStr string
	var finishedAtStr *string
	var evidenceStr string

	err := row.Scan(
		&rawID,
		&taskIDStr,
		&exec.Attempt,
		&statusStr,
		&startedAtStr,
		&finishedAtStr,
		&evidenceStr,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return orchestra.Execution{}, orchestra.ErrNotFound
		}
		return orchestra.Execution{}, fmt.Errorf("get execution %q: %w", id, err)
	}
	exec.ID = orchestra.ExecutionID(rawID)
	exec.TaskID = orchestra.TaskID(taskIDStr)
	exec.Status = orchestra.TaskStatus(statusStr)
	exec.StartedAt, _ = time.Parse(time.RFC3339Nano, startedAtStr)
	if finishedAtStr != nil {
		parsed, _ := time.Parse(time.RFC3339Nano, *finishedAtStr)
		exec.FinishedAt = &parsed
	}
	if err := json.Unmarshal([]byte(evidenceStr), &exec.EvidenceIDs); err != nil {
		exec.EvidenceIDs = []string{}
	}
	return exec, nil
}

func (s *Store) RegisterActive(ctx context.Context, taskID orchestra.TaskID, execID orchestra.ExecutionID, workerID string, now time.Time) error {
	if s == nil || s.db == nil {
		return orchestra.ErrStoreUnavailable
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}
	query := `INSERT INTO orchestra_active_tasks (task_id, execution_id, worker_id, started_at, heartbeat_at)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(task_id) DO UPDATE SET
			execution_id = excluded.execution_id,
			worker_id = excluded.worker_id,
			heartbeat_at = excluded.heartbeat_at;`

	_, err := s.db.ExecContext(ctx, query,
		string(taskID),
		string(execID),
		workerID,
		now.Format(time.RFC3339Nano),
		now.Format(time.RFC3339Nano),
	)
	if err != nil {
		return fmt.Errorf("register active task %q: %w", taskID, err)
	}
	return nil
}

func (s *Store) UnregisterActive(ctx context.Context, taskID orchestra.TaskID) error {
	if s == nil || s.db == nil {
		return orchestra.ErrStoreUnavailable
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	_, err := s.db.ExecContext(ctx, `DELETE FROM orchestra_active_tasks WHERE task_id = ?`, string(taskID))
	if err != nil {
		return fmt.Errorf("unregister active task %q: %w", taskID, err)
	}
	return nil
}

// RecoverInterrupted inspects tasks left in running/awaitingInspection or registered
// in orchestra_active_tasks, transitions them cleanly to failed or ready, appends
// recovery events, and clears active locks.
func (s *Store) RecoverInterrupted(ctx context.Context, reason string) ([]orchestra.TaskID, error) {
	if s == nil || s.db == nil {
		return nil, orchestra.ErrStoreUnavailable
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if reason == "" {
		reason = "recovered from interrupted process shutdown"
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin recovery: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	// Find tasks that are running or awaiting inspection
	rows, err := tx.QueryContext(ctx, `SELECT id, status, attempt_count, max_attempts FROM orchestra_tasks
		WHERE status IN (?, ?) ORDER BY id ASC`, orchestra.TaskRunning, orchestra.TaskAwaitingInspection)
	if err != nil {
		return nil, fmt.Errorf("query interrupted tasks: %w", err)
	}
	defer rows.Close()

	type interruptedTask struct {
		id       orchestra.TaskID
		status   orchestra.TaskStatus
		attempts int
		max      int
	}
	var interrupted []interruptedTask
	for rows.Next() {
		var it interruptedTask
		var rawID, rawStatus string
		if err := rows.Scan(&rawID, &rawStatus, &it.attempts, &it.max); err != nil {
			return nil, fmt.Errorf("scan interrupted task: %w", err)
		}
		it.id = orchestra.TaskID(rawID)
		it.status = orchestra.TaskStatus(rawStatus)
		interrupted = append(interrupted, it)
	}
	rows.Close()

	now := time.Now().UTC()
	var recoveredIDs []orchestra.TaskID

	for _, it := range interrupted {
		var newStatus orchestra.TaskStatus
		var eventType orchestra.EventType
		if it.attempts < it.max {
			newStatus = orchestra.TaskReady
			eventType = orchestra.EventTaskRetryRequested
		} else {
			newStatus = orchestra.TaskFailed
			eventType = orchestra.EventExecutionFailed
		}

		_, err := tx.ExecContext(ctx, `UPDATE orchestra_tasks SET status = ?, updated_at = ? WHERE id = ?`,
			string(newStatus), now.Format(time.RFC3339Nano), string(it.id))
		if err != nil {
			return nil, fmt.Errorf("update interrupted task %q: %w", it.id, err)
		}

		var maxSeq int64
		_ = tx.QueryRowContext(ctx, `SELECT COALESCE(MAX(sequence), 0) FROM orchestra_events WHERE task_id = ?`, string(it.id)).Scan(&maxSeq)
		eventID := fmt.Sprintf("%s-recovery-%d", it.id, now.UnixNano())
		_, err = tx.ExecContext(ctx, `INSERT INTO orchestra_events (
			id, task_id, execution_id, sequence, type, from_status, to_status, message, evidence_ids, occurred_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			eventID, string(it.id), "", maxSeq+1, string(eventType), string(it.status), string(newStatus), reason, "[]", now.Format(time.RFC3339Nano))
		if err != nil {
			return nil, fmt.Errorf("record recovery event for %q: %w", it.id, err)
		}
		recoveredIDs = append(recoveredIDs, it.id)
	}

	// Clean active table
	if _, err := tx.ExecContext(ctx, `DELETE FROM orchestra_active_tasks`); err != nil {
		return nil, fmt.Errorf("clear active tasks: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit recovery: %w", err)
	}
	return recoveredIDs, nil
}
