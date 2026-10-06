package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"sort"
	"time"

	"github.com/RahmatHadinata23758051/CortexOS/internal/cortex/workspace"
)

// NoteMetadataStore persists Vault metadata and hashes without storing the
// Markdown body. The filesystem Vault remains the human-readable content
// authority; this table is synchronization metadata only.
type NoteMetadataStore interface {
	UpsertNoteMetadata(context.Context, workspace.VaultNote) error
	GetNoteMetadata(context.Context, workspace.NoteID) (workspace.VaultNote, error)
	ListNoteMetadata(context.Context, workspace.ProjectID) ([]workspace.VaultNote, error)
	DeleteNoteMetadata(context.Context, workspace.NoteID) error
}

var _ NoteMetadataStore = (*Store)(nil)

func (s *Store) UpsertNoteMetadata(ctx context.Context, note workspace.VaultNote) error {
	if err := validateProjectContext(ctx); err != nil {
		return err
	}
	if note.ID == "" || note.ProjectID == "" || note.RelativePath == "" || note.ContentHash == "" {
		return workspace.NewError(workspace.ErrInvalidRequest, "note identity, project, path, and content hash are required")
	}
	if note.FormatVersion == "" {
		note.FormatVersion = "cortexos.vault.v1"
	}
	if note.Status == "" {
		note.Status = workspace.NoteStatusActive
	}
	if note.CreatedAt.IsZero() {
		note.CreatedAt = time.Now().UTC()
	}
	if note.UpdatedAt.IsZero() {
		note.UpdatedAt = note.CreatedAt
	}
	_, err := s.db.ExecContext(ctx, `INSERT INTO vault_notes(
		id, project_id, worktree_id, relative_path, title, format_version, source, author,
		created_at, updated_at, content_hash, status
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	ON CONFLICT(id) DO UPDATE SET
		project_id = excluded.project_id,
		worktree_id = excluded.worktree_id,
		relative_path = excluded.relative_path,
		title = excluded.title,
		format_version = excluded.format_version,
		source = excluded.source,
		author = excluded.author,
		created_at = excluded.created_at,
		updated_at = excluded.updated_at,
		content_hash = excluded.content_hash,
		status = excluded.status`,
		note.ID, note.ProjectID, note.WorktreeID, note.RelativePath, note.Title, note.FormatVersion,
		note.Source, note.Author, note.CreatedAt.UTC().Format(time.RFC3339Nano),
		note.UpdatedAt.UTC().Format(time.RFC3339Nano), note.ContentHash, note.Status)
	if err != nil {
		return workspace.WrapError(workspace.ErrConflict, "upsert Vault note metadata", err)
	}
	return nil
}

func (s *Store) GetNoteMetadata(ctx context.Context, id workspace.NoteID) (workspace.VaultNote, error) {
	if err := validateProjectContext(ctx); err != nil {
		return workspace.VaultNote{}, err
	}
	if id == "" {
		return workspace.VaultNote{}, workspace.NewError(workspace.ErrInvalidRequest, "note id is required")
	}
	row := s.db.QueryRowContext(ctx, `SELECT id, project_id, worktree_id, relative_path, title, format_version,
		source, author, created_at, updated_at, content_hash, status FROM vault_notes WHERE id = ?`, id)
	return scanNoteMetadata(row)
}

func (s *Store) ListNoteMetadata(ctx context.Context, projectID workspace.ProjectID) ([]workspace.VaultNote, error) {
	if err := validateProjectContext(ctx); err != nil {
		return nil, err
	}
	if projectID == "" {
		return nil, workspace.NewError(workspace.ErrInvalidRequest, "project id is required")
	}
	rows, err := s.db.QueryContext(ctx, `SELECT id, project_id, worktree_id, relative_path, title, format_version,
		source, author, created_at, updated_at, content_hash, status FROM vault_notes WHERE project_id = ? ORDER BY relative_path, id`, projectID)
	if err != nil {
		return nil, workspace.WrapError(workspace.ErrStorageUnavailable, "list Vault note metadata", err)
	}
	defer rows.Close()
	result := make([]workspace.VaultNote, 0)
	for rows.Next() {
		note, err := scanNoteMetadata(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, note)
	}
	if err := rows.Err(); err != nil {
		return nil, workspace.WrapError(workspace.ErrStorageUnavailable, "iterate Vault note metadata", err)
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].RelativePath == result[j].RelativePath {
			return result[i].ID < result[j].ID
		}
		return result[i].RelativePath < result[j].RelativePath
	})
	return result, nil
}

func (s *Store) DeleteNoteMetadata(ctx context.Context, id workspace.NoteID) error {
	if err := validateProjectContext(ctx); err != nil {
		return err
	}
	if id == "" {
		return workspace.NewError(workspace.ErrInvalidRequest, "note id is required")
	}
	result, err := s.db.ExecContext(ctx, `DELETE FROM vault_notes WHERE id = ?`, id)
	if err != nil {
		return workspace.WrapError(workspace.ErrStorageUnavailable, "delete Vault note metadata", err)
	}
	count, err := result.RowsAffected()
	if err != nil {
		return workspace.WrapError(workspace.ErrStorageUnavailable, "inspect Vault note metadata deletion", err)
	}
	if count != 1 {
		return workspace.NewError(workspace.ErrNotFound, "Vault note metadata does not exist")
	}
	return nil
}

type noteScanner interface {
	Scan(...any) error
}

func scanNoteMetadata(scanner noteScanner) (workspace.VaultNote, error) {
	var note workspace.VaultNote
	var createdAt, updatedAt string
	if err := scanner.Scan(&note.ID, &note.ProjectID, &note.WorktreeID, &note.RelativePath, &note.Title,
		&note.FormatVersion, &note.Source, &note.Author, &createdAt, &updatedAt, &note.ContentHash, &note.Status); err != nil {
		if err == sql.ErrNoRows {
			return workspace.VaultNote{}, workspace.NewError(workspace.ErrNotFound, "Vault note metadata does not exist")
		}
		return workspace.VaultNote{}, workspace.WrapError(workspace.ErrStorageUnavailable, "read Vault note metadata", err)
	}
	var err error
	note.CreatedAt, err = time.Parse(time.RFC3339Nano, createdAt)
	if err != nil {
		return workspace.VaultNote{}, workspace.WrapError(workspace.ErrStorageUnavailable, "decode Vault note created_at", err)
	}
	note.UpdatedAt, err = time.Parse(time.RFC3339Nano, updatedAt)
	if err != nil {
		return workspace.VaultNote{}, workspace.WrapError(workspace.ErrStorageUnavailable, "decode Vault note updated_at", err)
	}
	return note, nil
}

func (s *Store) String() string {
	if s == nil {
		return "<nil>"
	}
	return fmt.Sprintf("workspace sqlite store: %s", s.path)
}
