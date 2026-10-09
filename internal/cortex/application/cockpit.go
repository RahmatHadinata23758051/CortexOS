package application

import (
	"context"
	"errors"
	"time"
)

const CockpitBridgeSchemaVersion = "cortexos.cockpit.bridge.v1"

var ErrCockpitInvalidRequest = errors.New("invalid cockpit bridge request")

// CockpitPort is the application boundary for Cockpit observability and controlled mutations.
// It exposes safe, typed queries without leaking SQL, filesystem paths, process output,
// secrets, provider credentials, or internal engine state.
type CockpitPort interface {
	// GetCockpitRuntime returns the deterministic local runtime status.
	GetCockpitRuntime(ctx context.Context) (CockpitRuntimeSnapshot, error)
	// GetCockpitWorkspace returns the workspace snapshot with project and worktree facts.
	GetCockpitWorkspace(ctx context.Context, request CockpitWorkspaceRequest) (CockpitWorkspaceSnapshot, error)
	// RegisterCockpitProject registers a new project in the local workspace.
	RegisterCockpitProject(ctx context.Context, request CockpitProjectRequest) (CockpitProject, error)
	// QueryCockpitWorkspace queries the workspace retrieval index.
	QueryCockpitWorkspace(ctx context.Context, request CockpitQueryRequest) ([]CockpitQueryResult, error)
	// RebuildCockpitRetrieval triggers a retrieval index rebuild for a project.
	RebuildCockpitRetrieval(ctx context.Context, request CockpitMutationRequest) error
}

// CockpitRuntimeSnapshot is the deterministic read-only response for cockpit runtime status.
type CockpitRuntimeSnapshot struct {
	SchemaVersion string `json:"schemaVersion"`
	Status        string `json:"status"`      // "ready" | "degraded" | "unavailable"
	Environment   string `json:"environment"` // "local" | "production"
	Provider      string `json:"provider"`    // "disabled" | "configured"
}

// CockpitWorkspaceRequest is the versioned request for workspace snapshot.
type CockpitWorkspaceRequest struct {
	SchemaVersion string `json:"schemaVersion"`
	ProjectID     string `json:"projectId,omitempty"`
}

// CockpitProject is the safe, serialized form of a registered project.
type CockpitProject struct {
	ID            string    `json:"id"`
	Name          string    `json:"name"`
	DefaultBranch string    `json:"defaultBranch,omitempty"`
	Status        string    `json:"status"`
	CreatedAt     time.Time `json:"createdAt"`
	UpdatedAt     time.Time `json:"updatedAt"`
	SchemaVersion string    `json:"schemaVersion"`
}

// CockpitWorktree is the safe, serialized form of a worktree.
type CockpitWorktree struct {
	ID              string    `json:"id"`
	ProjectID       string    `json:"projectId"`
	Branch          string    `json:"branch,omitempty"`
	Revision        string    `json:"revision,omitempty"`
	Status          string    `json:"status"`
	Dirty           bool      `json:"dirty"`
	ActiveReference string    `json:"activeReference,omitempty"`
	CreatedAt       time.Time `json:"createdAt"`
	UpdatedAt       time.Time `json:"updatedAt"`
}

// CockpitWorkspaceSnapshot is the safe, serialized workspace state for the cockpit.
type CockpitWorkspaceSnapshot struct {
	SchemaVersion    string            `json:"schemaVersion"`
	Projects         []CockpitProject  `json:"projects"`
	Worktrees        []CockpitWorktree `json:"worktrees"`
	VaultNoteCount   int               `json:"vaultNoteCount"`
	WatcherState     string            `json:"watcherState"`
	WatcherErrorCode string            `json:"watcherErrorCode,omitempty"`
	RetrievalVersion string            `json:"retrievalVersion"`
	RetrievalState   string            `json:"retrievalState"`
}

// CockpitProjectRequest is the versioned request for project registration.
type CockpitProjectRequest struct {
	SchemaVersion  string `json:"schemaVersion"`
	ID             string `json:"id"`
	Name           string `json:"name"`
	RepositoryRoot string `json:"repositoryRoot"`
	VaultRoot      string `json:"vaultRoot"`
	DefaultBranch  string `json:"defaultBranch,omitempty"`
}

// CockpitQueryRequest is the versioned request for workspace query.
type CockpitQueryRequest struct {
	SchemaVersion string `json:"schemaVersion"`
	ProjectID     string `json:"projectId"`
	Query         string `json:"query"`
	Limit         int    `json:"limit,omitempty"`
}

// CockpitQueryResult is the safe, serialized form of a retrieval match.
type CockpitQueryResult struct {
	ID            string `json:"id"`
	NoteID        string `json:"noteId"`
	ProjectID     string `json:"projectId"`
	RelativePath  string `json:"relativePath"`
	SourceHash    string `json:"sourceHash"`
	Attribution   string `json:"attribution"`
	IndexedAt     string `json:"indexedAt"`
	Excerpt       string `json:"excerpt"`
	SchemaVersion string `json:"schemaVersion"`
}

// CockpitMutationRequest is the versioned request for workspace mutations.
type CockpitMutationRequest struct {
	SchemaVersion string `json:"schemaVersion"`
	ProjectID     string `json:"projectId"`
}

// CockpitBridgeError is the stable, generic error returned across the Cockpit bridge.
type CockpitBridgeError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func (e *CockpitBridgeError) Error() string {
	if e == nil {
		return "<nil>"
	}
	return e.Code + ": " + e.Message
}

// validateCockpitRequest checks context cancellation and schema version.
func validateCockpitRequest(ctx context.Context, schemaVersion string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if schemaVersion != CockpitBridgeSchemaVersion {
		return &CockpitBridgeError{Code: "cockpit.unsupported_version", Message: "cockpit bridge contract version is unsupported"}
	}
	return nil
}

// cockpitBridgeError maps internal errors to stable bridge error codes.
func cockpitBridgeError(err error) *CockpitBridgeError {
	if err == nil {
		return nil
	}
	code := "cockpit.internal"
	switch {
	case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
		code = "cockpit.canceled"
	}
	messages := map[string]string{
		"cockpit.canceled":            "cockpit request canceled",
		"cockpit.unsupported_version": "cockpit bridge contract version is unsupported",
	}
	message := messages[code]
	if message == "" {
		message = "cockpit request failed"
	}
	return &CockpitBridgeError{Code: code, Message: message}
}

// mapCockpitError maps internal errors to bridge errors with context.
func mapCockpitError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return &CockpitBridgeError{Code: "cockpit.canceled", Message: "cockpit request canceled"}
	}
	return cockpitBridgeError(err)
}
