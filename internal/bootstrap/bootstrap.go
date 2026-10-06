package bootstrap

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/RahmatHadinata23758051/CortexOS/internal/cortex/application"
	"github.com/RahmatHadinata23758051/CortexOS/internal/cortex/workspace"
	workspacegit "github.com/RahmatHadinata23758051/CortexOS/internal/cortex/workspace/git"
	"github.com/RahmatHadinata23758051/CortexOS/internal/cortex/workspace/retrieval"
	workspacesqlite "github.com/RahmatHadinata23758051/CortexOS/internal/cortex/workspace/sqlite"
	"github.com/RahmatHadinata23758051/CortexOS/internal/cortex/workspace/vault"
	"github.com/RahmatHadinata23758051/CortexOS/internal/cortex/workspace/watch"
	"github.com/RahmatHadinata23758051/CortexOS/internal/platform"
)

// OpenWorkspace composes the production Workspace adapters below a single
// application-owned data root. The returned close function owns the SQLite
// lifecycle; Markdown and retrieval files remain durable on disk.
func OpenWorkspace(ctx context.Context, dataRoot string) (*workspace.Service, func() error, error) {
	if ctx == nil {
		return nil, nil, workspace.NewError(workspace.ErrInvalidRequest, "context is required")
	}
	if err := ctx.Err(); err != nil {
		return nil, nil, workspace.CanceledError(err)
	}
	canonicalRoot, err := workspace.CanonicalRoot(dataRoot)
	if err != nil {
		return nil, nil, err
	}
	if err := os.MkdirAll(canonicalRoot, 0o700); err != nil {
		return nil, nil, workspace.WrapError(workspace.ErrStorageUnavailable, "prepare workspace data root", err)
	}

	state, err := workspacesqlite.Open(ctx, filepath.Join(canonicalRoot, "state", "workspace.db"))
	if err != nil {
		return nil, nil, err
	}
	closeState := func() error { return state.Close() }
	closeOnError := func(cause error) (*workspace.Service, func() error, error) {
		_ = closeState()
		return nil, nil, cause
	}

	vaultStore, err := vault.NewWithMetadata(filepath.Join(canonicalRoot, "vault"), state)
	if err != nil {
		return closeOnError(err)
	}
	index, err := retrieval.NewWithSource(filepath.Join(canonicalRoot, "retrieval"), vaultStore)
	if err != nil {
		return closeOnError(err)
	}
	gitAdapter, err := workspacegit.NewWithRegistry(state, nil, filepath.Join(canonicalRoot, "worktrees"))
	if err != nil {
		return closeOnError(err)
	}
	service, err := workspace.NewService(workspace.Dependencies{
		State: state, Projects: state, Worktrees: gitAdapter, Vault: vaultStore,
		Watcher: watch.New(), Retrieval: index,
	})
	if err != nil {
		return closeOnError(err)
	}
	if err := service.Open(ctx); err != nil {
		return closeOnError(err)
	}
	return service, closeState, nil
}

func defaultWorkspaceDataRoot() (string, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("resolve CortexOS config directory: %w", err)
	}
	return filepath.Join(configDir, "CortexOS"), nil
}

// Run constructs the application graph and starts the desktop lifecycle.
func Run() error {
	dataRoot, err := defaultWorkspaceDataRoot()
	if err != nil {
		return err
	}
	workspaceService, closeWorkspace, err := OpenWorkspace(context.Background(), dataRoot)
	if err != nil {
		return err
	}
	defer func() { _ = closeWorkspace() }()

	service := application.NewServiceWithWorkspace(workspaceService)
	app := platform.New()
	app.Bind(platform.NewBridge(service))
	return app.Run()
}
