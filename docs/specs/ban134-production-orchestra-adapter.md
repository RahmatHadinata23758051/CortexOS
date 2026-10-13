# BAN-134 Production Orchestra adapter (P8-01)

P7-05 (BAN-130) replaced the simulated frontend merge with a typed backend
command, `MergeOrchestraTask`. Without a wired Orchestra port in
`internal/bootstrap/bootstrap.go`, production merge failed closed with
`orchestra.storage_unavailable`. This spec documents the production adapter
that closes that residual gap.

## Architecture

- Go services/bridge own logic; React/Phaser presentation-only (unchanged).
- Phaser remains a read-only consumer of typed bridge/event state.
- ADR-0005: merge is allowed only when the Inspector accepts governed work:
  evidence immutable, merge transitions the plan, worktree clean.

## Components

### 1. Production adapter (`internal/cortex/orchestra/adapter.go`)

- `ProductionAdapter` implements `application.OrchestraPort`.
- Required by constructor: `TaskStore` (`orchestra.TaskStore`).
- Assumed by constructor: `HarnessStore` (harness evidence lookup) and
  `WorktreeInspector` (worktree cleanliness verification).
- `NewProductionAdapter` fails if `TaskStore` is nil.
- `MergeTask` governs authoritative merge:
  1. Load task from the durable store; reject non-`awaitingInspection` tasks.
  2. Resolve the latest execution (explicit store record or event timeline).
  3. Collect governed audit evidence from `HarnessStore`
     (`EvidenceForExecution`), falling back to the execution's pinned
     `EvidenceIDs`.
  4. Resolve worktree state through `WorktreeInspector`; dirty or invalid
     worktrees reject with `ErrDirtyWorktree` / `ErrInvalidWorktree`.
  5. Build `InspectionRequest` and call the injected `Inspector`
     (default `MergeAuthority{Capability: MergeCapability}`).
  6. On rejection, append `EventInspectionRejected` and return the domain error
     (mapped by `application` to `orchestra.inspection_rejected`).
  7. On acceptance, persist `AcceptedTask` (`TaskSuccess`) to SQLite and append
     `EventInspectionAccepted` with the caller note.
  8. Call `updatePlanGraph` for the task's project: every draft task whose
     dependencies are all `TaskSuccess` transitions to `TaskReady` and is
     persisted, with an `EventTaskValidated` timeline entry.
- `ListTasks`, `GetTask`, `ListEvents`, `CancelTask`, and `RetryTask` are
  implemented against the same durable store using existing state transitions.

### 2. Bootstrap composition (`internal/bootstrap/bootstrap.go`)

- `OpenOrchestra` opens two independent SQLite databases under the same
  canonical root as Workspace:
  - `state/orchestra.db` (orchestra tasks/executions/events),
  - `state/harness.db` (Harness audit evidence).
- It constructs `ProductionAdapter` with the two stores, a
  `workspaceWorktreeInspector` bridge (maps `workspace.Service.InspectWorktree`
  to `orchestra.WorktreeInspector`), and `MergeAuthority`.
- `Run` wires the adapter into the service graph:
  `application.NewServiceWithWorkspaceAndOrchestra(workspaceService, orchestraPort)`.
- The close function owns both SQLite lifecycles; failures during composition
  close any store that was opened before returning.

### 3. Error contract

| Failure                               | Backend error                       | Bridge code                      |
|---------------------------------------|-------------------------------------|----------------------------------|
| No port / no store                    | `ErrStoreUnavailable`               | `orchestra.storage_unavailable`  |
| Task not awaiting inspection          | `ErrInspectionRejected`             | `orchestra.inspection_rejected`  |
| Invalid / missing merge capability    | `ErrInspectionRejected`             | `orchestra.inspection_rejected`  |
| Missing or failed evidence            | `ErrMissingEvidence` / reject       | `orchestra.inspection_rejected`  |
| Dirty worktree                        | `ErrDirtyWorktree` (wrapped)        | `orchestra.inspection_rejected`* |
| Invalid worktree                      | `ErrInvalidWorktree` (wrapped)      | `orchestra.internal`*            |
| Terminal / invalid transition         | `ErrTerminalTask` / `ErrInvalidTransition` | `orchestra.terminal` / `orchestra.internal` |

\* `ErrDirtyWorktree` and `ErrInvalidWorktree` do not yet map to dedicated bridge
codes in `application.orchestraBridgeError`; the bridge currently reports them
as `orchestra.inspection_rejected` / `orchestra.internal` with the domain
message preserved in the returned error.

## Tests

- `internal/cortex/orchestra/adapter_test.go`
  - `MergeTask` succeeds and persists `TaskSuccess`.
  - Parent merge unblocks a dependent draft task to `TaskReady`.
  - Dirty worktree fails closed; task status unchanged.
  - Missing evidence fails closed; task status unchanged.
- `internal/cortex/orchestra/adapter_integration_test.go`
  - End-to-end chain through the production port:
    `platform.Bridge.MergeOrchestraTask` → `application.Service` →
    `ProductionAdapter` → `MergeAuthority` → SQLite persistence →
    event timeline + plan graph update.
  - Fail-closed chain for dirty worktrees through the same bridge entry point.
- `internal/bootstrap/bootstrap_test.go`
  - `OpenOrchestra` lifecycle: open, empty `ListTasks`, close cleanly.
- Regression: all existing `internal/cortex/...` and `internal/bootstrap` tests
  pass unchanged, including `TestBAN124_EndToEndIntegration_CockpitBridgeChain`
  and the orchestra SQLite suite.

## Release gate

- `go test ./internal/cortex/... ./internal/bootstrap/...` passes.
- `gofmt` clean on touched files.
- No SQL migration introduced: the adapter reuses the existing
  `cortexos.orchestra.sqlite.v1` and harness SQLite schemas.
