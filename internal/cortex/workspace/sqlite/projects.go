package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"sort"
	"time"

	"github.com/RahmatHadinata23758051/CortexOS/internal/cortex/workspace"
)

var (
	_ workspace.ProjectRegistry = (*Store)(nil)
	_ workspace.StateStore      = (*Store)(nil)
)

func (s *Store) Open(ctx context.Context) error {
	if s == nil || s.db == nil {
		return fmt.Errorf("open workspace store: database is unavailable")
	}
	if ctx == nil {
		return fmt.Errorf("open workspace store: context is required")
	}
	if err := ctx.Err(); err != nil {
		return workspace.CanceledError(err)
	}
	if err := s.db.PingContext(ctx); err != nil {
		return fmt.Errorf("open workspace store: %w", err)
	}
	return ApplyMigrations(ctx, s.db)
}

func (s *Store) SchemaVersion(ctx context.Context) (string, error) {
	if s == nil || s.db == nil {
		return "", workspace.NewError(workspace.ErrStorageUnavailable, "database is unavailable")
	}
	if ctx == nil {
		return "", workspace.NewError(workspace.ErrInvalidRequest, "context is required")
	}
	if err := ctx.Err(); err != nil {
		return "", workspace.CanceledError(err)
	}
	var version int
	if err := s.db.QueryRowContext(ctx, `SELECT COALESCE(MAX(version), 0) FROM workspace_schema`).Scan(&version); err != nil {
		return "", workspace.WrapError(workspace.ErrStorageUnavailable, "read schema version", err)
	}
	if version != len(migrations) {
		return "", workspace.NewError(workspace.ErrUnsupportedVersion, "workspace schema is not current")
	}
	return schemaVersion, nil
}

func (s *Store) RegisterProject(ctx context.Context, project workspace.Project) (workspace.Project, error) {
	if err := validateProjectContext(ctx); err != nil {
		return workspace.Project{}, err
	}
	normalized, err := normalizeProject(project)
	if err != nil {
		return workspace.Project{}, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return workspace.Project{}, workspace.WrapError(workspace.ErrStorageUnavailable, "begin project registration", err)
	}
	defer func() { _ = tx.Rollback() }()
	_, err = tx.ExecContext(ctx, `INSERT INTO projects(
		id, name, repository_root, vault_root, default_branch, status,
		created_at, updated_at, schema_version
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		normalized.ID, normalized.Name, normalized.RepositoryRoot, normalized.VaultRoot,
		normalized.DefaultBranch, normalized.Status, normalized.CreatedAt.Format(time.RFC3339Nano),
		normalized.UpdatedAt.Format(time.RFC3339Nano), normalized.SchemaVersion)
	if err != nil {
		return workspace.Project{}, workspace.WrapError(workspace.ErrConflict, "project already exists or violates storage constraints", err)
	}
	if err := tx.Commit(); err != nil {
		return workspace.Project{}, workspace.WrapError(workspace.ErrStorageUnavailable, "commit project registration", err)
	}
	return normalized, nil
}

func (s *Store) UpdateProject(ctx context.Context, project workspace.Project) (workspace.Project, error) {
	if err := validateProjectContext(ctx); err != nil {
		return workspace.Project{}, err
	}
	normalized, err := normalizeProject(project)
	if err != nil {
		return workspace.Project{}, err
	}
	result, err := s.db.ExecContext(ctx, `UPDATE projects SET
		name = ?, repository_root = ?, vault_root = ?, default_branch = ?,
		status = ?, updated_at = ?, schema_version = ? WHERE id = ?`,
		normalized.Name, normalized.RepositoryRoot, normalized.VaultRoot, normalized.DefaultBranch,
		normalized.Status, normalized.UpdatedAt.Format(time.RFC3339Nano), normalized.SchemaVersion, normalized.ID)
	if err != nil {
		return workspace.Project{}, workspace.WrapError(workspace.ErrStorageUnavailable, "update project", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return workspace.Project{}, workspace.WrapError(workspace.ErrStorageUnavailable, "inspect project update", err)
	}
	if rows != 1 {
		return workspace.Project{}, workspace.NewError(workspace.ErrNotFound, "project does not exist")
	}
	return normalized, nil
}

func (s *Store) ArchiveProject(ctx context.Context, id workspace.ProjectID) error {
	if err := validateProjectContext(ctx); err != nil {
		return err
	}
	if id == "" {
		return workspace.NewError(workspace.ErrInvalidRequest, "project id is required")
	}
	result, err := s.db.ExecContext(ctx, `UPDATE projects SET status = ?, updated_at = ? WHERE id = ?`,
		workspace.ProjectStatusArchived, time.Now().UTC().Format(time.RFC3339Nano), id)
	if err != nil {
		return workspace.WrapError(workspace.ErrStorageUnavailable, "archive project", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return workspace.WrapError(workspace.ErrStorageUnavailable, "inspect project archive", err)
	}
	if rows != 1 {
		return workspace.NewError(workspace.ErrNotFound, "project does not exist")
	}
	return nil
}

func (s *Store) GetProject(ctx context.Context, id workspace.ProjectID) (workspace.Project, error) {
	if err := validateProjectContext(ctx); err != nil {
		return workspace.Project{}, err
	}
	if id == "" {
		return workspace.Project{}, workspace.NewError(workspace.ErrInvalidRequest, "project id is required")
	}
	row := s.db.QueryRowContext(ctx, `SELECT id, name, repository_root, vault_root, default_branch,
		status, created_at, updated_at, schema_version FROM projects WHERE id = ?`, id)
	project, err := scanProject(row)
	if err != nil {
		if err == sql.ErrNoRows {
			return workspace.Project{}, workspace.NewError(workspace.ErrNotFound, "project does not exist")
		}
		return workspace.Project{}, workspace.WrapError(workspace.ErrStorageUnavailable, "read project", err)
	}
	return project, nil
}

func (s *Store) ListProjects(ctx context.Context) ([]workspace.Project, error) {
	if err := validateProjectContext(ctx); err != nil {
		return nil, err
	}
	rows, err := s.db.QueryContext(ctx, `SELECT id, name, repository_root, vault_root, default_branch,
		status, created_at, updated_at, schema_version FROM projects ORDER BY id ASC`)
	if err != nil {
		return nil, workspace.WrapError(workspace.ErrStorageUnavailable, "list projects", err)
	}
	defer rows.Close()
	projects := make([]workspace.Project, 0)
	for rows.Next() {
		project, err := scanProject(rows)
		if err != nil {
			return nil, workspace.WrapError(workspace.ErrStorageUnavailable, "decode project", err)
		}
		projects = append(projects, project)
	}
	if err := rows.Err(); err != nil {
		return nil, workspace.WrapError(workspace.ErrStorageUnavailable, "iterate projects", err)
	}
	sort.Slice(projects, func(i, j int) bool { return projects[i].ID < projects[j].ID })
	return projects, nil
}

func validateProjectContext(ctx context.Context) error {
	if ctx == nil {
		return workspace.NewError(workspace.ErrInvalidRequest, "context is required")
	}
	if err := ctx.Err(); err != nil {
		return workspace.CanceledError(err)
	}
	if s := ctx.Value(storeContextKey{}); s != nil {
		return nil
	}
	return nil
}

type projectScanner interface {
	Scan(...any) error
}

func scanProject(scanner projectScanner) (workspace.Project, error) {
	var project workspace.Project
	var createdAt, updatedAt string
	if err := scanner.Scan(&project.ID, &project.Name, &project.RepositoryRoot, &project.VaultRoot,
		&project.DefaultBranch, &project.Status, &createdAt, &updatedAt, &project.SchemaVersion); err != nil {
		return workspace.Project{}, err
	}
	project.CreatedAt, _ = time.Parse(time.RFC3339Nano, createdAt)
	project.UpdatedAt, _ = time.Parse(time.RFC3339Nano, updatedAt)
	if project.CreatedAt.IsZero() || project.UpdatedAt.IsZero() {
		return workspace.Project{}, fmt.Errorf("project has invalid timestamps")
	}
	return project, nil
}

func normalizeProject(project workspace.Project) (workspace.Project, error) {
	if project.ID == "" || project.Name == "" {
		return workspace.Project{}, workspace.NewError(workspace.ErrInvalidRequest, "project id and name are required")
	}
	repositoryRoot, err := workspace.CanonicalRoot(project.RepositoryRoot)
	if err != nil {
		return workspace.Project{}, err
	}
	vaultRoot, err := workspace.CanonicalRoot(project.VaultRoot)
	if err != nil {
		return workspace.Project{}, err
	}
	if project.SchemaVersion != "" && project.SchemaVersion != workspace.ContractVersion {
		return workspace.Project{}, workspace.NewError(workspace.ErrUnsupportedVersion, "project schema version is unsupported")
	}
	now := time.Now().UTC()
	if project.CreatedAt.IsZero() {
		project.CreatedAt = now
	}
	if project.UpdatedAt.IsZero() {
		project.UpdatedAt = now
	}
	project.RepositoryRoot = repositoryRoot
	project.VaultRoot = vaultRoot
	project.SchemaVersion = workspace.ContractVersion
	if project.Status == "" {
		project.Status = workspace.ProjectStatusActive
	}
	return project, nil
}

type storeContextKey struct{}
