package workspace

import (
	"context"
	"sort"
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
	open      bool
}

func NewMemoryWorkspace() *MemoryWorkspace {
	return &MemoryWorkspace{
		projects:  make(map[ProjectID]Project),
		worktrees: make(map[WorktreeID]Worktree),
		notes:     make(map[NoteID]VaultNote),
		documents: make(map[string]RetrievalDocument),
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
	return nil
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

func checkContext(ctx context.Context) error {
	if ctx == nil {
		return NewError(ErrInvalidRequest, "context is required")
	}
	if err := ctx.Err(); err != nil {
		return CanceledError(err)
	}
	return nil
}
