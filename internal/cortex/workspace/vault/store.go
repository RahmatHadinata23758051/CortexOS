package vault

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/RahmatHadinata23758051/CortexOS/internal/cortex/workspace"
)

var _ workspace.VaultStore = (*Store)(nil)

type MetadataStore interface {
	UpsertNoteMetadata(context.Context, workspace.VaultNote) error
	DeleteNoteMetadata(context.Context, workspace.NoteID) error
}

type Store struct {
	root     string
	metadata MetadataStore
}

func New(root string) (*Store, error) {
	return NewWithMetadata(root, nil)
}

func NewWithMetadata(root string, metadata MetadataStore) (*Store, error) {
	canonicalRoot, err := workspace.CanonicalRoot(root)
	if err != nil {
		return nil, err
	}
	return &Store{root: canonicalRoot, metadata: metadata}, nil
}

func (s *Store) Root() string {
	if s == nil {
		return ""
	}
	return s.root
}

func (s *Store) GetNote(ctx context.Context, id workspace.NoteID) (workspace.VaultNote, error) {
	if err := requireContext(ctx); err != nil {
		return workspace.VaultNote{}, err
	}
	if id == "" {
		return workspace.VaultNote{}, workspace.NewError(workspace.ErrInvalidRequest, "note id is required")
	}
	files, err := s.noteFiles()
	if err != nil {
		return workspace.VaultNote{}, err
	}
	for _, path := range files {
		document, err := s.readDocument(path)
		if err != nil {
			return workspace.VaultNote{}, err
		}
		if document.Note.ID == id {
			return document.Note, nil
		}
	}
	return workspace.VaultNote{}, workspace.NewError(workspace.ErrNotFound, "Vault note does not exist")
}

func (s *Store) ListNotes(ctx context.Context, projectID workspace.ProjectID) ([]workspace.VaultNote, error) {
	if err := requireContext(ctx); err != nil {
		return nil, err
	}
	if projectID == "" {
		return nil, workspace.NewError(workspace.ErrInvalidRequest, "project id is required")
	}
	files, err := s.noteFiles()
	if err != nil {
		return nil, err
	}
	notes := make([]workspace.VaultNote, 0)
	for _, path := range files {
		document, err := s.readDocument(path)
		if err != nil {
			return nil, err
		}
		if document.Note.ProjectID == projectID {
			notes = append(notes, document.Note)
		}
	}
	sort.Slice(notes, func(i, j int) bool {
		if notes[i].RelativePath == notes[j].RelativePath {
			return notes[i].ID < notes[j].ID
		}
		return notes[i].RelativePath < notes[j].RelativePath
	})
	return notes, nil
}

func (s *Store) CreateNote(ctx context.Context, note workspace.VaultNote) (workspace.VaultNote, error) {
	if err := requireContext(ctx); err != nil {
		return workspace.VaultNote{}, err
	}
	if note.FormatVersion == "" {
		note.FormatVersion = FormatVersion
	}
	if note.CreatedAt.IsZero() {
		note.CreatedAt = time.Now().UTC()
	}
	if note.UpdatedAt.IsZero() {
		note.UpdatedAt = note.CreatedAt
	}
	if note.Status == "" {
		note.Status = workspace.NoteStatusActive
	}
	if _, err := Serialize(note); err != nil {
		return workspace.VaultNote{}, err
	}
	target, err := s.notePath(note.RelativePath)
	if err != nil {
		return workspace.VaultNote{}, err
	}
	if _, err := os.Stat(target); err == nil {
		return workspace.VaultNote{}, workspace.NewError(workspace.ErrConflict, "Vault note path already exists")
	} else if !errors.Is(err, os.ErrNotExist) {
		return workspace.VaultNote{}, workspace.WrapError(workspace.ErrStorageUnavailable, "inspect Vault note path", err)
	}
	if existing, lookupErr := s.GetNote(ctx, note.ID); lookupErr == nil {
		if existing.ID == note.ID {
			return workspace.VaultNote{}, workspace.NewError(workspace.ErrConflict, "Vault note id already exists")
		}
	} else if workspace.ErrorCodeOf(lookupErr) != workspace.ErrNotFound {
		return workspace.VaultNote{}, lookupErr
	}
	data, err := Serialize(note)
	if err != nil {
		return workspace.VaultNote{}, err
	}
	if err := atomicWrite(ctx, target, data); err != nil {
		return workspace.VaultNote{}, err
	}
	document, err := s.readDocument(target)
	if err != nil {
		return workspace.VaultNote{}, err
	}
	if s.metadata != nil {
		if err := s.metadata.UpsertNoteMetadata(ctx, document.Note); err != nil {
			return workspace.VaultNote{}, workspace.WrapError(workspace.ErrStorageUnavailable, "synchronize Vault note metadata", err)
		}
	}
	return document.Note, nil
}

func (s *Store) UpdateNote(ctx context.Context, note workspace.VaultNote, expectedHash string) (workspace.VaultNote, error) {
	if err := requireContext(ctx); err != nil {
		return workspace.VaultNote{}, err
	}
	target, err := s.notePath(note.RelativePath)
	if err != nil {
		return workspace.VaultNote{}, err
	}
	current, err := s.readDocument(target)
	if err != nil {
		return workspace.VaultNote{}, err
	}
	if current.Note.ID != note.ID {
		return workspace.VaultNote{}, workspace.NewError(workspace.ErrConflict, "Vault note identity does not match target")
	}
	if expectedHash != "" && current.Note.ContentHash != expectedHash {
		return workspace.VaultNote{}, workspace.NewError(workspace.ErrConflict, "Vault note was edited externally")
	}
	if note.FormatVersion == "" {
		note.FormatVersion = FormatVersion
	}
	if note.CreatedAt.IsZero() {
		note.CreatedAt = current.Note.CreatedAt
	}
	if note.UpdatedAt.IsZero() || !note.UpdatedAt.After(current.Note.UpdatedAt) {
		note.UpdatedAt = time.Now().UTC()
	}
	data, err := Serialize(note)
	if err != nil {
		return workspace.VaultNote{}, err
	}
	if err := atomicWrite(ctx, target, data); err != nil {
		return workspace.VaultNote{}, err
	}
	document, err := s.readDocument(target)
	if err != nil {
		return workspace.VaultNote{}, err
	}
	if s.metadata != nil {
		if err := s.metadata.UpsertNoteMetadata(ctx, document.Note); err != nil {
			return workspace.VaultNote{}, workspace.WrapError(workspace.ErrStorageUnavailable, "synchronize Vault note metadata", err)
		}
	}
	return document.Note, nil
}

func (s *Store) DeleteNote(ctx context.Context, id workspace.NoteID) error {
	if err := requireContext(ctx); err != nil {
		return err
	}
	note, err := s.GetNote(ctx, id)
	if err != nil {
		return err
	}
	target, err := s.notePath(note.RelativePath)
	if err != nil {
		return err
	}
	if err := os.Remove(target); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return workspace.NewError(workspace.ErrNotFound, "Vault note does not exist")
		}
		return workspace.WrapError(workspace.ErrStorageUnavailable, "delete Vault note", err)
	}
	if s.metadata != nil {
		if err := s.metadata.DeleteNoteMetadata(ctx, id); err != nil {
			return workspace.WrapError(workspace.ErrStorageUnavailable, "delete Vault note metadata", err)
		}
	}
	return nil
}

func (s *Store) notePath(relativePath string) (string, error) {
	if err := os.MkdirAll(s.root, 0o700); err != nil {
		return "", workspace.WrapError(workspace.ErrStorageUnavailable, "create Vault root", err)
	}
	canonicalRelative, err := validateRelativeNotePath(relativePath)
	if err != nil {
		return "", err
	}
	path, err := workspace.ContainedPath(s.root, canonicalRelative)
	if err != nil {
		return "", err
	}
	if err := verifyExistingContainment(s.root, path); err != nil {
		return "", err
	}
	return path, nil
}

func (s *Store) noteFiles() ([]string, error) {
	if err := os.MkdirAll(s.root, 0o700); err != nil {
		return nil, workspace.WrapError(workspace.ErrStorageUnavailable, "create Vault root", err)
	}
	files := make([]string, 0)
	err := filepath.WalkDir(s.root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			if path != s.root {
				if containmentErr := verifyExistingContainment(s.root, path); containmentErr != nil {
					return containmentErr
				}
			}
			return nil
		}
		if !strings.EqualFold(filepath.Ext(entry.Name()), ".md") {
			return nil
		}
		if containmentErr := verifyExistingContainment(s.root, path); containmentErr != nil {
			return containmentErr
		}
		files = append(files, path)
		return nil
	})
	if err != nil {
		return nil, workspace.WrapError(workspace.ErrStorageUnavailable, "list Vault notes", err)
	}
	sort.Strings(files)
	return files, nil
}

func (s *Store) readDocument(path string) (Document, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Document{}, workspace.WrapError(workspace.ErrStorageUnavailable, "read Vault note", err)
	}
	document, err := Parse(data)
	if err != nil {
		return Document{}, err
	}
	return document, nil
}

func atomicWrite(ctx context.Context, target string, data []byte) error {
	if err := requireContext(ctx); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
		return workspace.WrapError(workspace.ErrStorageUnavailable, "create Vault note directory", err)
	}
	temporary, err := os.CreateTemp(filepath.Dir(target), ".cortexos-note-*")
	if err != nil {
		return workspace.WrapError(workspace.ErrStorageUnavailable, "create Vault note temporary file", err)
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	if err := temporary.Chmod(0o600); err != nil {
		_ = temporary.Close()
		return workspace.WrapError(workspace.ErrStorageUnavailable, "secure Vault note temporary file", err)
	}
	if _, err := temporary.Write(data); err != nil {
		_ = temporary.Close()
		return workspace.WrapError(workspace.ErrStorageUnavailable, "write Vault note temporary file", err)
	}
	if err := temporary.Sync(); err != nil {
		_ = temporary.Close()
		return workspace.WrapError(workspace.ErrStorageUnavailable, "flush Vault note temporary file", err)
	}
	if err := temporary.Close(); err != nil {
		return workspace.WrapError(workspace.ErrStorageUnavailable, "close Vault note temporary file", err)
	}
	if err := ctx.Err(); err != nil {
		return workspace.CanceledError(err)
	}
	if err := os.Rename(temporaryPath, target); err != nil {
		return workspace.WrapError(workspace.ErrStorageUnavailable, "replace Vault note atomically", err)
	}
	return nil
}

func requireContext(ctx context.Context) error {
	if ctx == nil {
		return workspace.NewError(workspace.ErrInvalidRequest, "context is required")
	}
	if err := ctx.Err(); err != nil {
		return workspace.CanceledError(err)
	}
	return nil
}

func verifyExistingContainment(root, target string) error {
	resolvedRoot, err := filepath.EvalSymlinks(root)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return workspace.WrapError(workspace.ErrPathDenied, "Vault root link cannot be resolved", err)
	}
	if err != nil {
		resolvedRoot = filepath.Clean(root)
	}
	ancestor := target
	for {
		resolved, resolveErr := filepath.EvalSymlinks(ancestor)
		if resolveErr == nil {
			relative, relativeErr := filepath.Rel(resolvedRoot, resolved)
			if relativeErr != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) || filepath.IsAbs(relative) {
				return workspace.NewError(workspace.ErrPathDenied, "Vault note path resolves outside the approved root")
			}
			return nil
		}
		if !errors.Is(resolveErr, os.ErrNotExist) {
			return workspace.WrapError(workspace.ErrPathDenied, "Vault note link cannot be resolved", resolveErr)
		}
		parent := filepath.Dir(ancestor)
		if parent == ancestor {
			return nil
		}
		ancestor = parent
	}
}
