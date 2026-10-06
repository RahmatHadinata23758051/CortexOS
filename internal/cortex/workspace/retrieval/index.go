package retrieval

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"unicode"
	"unicode/utf8"

	"github.com/RahmatHadinata23758051/CortexOS/internal/cortex/workspace"
)

const (
	IndexVersion = "cortexos.retrieval.v1"
	maxLimit     = 100
)

var _ workspace.RetrievalIndex = (*Index)(nil)

type Index struct {
	root   string
	source workspace.VaultStore
	mu     sync.RWMutex
	data   persisted
}

type persisted struct {
	Version   string                        `json:"version"`
	Documents []workspace.RetrievalDocument `json:"documents"`
}

func New(root string) (*Index, error) {
	return NewWithSource(root, nil)
}

func NewWithSource(root string, source workspace.VaultStore) (*Index, error) {
	canonical, err := workspace.CanonicalRoot(root)
	if err != nil {
		return nil, err
	}
	return &Index{root: canonical, source: source, data: persisted{Version: IndexVersion, Documents: []workspace.RetrievalDocument{}}}, nil
}

func (i *Index) Path() string {
	if i == nil {
		return ""
	}
	return filepath.Join(i.root, "retrieval-index.json")
}

func (i *Index) Upsert(ctx context.Context, document workspace.RetrievalDocument) error {
	if err := checkContext(ctx); err != nil {
		return err
	}
	if err := validateDocument(document); err != nil {
		return err
	}
	i.mu.Lock()
	defer i.mu.Unlock()
	if err := i.loadLocked(); err != nil {
		return err
	}
	updated := make([]workspace.RetrievalDocument, 0, len(i.data.Documents)+1)
	for _, item := range i.data.Documents {
		if item.ID != document.ID {
			updated = append(updated, item)
		}
	}
	updated = append(updated, document)
	sortDocuments(updated)
	i.data.Documents = updated
	return i.persistLocked(ctx)
}

func (i *Index) Remove(ctx context.Context, id string) error {
	if err := checkContext(ctx); err != nil {
		return err
	}
	if strings.TrimSpace(id) == "" {
		return workspace.NewError(workspace.ErrInvalidRequest, "retrieval document id is required")
	}
	i.mu.Lock()
	defer i.mu.Unlock()
	if err := i.loadLocked(); err != nil {
		return err
	}
	updated := make([]workspace.RetrievalDocument, 0, len(i.data.Documents))
	found := false
	for _, item := range i.data.Documents {
		if item.ID == id {
			found = true
			continue
		}
		updated = append(updated, item)
	}
	if !found {
		return workspace.NewError(workspace.ErrNotFound, "retrieval document does not exist")
	}
	i.data.Documents = updated
	return i.persistLocked(ctx)
}

func (i *Index) Query(ctx context.Context, projectID workspace.ProjectID, query string, limit int) ([]workspace.RetrievalDocument, error) {
	if err := checkContext(ctx); err != nil {
		return nil, err
	}
	if projectID == "" || strings.TrimSpace(query) == "" {
		return nil, workspace.NewError(workspace.ErrInvalidRequest, "project id and query are required")
	}
	if limit < 1 {
		return nil, workspace.NewError(workspace.ErrInvalidRequest, "query limit must be positive")
	}
	if limit > maxLimit {
		limit = maxLimit
	}
	tokens := tokenize(query)
	if len(tokens) == 0 {
		return nil, workspace.NewError(workspace.ErrInvalidRequest, "query has no searchable tokens")
	}
	i.mu.Lock()
	defer i.mu.Unlock()
	if err := i.loadLocked(); err != nil {
		return nil, err
	}
	type scored struct {
		doc   workspace.RetrievalDocument
		score int
	}
	matches := make([]scored, 0)
	for _, document := range i.data.Documents {
		if document.ProjectID != projectID {
			continue
		}
		contentTokens := tokenSet(document.Content)
		score := 0
		for _, token := range tokens {
			if contentTokens[token] {
				score++
			}
		}
		if score > 0 {
			matches = append(matches, scored{doc: document, score: score})
		}
	}
	sort.Slice(matches, func(left, right int) bool {
		if matches[left].score != matches[right].score {
			return matches[left].score > matches[right].score
		}
		return lessDocument(matches[left].doc, matches[right].doc)
	})
	result := make([]workspace.RetrievalDocument, 0, min(limit, len(matches)))
	for _, match := range matches[:min(limit, len(matches))] {
		result = append(result, match.doc)
	}
	return result, nil
}

func (i *Index) Rebuild(ctx context.Context, projectID workspace.ProjectID) error {
	if err := checkContext(ctx); err != nil {
		return err
	}
	if projectID == "" {
		return workspace.NewError(workspace.ErrInvalidRequest, "project id is required")
	}
	if i.source == nil {
		return workspace.NewError(workspace.ErrInvalidRequest, "retrieval source is required for rebuild")
	}
	notes, err := i.source.ListNotes(ctx, projectID)
	if err != nil {
		return workspace.CanceledError(err)
	}
	documents := make([]workspace.RetrievalDocument, 0, len(notes))
	for _, note := range notes {
		if err := checkContext(ctx); err != nil {
			return err
		}
		documents = append(documents, DocumentFromNote(note))
	}
	return i.replace(ctx, projectID, documents)
}

func (i *Index) RebuildFrom(ctx context.Context, projectID workspace.ProjectID, documents []workspace.RetrievalDocument) error {
	if err := checkContext(ctx); err != nil {
		return err
	}
	filtered := make([]workspace.RetrievalDocument, 0, len(documents))
	for _, document := range documents {
		if document.ProjectID != projectID {
			return workspace.NewError(workspace.ErrConflict, "retrieval rebuild document belongs to another project")
		}
		if err := validateDocument(document); err != nil {
			return err
		}
		filtered = append(filtered, document)
	}
	return i.replace(ctx, projectID, filtered)
}

func (i *Index) replace(ctx context.Context, projectID workspace.ProjectID, documents []workspace.RetrievalDocument) error {
	i.mu.Lock()
	defer i.mu.Unlock()
	if documents == nil {
		if err := i.loadLocked(); err != nil {
			return err
		}
		kept := make([]workspace.RetrievalDocument, 0, len(i.data.Documents))
		for _, document := range i.data.Documents {
			if document.ProjectID != projectID {
				kept = append(kept, document)
			}
		}
		documents = kept
	}
	if err := checkContext(ctx); err != nil {
		return err
	}
	sortDocuments(documents)
	i.data = persisted{Version: IndexVersion, Documents: documents}
	return i.persistLocked(ctx)
}

func (i *Index) loadLocked() error {
	data, err := os.ReadFile(i.Path())
	if errors.Is(err, os.ErrNotExist) {
		i.data = persisted{Version: IndexVersion, Documents: []workspace.RetrievalDocument{}}
		return nil
	}
	if err != nil {
		return workspace.WrapError(workspace.ErrStorageUnavailable, "read retrieval index", err)
	}
	var decoded persisted
	if err := json.Unmarshal(data, &decoded); err != nil {
		return workspace.WrapError(workspace.ErrRetrievalCorrupt, "decode retrieval index", err)
	}
	if decoded.Version != IndexVersion {
		return workspace.NewError(workspace.ErrRetrievalCorrupt, "retrieval index version is unsupported")
	}
	for _, document := range decoded.Documents {
		if err := validateDocument(document); err != nil {
			return workspace.WrapError(workspace.ErrRetrievalCorrupt, "retrieval index document is invalid", err)
		}
	}
	sortDocuments(decoded.Documents)
	i.data = decoded
	return nil
}

func (i *Index) persistLocked(ctx context.Context) error {
	if err := checkContext(ctx); err != nil {
		return err
	}
	if err := os.MkdirAll(i.root, 0o700); err != nil {
		return workspace.WrapError(workspace.ErrStorageUnavailable, "create retrieval index root", err)
	}
	data, err := json.MarshalIndent(i.data, "", "  ")
	if err != nil {
		return workspace.WrapError(workspace.ErrRetrievalCorrupt, "encode retrieval index", err)
	}
	temporary, err := os.CreateTemp(i.root, ".retrieval-index-*")
	if err != nil {
		return workspace.WrapError(workspace.ErrStorageUnavailable, "create retrieval index temporary file", err)
	}
	name := temporary.Name()
	defer os.Remove(name)
	if err := temporary.Chmod(0o600); err != nil {
		_ = temporary.Close()
		return workspace.WrapError(workspace.ErrStorageUnavailable, "secure retrieval index temporary file", err)
	}
	if _, err := temporary.Write(data); err != nil {
		_ = temporary.Close()
		return workspace.WrapError(workspace.ErrStorageUnavailable, "write retrieval index temporary file", err)
	}
	if err := temporary.Sync(); err != nil {
		_ = temporary.Close()
		return workspace.WrapError(workspace.ErrStorageUnavailable, "flush retrieval index temporary file", err)
	}
	if err := temporary.Close(); err != nil {
		return workspace.WrapError(workspace.ErrStorageUnavailable, "close retrieval index temporary file", err)
	}
	if err := ctx.Err(); err != nil {
		return workspace.CanceledError(err)
	}
	if err := os.Rename(name, i.Path()); err != nil {
		return workspace.WrapError(workspace.ErrStorageUnavailable, "replace retrieval index", err)
	}
	return nil
}

func validateDocument(document workspace.RetrievalDocument) error {
	if document.ID == "" || document.NoteID == "" || document.ProjectID == "" || document.RelativePath == "" || document.SourceHash == "" || document.IndexVersion != IndexVersion {
		return workspace.NewError(workspace.ErrInvalidRequest, "retrieval document identity, source, path, and version are required")
	}
	if !utf8.ValidString(document.Content) || !utf8.ValidString(document.Attribution) {
		return workspace.NewError(workspace.ErrInvalidRequest, "retrieval document contains invalid UTF-8")
	}
	if document.ID != DocumentID(document.ProjectID, document.NoteID, document.RelativePath) {
		return workspace.NewError(workspace.ErrInvalidRequest, "retrieval document id is not canonical")
	}
	return nil
}

func DocumentFromNote(note workspace.VaultNote) workspace.RetrievalDocument {
	return workspace.RetrievalDocument{
		ID:           DocumentID(note.ProjectID, note.ID, note.RelativePath),
		NoteID:       note.ID,
		ProjectID:    note.ProjectID,
		WorktreeID:   note.WorktreeID,
		RelativePath: note.RelativePath,
		SourceHash:   note.ContentHash,
		IndexVersion: IndexVersion,
		Content:      note.Body,
		Attribution:  note.Source + "/" + note.Author,
		IndexedAt:    note.UpdatedAt.UTC(),
	}
}

func DocumentID(projectID workspace.ProjectID, noteID workspace.NoteID, relativePath string) string {
	hash := sha256.Sum256([]byte(IndexVersion + "\x00" + string(projectID) + "\x00" + string(noteID) + "\x00" + relativePath))
	return "sha256:" + hex.EncodeToString(hash[:])
}

func tokenize(value string) []string {
	if !utf8.ValidString(value) {
		return nil
	}
	result := make([]string, 0)
	var builder strings.Builder
	flush := func() {
		if builder.Len() > 0 {
			result = append(result, builder.String())
			builder.Reset()
		}
	}
	for _, character := range strings.ToLower(value) {
		if unicode.IsLetter(character) || unicode.IsNumber(character) {
			builder.WriteRune(character)
			continue
		}
		flush()
	}
	flush()
	return result
}

func tokenSet(value string) map[string]bool {
	set := make(map[string]bool)
	for _, token := range tokenize(value) {
		set[token] = true
	}
	return set
}

func sortDocuments(documents []workspace.RetrievalDocument) {
	sort.Slice(documents, func(left, right int) bool { return lessDocument(documents[left], documents[right]) })
}

func lessDocument(left, right workspace.RetrievalDocument) bool {
	if left.ID != right.ID {
		return left.ID < right.ID
	}
	if left.ProjectID != right.ProjectID {
		return left.ProjectID < right.ProjectID
	}
	if left.NoteID != right.NoteID {
		return left.NoteID < right.NoteID
	}
	return left.RelativePath < right.RelativePath
}

func min(left, right int) int {
	if left < right {
		return left
	}
	return right
}

func checkContext(ctx context.Context) error {
	if ctx == nil {
		return workspace.NewError(workspace.ErrInvalidRequest, "context is required")
	}
	if err := ctx.Err(); err != nil {
		return workspace.CanceledError(err)
	}
	return nil
}
