package workspace

import "context"

type ProjectRegistry interface {
	RegisterProject(context.Context, Project) (Project, error)
	UpdateProject(context.Context, Project) (Project, error)
	ArchiveProject(context.Context, ProjectID) error
	GetProject(context.Context, ProjectID) (Project, error)
	ListProjects(context.Context) ([]Project, error)
}

type WorktreeManager interface {
	InspectWorktree(context.Context, WorktreeID) (Worktree, error)
	CreateWorktree(context.Context, Worktree) (Worktree, error)
	ListWorktrees(context.Context, ProjectID) ([]Worktree, error)
	RemoveWorktree(context.Context, WorktreeID) error
}

type StateStore interface {
	Open(context.Context) error
	Close() error
	SchemaVersion(context.Context) (string, error)
}

type VaultStore interface {
	GetNote(context.Context, NoteID) (VaultNote, error)
	ListNotes(context.Context, ProjectID) ([]VaultNote, error)
	CreateNote(context.Context, VaultNote) (VaultNote, error)
	UpdateNote(context.Context, VaultNote, string) (VaultNote, error)
	DeleteNote(context.Context, NoteID) error
}

type FileWatcher interface {
	Start(context.Context, string) (<-chan FileChange, error)
	Stop() error
}

type RetrievalIndex interface {
	Upsert(context.Context, RetrievalDocument) error
	Remove(context.Context, string) error
	Query(context.Context, ProjectID, string, int) ([]RetrievalDocument, error)
	Rebuild(context.Context, ProjectID) error
}
