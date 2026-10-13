package bootstrap

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/RahmatHadinata23758051/CortexOS/internal/cortex/application"
	harnesssqlite "github.com/RahmatHadinata23758051/CortexOS/internal/cortex/harness/sqlite"
	"github.com/RahmatHadinata23758051/CortexOS/internal/cortex/orchestra"
	orchestrasqlite "github.com/RahmatHadinata23758051/CortexOS/internal/cortex/orchestra/sqlite"
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

// OpenOrchestra composes the production Orchestra adapter over durable SQLite state,
// Harness audit evidence, and Workspace worktree cleanliness verification. The adapter
// owns merge authority gating (ADR-0005) and task-plan persistence. The returned close
// function owns the Orchestra and Harness SQLite lifecycles.
func OpenOrchestra(ctx context.Context, canonicalRoot string, workspaceService *workspace.Service) (application.OrchestraPort, func() error, error) {
	if ctx == nil {
		return nil, nil, orchestra.ErrInvalidTask
	}
	if err := ctx.Err(); err != nil {
		return nil, nil, err
	}
	if canonicalRoot == "" {
		return nil, nil, workspace.NewError(workspace.ErrInvalidRequest, "canonical root is required")
	}
	orchStore, err := orchestrasqlite.Open(ctx, filepath.Join(canonicalRoot, "state", "orchestra.db"))
	if err != nil {
		return nil, nil, err
	}

	harnessStore, err := harnesssqlite.Open(ctx, filepath.Join(canonicalRoot, "state", "harness.db"))
	if err != nil {
		_ = orchStore.Close()
		return nil, nil, err
	}
	closeAll := func() error {
		var firstErr, secondErr error
		if orchStore != nil {
			firstErr = orchStore.Close()
		}
		if harnessStore != nil {
			secondErr = harnessStore.Close()
		}
		if firstErr != nil {
			return firstErr
		}
		return secondErr
	}

	adapter, err := orchestra.NewProductionAdapter(orchestra.ProductionAdapterConfig{
		TaskStore:         orchStore,
		HarnessStore:      harnessStore,
		WorktreeInspector: &workspaceWorktreeInspector{workspace: workspaceService},
		Inspector:         orchestra.MergeAuthority{Capability: orchestra.MergeCapability},
	})
	if err != nil {
		_ = closeAll()
		return nil, nil, err
	}
	return adapter, closeAll, nil
}

// workspaceWorktreeInspector adapts workspace.Service worktree inspection to the
// orchestra.WorktreeInspector contract without creating a package import cycle.
type workspaceWorktreeInspector struct {
	workspace *workspace.Service
}

func (w *workspaceWorktreeInspector) InspectWorktreeState(ctx context.Context, worktreeID string) (orchestra.WorktreeState, error) {
	if w == nil || w.workspace == nil {
		return orchestra.WorktreeState{WorktreeID: worktreeID, Valid: false}, nil
	}
	wt, err := w.workspace.InspectWorktree(ctx, workspace.WorktreeID(worktreeID))
	if err != nil {
		return orchestra.WorktreeState{WorktreeID: worktreeID, Valid: false}, nil
	}
	return orchestra.WorktreeState{
		WorktreeID: string(wt.ID),
		Revision:   wt.Revision,
		Dirty:      wt.Dirty,
		Valid:      wt.Status == workspace.WorktreeStatusActive,
	}, nil
}

func defaultWorkspaceDataRoot() (string, error) {
	if custom := os.Getenv("CORTEXOS_DATA_ROOT"); custom != "" {
		return custom, nil
	}
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
	ctx := context.Background()
	workspaceService, closeWorkspace, err := OpenWorkspace(ctx, dataRoot)
	if err != nil {
		return err
	}
	defer func() { _ = closeWorkspace() }()

	canonicalRoot, err := workspace.CanonicalRoot(dataRoot)
	if err != nil {
		return err
	}
	orchestraPort, closeOrchestra, err := OpenOrchestra(ctx, canonicalRoot, workspaceService)
	if err != nil {
		return err
	}
	defer func() { _ = closeOrchestra() }()

	service := application.NewServiceWithWorkspaceAndOrchestra(workspaceService, orchestraPort)
	app := platform.New()
	bridge := platform.NewBridge(service)
	app.OnStartup(bridge.SetRuntimeContext)
	app.Bind(bridge)

	// P8-03 packaged smoke probe: when CORTEXOS_SMOKE_MERGE_TASK and CORTEXOS_SMOKE_QUIT_MS
	// are both set, run a single merge via the production bridge and write result to
	// CORTEXOS_SMOKE_RESULT_FILE. This only activates in test environments.
	if taskID := os.Getenv("CORTEXOS_SMOKE_MERGE_TASK"); taskID != "" {
		if _, ok := smokeQuitDelay(os.Getenv("CORTEXOS_SMOKE_QUIT_MS")); ok {
			app.OnStartup(func(ctx context.Context) {
				result, err := bridge.MergeOrchestraTask(application.OrchestraMergeRequest{
					SchemaVersion: application.OrchestraBridgeSchemaVersion,
					TaskID:        taskID,
					Note:          "packaged smoke merge",
				})
				if resultFile := os.Getenv("CORTEXOS_SMOKE_RESULT_FILE"); resultFile != "" {
					payload := map[string]any{
						"taskID": result.TaskID,
						"status": result.Status,
						"schema": result.SchemaVersion,
						"error":  "",
					}
					if err != nil {
						payload["error"] = err.Error()
					}
					if b, _ := json.Marshal(payload); b != nil {
						_ = os.WriteFile(resultFile, b, 0o600)
					}
				}
			})
		}
	}

	return app.Run()
}

func smokeQuitDelay(raw string) (time.Duration, bool) {
	if raw == "" {
		return 0, false
	}
	ms, err := strconv.Atoi(raw)
	if err != nil || ms < 0 {
		return 0, false
	}
	return time.Duration(ms) * time.Millisecond, true
}
