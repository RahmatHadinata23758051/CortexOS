package workspace

import (
	"context"
	"sort"
	"strings"
	"sync"
)

// MemoryWorkspace is a deterministic reference adapter for contract tests. It
// is not a production persistence fallback and its state is lost on close.
type MemoryWorkspace struct {
	mu        sync.RWMutex
	projects  map[ProjectID]Project
	worktrees map[WorktreeID]Worktree
	notes     map[NoteID]VaultNote
	documents map[string]RetrievalDocument
	changes   chan FileChange
	open      bool
	watching  bool
}

func NewMemoryWorkspace() *MemoryWorkspace {
	return &MemoryWorkspace{
		projects:  make(map[ProjectID]Project),
		worktrees: make(map[WorktreeID]Worktree),
		notes:     make(map[NoteID]VaultNote),
		documents: make(map[string]RetrievalDocument),
		changes:   make(chan FileChange, 16),
	}
}

func (m *MemoryWorkspace) Open(ctx context.Context) error {
	if err := checkContext(ctx); err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.open = true
	return nil
}

func (m *MemoryWorkspace) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.open = false
	m.watching = false
	return nil
}

func (m *MemoryWorkspace) Start(ctx context.Context, root string) (<-chan FileChange, error) {
	if err := checkContext(ctx); err != nil {
		return nil, err
	}
	if _, err := CanonicalRoot(root); err != nil {
		return nil, err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.watching {
		return nil, NewError(ErrConflict, "memory watcher is already running")
	}
	m.watching = true
	return m.changes, nil
}

func (m *MemoryWorkspace) Stop() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.watching = false
	return nil
}

// PublishChange injects a synthetic event for contract tests. Production
// watchers must produce events only from approved filesystem observations.
func (m *MemoryWorkspace) PublishChange(ctx context.Context, change FileChange) error {
	if err := checkContext(ctx); err != nil {
		return err
	}
	m.mu.RLock()
	watching := m.watching
	changes := m.changes
	m.mu.RUnlock()
	if !watching {
		return NewError(ErrStorageUnavailable, "memory watcher is not running")
	}
	select {
	case changes <- change:
		return nil
	default:
		return NewError(ErrWatcherOverflow, "memory watcher event buffer is full")
	}
}

func (m *MemoryWorkspace) SchemaVersion(ctx context.Context) (string, error) {
	if err := checkContext(ctx); err != nil {
		return "", err
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	if !m.open {
		return "", NewError(ErrStorageUnavailable, "memory workspace is not open")
	}
	return ContractVersion, nil
}

func (m *MemoryWorkspace) RegisterProject(ctx context.Context, project Project) (Project, error) {
	if err := checkContext(ctx); err != nil {
		return Project{}, err
	}
	if project.ID == "" || project.Name == "" {
		return Project{}, NewError(ErrInvalidRequest, "project id and name are required")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, exists := m.projects[project.ID]; exists {
		return Project{}, NewError(ErrConflict, "project already exists")
	}
	if project.SchemaVersion == "" {
		project.SchemaVersion = ContractVersion
	}
	if project.Status == "" {
		project.Status = ProjectStatusActive
	}
	m.projects[project.ID] = project
	return project, nil
}

func (m *MemoryWorkspace) UpdateProject(ctx context.Context, project Project) (Project, error) {
	if err := checkContext(ctx); err != nil {
		return Project{}, err
	}
	if project.ID == "" || project.Name == "" {
		return Project{}, NewError(ErrInvalidRequest, "project id and name are required")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, exists := m.projects[project.ID]; !exists {
		return Project{}, NewError(ErrNotFound, "project does not exist")
	}
	m.projects[project.ID] = project
	return project, nil
}

func (m *MemoryWorkspace) ArchiveProject(ctx context.Context, id ProjectID) error {
	if err := checkContext(ctx); err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	project, exists := m.projects[id]
	if !exists {
		return NewError(ErrNotFound, "project does not exist")
	}
	project.Status = ProjectStatusArchived
	m.projects[id] = project
	return nil
}

func (m *MemoryWorkspace) GetProject(ctx context.Context, id ProjectID) (Project, error) {
	if err := checkContext(ctx); err != nil {
		return Project{}, err
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	project, exists := m.projects[id]
	if !exists {
		return Project{}, NewError(ErrNotFound, "project does not exist")
	}
	return project, nil
}

func (m *MemoryWorkspace) ListProjects(ctx context.Context) ([]Project, error) {
	if err := checkContext(ctx); err != nil {
		return nil, err
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	projects := make([]Project, 0, len(m.projects))
	for _, project := range m.projects {
		projects = append(projects, project)
	}
	sort.Slice(projects, func(i, j int) bool { return projects[i].ID < projects[j].ID })
	return projects, nil
}

func (m *MemoryWorkspace) InspectWorktree(ctx context.Context, id WorktreeID) (Worktree, error) {
	if err := checkContext(ctx); err != nil {
		return Worktree{}, err
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	worktree, exists := m.worktrees[id]
	if !exists {
		return Worktree{}, NewError(ErrNotFound, "worktree does not exist")
	}
	return worktree, nil
}

func (m *MemoryWorkspace) CreateWorktree(ctx context.Context, worktree Worktree) (Worktree, error) {
	if err := checkContext(ctx); err != nil {
		return Worktree{}, err
	}
	if worktree.ID == "" || worktree.ProjectID == "" || worktree.Path == "" {
		return Worktree{}, NewError(ErrInvalidRequest, "worktree id, project id, and path are required")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, exists := m.projects[worktree.ProjectID]; !exists {
		return Worktree{}, NewError(ErrNotFound, "project does not exist")
	}
	if _, exists := m.worktrees[worktree.ID]; exists {
		return Worktree{}, NewError(ErrConflict, "worktree already exists")
	}
	if worktree.Status == "" {
		worktree.Status = WorktreeStatusActive
	}
	m.worktrees[worktree.ID] = worktree
	return worktree, nil
}

func (m *MemoryWorkspace) ListWorktrees(ctx context.Context, projectID ProjectID) ([]Worktree, error) {
	if err := checkContext(ctx); err != nil {
		return nil, err
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	worktrees := make([]Worktree, 0)
	for _, worktree := range m.worktrees {
		if worktree.ProjectID == projectID {
			worktrees = append(worktrees, worktree)
		}
	}
	sort.Slice(worktrees, func(i, j int) bool { return worktrees[i].ID < worktrees[j].ID })
	return worktrees, nil
}

func (m *MemoryWorkspace) RemoveWorktree(ctx context.Context, id WorktreeID) error {
	if err := checkContext(ctx); err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	worktree, exists := m.worktrees[id]
	if !exists {
		return NewError(ErrNotFound, "worktree does not exist")
	}
	worktree.Status = WorktreeStatusRemoved
	m.worktrees[id] = worktree
	return nil
}

func (m *MemoryWorkspace) GetNote(ctx context.Context, id NoteID) (VaultNote, error) {
	if err := checkContext(ctx); err != nil {
		return VaultNote{}, err
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	note, exists := m.notes[id]
	if !exists {
		return VaultNote{}, NewError(ErrNotFound, "note does not exist")
	}
	return note, nil
}

func (m *MemoryWorkspace) ListNotes(ctx context.Context, projectID ProjectID) ([]VaultNote, error) {
	if err := checkContext(ctx); err != nil {
		return nil, err
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	notes := make([]VaultNote, 0)
	for _, note := range m.notes {
		if note.ProjectID == projectID {
			notes = append(notes, note)
		}
	}
	sort.Slice(notes, func(i, j int) bool { return notes[i].ID < notes[j].ID })
	return notes, nil
}

func (m *MemoryWorkspace) CreateNote(ctx context.Context, note VaultNote) (VaultNote, error) {
	if err := checkContext(ctx); err != nil {
		return VaultNote{}, err
	}
	if note.ID == "" || note.ProjectID == "" || note.RelativePath == "" {
		return VaultNote{}, NewError(ErrInvalidRequest, "note id, project id, and relative path are required")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, exists := m.projects[note.ProjectID]; !exists {
		return VaultNote{}, NewError(ErrNotFound, "project does not exist")
	}
	if _, exists := m.notes[note.ID]; exists {
		return VaultNote{}, NewError(ErrConflict, "note already exists")
	}
	if note.Status == "" {
		note.Status = NoteStatusActive
	}
	m.notes[note.ID] = note
	return note, nil
}

func (m *MemoryWorkspace) UpdateNote(ctx context.Context, note VaultNote, expectedHash string) (VaultNote, error) {
	if err := checkContext(ctx); err != nil {
		return VaultNote{}, err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	current, exists := m.notes[note.ID]
	if !exists {
		return VaultNote{}, NewError(ErrNotFound, "note does not exist")
	}
	if expectedHash != "" && current.ContentHash != expectedHash {
		return VaultNote{}, NewError(ErrConflict, "note content changed externally")
	}
	m.notes[note.ID] = note
	return note, nil
}

func (m *MemoryWorkspace) DeleteNote(ctx context.Context, id NoteID) error {
	if err := checkContext(ctx); err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, exists := m.notes[id]; !exists {
		return NewError(ErrNotFound, "note does not exist")
	}
	delete(m.notes, id)
	return nil
}

func (m *MemoryWorkspace) Upsert(ctx context.Context, document RetrievalDocument) error {
	if err := checkContext(ctx); err != nil {
		return err
	}
	if document.ID == "" || document.NoteID == "" {
		return NewError(ErrInvalidRequest, "document id and note id are required")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.documents[document.ID] = document
	return nil
}

func (m *MemoryWorkspace) Remove(ctx context.Context, id string) error {
	if err := checkContext(ctx); err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, exists := m.documents[id]; !exists {
		return NewError(ErrNotFound, "retrieval document does not exist")
	}
	delete(m.documents, id)
	return nil
}

func (m *MemoryWorkspace) Query(ctx context.Context, projectID ProjectID, query string, limit int) ([]RetrievalDocument, error) {
	if err := checkContext(ctx); err != nil {
		return nil, err
	}
	if limit < 1 {
		return nil, NewError(ErrInvalidRequest, "query limit must be positive")
	}
	needle := strings.ToLower(strings.TrimSpace(query))
	m.mu.RLock()
	defer m.mu.RUnlock()
	matches := make([]RetrievalDocument, 0)
	for _, document := range m.documents {
		if document.ProjectID == projectID && (needle == "" || strings.Contains(strings.ToLower(document.Content), needle)) {
			matches = append(matches, document)
		}
	}
	sort.Slice(matches, func(i, j int) bool { return matches[i].ID < matches[j].ID })
	if len(matches) > limit {
		matches = matches[:limit]
	}
	return matches, nil
}

func (m *MemoryWorkspace) Rebuild(ctx context.Context, projectID ProjectID) error {
	return checkContext(ctx)
}

func checkContext(ctx context.Context) error {
	if ctx == nil {
		return NewError(ErrInvalidRequest, "context is required")
	}
	if err := ctx.Err(); err != nil {
		return CanceledError(err)
	}
	return nil
}
