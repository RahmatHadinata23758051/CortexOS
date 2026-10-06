# Phase 2 Dependency Baseline

**Issue:** BAN-42  
**Status:** Proposed and implementation-pinned  
**Scope:** Workspace persistence, file observation, Git isolation, and local retrieval

## Selection summary

Phase 2 keeps the Workspace core behind narrow internal ports. Only persistence and filesystem observation require direct external Go modules. Git and retrieval use constrained internal adapters so they do not introduce a large runtime dependency surface.

| Capability | Selection | Version | Owner | License | Rationale |
| --- | --- | --- | --- | --- | --- |
| SQLite | `modernc.org/sqlite` | `v1.59.0` | Workspace | BSD-3-Clause; bundled SQLite public domain and third-party notices | Pure-Go `database/sql` driver; avoids CGO for the Windows Wails baseline. |
| File watching | `github.com/fsnotify/fsnotify` | `v1.10.1` | Workspace | BSD-3-Clause | Mature cross-platform OS notification API with Windows `ReadDirectoryChangesW` support. |
| Git | Go standard library plus a constrained process adapter | N/A | Workspace | Go standard library; Git remains an external user-installed tool | Keeps Git behavior behind `WorktreeManager`; avoids exposing arbitrary command execution or coupling the domain to a Git library. |
| Retrieval | Go standard library deterministic index | N/A | Workspace | Go standard library | First retrieval slice is local, model-free, deterministic, bounded, and rebuildable. |

The exact module versions are recorded in `go.mod` and `go.sum`. No provider SDK, Linear client, vector database, semantic model, or network service is part of this baseline.

## Compatibility evidence

- Repository toolchain: Go 1.25.0 minimum; local validation uses Go 1.26.3 on Windows/amd64.
- `modernc.org/sqlite@v1.59.0` is pure Go/no-CGO and supports the Windows/amd64 validation path. Its upstream package metadata identifies BSD-3-Clause licensing and SQLite public-domain licensing.
- `github.com/fsnotify/fsnotify@v1.10.1` requires Go 1.23 or newer and documents Windows support through `ReadDirectoryChangesW`.
- CGO remains disabled for the supported local validation path. A future CGO-backed driver is a separate decision and cannot be introduced as an incidental replacement.
- Wails remains isolated in `internal/platform`; neither selected Workspace dependency is imported by frontend or Wails binding code directly.

## Dependency and transitive risk

### `modernc.org/sqlite v1.59.0`

- Direct purpose: local SQLite persistence through `database/sql`.
- License: BSD-3-Clause for the Go module; upstream documents SQLite as public domain and includes third-party license/SBOM material.
- Risk: the module has a substantial generated pure-Go SQLite implementation and transitive `modernc.org/*` dependencies. It may be slower than native SQLite for CPU-heavy workloads and requires careful database pool/concurrency configuration.
- Controls: isolate imports in `internal/cortex/workspace/sqlite`; set bounded `database/sql` pool limits; use migrations and transactions; run Windows/amd64 tests; record the module graph and audit output.
- Removal path: `StateStore` and migration ports remain internal. A future driver can implement the same ports only after a separate compatibility, license, CGO, and migration review.

### `github.com/fsnotify/fsnotify v1.10.1`

- Direct purpose: OS file-change notifications for approved Workspace roots.
- License: BSD-3-Clause.
- Risk: native notification systems can duplicate, reorder, or lose events; recursive watching is not automatic; network filesystems may not provide reliable notifications.
- Controls: watch directories, not individual files; debounce/coalesce; bound channels; surface overflow/errors; rescan authoritative Vault content; make stop idempotent; never treat an event as a successful write or index refresh.
- Removal path: `FileWatcher` is an internal port. A polling or platform-specific implementation can replace fsnotify without changing Workspace contracts.

### Git process adapter

- Direct dependency: none.
- Risk: Git availability, version differences, path quoting, output changes, dirty worktrees, and destructive commands.
- Controls: use a fixed executable name discovered through platform policy; pass argument arrays, never shell strings; construct arguments only from validated typed values; allow only a finite operation set; validate repository/worktree roots before every operation; parse structured output; inspect dirty state before cleanup; redact output before errors.
- Removal path: the `WorktreeManager` port permits a future library adapter, but no arbitrary process API may cross the Workspace/application or Wails boundary.

### Deterministic retrieval

- Direct dependency: none.
- Risk: naive tokenization/ranking can produce poor relevance and index size can grow with Vault content.
- Controls: version normalization/tokenization; bound query and document sizes; stable tie-breakers; persist source IDs/hashes; keep the index rebuildable; measure before adding a third-party search engine.
- Removal path: `RetrievalIndex` is an internal port. A future engine must preserve source attribution, versioning, deterministic fallback, and rebuild semantics.

## License and provenance sources

- SQLite Go driver: <https://pkg.go.dev/modernc.org/sqlite@v1.59.0>
- SQLite source/license: <https://gitlab.com/cznic/sqlite/-/tree/v1.59.0>
- fsnotify package: <https://pkg.go.dev/github.com/fsnotify/fsnotify@v1.10.1>
- fsnotify repository/license: <https://github.com/fsnotify/fsnotify/tree/v1.10.1>
- Go standard library: <https://go.dev/LICENSE>
- Dependency policy: `docs/specs/dependency-policy.md`

The distribution/release process must preserve upstream notices required by the selected modules. No harvested source is used by this baseline.

## Locking and validation

Dependency changes are separate atomic commits from feature implementation. Before each dependency commit:

```powershell
go mod tidy
go list -m all
go test ./...
go vet ./...
git diff --check
git status --short --ignored
```

The Phase 2 repository gate additionally runs frontend checks, Wails build, secret review, tracked-artifact review, and dependency audit. All validation must work offline after dependencies are present locally and must not require Linear, provider keys, or a hosted service.

## Rejected alternatives

- **CGO-backed SQLite as the default:** rejected for the initial Windows Wails baseline because it adds compiler/runtime packaging risk and weakens reproducibility.
- **Git library as a first dependency:** deferred because the required operation surface is narrow and Git worktree safety depends more on policy and inspection than on a broad API. Reconsider only behind `WorktreeManager`.
- **Semantic/vector retrieval package:** rejected for Phase 2 because it adds model/index portability and nondeterminism before the authoritative Vault and deterministic retrieval contracts are proven.
- **Network/cloud persistence:** rejected by ADR-0001; local execution must remain useful offline.

## Acceptance evidence

- [x] Exact direct module versions and purposes are recorded.
- [x] License and provenance sources are recorded.
- [x] Windows/Go/CGO compatibility assumptions are explicit.
- [x] Git cannot become an arbitrary command bridge.
- [x] Retrieval has no network or provider-key requirement.
- [x] Direct Go modules are pinned in `go.mod`/`go.sum` and pass the repository validation workflow; the environment remains explicit about Windows race-test and offline-package-install caveats.
