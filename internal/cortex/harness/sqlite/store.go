package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/RahmatHadinata23758051/CortexOS/internal/cortex/harness"

	_ "modernc.org/sqlite"
)

var (
	// These aliases let application error mapping remain stable without exposing
	// SQLite implementation details through the bridge.
	ErrStoreUnavailable = harness.ErrStoreUnavailable
	ErrOwnership        = harness.ErrOwnership
	ErrNotFound         = harness.ErrNotFound
)

// Store owns durable Harness state. It deliberately exposes typed operations
// rather than allowing callers to persist arbitrary process or output data.
type Store struct {
	db   *sql.DB
	path string
}

var recoveryMutex sync.Mutex

func Open(ctx context.Context, path string) (*Store, error) {
	if ctx == nil {
		return nil, fmt.Errorf("open harness database: context is required")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if path == "" || !filepath.IsAbs(path) {
		return nil, fmt.Errorf("open harness database: path must be absolute")
	}
	cleanPath, err := filepath.Abs(filepath.Clean(path))
	if err != nil {
		return nil, fmt.Errorf("open harness database: canonicalize path: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(cleanPath), 0o700); err != nil {
		return nil, fmt.Errorf("open harness database: create parent directory: %w", err)
	}
	db, err := sql.Open("sqlite", cleanPath)
	if err != nil {
		return nil, fmt.Errorf("open harness database: %w", err)
	}
	// A single connection prevents per-connection PRAGMA differences and makes
	// migration/recovery ordering deterministic for this process.
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	store := &Store{db: db, path: cleanPath}
	if _, err := db.ExecContext(ctx, `PRAGMA foreign_keys = ON`); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("open harness database: enable foreign keys: %w", err)
	}
	if err := ApplyMigrations(ctx, db); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("open harness database: %w", err)
	}
	return store, nil
}

func (s *Store) DB() *sql.DB {
	if s == nil {
		return nil
	}
	return s.db
}

func (s *Store) Path() string {
	if s == nil {
		return ""
	}
	return s.path
}

func (s *Store) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}

func checkStore(ctx context.Context, s *Store) error {
	if s == nil || s.db == nil {
		return ErrStoreUnavailable
	}
	if ctx == nil {
		return errors.New("context is required")
	}
	return ctx.Err()
}

// PolicyState is the persisted, versioned policy configuration metadata.
type PolicyState struct {
	ID              string
	Name            string
	ContractVersion string
	Rules           harness.Policy
	CreatedAt       time.Time
	UpdatedAt       time.Time
	Active          bool
}

func (s *Store) SavePolicy(ctx context.Context, state PolicyState) error {
	if err := checkStore(ctx, s); err != nil {
		return err
	}
	if state.ID == "" || state.Name == "" {
		return fmt.Errorf("policy id and name are required")
	}
	if state.ContractVersion == "" {
		state.ContractVersion = harness.PolicyContractVersion
	}
	if err := state.Rules.Validate(); err != nil {
		return err
	}
	if state.CreatedAt.IsZero() {
		state.CreatedAt = time.Now().UTC()
	}
	if state.UpdatedAt.IsZero() {
		state.UpdatedAt = state.CreatedAt
	}
	rules, err := json.Marshal(state.Rules)
	if err != nil {
		return fmt.Errorf("marshal policy: %w", err)
	}
	_, err = s.db.ExecContext(ctx, `INSERT INTO harness_policies
		(id, name, contract_version, rules_json, created_at, updated_at, active)
		VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET name=excluded.name,
		contract_version=excluded.contract_version, rules_json=excluded.rules_json,
		updated_at=excluded.updated_at, active=excluded.active`,
		state.ID, state.Name, state.ContractVersion, string(harness.RedactBytes(rules)),
		state.CreatedAt.UTC().Format(time.RFC3339Nano), state.UpdatedAt.UTC().Format(time.RFC3339Nano), boolInt(state.Active))
	if err != nil {
		return fmt.Errorf("save policy %q: %w", state.ID, err)
	}
	return nil
}

func (s *Store) GetPolicy(ctx context.Context, id string) (PolicyState, error) {
	if err := checkStore(ctx, s); err != nil {
		return PolicyState{}, err
	}
	var state PolicyState
	var rules, created, updated string
	var active int
	err := s.db.QueryRowContext(ctx, `SELECT id, name, contract_version, rules_json, created_at, updated_at, active
		FROM harness_policies WHERE id = ?`, id).Scan(&state.ID, &state.Name, &state.ContractVersion, &rules, &created, &updated, &active)
	if errors.Is(err, sql.ErrNoRows) {
		return PolicyState{}, ErrNotFound
	}
	if err != nil {
		return PolicyState{}, fmt.Errorf("get policy %q: %w", id, err)
	}
	if err := json.Unmarshal([]byte(rules), &state.Rules); err != nil {
		return PolicyState{}, fmt.Errorf("decode policy %q: %w", id, err)
	}
	state.CreatedAt, _ = time.Parse(time.RFC3339Nano, created)
	state.UpdatedAt, _ = time.Parse(time.RFC3339Nano, updated)
	state.Active = active != 0
	return state, nil
}

// ToolState is the persisted tool configuration. Schemas are redacted JSON,
// never command output, environment, credentials, or host paths.
type ToolState struct {
	ID           string
	Definition   harness.ToolDefinition
	RegisteredAt time.Time
	UpdatedAt    time.Time
	Active       bool
}

func (s *Store) SaveTool(ctx context.Context, state ToolState) error {
	if err := checkStore(ctx, s); err != nil {
		return err
	}
	if state.ID == "" {
		return errors.New("tool id is required")
	}
	if err := state.Definition.Validate(); err != nil {
		return err
	}
	if state.RegisteredAt.IsZero() {
		state.RegisteredAt = time.Now().UTC()
	}
	if state.UpdatedAt.IsZero() {
		state.UpdatedAt = state.RegisteredAt
	}
	caps, err := json.Marshal(state.Definition.Capabilities)
	if err != nil {
		return fmt.Errorf("marshal tool capabilities: %w", err)
	}
	_, err = s.db.ExecContext(ctx, `INSERT INTO harness_tools
		(id, name, kind, description, capabilities_json, input_schema_json, output_schema_json,
		timeout_ms, requires_ask, version, registered_at, updated_at, active)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET name=excluded.name, kind=excluded.kind,
		description=excluded.description, capabilities_json=excluded.capabilities_json,
		input_schema_json=excluded.input_schema_json, output_schema_json=excluded.output_schema_json,
		timeout_ms=excluded.timeout_ms, requires_ask=excluded.requires_ask, version=excluded.version,
		updated_at=excluded.updated_at, active=excluded.active`,
		state.ID, state.Definition.Name, state.Definition.Kind, harness.RedactString(state.Definition.Description),
		harness.RedactBytes(caps), harness.RedactBytes(state.Definition.InputSchema), harness.RedactBytes(state.Definition.OutputSchema),
		state.Definition.Timeout.Milliseconds(), boolInt(state.Definition.RequiresAsk), state.Definition.Version,
		state.RegisteredAt.UTC().Format(time.RFC3339Nano), state.UpdatedAt.UTC().Format(time.RFC3339Nano), boolInt(state.Active))
	if err != nil {
		return fmt.Errorf("save tool %q: %w", state.ID, err)
	}
	return nil
}

// WorkerState identifies the owning OS/application process. ownerProcessID is
// an application-generated stable lease identity, not a path or secret.
type WorkerState struct {
	ID              string
	InstanceID      string
	Kind            harness.ToolKind
	Class           harness.EngineClass
	Version         string
	SchemaVersion   string
	OwnerPID        int
	OwnerProcessID  string
	RegisteredAt    time.Time
	LastHeartbeatAt time.Time
	Status          string
	Metadata        map[string]any
}

func (s *Store) RegisterWorker(ctx context.Context, worker WorkerState) error {
	if err := checkStore(ctx, s); err != nil {
		return err
	}
	if worker.ID == "" || worker.InstanceID == "" || worker.OwnerProcessID == "" {
		return errors.New("worker id, instance id, and owner process id are required")
	}
	if worker.OwnerPID < 0 {
		return errors.New("worker owner pid must not be negative")
	}
	if worker.RegisteredAt.IsZero() {
		worker.RegisteredAt = time.Now().UTC()
	}
	if worker.LastHeartbeatAt.IsZero() {
		worker.LastHeartbeatAt = worker.RegisteredAt
	}
	if worker.Status == "" {
		worker.Status = "healthy"
	}
	metadata, err := marshalRedacted(worker.Metadata)
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx, `INSERT INTO harness_workers
		(id, instance_id, kind, class, version, schema_version, owner_pid, owner_process_id,
		registered_at, last_heartbeat_at, status, metadata_json)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET instance_id=excluded.instance_id, kind=excluded.kind,
		class=excluded.class, version=excluded.version, schema_version=excluded.schema_version,
		owner_pid=excluded.owner_pid, owner_process_id=excluded.owner_process_id,
		last_heartbeat_at=excluded.last_heartbeat_at, status=excluded.status, metadata_json=excluded.metadata_json`,
		worker.ID, worker.InstanceID, worker.Kind, worker.Class, worker.Version, worker.SchemaVersion,
		worker.OwnerPID, worker.OwnerProcessID, worker.RegisteredAt.UTC().Format(time.RFC3339Nano),
		worker.LastHeartbeatAt.UTC().Format(time.RFC3339Nano), worker.Status, metadata)
	if err != nil {
		return fmt.Errorf("register worker %q: %w", worker.ID, err)
	}
	return nil
}

func (s *Store) HeartbeatWorker(ctx context.Context, workerID, ownerProcessID string, at time.Time) error {
	if err := checkStore(ctx, s); err != nil {
		return err
	}
	if at.IsZero() {
		at = time.Now().UTC()
	}
	result, err := s.db.ExecContext(ctx, `UPDATE harness_workers SET last_heartbeat_at=?, status='healthy'
		WHERE id=? AND owner_process_id=?`, at.UTC().Format(time.RFC3339Nano), workerID, ownerProcessID)
	if err != nil {
		return fmt.Errorf("heartbeat worker %q: %w", workerID, err)
	}
	if n, _ := result.RowsAffected(); n != 1 {
		return ownershipOrNotFound(ctx, s, workerID)
	}
	return nil
}

func (s *Store) DeleteWorker(ctx context.Context, workerID, ownerProcessID string) error {
	if err := checkStore(ctx, s); err != nil {
		return err
	}
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		return fmt.Errorf("delete worker: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	var owner string
	if err := tx.QueryRowContext(ctx, `SELECT owner_process_id FROM harness_workers WHERE id=?`, workerID).Scan(&owner); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		}
		return err
	}
	if owner != ownerProcessID {
		return ErrOwnership
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM harness_workers WHERE id=? AND owner_process_id=?`, workerID, ownerProcessID); err != nil {
		return fmt.Errorf("delete worker: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit delete worker: %w", err)
	}
	return nil
}

// ReservationState is a durable lease. Release is owner-checked and idempotent.
type ReservationState struct {
	ID             string
	TaskID         string
	EngineClass    harness.EngineClass
	Priority       harness.Priority
	MemoryMB       int
	CPUPriority    int
	TraceID        string
	OwnerWorkerID  string
	OwnerProcessID string
	CreatedAt      time.Time
	ReleasedAt     *time.Time
	Active         bool
}

func (s *Store) SaveReservation(ctx context.Context, reservation ReservationState) error {
	if err := checkStore(ctx, s); err != nil {
		return err
	}
	if reservation.ID == "" || reservation.TaskID == "" || reservation.OwnerWorkerID == "" || reservation.OwnerProcessID == "" {
		return errors.New("reservation id, task, owner worker, and owner process are required")
	}
	if reservation.CreatedAt.IsZero() {
		reservation.CreatedAt = time.Now().UTC()
	}
	var released any
	if reservation.ReleasedAt != nil {
		released = reservation.ReleasedAt.UTC().Format(time.RFC3339Nano)
	}
	_, err := s.db.ExecContext(ctx, `INSERT INTO harness_reservations
		(id, task_id, engine_class, priority, memory_mb, cpu_priority, trace_id, owner_worker_id,
		owner_process_id, created_at, released_at, active) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET released_at=excluded.released_at, active=excluded.active`,
		reservation.ID, reservation.TaskID, reservation.EngineClass, reservation.Priority, reservation.MemoryMB,
		reservation.CPUPriority, harness.RedactString(reservation.TraceID), reservation.OwnerWorkerID,
		reservation.OwnerProcessID, reservation.CreatedAt.UTC().Format(time.RFC3339Nano), released, boolInt(reservation.Active))
	if err != nil {
		return fmt.Errorf("save reservation %q: %w", reservation.ID, err)
	}
	return nil
}

// ListWorkers returns internal worker records for the application mapper.
func (s *Store) ListWorkers(ctx context.Context) ([]WorkerState, error) {
	if err := checkStore(ctx, s); err != nil {
		return nil, err
	}
	rows, err := s.db.QueryContext(ctx, `SELECT id, instance_id, kind, class, version, schema_version, owner_pid, owner_process_id, registered_at, last_heartbeat_at, status, metadata_json FROM harness_workers ORDER BY registered_at DESC`)
	if err != nil {
		return nil, fmt.Errorf("list workers: %w", err)
	}
	defer rows.Close()
	var workers []WorkerState
	for rows.Next() {
		var w WorkerState
		var registered, heartbeat, metadata string
		if err := rows.Scan(&w.ID, &w.InstanceID, &w.Kind, &w.Class, &w.Version, &w.SchemaVersion, &w.OwnerPID, &w.OwnerProcessID, &registered, &heartbeat, &w.Status, &metadata); err != nil {
			return nil, err
		}
		w.RegisteredAt, _ = time.Parse(time.RFC3339Nano, registered)
		w.LastHeartbeatAt, _ = time.Parse(time.RFC3339Nano, heartbeat)
		_ = json.Unmarshal([]byte(metadata), &w.Metadata)
		workers = append(workers, w)
	}
	return workers, rows.Err()
}

// ListActiveExecutions returns internal active reservation records for the application mapper.
func (s *Store) ListActiveExecutions(ctx context.Context) ([]ReservationState, error) {
	if err := checkStore(ctx, s); err != nil {
		return nil, err
	}
	rows, err := s.db.QueryContext(ctx, `SELECT id, task_id, engine_class, priority, memory_mb, cpu_priority, trace_id, owner_worker_id, owner_process_id, created_at, released_at, active FROM harness_reservations WHERE active=1 ORDER BY created_at DESC`)
	if err != nil {
		return nil, fmt.Errorf("list active executions: %w", err)
	}
	defer rows.Close()
	var reservations []ReservationState
	for rows.Next() {
		var r ReservationState
		var created string
		var released sql.NullString
		if err := rows.Scan(&r.ID, &r.TaskID, &r.EngineClass, &r.Priority, &r.MemoryMB, &r.CPUPriority, &r.TraceID, &r.OwnerWorkerID, &r.OwnerProcessID, &created, &released, &r.Active); err != nil {
			return nil, err
		}
		r.CreatedAt, _ = time.Parse(time.RFC3339Nano, created)
		if released.Valid && released.String != "" {
			if t, err := time.Parse(time.RFC3339Nano, released.String); err == nil {
				r.ReleasedAt = &t
			}
		}
		reservations = append(reservations, r)
	}
	return reservations, rows.Err()
}

// ListCapabilities returns persisted capability bindings.
func (s *Store) ListCapabilities(ctx context.Context) ([]harness.CapabilityBinding, error) {
	if err := checkStore(ctx, s); err != nil {
		return nil, err
	}
	rows, err := s.db.QueryContext(ctx, `SELECT capability, description, requires_ask, default_effect FROM harness_capabilities ORDER BY capability`)
	if err != nil {
		return nil, fmt.Errorf("list capabilities: %w", err)
	}
	defer rows.Close()
	var caps []harness.CapabilityBinding
	for rows.Next() {
		var c harness.CapabilityBinding
		var requiresAsk int
		if err := rows.Scan(&c.Capability, &c.Description, &requiresAsk, &c.DefaultEffect); err != nil {
			return nil, err
		}
		c.RequiresAsk = requiresAsk != 0
		caps = append(caps, c)
	}
	return caps, rows.Err()
}

// ListEvidence is the typed read-only evidence operation used by the application port.
func (s *Store) ListEvidence(ctx context.Context, executionID string) ([]harness.EvidenceRecord, error) {
	return s.EvidenceForExecution(ctx, executionID)
}

func (s *Store) ReleaseReservation(ctx context.Context, reservationID, ownerProcessID string, at time.Time) error {
	if err := checkStore(ctx, s); err != nil {
		return err
	}
	if at.IsZero() {
		at = time.Now().UTC()
	}
	result, err := s.db.ExecContext(ctx, `UPDATE harness_reservations SET active=0, released_at=?
		WHERE id=? AND owner_process_id=? AND active=1`, at.UTC().Format(time.RFC3339Nano), reservationID, ownerProcessID)
	if err != nil {
		return fmt.Errorf("release reservation %q: %w", reservationID, err)
	}
	if n, _ := result.RowsAffected(); n == 1 {
		return nil
	}
	var owner string
	err = s.db.QueryRowContext(ctx, `SELECT owner_process_id FROM harness_reservations WHERE id=?`, reservationID).Scan(&owner)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	if owner != ownerProcessID {
		return ErrOwnership
	}
	return nil // already released by this owner
}

func (s *Store) RecordHeartbeat(ctx context.Context, heartbeatID, workerID, processID string, healthy bool, memoryMB, cpuPriority int, at time.Time, details map[string]any) error {
	if err := checkStore(ctx, s); err != nil {
		return err
	}
	if heartbeatID == "" || workerID == "" || processID == "" {
		return errors.New("heartbeat id, worker id, and process id are required")
	}
	if at.IsZero() {
		at = time.Now().UTC()
	}
	detailsJSON, err := marshalRedacted(details)
	if err != nil {
		return err
	}
	var owner string
	if err := s.db.QueryRowContext(ctx, `SELECT owner_process_id FROM harness_workers WHERE id=?`, workerID).Scan(&owner); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		}
		return err
	}
	if owner != processID {
		return ErrOwnership
	}
	_, err = s.db.ExecContext(ctx, `INSERT INTO harness_heartbeats
		(id, worker_id, process_id, healthy, memory_mb, cpu_priority, at, details_json) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		heartbeatID, workerID, processID, boolInt(healthy), memoryMB, cpuPriority, at.UTC().Format(time.RFC3339Nano), detailsJSON)
	if err != nil {
		return fmt.Errorf("record heartbeat: %w", err)
	}
	return nil
}

func (s *Store) SaveEvidence(ctx context.Context, record harness.EvidenceRecord) error {
	if err := checkStore(ctx, s); err != nil {
		return err
	}
	if record.ID == "" || record.ExecutionID == "" || record.TaskID == "" || record.WorktreeID == "" || record.Digest == "" {
		return errors.New("evidence id, execution, task, worktree, and digest are required")
	}
	if len(record.RedactedPayload) == 0 || !json.Valid(record.RedactedPayload) {
		return errors.New("evidence payload must be valid redacted JSON")
	}
	audit, err := json.Marshal(record.Audit)
	if err != nil {
		return fmt.Errorf("marshal evidence audit: %w", err)
	}
	collected := record.CollectedAt
	if collected.IsZero() {
		collected = time.Now().UTC()
	}
	_, err = s.db.ExecContext(ctx, `INSERT INTO harness_evidence
		(id, contract_version, execution_id, task_id, worktree_id, kind, digest,
		redacted_payload_json, audit_json, collected_at, schema_version) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO NOTHING`, record.ID, record.ContractVersion, record.ExecutionID, record.TaskID,
		record.WorktreeID, record.Kind, record.Digest, harness.RedactBytes(record.RedactedPayload),
		harness.RedactBytes(audit), collected.UTC().Format(time.RFC3339Nano), record.SchemaVersion)
	if err != nil {
		return fmt.Errorf("save evidence %q: %w", record.ID, err)
	}
	return nil
}

func (s *Store) EvidenceForExecution(ctx context.Context, executionID string) ([]harness.EvidenceRecord, error) {
	if err := checkStore(ctx, s); err != nil {
		return nil, err
	}
	rows, err := s.db.QueryContext(ctx, `SELECT id, contract_version, execution_id, task_id, worktree_id, kind, digest,
		redacted_payload_json, audit_json, collected_at, schema_version FROM harness_evidence WHERE execution_id=? ORDER BY collected_at, id`, executionID)
	if err != nil {
		return nil, fmt.Errorf("list evidence: %w", err)
	}
	defer rows.Close()
	var records []harness.EvidenceRecord
	for rows.Next() {
		var r harness.EvidenceRecord
		var kind, payload, audit, collected string
		if err := rows.Scan(&r.ID, &r.ContractVersion, &r.ExecutionID, &r.TaskID, &r.WorktreeID, &kind, &r.Digest, &payload, &audit, &collected, &r.SchemaVersion); err != nil {
			return nil, err
		}
		r.Kind = harness.EvidenceKind(kind)
		r.RedactedPayload = json.RawMessage(payload)
		_ = json.Unmarshal([]byte(audit), &r.Audit)
		r.CollectedAt, _ = time.Parse(time.RFC3339Nano, collected)
		records = append(records, r)
	}
	return records, rows.Err()
}

// RecoverInterrupted atomically marks active workers as interrupted and
// releases their leases. It never creates a successful result or mutates
// evidence. Repeating it is a no-op, making startup recovery idempotent.
func (s *Store) RecoverInterrupted(ctx context.Context, now time.Time) ([]string, error) {
	if err := checkStore(ctx, s); err != nil {
		return nil, err
	}
	recoveryMutex.Lock()
	defer recoveryMutex.Unlock()
	if now.IsZero() {
		now = time.Now().UTC()
	}
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		return nil, fmt.Errorf("begin harness recovery: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	rows, err := tx.QueryContext(ctx, `SELECT id FROM harness_workers WHERE status IN ('healthy','running') ORDER BY id`)
	if err != nil {
		return nil, fmt.Errorf("find interrupted workers: %w", err)
	}
	var workerIDs []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return nil, err
		}
		workerIDs = append(workerIDs, id)
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	for _, workerID := range workerIDs {
		if _, err := tx.ExecContext(ctx, `UPDATE harness_workers SET status='interrupted', last_heartbeat_at=? WHERE id=? AND status IN ('healthy','running')`, now.UTC().Format(time.RFC3339Nano), workerID); err != nil {
			return nil, fmt.Errorf("mark worker interrupted: %w", err)
		}
		if _, err := tx.ExecContext(ctx, `UPDATE harness_reservations SET active=0, released_at=? WHERE owner_worker_id=? AND active=1`, now.UTC().Format(time.RFC3339Nano), workerID); err != nil {
			return nil, fmt.Errorf("release worker reservations: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit harness recovery: %w", err)
	}
	return workerIDs, nil
}

// ApprovalState is the persisted approval record for policy "ask" decisions.
type ApprovalState struct {
	ID          string
	ExecutionID string
	ToolName    string
	Action      string
	Approver    string
	Reason      string
	GrantedAt   time.Time
}

func (s *Store) SaveApproval(ctx context.Context, approval ApprovalState) error {
	if err := checkStore(ctx, s); err != nil {
		return err
	}
	if approval.ID == "" || approval.ExecutionID == "" || approval.Approver == "" {
		return errors.New("approval id, execution id, and approver are required")
	}
	if approval.GrantedAt.IsZero() {
		approval.GrantedAt = time.Now().UTC()
	}
	_, err := s.db.ExecContext(ctx, `INSERT INTO harness_approvals
		(id, execution_id, tool_name, action, approver, reason, granted_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(execution_id) DO UPDATE SET tool_name=excluded.tool_name,
		action=excluded.action, approver=excluded.approver, reason=excluded.reason,
		granted_at=excluded.granted_at`,
		approval.ID, approval.ExecutionID, approval.ToolName, approval.Action,
		harness.RedactString(approval.Approver), harness.RedactString(approval.Reason),
		approval.GrantedAt.UTC().Format(time.RFC3339Nano))
	if err != nil {
		return fmt.Errorf("save approval %q: %w", approval.ID, err)
	}
	return nil
}

func (s *Store) GetApproval(ctx context.Context, executionID string) (ApprovalState, error) {
	if err := checkStore(ctx, s); err != nil {
		return ApprovalState{}, err
	}
	var app ApprovalState
	var granted string
	var toolName, action, reason sql.NullString
	err := s.db.QueryRowContext(ctx, `SELECT id, execution_id, tool_name, action, approver, reason, granted_at
		FROM harness_approvals WHERE execution_id = ?`, executionID).Scan(
		&app.ID, &app.ExecutionID, &toolName, &action, &app.Approver, &reason, &granted)
	if errors.Is(err, sql.ErrNoRows) {
		return ApprovalState{}, ErrNotFound
	}
	if err != nil {
		return ApprovalState{}, fmt.Errorf("get approval %q: %w", executionID, err)
	}
	if toolName.Valid {
		app.ToolName = toolName.String
	}
	if action.Valid {
		app.Action = action.String
	}
	if reason.Valid {
		app.Reason = reason.String
	}
	app.GrantedAt, _ = time.Parse(time.RFC3339Nano, granted)
	return app, nil
}

func (s *Store) DeleteApproval(ctx context.Context, executionID string) error {
	if err := checkStore(ctx, s); err != nil {
		return err
	}
	result, err := s.db.ExecContext(ctx, `DELETE FROM harness_approvals WHERE execution_id = ?`, executionID)
	if err != nil {
		return fmt.Errorf("delete approval %q: %w", executionID, err)
	}
	if n, _ := result.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

func ownershipOrNotFound(ctx context.Context, s *Store, id string) error {
	var owner string
	err := s.db.QueryRowContext(ctx, `SELECT owner_process_id FROM harness_workers WHERE id=?`, id).Scan(&owner)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	return ErrOwnership
}

func marshalRedacted(value any) (string, error) {
	if value == nil {
		return "{}", nil
	}
	raw, err := json.Marshal(value)
	if err != nil {
		return "", fmt.Errorf("marshal metadata: %w", err)
	}
	return string(harness.RedactBytes(raw)), nil
}

func boolInt(value bool) int {
	if value {
		return 1
	}
	return 0
}
