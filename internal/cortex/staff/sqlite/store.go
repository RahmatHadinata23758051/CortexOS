package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/RahmatHadinata23758051/CortexOS/internal/cortex/staff"

	_ "modernc.org/sqlite"
)

// ErrStoreUnavailable indicates the store is not open or has been closed.
var ErrStoreUnavailable = errors.New("staff store unavailable")

// ErrNotFound indicates a staff definition was not found.
var ErrNotFound = errors.New("staff definition not found")

// Store is the SQLite-backed implementation of staff.Store.
// Its database connection is deliberately private; callers can only use
// the validated Staff operations below.
type Store struct {
	db   *sql.DB
	path string
	mu   sync.RWMutex
}

var _ staff.Store = (*Store)(nil)

// Open creates (or opens) a Staff database, enables foreign-key enforcement,
// and applies all known migrations before returning a usable store.
func Open(ctx context.Context, path string) (*Store, error) {
	if ctx == nil {
		return nil, fmt.Errorf("open staff database: context is required")
	}
	if err := ctx.Err(); err != nil {
		return nil, staff.CanceledError(err)
	}
	if path == "" || !filepath.IsAbs(path) {
		return nil, fmt.Errorf("open staff database: path must be absolute")
	}
	cleanPath, err := filepath.Abs(filepath.Clean(path))
	if err != nil {
		return nil, fmt.Errorf("open staff database: canonicalize path: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(cleanPath), 0o700); err != nil {
		return nil, fmt.Errorf("open staff database: create parent directory: %w", err)
	}
	db, err := sql.Open("sqlite", cleanPath)
	if err != nil {
		return nil, fmt.Errorf("open staff database: %w", err)
	}
	// SQLite PRAGMAs are connection-local. One connection keeps the setting
	// invariant and also makes migration ordering deterministic.
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	store := &Store{db: db, path: cleanPath}
	if err := store.Open(ctx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("open staff database: %w", err)
	}
	return store, nil
}

// New creates a Store without opening the database. Call Open(ctx) before use.
// This is useful when the database path is known at construction time but
// the context is available later.
func New(path string) (*Store, error) {
	if path == "" || !filepath.IsAbs(path) {
		return nil, fmt.Errorf("new staff store: path must be absolute")
	}
	cleanPath, err := filepath.Abs(filepath.Clean(path))
	if err != nil {
		return nil, fmt.Errorf("new staff store: canonicalize path: %w", err)
	}
	return &Store{path: cleanPath}, nil
}

// Open satisfies staff.Store and is also useful for retrying migration setup
// on a store constructed by New. It is idempotent: if the store is already
// open it re-enables foreign keys and reapplies migrations.
func (s *Store) Open(ctx context.Context) error {
	if s == nil {
		return ErrStoreUnavailable
	}
	if ctx == nil {
		return staff.WrapError(staff.ErrInvalidRequest, "context is required", nil)
	}
	if err := ctx.Err(); err != nil {
		return staff.CanceledError(err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if s.db != nil {
		if _, err := s.db.ExecContext(ctx, `PRAGMA foreign_keys = ON;`); err != nil {
			return fmt.Errorf("enable staff foreign keys: %w", err)
		}
		return ApplyMigrations(ctx, s.db)
	}
	if s.path == "" {
		return fmt.Errorf("open staff database: path is required")
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0o700); err != nil {
		return fmt.Errorf("open staff database: create parent directory: %w", err)
	}
	db, err := sql.Open("sqlite", s.path)
	if err != nil {
		return fmt.Errorf("open staff database: %w", err)
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	s.db = db
	if _, err := s.db.ExecContext(ctx, `PRAGMA foreign_keys = ON;`); err != nil {
		_ = s.db.Close()
		s.db = nil
		return fmt.Errorf("enable staff foreign keys: %w", err)
	}
	if err := ApplyMigrations(ctx, s.db); err != nil {
		_ = s.db.Close()
		s.db = nil
		return fmt.Errorf("apply staff migrations: %w", err)
	}
	return nil
}

func (s *Store) Close() error {
	if s == nil {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.db == nil {
		return nil
	}
	err := s.db.Close()
	s.db = nil
	return err
}

func checkStore(ctx context.Context, s *Store) error {
	if s == nil {
		return staff.WrapError(staff.ErrInternal, "staff store is unavailable", ErrStoreUnavailable)
	}
	if ctx == nil {
		return staff.WrapError(staff.ErrInvalidRequest, "context is required", nil)
	}
	if err := ctx.Err(); err != nil {
		return staff.CanceledError(err)
	}
	s.mu.RLock()
	open := s.db != nil
	s.mu.RUnlock()
	if !open {
		return staff.WrapError(staff.ErrInternal, "staff store is unavailable", ErrStoreUnavailable)
	}
	return nil
}

func (s *Store) SchemaVersion(ctx context.Context) (string, error) {
	if err := checkStore(ctx, s); err != nil {
		return "", err
	}
	return staff.ContractVersion, nil
}

func (s *Store) SaveDefinition(ctx context.Context, definition staff.Definition) (staff.Definition, error) {
	if err := checkStore(ctx, s); err != nil {
		return staff.Definition{}, err
	}
	if err := definition.Validate(); err != nil {
		return staff.Definition{}, err
	}
	if err := ctx.Err(); err != nil {
		return staff.Definition{}, staff.CanceledError(err)
	}

	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		return staff.Definition{}, dbError("begin save staff", err)
	}
	defer func() { _ = tx.Rollback() }()

	_, err = tx.ExecContext(ctx, `INSERT INTO staff_definitions
		(id, name, role, workspace_id, project_id, worktree_id, lifecycle, availability,
		created_at, updated_at, schema_version)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET name=excluded.name, role=excluded.role,
		workspace_id=excluded.workspace_id, project_id=excluded.project_id,
		worktree_id=excluded.worktree_id, lifecycle=excluded.lifecycle,
		availability=excluded.availability, created_at=excluded.created_at,
		updated_at=excluded.updated_at, schema_version=excluded.schema_version`,
		string(definition.ID), definition.Name, string(definition.Role),
		string(definition.Workspace.WorkspaceID), string(definition.Workspace.ProjectID),
		string(definition.Workspace.WorktreeID), string(definition.Lifecycle),
		string(definition.Availability), formatTime(definition.CreatedAt),
		formatTime(definition.UpdatedAt), definition.SchemaVersion)
	if err != nil {
		return staff.Definition{}, dbError("save staff definition", err)
	}
	if err := replaceCollections(ctx, tx, definition); err != nil {
		return staff.Definition{}, err
	}
	if err := ctx.Err(); err != nil {
		return staff.Definition{}, staff.CanceledError(err)
	}
	if err := tx.Commit(); err != nil {
		return staff.Definition{}, dbError("commit staff definition", err)
	}
	return s.GetDefinition(ctx, definition.ID)
}

func replaceCollections(ctx context.Context, tx *sql.Tx, definition staff.Definition) error {
	id := string(definition.ID)
	for _, table := range []string{"staff_capabilities", "staff_permissions", "staff_skills", "staff_memory"} {
		if _, err := tx.ExecContext(ctx, "DELETE FROM "+table+" WHERE staff_id = ?", id); err != nil {
			return dbError("replace staff collection", err)
		}
	}
	for position, capability := range definition.Capabilities {
		if _, err := tx.ExecContext(ctx, `INSERT INTO staff_capabilities(staff_id, position, capability) VALUES (?, ?, ?)`, id, position, string(capability)); err != nil {
			return dbError("save staff capability", err)
		}
	}
	for position, permission := range definition.Permissions {
		if _, err := tx.ExecContext(ctx, `INSERT INTO staff_permissions(staff_id, position, permission_id, action, resource, effect, priority) VALUES (?, ?, ?, ?, ?, ?, ?)`, id, position, permission.ID, permission.Action, permission.Resource, string(permission.Effect), permission.Priority); err != nil {
			return dbError("save staff permission", err)
		}
	}
	for position, skill := range definition.Skills {
		if _, err := tx.ExecContext(ctx, `INSERT INTO staff_skills(staff_id, position, skill_id, skill_version) VALUES (?, ?, ?, ?)`, id, position, skill.ID, skill.Version); err != nil {
			return dbError("save staff skill reference", err)
		}
	}
	for position, memory := range definition.Memory {
		if _, err := tx.ExecContext(ctx, `INSERT INTO staff_memory(staff_id, position, memory_id, memory_kind, memory_version) VALUES (?, ?, ?, ?, ?)`, id, position, memory.ID, memory.Kind, memory.Version); err != nil {
			return dbError("save staff memory reference", err)
		}
	}
	return nil
}

func (s *Store) GetDefinition(ctx context.Context, id staff.StaffID) (staff.Definition, error) {
	if err := checkStore(ctx, s); err != nil {
		return staff.Definition{}, err
	}
	if err := validateID(id); err != nil {
		return staff.Definition{}, err
	}
	var d staff.Definition
	var role, workspaceID, projectID, worktreeID, lifecycle, availability, created, updated string
	err := s.db.QueryRowContext(ctx, `SELECT id, name, role, workspace_id, project_id, worktree_id,
		lifecycle, availability, created_at, updated_at, schema_version
		FROM staff_definitions WHERE id = ?`, string(id)).Scan(
		&d.ID, &d.Name, &role, &workspaceID, &projectID, &worktreeID, &lifecycle,
		&availability, &created, &updated, &d.SchemaVersion)
	if errors.Is(err, sql.ErrNoRows) {
		return staff.Definition{}, staff.WrapError(staff.ErrNotFound, "staff definition not found", err)
	}
	if err != nil {
		return staff.Definition{}, dbError("get staff definition", err)
	}
	d.Role = staff.Role(role)
	d.Workspace = staff.WorkspaceAssignment{WorkspaceID: staff.WorkspaceID(workspaceID), ProjectID: staff.ProjectID(projectID), WorktreeID: staff.WorktreeID(worktreeID)}
	d.Lifecycle = staff.LifecycleState(lifecycle)
	d.Availability = staff.AvailabilityState(availability)
	d.CreatedAt, err = parseTime(created)
	if err != nil {
		return staff.Definition{}, corruptError("created_at", err)
	}
	d.UpdatedAt, err = parseTime(updated)
	if err != nil {
		return staff.Definition{}, corruptError("updated_at", err)
	}
	d.Capabilities = []staff.Capability{}
	d.Permissions = staff.PermissionSet{}
	d.Skills = []staff.SkillReference{}
	d.Memory = []staff.MemoryReference{}
	if err := loadCollections(ctx, s.db, &d); err != nil {
		return staff.Definition{}, err
	}
	if err := d.Validate(); err != nil {
		return staff.Definition{}, corruptValidationError(err)
	}
	return d, nil
}

func loadCollections(ctx context.Context, db *sql.DB, d *staff.Definition) error {
	rows, err := db.QueryContext(ctx, `SELECT capability FROM staff_capabilities WHERE staff_id = ? ORDER BY position ASC`, string(d.ID))
	if err != nil {
		return dbError("read staff capabilities", err)
	}
	for rows.Next() {
		var value string
		if err := rows.Scan(&value); err != nil {
			_ = rows.Close()
			return corruptError("capability", err)
		}
		d.Capabilities = append(d.Capabilities, staff.Capability(value))
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return dbError("iterate staff capabilities", err)
	}
	if err := rows.Close(); err != nil {
		return dbError("close staff capabilities", err)
	}

	rows, err = db.QueryContext(ctx, `SELECT permission_id, action, resource, effect, priority FROM staff_permissions WHERE staff_id = ? ORDER BY position ASC`, string(d.ID))
	if err != nil {
		return dbError("read staff permissions", err)
	}
	for rows.Next() {
		var p staff.Permission
		var effect string
		if err := rows.Scan(&p.ID, &p.Action, &p.Resource, &effect, &p.Priority); err != nil {
			_ = rows.Close()
			return corruptError("permission", err)
		}
		p.Effect = staff.PermissionEffect(effect)
		d.Permissions = append(d.Permissions, p)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return dbError("iterate staff permissions", err)
	}
	if err := rows.Close(); err != nil {
		return dbError("close staff permissions", err)
	}

	rows, err = db.QueryContext(ctx, `SELECT skill_id, skill_version FROM staff_skills WHERE staff_id = ? ORDER BY position ASC`, string(d.ID))
	if err != nil {
		return dbError("read staff skills", err)
	}
	for rows.Next() {
		var ref staff.SkillReference
		if err := rows.Scan(&ref.ID, &ref.Version); err != nil {
			_ = rows.Close()
			return corruptError("skill reference", err)
		}
		d.Skills = append(d.Skills, ref)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return dbError("iterate staff skills", err)
	}
	if err := rows.Close(); err != nil {
		return dbError("close staff skills", err)
	}

	rows, err = db.QueryContext(ctx, `SELECT memory_id, memory_kind, memory_version FROM staff_memory WHERE staff_id = ? ORDER BY position ASC`, string(d.ID))
	if err != nil {
		return dbError("read staff memory", err)
	}
	for rows.Next() {
		var ref staff.MemoryReference
		if err := rows.Scan(&ref.ID, &ref.Kind, &ref.Version); err != nil {
			_ = rows.Close()
			return corruptError("memory reference", err)
		}
		d.Memory = append(d.Memory, ref)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return dbError("iterate staff memory", err)
	}
	if err := rows.Close(); err != nil {
		return dbError("close staff memory", err)
	}
	return nil
}

func (s *Store) DeleteDefinition(ctx context.Context, id staff.StaffID) error {
	if err := checkStore(ctx, s); err != nil {
		return err
	}
	if err := validateID(id); err != nil {
		return err
	}
	result, err := s.db.ExecContext(ctx, `DELETE FROM staff_definitions WHERE id = ?`, string(id))
	if err != nil {
		return dbError("delete staff definition", err)
	}
	n, err := result.RowsAffected()
	if err != nil {
		return dbError("check deleted staff definition", err)
	}
	if n == 0 {
		return staff.WrapError(staff.ErrNotFound, "staff definition not found", sql.ErrNoRows)
	}
	return nil
}

func (s *Store) ListDefinitions(ctx context.Context) ([]staff.Definition, error) {
	return s.list(ctx, staff.Filter{})
}

func (s *Store) ListDefinitionsByWorkspace(ctx context.Context, id staff.WorkspaceID) ([]staff.Definition, error) {
	return s.list(ctx, staff.Filter{WorkspaceID: &id})
}

func (s *Store) ListDefinitionsByProject(ctx context.Context, id staff.ProjectID) ([]staff.Definition, error) {
	return s.list(ctx, staff.Filter{ProjectID: &id})
}

func (s *Store) ListDefinitionsByRole(ctx context.Context, role staff.Role) ([]staff.Definition, error) {
	return s.list(ctx, staff.Filter{Role: &role})
}

func (s *Store) ListDefinitionsWithFilter(ctx context.Context, filter staff.Filter) ([]staff.Definition, error) {
	return s.list(ctx, filter)
}

func (s *Store) list(ctx context.Context, filter staff.Filter) ([]staff.Definition, error) {
	if err := checkStore(ctx, s); err != nil {
		return nil, err
	}
	if err := filter.Validate(); err != nil {
		return nil, err
	}
	query := `SELECT id FROM staff_definitions`
	args := make([]any, 0, 4)
	conditions := make([]string, 0, 4)
	if filter.WorkspaceID != nil {
		conditions = append(conditions, "workspace_id = ?")
		args = append(args, string(*filter.WorkspaceID))
	}
	if filter.ProjectID != nil {
		conditions = append(conditions, "project_id = ?")
		args = append(args, string(*filter.ProjectID))
	}
	if filter.Role != nil {
		conditions = append(conditions, "role = ?")
		args = append(args, string(*filter.Role))
	}
	if filter.ActiveOnly {
		conditions = append(conditions, "lifecycle = ?")
		args = append(args, string(staff.LifecycleActive))
	}
	if len(conditions) > 0 {
		query += " WHERE " + joinConditions(conditions)
	}
	query += " ORDER BY id ASC"
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, dbError("list staff definitions", err)
	}
	var ids []staff.StaffID
	for rows.Next() {
		var id staff.StaffID
		if err := rows.Scan(&id); err != nil {
			_ = rows.Close()
			return nil, dbError("decode staff list", err)
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return nil, dbError("iterate staff list", err)
	}
	if err := rows.Close(); err != nil {
		return nil, dbError("close staff list", err)
	}
	definitions := make([]staff.Definition, 0, len(ids))
	for _, id := range ids {
		definition, err := s.GetDefinition(ctx, id)
		if err != nil {
			return nil, err
		}
		definitions = append(definitions, definition)
	}
	return definitions, nil
}

func validateID(id staff.StaffID) error {
	if string(id) == "" {
		return staff.WrapError(staff.ErrInvalidRequest, "staff id is required", nil)
	}
	if err := (staff.Identity{ID: id, Name: "placeholder"}).Validate(); err != nil {
		return err
	}
	return nil
}

func formatTime(t time.Time) string {
	return t.UTC().Format(time.RFC3339Nano)
}

func parseTime(value string) (time.Time, error) {
	return time.Parse(time.RFC3339Nano, value)
}

func dbError(operation string, err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return staff.CanceledError(err)
	}
	return staff.WrapError(staff.ErrInternal, operation, err)
}

func corruptValidationError(err error) error {
	return err
}

func corruptError(field string, err error) error {
	return staff.WrapError(staff.ErrInternal, "corrupt staff record: "+field, err)
}

func joinConditions(conditions []string) string {
	if len(conditions) == 0 {
		return ""
	}
	result := conditions[0]
	for _, condition := range conditions[1:] {
		result += " AND " + condition
	}
	return result
}