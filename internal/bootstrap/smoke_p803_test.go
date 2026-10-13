//go:build p803_smoke
// +build p803_smoke

package bootstrap_test

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/RahmatHadinata23758051/CortexOS/internal/cortex/harness"
	harnesssqlite "github.com/RahmatHadinata23758051/CortexOS/internal/cortex/harness/sqlite"
	"github.com/RahmatHadinata23758051/CortexOS/internal/cortex/orchestra"
	orchestrasqlite "github.com/RahmatHadinata23758051/CortexOS/internal/cortex/orchestra/sqlite"
	"github.com/RahmatHadinata23758051/CortexOS/internal/cortex/workspace"
	workspacegit "github.com/RahmatHadinata23758051/CortexOS/internal/cortex/workspace/git"
	workspacesqlite "github.com/RahmatHadinata23758051/CortexOS/internal/cortex/workspace/sqlite"
)

func TestPackagedWailsSmoke_MergePipeline(t *testing.T) {
	if _, err := exec.LookPath("wails"); err != nil {
		t.Skip("wails CLI not installed")
	}
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not installed")
	}

	ctx := context.Background()
	root := t.TempDir()

	// 1. Create a temporary git repository to serve as project root
	repoRoot := filepath.Join(root, "repository")
	if err := initGitRepository(repoRoot); err != nil {
		t.Fatalf("init git repo: %v", err)
	}

	// 2. Prepare data root for CortexOS (will be passed via CORTEXOS_DATA_ROOT)
	dataRoot := filepath.Join(root, "data")

	// 3. Seed all three SQLite databases using production adapters
	// Workspace state
	wsState, err := workspacesqlite.Open(ctx, filepath.Join(dataRoot, "state", "workspace.db"))
	if err != nil {
		t.Fatalf("open workspace db: %v", err)
	}
	defer wsState.Close()

	// Register project
	proj, err := wsState.RegisterProject(ctx, workspace.Project{
		ID:             "smoke-project",
		Name:           "Smoke Project",
		RepositoryRoot: repoRoot,
		VaultRoot:      filepath.Join(dataRoot, "vault"),
		DefaultBranch:  "main",
		Status:         workspace.ProjectStatusActive,
		CreatedAt:      time.Now().UTC(),
		UpdatedAt:      time.Now().UTC(),
		SchemaVersion:  workspace.ContractVersion,
	})
	if err != nil {
		t.Fatalf("register project: %v", err)
	}

	// Create a Git worktree for the project (needed for worktree cleanliness check)
	gitAdapter, err := workspacegit.NewWithRegistry(wsState, nil, filepath.Join(dataRoot, "worktrees"))
	if err != nil {
		t.Fatalf("create git adapter: %v", err)
	}
	wtPath := filepath.Join(dataRoot, "worktrees", "smoke-project", "smoke-worktree")
	wt, err := gitAdapter.CreateWorktree(ctx, workspace.Worktree{
		ID:        "smoke-worktree",
		ProjectID: proj.ID,
		Path:      wtPath,
		Branch:    "feature/smoke",
	})
	if err != nil {
		t.Fatalf("create worktree: %v", err)
	}
	if wt.Dirty || wt.Revision == "" {
		t.Fatalf("worktree not clean: dirty=%v, rev=%s", wt.Dirty, wt.Revision)
	}

	// Harness evidence store
	harnessStore, err := harnesssqlite.Open(ctx, filepath.Join(dataRoot, "state", "harness.db"))
	if err != nil {
		t.Fatalf("open harness db: %v", err)
	}
	defer harnessStore.Close()

	// Orchestra store
	orchStore, err := orchestrasqlite.Open(ctx, filepath.Join(dataRoot, "state", "orchestra.db"))
	if err != nil {
		t.Fatalf("open orchestra db: %v", err)
	}
	defer orchStore.Close()

	// 4. Seed Orchestra task graph: parent (awaitingInspection) + child (draft, depends on parent)
	parentTask := orchestra.Task{
		ID:                 "smoke-parent",
		ProjectID:          "smoke-project",
		WorktreeID:         string(wt.ID),
		Title:              "Smoke Parent Task",
		AcceptanceCriteria: []string{"passes smoke test"},
		Status:             orchestra.TaskAwaitingInspection,
		MaxAttempts:        3,
		AttemptCount:       1,
		CreatedAt:          time.Now().UTC(),
		UpdatedAt:          time.Now().UTC(),
		SchemaVersion:      orchestra.ContractVersion,
	}
	if err := orchStore.SaveTask(ctx, parentTask); err != nil {
		t.Fatalf("save parent task: %v", err)
	}

	childTask := orchestra.Task{
		ID:                 "smoke-child",
		ProjectID:          "smoke-project",
		WorktreeID:         string(wt.ID),
		Title:              "Smoke Child Task",
		AcceptanceCriteria: []string{"parent done"},
		Dependencies:       []orchestra.TaskID{parentTask.ID},
		Status:             orchestra.TaskDraft,
		MaxAttempts:        3,
		AttemptCount:       0,
		CreatedAt:          time.Now().UTC(),
		UpdatedAt:          time.Now().UTC(),
		SchemaVersion:      orchestra.ContractVersion,
	}
	if err := orchStore.SaveTask(ctx, childTask); err != nil {
		t.Fatalf("save child task: %v", err)
	}

	// 5. Seed execution and evidence in Harness
	execID := orchestra.ExecutionID("smoke-parent-exec-1")
	taskExec := orchestra.Execution{
		ID:          execID,
		TaskID:      parentTask.ID,
		Attempt:     1,
		Status:      orchestra.TaskAwaitingInspection,
		StartedAt:   time.Now().UTC(),
		EvidenceIDs: []string{"smoke-ev-1"},
	}
	if err := orchStore.SaveExecution(ctx, taskExec); err != nil {
		t.Fatalf("save execution: %v", err)
	}

	evRecord, err := harness.NewEvidenceRecord(string(execID), string(parentTask.ID), string(wt.ID), harness.EvidenceKindCommandExecution, map[string]string{
		"suite": "smoke",
		"pass":  "true",
	})
	if err != nil {
		t.Fatalf("create evidence record: %v", err)
	}
	evRecord.ID = "smoke-ev-1"
	if err := harnessStore.SaveEvidence(ctx, evRecord); err != nil {
		t.Fatalf("save evidence: %v", err)
	}

	// Close all SQLite stores BEFORE building to release file locks (Windows)
	_ = harnessStore.Close()
	_ = orchStore.Close()
	_ = wsState.Close()

	// 6. Build Wails binary from the repository root
	repoRootPath, err := filepath.Abs("../..")
	if err != nil {
		t.Fatalf("resolve repo root: %v", err)
	}
	binaryName := "cortexos-smoke.exe"
	binaryPath := filepath.Join(root, binaryName)
	cmd := exec.Command("wails", "build", "-clean", "-nopackage", "-o", binaryName)
	cmd.Dir = repoRootPath
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = append(os.Environ(), "CGO_ENABLED=1")
	if err := cmd.Run(); err != nil {
		t.Fatalf("wails build failed: %v", err)
	}
	// Wails outputs to build/bin by default if -o is a relative name
	actualBinary := filepath.Join(repoRootPath, "build", "bin", binaryName)
	if _, err := os.Stat(actualBinary); err != nil {
		actualBinary = binaryPath
		if _, err := os.Stat(actualBinary); err != nil {
			t.Fatalf("binary not found at %s or %s", filepath.Join(repoRootPath, "build", "bin", binaryName), binaryPath)
		}
	}
	t.Logf("built binary: %s", actualBinary)

	// 7. Run packaged binary with smoke env vars
	resultFile := filepath.Join(root, "smoke-result.json")
	env := append(os.Environ(),
		"CORTEXOS_DATA_ROOT="+dataRoot,
		"CORTEXOS_SMOKE_QUIT_MS=5000",
		"CORTEXOS_SMOKE_MERGE_TASK=smoke-parent",
		"CORTEXOS_SMOKE_RESULT_FILE="+resultFile,
	)
	runCmd := exec.Command(actualBinary)
	runCmd.Env = env
	runCmd.Stdout = os.Stdout
	runCmd.Stderr = os.Stderr

	t.Logf("starting packaged binary with smoke probe...")
	start := time.Now()
	if err := runCmd.Run(); err != nil {
		t.Fatalf("packaged binary exited with error: %v", err)
	}
	t.Logf("binary exited cleanly in %v", time.Since(start))

	// 8. Validate result file
	resultData, err := os.ReadFile(resultFile)
	if err != nil {
		t.Fatalf("read result file: %v", err)
	}
	var result map[string]any
	if err := json.Unmarshal(resultData, &result); err != nil {
		t.Fatalf("parse result JSON: %v", err)
	}
	if result["error"] != nil && result["error"] != "" {
		t.Fatalf("merge returned error: %s", result["error"])
	}
	if result["status"] != string(orchestra.TaskSuccess) {
		t.Fatalf("expected status %s, got %s", orchestra.TaskSuccess, result["status"])
	}
	if result["taskID"] != string(parentTask.ID) {
		t.Fatalf("expected taskID %s, got %s", parentTask.ID, result["taskID"])
	}
	t.Logf("smoke result: %s", string(resultData))

	// 9. Verify final state in Orchestra SQLite (independent of bridge result)
	// Reopen stores to ensure persistence survived process boundary
	orchStore2, err := orchestrasqlite.Open(ctx, filepath.Join(dataRoot, "state", "orchestra.db"))
	if err != nil {
		t.Fatalf("reopen orchestra db: %v", err)
	}
	defer orchStore2.Close()

	parent, err := orchStore2.GetTask(ctx, parentTask.ID)
	if err != nil {
		t.Fatalf("get parent task: %v", err)
	}
	if parent.Status != orchestra.TaskSuccess {
		t.Fatalf("parent status = %s, want %s", parent.Status, orchestra.TaskSuccess)
	}

	child, err := orchStore2.GetTask(ctx, childTask.ID)
	if err != nil {
		t.Fatalf("get child task: %v", err)
	}
	if child.Status != orchestra.TaskReady {
		t.Fatalf("child status = %s, want %s", child.Status, orchestra.TaskReady)
	}

	// Verify events
	parentEvents, err := orchStore2.ListEvents(ctx, parentTask.ID)
	if err != nil {
		t.Fatalf("list parent events: %v", err)
	}
	foundAccepted := false
	for _, ev := range parentEvents {
		if ev.Type == orchestra.EventInspectionAccepted && ev.To == orchestra.TaskSuccess {
			foundAccepted = true
			break
		}
	}
	if !foundAccepted {
		t.Fatal("parent events missing EventInspectionAccepted -> TaskSuccess")
	}

	childEvents, err := orchStore2.ListEvents(ctx, childTask.ID)
	if err != nil {
		t.Fatalf("list child events: %v", err)
	}
	foundValidated := false
	for _, ev := range childEvents {
		if ev.Type == orchestra.EventTaskValidated && ev.To == orchestra.TaskReady {
			foundValidated = true
			break
		}
	}
	if !foundValidated {
		t.Fatal("child events missing EventTaskValidated -> TaskReady")
	}

	t.Log("P8-03 packaged Wails smoke test PASSED")
}

func initGitRepository(repoRoot string) error {
	// Create the directory first
	if err := os.MkdirAll(repoRoot, 0o755); err != nil {
		return fmt.Errorf("create repo dir: %w", err)
	}
	cmds := [][]string{
		{"init"},
		{"config", "user.email", "smoke@test"},
		{"config", "user.name", "Smoke Test"},
		{"commit", "--allow-empty", "-m", "initial commit"},
	}
	for _, args := range cmds {
		cmd := exec.Command("git", args...)
		cmd.Dir = repoRoot
		if out, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("git %v failed: %v\n%s", args, err, out)
		}
	}
	return nil
}
