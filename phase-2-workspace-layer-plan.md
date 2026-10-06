# Plan: Phase 2 — Workspace Layer

**Generated**: 2026-10-06  
**Estimated Complexity**: High

## Overview

Phase 1 is complete and the repository is at a clean, pushed checkpoint (`f53f6efc9c363ead03db48be37c417cb14634121`, also `origin/main`). The Phase 1 Go, frontend, dependency, Wails, and repository checks are implemented and passed individually; the aggregate PowerShell gate currently reaches the frontend test step but exceeds the 120-second command timeout in this harness, so that timeout must remain visible and the test/build commands should be rerun with a larger timeout before Phase 2 implementation is declared started.

Phase 2 introduces the first durable runtime layer: Workspace. It must remain independent from Wails, React, Linear, Orchestra, Harness, Staff, and engine adapters. The implementation should establish explicit contracts and ports first, then add safe local adapters for project/worktree registration, SQLite state, Markdown Vault notes, file watching, and rebuildable local retrieval. A vertical slice should be exposed through the existing application bridge only after the domain/application contracts are stable.

The phase is deliberately limited to Workspace facts and controlled mutations. It does not implement task orchestration, child-process execution, engine adapters, staff workflows, production cockpit UX, cloud sync, or Linear runtime integration.

## Phase outcome

A clean checkout can:

1. Register and query a project with normalized, policy-checked paths.
2. Create, inspect, and safely remove an isolated Git worktree through a controlled Workspace service.
3. Open and migrate a local SQLite database with versioned schema and transactional state for projects/worktrees.
4. Create, read, update, and list attributed Markdown Vault notes within the allowed workspace boundary.
5. Observe relevant filesystem changes with debounced, cancellable events and explicit overflow/error behavior.
6. Build and query a deterministic local retrieval index derived from Vault content, with a rebuild path.
7. Return typed Workspace summaries through the narrow Wails bridge without exposing arbitrary filesystem or shell access.
8. Pass unit, integration, race, frontend, Wails, security, path-boundary, migration, and clean-checkout validation gates.

## Prerequisites

- Phase 1 checkpoint is pushed and `HEAD == origin/main`.
- Phase 1 aggregate validation is rerun with sufficient timeout; no known Phase 1 error is silently waived.
- Linear Phase 2 milestone remains `Phase 2 — Workspace Layer` (`54eaf1b7-8a7e-4845-99a0-ae8992828503`).
- Accepted ADRs: local-first storage, Git worktree isolation, validation authority, and architecture boundaries.
- A decision is recorded before implementation for the SQLite driver, Git implementation strategy, watcher library, and retrieval indexing strategy, including licenses and removal/fallback plans.
- Test fixtures use temporary directories and disposable Git repositories; no repository under development is mutated by tests.

## Non-goals and explicit boundaries

- No Orchestra task graph, DAG scheduling, retries, inspection, or merge authority.
- No Harness tool broker, sandbox, command broker, worker pool, or engine process spawning.
- No Staff definitions, permissions, memory assignment, or engine routing.
- No Phaser or production cockpit features.
- No arbitrary frontend filesystem access, shell access, or path passthrough.
- No cloud synchronization, hosted database, Linear runtime client, or network prerequisite.
- No semantic/vector model dependency in the first retrieval slice; retrieval is local, deterministic, and rebuildable.
- No silent fallback for invalid paths, missing migrations, watcher overflow, corrupted index data, or Git ambiguity.

## Sprint 0: Phase entry, contracts, and dependency decisions

**Goal**: Freeze the Workspace contract and implementation choices before touching persistence or filesystem behavior.

**Demo/Validation**:
- The Phase 2 backlog is recorded in Linear with dependency order and acceptance criteria.
- A Workspace contract document and ADR updates identify ownership, invariants, error taxonomy, and forbidden imports.
- Dependency/license checks pass and no product code depends on Linear.

### Task 0.1: Verify Phase 1 exit evidence and clean starting point
- **Location**: `scripts/validate-repository.ps1`, `docs/development-workflow.md`, Linear Phase 1 issues/comments
- **Description**: Rerun the Phase 1 gates with an explicit extended timeout, capture exact results, confirm `git status`, `git diff --check`, `HEAD == origin/main`, ignored paths, and absence of tracked secrets/generated output. If the frontend test timeout reproduces, determine whether it is harness timeout or a real test hang before Phase 2 starts.
- **Dependencies**: None
- **Acceptance Criteria**:
  - Go tests, frontend typecheck/lint/test/build, Wails build, audit, and repository checks have explicit pass/fail evidence.
  - Any remaining platform caveat or timeout is documented as a blocker or accepted environment limitation, never hidden.
  - The Phase 2 implementation begins from the pushed Phase 1 checkpoint.
- **Validation**: `go test ./... -race -count=1`; `npm ci`; `npm run typecheck`; `npm run lint`; `npm test`; `npm run build`; `wails build -nopackage`; repository script with an extended command timeout.

### Task 0.2: Write Workspace contracts and error taxonomy
- **Location**: `docs/specs/workspace-contracts.md`, `internal/cortex/workspace/`
- **Description**: Define Project, Worktree, VaultNote, FileChange, RetrievalDocument, and WorkspaceSnapshot DTOs; normalized path rules; project/worktree ownership; mutation semantics; context cancellation; stable error categories; event ordering; attribution requirements; and adapter ports. Specify which operations are queries versus controlled mutations.
- **Dependencies**: Task 0.1
- **Acceptance Criteria**:
  - Contracts are Wails-independent and do not import React, platform, or Linear packages.
  - Every path-bearing input has a canonicalization and containment rule.
  - Every mutation has idempotency/rollback behavior and explicit failure semantics.
  - Retrieval indexes are documented as rebuildable derivatives, not authoritative state.
- **Validation**: Architecture review against `docs/architecture/overview.md` and ADR-0001/0002; `go list` import-boundary inspection; contract unit tests for normalization and error mapping.

### Task 0.3: Select and pin Phase 2 dependencies and adapter strategy
- **Location**: `go.mod`, `go.sum`, `docs/specs/phase-2-dependency-baseline.md`
- **Description**: Evaluate a SQLite driver, Git library or narrowly wrapped Git binary strategy, filesystem watcher, and retrieval/indexing implementation. Prefer the smallest mature dependencies compatible with the supported Go/Windows baseline. Record versions, licenses, transitive risk, operational caveats, and removal/fallback paths before adoption.
- **Dependencies**: Task 0.2
- **Acceptance Criteria**:
  - Direct dependencies have a documented reason and license evidence.
  - The chosen Git strategy cannot become arbitrary command execution from UI input.
  - SQLite migration and concurrency behavior are compatible with local desktop use.
  - Retrieval has deterministic behavior and no mandatory network/model service.
- **Validation**: `go mod tidy`; dependency/license audit; minimal compile smoke tests; review against `docs/specs/dependency-policy.md`.

## Sprint 1: Workspace domain contracts and path policy

**Goal**: Build the Wails-independent domain model and policy layer that all adapters must obey.

**Demo/Validation**:
- Unit tests prove path normalization, containment, project/worktree identity, and error behavior on Windows-style and relative inputs.
- The package compiles without importing `internal/platform`, Wails, or frontend code.

### Task 1.1: Add Workspace entities and stable identifiers
- **Location**: `internal/cortex/workspace/model.go`, `internal/cortex/workspace/model_test.go`
- **Description**: Define immutable identity/value types for projects, worktrees, notes, file changes, retrieval documents, and workspace snapshots. Include schema/version fields where data crosses an adapter boundary.
- **Dependencies**: 0.2
- **Acceptance Criteria**:
  - IDs are generated/validated through one policy, not ad hoc strings.
  - DTOs are serializable without leaking OS-specific implementation state.
  - Zero values and invalid states are either rejected or explicitly represented.
- **Validation**: Table-driven unit tests and JSON round-trip tests.

### Task 1.2: Implement canonical path and containment policy
- **Location**: `internal/cortex/workspace/paths.go`, `internal/cortex/workspace/paths_test.go`
- **Description**: Canonicalize absolute roots, reject traversal and unsafe reparse/symlink escapes according to the supported platform policy, and provide explicit checks for project root, worktree root, Vault root, and note-relative paths.
- **Dependencies**: 1.1
- **Acceptance Criteria**:
  - Relative paths, `..`, mixed separators, missing roots, and outside-root paths have deterministic errors.
  - Canonical checks are performed before filesystem mutation.
  - Tests do not depend on the developer's real home or repository.
- **Validation**: Windows-focused unit tests plus temporary-directory integration tests for inside/outside/symlink cases where supported.

### Task 1.3: Add Workspace service ports and in-memory reference implementation
- **Location**: `internal/cortex/workspace/ports.go`, `internal/cortex/workspace/memory.go`, tests
- **Description**: Define narrow ports for project registry, worktree manager, state store, Vault store, watcher, and retrieval index. Add an in-memory implementation for contract tests, not as a production persistence fallback.
- **Dependencies**: 1.1, 1.2
- **Acceptance Criteria**:
  - Ports express controlled operations and context cancellation.
  - Ports do not expose raw SQL connections, `exec.Command`, or unrestricted filesystem handles.
  - Contract tests can run against memory and later adapters.
- **Validation**: Interface compile checks and shared contract test suite.

## Sprint 2: SQLite state and project/worktree registry

**Goal**: Persist authoritative Workspace metadata locally and safely manage Git worktree isolation.

**Demo/Validation**:
- A disposable repository can be registered, queried after reopening the database, assigned a worktree, and cleaned up with dirty-state protection.
- Migration tests prove fresh install, upgrade, rollback refusal, and concurrent open behavior.

### Task 2.1: Create versioned SQLite schema and migration runner
- **Location**: `internal/cortex/workspace/sqlite/`, `migrations/`, `docs/specs/workspace-storage.md`
- **Description**: Add schema metadata, migration ordering, transaction boundaries, busy-timeout/journal configuration, and a migration runner for projects, worktrees, notes metadata, watcher checkpoints, and retrieval documents. Keep migrations embedded/versioned and fail closed on unknown or partial states.
- **Dependencies**: 0.3, 1.3
- **Acceptance Criteria**:
  - Fresh database creates the complete schema deterministically.
  - Reopening is safe; migrations are applied exactly once and are recorded.
  - Failed migrations do not leave an apparently healthy partial schema.
  - Database paths are policy-checked and excluded from Git.
- **Validation**: Migration integration tests, interrupted-transaction test, concurrent-reader test, `go test -race`.

### Task 2.2: Implement project registration and query repository
- **Location**: `internal/cortex/workspace/sqlite/projects.go`, repository tests
- **Description**: Persist normalized project identity, repository root, Vault root, default branch metadata, timestamps, and status. Enforce uniqueness and transactional updates.
- **Dependencies**: 2.1
- **Acceptance Criteria**:
  - Duplicate registration is deterministic and does not silently overwrite.
  - Returned paths are canonical and policy-approved.
  - Query results are stable and do not expose secrets or machine-local environment values.
- **Validation**: CRUD, uniqueness, reopen, cancellation, and corrupted-record tests.

### Task 2.3: Implement controlled Git worktree lifecycle
- **Location**: `internal/cortex/workspace/worktree.go`, `internal/cortex/workspace/git/`, tests, ADR update if needed
- **Description**: Add inspect/create/remove/list operations behind a narrow Git adapter. Validate repository and worktree roots, branch naming, existing changes, detached state, cleanup safety, and non-Git fallback behavior. Never accept arbitrary command/path arguments from the bridge.
- **Dependencies**: 1.2, 2.2, ADR-0002
- **Acceptance Criteria**:
  - Worktrees are isolated under a policy-controlled root and linked to a registered project.
  - Existing dirty changes or active references block destructive cleanup with a typed error.
  - Git output is parsed into typed facts; raw process output is not a success verdict.
  - Non-Git repositories fail explicitly with documented behavior.
- **Validation**: Disposable Git repository integration tests for create/list/inspect/remove, dirty worktree protection, duplicate branch/path, cancellation, and failure injection.

### Task 2.4: Add Workspace application service and snapshot query
- **Location**: `internal/cortex/workspace/service.go`, tests
- **Description**: Compose path policy, state store, project registry, and worktree manager into a Wails-independent application service exposing read-only snapshot queries and controlled project/worktree commands.
- **Dependencies**: 2.2, 2.3
- **Acceptance Criteria**:
  - Service orchestration does not know Wails lifecycle or React DTO generation.
  - Queries are read-only and mutations are explicit commands.
  - Context cancellation and typed errors propagate consistently.
- **Validation**: Service tests against memory and SQLite/Git adapters.

## Sprint 3: Markdown Vault and durable attribution

**Goal**: Make Markdown Vault notes a safe, human-readable source of durable workspace knowledge.

**Demo/Validation**:
- A note can be created, read, updated, listed, and deleted only within the configured Vault root; metadata is persisted and attribution is visible.
- Malformed front matter, path traversal, duplicate IDs, and concurrent edits fail explicitly.

### Task 3.1: Define note format and attribution contract
- **Location**: `docs/specs/markdown-vault-contract.md`, `internal/cortex/workspace/vault/model.go`
- **Description**: Specify front matter fields, note ID, title, project/worktree scope, source/author attribution, created/updated timestamps, content hash, and format version. Define safe filename/slug rules and conflict behavior.
- **Dependencies**: 0.2, 2.1
- **Acceptance Criteria**:
  - Every durable note has attribution and stable identity.
  - Unknown/invalid metadata is not silently discarded.
  - The Markdown body remains human-editable.
- **Validation**: Fixture tests for valid, missing, unknown, malformed, and versioned front matter.

### Task 3.2: Implement Vault filesystem adapter
- **Location**: `internal/cortex/workspace/vault/`, tests
- **Description**: Implement atomic write/read/update/list/delete with temporary-file + rename semantics, containment checks, conflict detection using content hashes, and metadata synchronization with SQLite.
- **Dependencies**: 3.1, 2.1, 1.2
- **Acceptance Criteria**:
  - Writes cannot escape the Vault root.
  - Partial writes are not presented as valid notes.
  - Concurrent external edits produce a typed conflict rather than silent overwrite.
  - Delete behavior is explicit and recoverable through the documented policy.
- **Validation**: Temporary filesystem tests, crash/rename simulation where practical, hash conflict tests, and permission/error tests.

### Task 3.3: Add Vault service queries and mutations
- **Location**: `internal/cortex/workspace/vault/service.go`, tests
- **Description**: Expose project-scoped note operations through Workspace ports, including list filtering, deterministic ordering, content hashing, and explicit attribution from the caller context.
- **Dependencies**: 3.2
- **Acceptance Criteria**:
  - No raw filesystem handles or unrestricted paths leave the service.
  - Listing is deterministic and bounded.
  - Note operations are cancellable and testable without Wails.
- **Validation**: Contract tests against memory and filesystem adapters.

## Sprint 4: File watching and local retrieval

**Goal**: Maintain safe change awareness and a deterministic rebuildable retrieval index over Vault content.

**Demo/Validation**:
- A file change under an approved root produces a debounced typed event; changes outside the root are ignored or rejected according to policy.
- Rebuilding the index from the same Vault produces the same documents and query ordering.

### Task 4.1: Implement policy-scoped file watcher
- **Location**: `internal/cortex/workspace/watch/`, tests
- **Description**: Add start/stop lifecycle, root containment, debounce/coalescing, rename/delete handling, cancellation, bounded event delivery, and overflow/error signaling. Watch events must trigger re-ingestion requests, not mutate authoritative state directly.
- **Dependencies**: 1.2, 3.2, 0.3
- **Acceptance Criteria**:
  - Watchers never observe outside the configured root.
  - Stop is idempotent and does not leak goroutines.
  - Overflow, watcher failure, and unsupported filesystem behavior are explicit.
  - Event ordering/coalescing semantics are documented and tested.
- **Validation**: Temporary-directory integration tests, race tests, cancellation/leak checks, and failure injection.

### Task 4.2: Define deterministic retrieval document and index contract
- **Location**: `docs/specs/retrieval-contract.md`, `internal/cortex/workspace/retrieval/`
- **Description**: Define document IDs, source hashes, tokenizer/normalization rules, index version, rebuild semantics, query limits, ranking tie-breakers, and stale-document behavior. Keep the first implementation local and model-free.
- **Dependencies**: 3.1, 0.3
- **Acceptance Criteria**:
  - Same source corpus and index version produce stable output.
  - Index entries retain source attribution and content hashes.
  - Stale or corrupt indexes can be discarded and rebuilt without data loss.
- **Validation**: Golden corpus tests and deterministic hash/order assertions.

### Task 4.3: Implement rebuildable local retrieval index
- **Location**: `internal/cortex/workspace/retrieval/`, tests
- **Description**: Build/update/query/rebuild operations over Vault documents using the selected deterministic strategy. Persist only rebuildable derivatives and keep authoritative note content in Markdown plus metadata state.
- **Dependencies**: 4.2, 3.3
- **Acceptance Criteria**:
  - Query results are bounded, deterministic, attributable, and cancellable.
  - Rebuild repairs missing/corrupt index state.
  - Index updates do not silently delete authoritative notes.
  - No network or provider key is required.
- **Validation**: Golden tests, corruption recovery, incremental update/delete, cancellation, and race tests.

### Task 4.4: Connect watcher events to retrieval refresh
- **Location**: `internal/cortex/workspace/service.go`, `watch/`, `retrieval/`, tests
- **Description**: Add an explicit coordinator that translates approved Vault change events into debounced index refresh work. Keep refresh observable and failure-reported; do not make the watcher claim that indexing succeeded.
- **Dependencies**: 4.1, 4.3
- **Acceptance Criteria**:
  - A watcher event yields a refresh request with source path/hash, not an execution success verdict.
  - Failed refreshes remain visible and retry/rebuild is explicit.
  - Shutdown drains or cancels work according to documented policy.
- **Validation**: End-to-end temporary Vault test with create/edit/delete/rename and induced index failure.

## Sprint 5: Wails bridge vertical slice and frontend workspace smoke UI

**Goal**: Prove the Workspace layer through the existing application boundary without widening frontend authority.

**Demo/Validation**:
- Wails opens a registered-project/worktree/Vault/retrieval summary using typed bridge DTOs; frontend shows loading/success/error states; no direct filesystem or shell APIs are available to React.

### Task 5.1: Add typed Workspace bridge DTOs and adapter methods
- **Location**: `internal/platform/bridge.go`, `internal/cortex/application/`, `web/src/types/`, `web/src/bridge/`
- **Description**: Extend the narrow bridge with versioned Workspace snapshot/query commands and explicit mutation request DTOs. Map typed domain errors to stable frontend-safe error codes/messages without leaking local paths, secrets, or process output.
- **Dependencies**: 2.4, 3.3, 4.3
- **Acceptance Criteria**:
  - Wails bridge remains a thin adapter.
  - Frontend types match Go DTOs and include loading/error states.
  - No arbitrary path, shell, or raw database operation is exposed.
- **Validation**: Go bridge tests, TypeScript typecheck, frontend bridge tests, and import-boundary review.

### Task 5.2: Build a minimal Workspace smoke cockpit
- **Location**: `web/src/App.tsx`, `web/src/components/`, tests
- **Description**: Replace/extend the Phase 1 smoke content with a small functional view of project/worktree status, Vault note counts, watcher/index state, and explicit error states. Keep presentation separate from Workspace domain logic.
- **Dependencies**: 5.1
- **Acceptance Criteria**:
  - UI renders deterministic empty/loading/success/error states.
  - No final cockpit or Phaser work is introduced.
  - Accessibility and responsive behavior are adequate for the smoke flow.
- **Validation**: Vitest component tests, `npm run typecheck`, `npm run lint`, `npm run build`, Wails build.

### Task 5.3: Add controlled mutation smoke flow
- **Location**: `internal/cortex/application/`, `internal/platform/`, `web/src/bridge/`, `web/src/components/`
- **Description**: Add one safe demo command such as registering a project from a platform-selected/validated location or creating a Vault note from structured fields. Do not expose arbitrary file writes or path passthrough.
- **Dependencies**: 5.2
- **Acceptance Criteria**:
  - Mutation uses validated, structured input and returns typed result/error.
  - Duplicate, cancellation, and invalid-input behavior is visible.
  - Tests prove the frontend cannot invoke unrestricted filesystem behavior.
- **Validation**: Service/bridge tests, frontend interaction tests, path boundary tests, and manual Wails smoke run.

## Sprint 6: Integration, security, and Phase 2 exit

**Goal**: Make the Workspace layer reproducible, reviewable, and ready for Orchestra consumers.

**Demo/Validation**:
- A clean checkout runs the complete validation suite and demonstrates the Workspace vertical slice against disposable local state.
- Phase 2 issues have evidence, review comments, and no unresolved critical blocker.

### Task 6.1: Add end-to-end disposable Workspace fixture harness
- **Location**: `internal/cortex/workspace/testutil/`, `test/`, `scripts/validate-repository.ps1`
- **Description**: Create disposable SQLite/Vault/Git fixtures and a deterministic scenario runner for register → worktree → note → watch → index → query → reopen. Ensure fixtures cannot point at the real repository and are cleaned up after failure.
- **Dependencies**: 2.4, 3.3, 4.4, 5.3
- **Acceptance Criteria**:
  - Scenario passes repeatedly from a clean checkout.
  - Failure cleanup is bounded and does not remove non-fixture paths.
  - Tests can run offline and without Linear or provider credentials.
- **Validation**: `go test ./... -race -count=1`; repeated scenario runs; Windows path and permission coverage.

### Task 6.2: Add Workspace security and dependency gates
- **Location**: `scripts/validate-repository.ps1`, `docs/specs/phase-2-dependency-baseline.md`, `docs/specs/workspace-security.md`
- **Description**: Extend validation for path traversal, symlink/reparse behavior, database permissions, secret redaction, dependency/license evidence, generated artifact exclusion, and forbidden imports. Add explicit checks for unsafe command invocation and tracked local state.
- **Dependencies**: 2.1, 2.3, 4.3, 5.1
- **Acceptance Criteria**:
  - Unsafe path and secret cases fail closed.
  - Validation output identifies the exact failed gate.
  - No production gate requires Linear or network access.
- **Validation**: Positive and negative script tests; `git check-ignore`; secret scan; dependency audit; Go/frontend checks.

### Task 6.3: Complete Phase 2 documentation and review record
- **Location**: `docs/architecture/overview.md`, `docs/adr/`, `docs/specs/`, Linear milestone/issues
- **Description**: Update architecture/ADRs with implemented Workspace ownership, migration policy, watcher/index semantics, fallback behavior, known platform caveats, and evidence links. Record review and go/no-go for Phase 3.
- **Dependencies**: 6.1, 6.2
- **Acceptance Criteria**:
  - Docs agree with code and no Phase 2 scope silently crosses into Orchestra/Harness.
  - Every completed issue includes exact validation evidence and residual risks.
  - Phase 2 milestone is not marked complete while blockers or unexplained test failures remain.
- **Validation**: Documentation link/scope review, diff review, full validation suite, Linear review comment, Project Lead approval.

## Linear issue decomposition for sub-agents

Create the following issue sequence in Linear under `Phase 2 — Workspace Layer` before assigning implementation work. Each issue should include the corresponding acceptance criteria and dependencies above. Use one issue per cohesive checkpoint; sub-agents should not modify another issue's scope without PM approval.

1. **Workspace Phase 2 entry evidence and contract freeze** — Tasks 0.1–0.2; Architecture/Documentation.
2. **Pin Phase 2 persistence, Git, watcher, and retrieval dependencies** — Task 0.3; Infrastructure/Architecture.
3. **Implement Workspace entities, path policy, and contract-test ports** — Tasks 1.1–1.3; Feature/Architecture.
4. **Implement SQLite migrations and project registry** — Tasks 2.1–2.2; Feature/Infrastructure.
5. **Implement controlled Git worktree lifecycle** — Task 2.3; Feature/Security.
6. **Compose Workspace application service and snapshot query** — Task 2.4; Feature.
7. **Define Markdown Vault format and attribution contract** — Task 3.1; Architecture/Documentation.
8. **Implement Markdown Vault adapter and service** — Tasks 3.2–3.3; Feature/Security.
9. **Implement policy-scoped file watching** — Task 4.1; Feature/Security.
10. **Define and implement deterministic local retrieval** — Tasks 4.2–4.3; Feature/Architecture.
11. **Connect file watching to retrieval refresh** — Task 4.4; Feature.
12. **Expose Workspace through typed Wails bridge** — Task 5.1; Feature/Architecture.
13. **Build Workspace smoke cockpit and controlled mutation flow** — Tasks 5.2–5.3; Feature.
14. **Add end-to-end Workspace fixture and full quality gates** — Tasks 6.1–6.2; Infrastructure/Security.
15. **Complete Phase 2 review and approve Phase 3 entry** — Task 6.3; Documentation/Architecture.

## Recommended sub-agent assignment model

- **Architecture/documentation agent**: issues 1, 2, 7, and 15. Produces contracts/ADRs first; cannot approve its own implementation.
- **Workspace domain agent**: issue 3. Owns pure Go entities, path policy, ports, and contract tests.
- **Persistence agent**: issue 4. Owns SQLite migrations and project metadata only.
- **Git isolation/security agent**: issue 5. Owns worktree lifecycle and destructive-operation safeguards.
- **Application composition agent**: issue 6. Composes ports; must not move adapter logic into Wails.
- **Vault agent**: issue 8. Owns Markdown I/O, attribution, atomic writes, and conflict handling.
- **Watcher/retrieval agents**: issues 9–11. Coordinate through the documented event/index contracts; no direct mutation of authoritative state from watcher callbacks.
- **Bridge/UI agent**: issues 12–13. Starts only after service contracts are stable; no domain logic in React.
- **Quality/security agent**: issue 14. Runs negative tests and repository gates; reports failures instead of weakening checks.
- **PM/Inspector**: owns issue dependencies, integration to `main`, final diff review, and Linear evidence. No sub-agent marks its own issue Done.

## Testing strategy

- **Unit**: path normalization, IDs, DTOs, front matter, ranking, error mapping, migration definitions.
- **Contract**: identical Workspace behavior through in-memory, SQLite, Vault, watcher, and retrieval adapters where applicable.
- **Integration**: disposable Git repositories, SQLite reopen/migration, Vault atomic writes, watcher events, index rebuild/query.
- **End-to-end**: Workspace service through Wails bridge and frontend loading/success/error/mutation states.
- **Concurrency**: `go test -race ./...`, cancellation, watcher shutdown, SQLite concurrent access, index refresh coalescing.
- **Negative/security**: traversal, symlink/reparse escape, dirty worktree cleanup, malformed note metadata, corrupt index, permission failures, secret/path leakage, unsupported non-Git repository.
- **Reproducibility**: offline clean checkout, repeated deterministic retrieval results, no Linear/provider credential/network requirement.

## Potential risks & gotchas

- **SQLite driver choice can determine portability and CGO requirements.** Record whether the selected driver works with the Wails Windows build and CI environment before implementation.
- **Git worktree cleanup can destroy uncommitted work.** Require explicit dirty-state inspection and fail closed by default; never infer safety from a successful process exit alone.
- **Windows path and reparse-point behavior is subtle.** Test canonical containment and symlink/junction cases on the target platform; do not rely solely on string prefix checks.
- **Filesystem watcher events are lossy.** Treat events as invalidation hints; provide rescan/rebuild behavior and surface overflow/error states.
- **Markdown files can be edited externally.** Use hashes/versions to prevent silent overwrite and preserve attribution.
- **Retrieval indexes are derivatives.** Never make the index the source of truth; corruption must be recoverable by rebuild.
- **Frontend bridge expansion can accidentally become an arbitrary filesystem API.** Expose structured project/worktree/note commands only, with stable error codes and redacted paths.
- **Phase 1 aggregate validation timed out at 120 seconds in this harness.** Rerun with a larger timeout and inspect for a real Vitest hang before claiming a clean phase boundary.
- **A local `ANTHROPIC_AUTH_TOKEN` was visible in prior verbose build output.** It was not committed; rotate/revoke it before any future diagnostic capture and keep secrets outside logs.

## Rollback plan

- Keep each issue atomic and commit-scoped; revert the issue commit rather than resetting shared history.
- If a dependency introduces unacceptable licensing, CGO, or portability risk, revert its baseline issue and replace the adapter behind the already-defined port.
- If SQLite schema changes are unsafe, stop migration rollout, preserve the previous database, add a forward repair/rebuild migration, and never delete user state as an implicit rollback.
- If Git worktree cleanup or path policy is unsafe, disable the mutation command at the application boundary and retain read-only project inspection until repaired.
- If watcher/retrieval behavior is unstable, ship the authoritative Vault/SQLite paths without automatic indexing, expose rebuild explicitly, and do not present stale retrieval as current.
- If the Workspace bridge is not ready, keep the Phase 1 runtime snapshot bridge and defer UI integration; do not leak adapter APIs into Wails.
