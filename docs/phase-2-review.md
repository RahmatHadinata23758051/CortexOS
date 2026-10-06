# Phase 2 Workspace Layer Review

**Review issue:** BAN-55
**Milestone:** Phase 2 — Workspace Layer
**Review date:** 2026-10-06
**Decision:** **GO for Phase 3 entry, with documented non-critical platform caveats**

## Scope and ownership

Phase 2 delivers a Wails-independent Workspace core and a typed bridge/UI slice. The Workspace service owns project registration, controlled Git worktrees, SQLite structured metadata, Markdown Vault content, bounded filesystem observation, and rebuildable deterministic retrieval. Markdown remains the editable durable content authority; SQLite stores structured metadata and synchronization hashes; the JSON retrieval index is disposable and rebuildable.

The application bridge exposes only `cortexos.workspace.v1` DTOs. It does not expose raw absolute paths in responses, SQL, filesystem handles, shell commands, process output, secrets, watcher channels, or retrieval storage. React submits typed requests and renders loading, success, empty, malformed-response, duplicate, cancellation, and backend-error states. The cockpit intentionally excludes Orchestra, Harness, Staff, Phaser, cloud sync, provider SDKs, and production execution controls.

## Implemented surface

| Area | Evidence | Review result |
| --- | --- | --- |
| Domain ports and service | `internal/cortex/workspace/{model.go,ports.go,service.go}` | Narrow interfaces; no UI/Wails/runtime-management dependency. |
| Path safety | `paths.go`, Git/Vault/watch security tests | Canonical root/relative checks, component containment, link/reparse checks, mutation-time rechecks. |
| SQLite authority | `sqlite/` migrations, project and note metadata stores | Versioned migrations, bounded connection pool, reopen coverage, Markdown body excluded from metadata. |
| Markdown Vault | `vault/` model/store tests | Strict `cortexos.vault.v1`, attribution/timestamps/hashes, atomic secure writes, optimistic conflict checks. |
| Git isolation | `git/` adapter and disposable repository tests | Fixed argument-array process adapter, repository validation, worktree-root boundary, dirty/locked cleanup rejection. |
| Watch and refresh | `watch/`, `refresh/`, BAN-51 integration test | Bounded/debounced observations, explicit overflow/error, root/path filtering, rebuild from authoritative Vault. |
| Retrieval | `retrieval/`, BAN-50 tests | Local/model-free/network-free, deterministic ranking and IDs, bounded queries, atomic rebuild, corruption handling. |
| Typed bridge | `internal/cortex/application/workspace.go`, `internal/platform/bridge.go`, TypeScript DTOs | Versioned DTOs and stable redacted error mapping; path-free snapshots. |
| Smoke cockpit | `web/src/App.tsx`, styles/tests, `workspace-smoke-contract.md` | Responsive local-control surface with one controlled project-registration mutation. |
| Production composition | `internal/bootstrap/bootstrap.go` | OS config-root composition for SQLite, Vault, retrieval, Git, watcher, and lifecycle close. |

## End-to-end fixture

`internal/cortex/workspace/phase2_integration_test.go` exercises a disposable sequence:

1. initialize a temporary Git repository;
2. open SQLite and register a project;
3. create and inspect an isolated worktree;
4. create an attributed Markdown Vault note;
5. rebuild and query deterministic retrieval;
6. start fsnotify and refresh coordination;
7. update the note, emit a content-bearing save event, and verify retrieval reflects the new source hash;
8. stop safely and reopen SQLite/Git state to verify durable project, worktree, and note metadata.

The test uses only temporary roots and cleans them through `t.TempDir`, including failure paths via explicit close/stop calls.

## Validation evidence

Latest successful gates on Windows/amd64:

- `go test ./... -count=1`
- `go vet ./...`
- `go test ./internal/cortex/workspace -run TestPhase2RegisterNoteWatchQueryAndReopen -count=10`
- frontend `npm run typecheck`
- frontend `npm run lint`
- frontend Vitest: 4 files / 8 tests passed
- frontend `npm run build`
- `scripts/validate-repository.ps1 -SkipHarvest`: Workspace import/dependency review, Go gates, frontend gates, npm audit with 0 vulnerabilities, Wails shell build, secret review, offline/runtime-scope review, ignore policy, and tracked-artifact review passed
- browser smoke: standalone Vite app returned HTTP 200 and rendered the Workspace cockpit accessibility tree; without Wails bindings it correctly displayed the safe unavailable bridge state

## Known caveats and residual risks

1. `go test -race ./internal/cortex/workspace/refresh -count=1` is unavailable in the current Windows environment because `runtime/cgo/cgo.exe` exits with status 2. Normal tests, integration tests, vet, and the repository gates pass. This is an environment/toolchain limitation, not an accepted race-test result.
2. `npm ci` can fail with Windows `EPERM` when a Vite/esbuild process still holds `web/node_modules/@esbuild/.../esbuild.exe`. Stop local dev processes and rerun; the clean rerun passed.
3. A standalone Vite browser smoke cannot provide generated Wails bindings. The UI intentionally renders the structured `workspace.internal` unavailable state. The Wails shell build passed, but a full interactive desktop session requires a Windows desktop host.
4. The coordinator currently exposes refresh requests for observability and invokes the authoritative rebuild handler; watcher events remain hints. A future phase may persist watcher checkpoints or expose richer health history without changing this contract.
5. Retrieval is lexical and bounded by design. Semantic/vector retrieval is explicitly deferred until authoritative content, attribution, and deterministic fallback remain stable under a separate decision.

No unresolved critical build, security, license, dependency, or acceptance blocker remains for Phase 2. The residual items above are explicit, non-critical follow-ups.

## Phase 3 recommendation

**GO.** Preserve the Workspace boundary and typed bridge as the foundation. Phase 3 work must not move filesystem, SQL, Git process, watcher, or retrieval authority into React/Wails, and must not treat the smoke cockpit as a production execution surface. Project Lead approval and milestone closure should be recorded in Linear against BAN-55.
