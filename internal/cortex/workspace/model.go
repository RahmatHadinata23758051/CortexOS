package workspace

import "time"

const (
	ContractVersion       = "cortexos.workspace.v1"
	ProjectStatusActive   = "active"
	ProjectStatusArchived = "archived"
	WorktreeStatusActive  = "active"
	WorktreeStatusRemoved = "removed"
	NoteStatusActive      = "active"
	NoteStatusArchived    = "archived"
)

type ProjectID string
type WorktreeID string
type NoteID string
type EventID string

type Project struct {
	ID             ProjectID `json:"id"`
	Name           string    `json:"name"`
	RepositoryRoot string    `json:"repositoryRoot"`
	VaultRoot      string    `json:"vaultRoot"`
	DefaultBranch  string    `json:"defaultBranch,omitempty"`
	Status         string    `json:"status"`
	CreatedAt      time.Time `json:"createdAt"`
	UpdatedAt      time.Time `json:"updatedAt"`
	SchemaVersion  string    `json:"schemaVersion"`
}

type Worktree struct {
	ID              WorktreeID `json:"id"`
	ProjectID       ProjectID  `json:"projectId"`
	Path            string     `json:"path"`
	Branch          string     `json:"branch,omitempty"`
	Revision        string     `json:"revision,omitempty"`
	Status          string     `json:"status"`
	Dirty           bool       `json:"dirty"`
	ActiveReference string     `json:"activeReference,omitempty"`
	CreatedAt       time.Time  `json:"createdAt"`
	UpdatedAt       time.Time  `json:"updatedAt"`
}

type VaultNote struct {
	ID            NoteID     `json:"id"`
	ProjectID     ProjectID  `json:"projectId"`
	WorktreeID    WorktreeID `json:"worktreeId,omitempty"`
	RelativePath  string     `json:"relativePath"`
	Title         string     `json:"title"`
	Body          string     `json:"body"`
	FormatVersion string     `json:"formatVersion"`
	Source        string     `json:"source"`
	Author        string     `json:"author"`
	CreatedAt     time.Time  `json:"createdAt"`
	UpdatedAt     time.Time  `json:"updatedAt"`
	ContentHash   string     `json:"contentHash"`
	Status        string     `json:"status"`
}

type FileChangeOperation string

const (
	FileCreated        FileChangeOperation = "created"
	FileModified       FileChangeOperation = "modified"
	FileRemoved        FileChangeOperation = "removed"
	FileRenamed        FileChangeOperation = "renamed"
	FileRescanRequired FileChangeOperation = "rescanRequired"
	FileError          FileChangeOperation = "error"
)

type FileChange struct {
	ID            EventID             `json:"id"`
	CorrelationID string              `json:"correlationId"`
	RootID        string              `json:"rootId"`
	RelativePath  string              `json:"relativePath"`
	Operation     FileChangeOperation `json:"operation"`
	ObservedAt    time.Time           `json:"observedAt"`
	ContentHash   string              `json:"contentHash,omitempty"`
	ErrorCode     string              `json:"errorCode,omitempty"`
}

type RetrievalDocument struct {
	ID           string     `json:"id"`
	NoteID       NoteID     `json:"noteId"`
	ProjectID    ProjectID  `json:"projectId"`
	WorktreeID   WorktreeID `json:"worktreeId,omitempty"`
	RelativePath string     `json:"relativePath"`
	SourceHash   string     `json:"sourceHash"`
	IndexVersion string     `json:"indexVersion"`
	Content      string     `json:"content"`
	Attribution  string     `json:"attribution"`
	IndexedAt    time.Time  `json:"indexedAt"`
}

type WorkspaceSnapshot struct {
	SchemaVersion    string     `json:"schemaVersion"`
	Projects         []Project  `json:"projects"`
	Worktrees        []Worktree `json:"worktrees"`
	VaultNoteCount   int        `json:"vaultNoteCount"`
	WatcherState     string     `json:"watcherState"`
	WatcherErrorCode string     `json:"watcherErrorCode,omitempty"`
	RetrievalVersion string     `json:"retrievalVersion"`
	RetrievalState   string     `json:"retrievalState"`
}
