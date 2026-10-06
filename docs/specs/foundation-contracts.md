# CortexOS Foundation Contracts

**Issue:** BAN-28
**Status:** Phase 0 contract
**Purpose:** Define the smallest reproducible Go + Wails + React/TypeScript/Phaser foundation without implementing product features.

## Contract principles

1. The repository module is `github.com/RahmatHadinata23758051/CortexOS`.
2. Go owns the desktop application entry point and domain/application services.
3. Wails is the desktop bridge and lifecycle host; UI code does not reach the filesystem directly.
4. React/TypeScript owns presentation and typed bridge calls.
5. Phaser is an optional presentation subsystem for the virtual-office scene, not a dependency of core task execution.
6. Workspace, Orchestra, Harness, Staff, and Engine boundaries remain independent of Wails bindings.
7. Dependencies must be pinned through committed lock/module files and reviewed for license compatibility.

## Planned repository entry points

```text
cmd/cortexos/main.go          # Wails application entry point
internal/                     # non-exported runtime packages
  platform/                   # OS/Wails boundary adapters
  shared/                     # narrow shared primitives only
  cortex/workspace/            # Workspace layer
  cortex/orchestra/            # Orchestra layer
  cortex/harness/              # Harness layer
  cortex/staff/                # Staff definitions and policy
  engine/native/               # deterministic native adapters
  engine/pi/                   # Pi adapter
  engine/omp/                  # OMP adapter
web/                           # React/TypeScript frontend
  src/components/              # reusable UI
  src/features/                # product slices
  src/engine/                  # Phaser presentation only
  src/types/                   # bridge/domain types
configs/                       # safe defaults and schemas
scripts/                       # reproducible developer commands
test/                          # integration, E2E, fixtures
docs/                          # architecture, ADR, specs
```

The current scaffold contains placeholders only. `cmd/cortexos`, `internal/cortex`, and runtime implementations are Phase 1 scope; this issue documents their contract rather than creating an empty executable.

## Go contract

- `go.mod` is the source of truth for the Go module and direct versions.
- `cmd/cortexos/main.go` will construct platform bindings and start Wails.
- Domain packages must not import Wails or frontend packages.
- Platform bindings expose application commands/queries through explicit DTOs; they do not expose arbitrary filesystem or shell access.
- Go tests use `go test ./...`; formatting uses `gofmt`; static checks use the selected project linter after it is chosen in Phase 1.

### Proposed application boundary

```go
type AppService interface {
    GetRuntimeSnapshot(ctx context.Context) (RuntimeSnapshot, error)
    SubmitPlan(ctx context.Context, input PlanInput) (PlanID, error)
}
```

This is a contract sketch, not an implementation. Concrete DTOs and error taxonomy belong to the Phase 1 vertical slice.

## Wails contract

- Wails bindings are a thin platform adapter around application services.
- Binding methods accept validated input and return serializable DTOs/errors.
- Wails lifecycle owns startup/shutdown hooks only; it does not own scheduling, persistence, or engine process policy.
- Development uses the Wails CLI; production packaging uses the Wails build command for the target platform.
- A smoke app must prove: application starts, frontend loads, one typed bridge call returns a deterministic response, and shutdown is clean.

## Frontend contract

- TypeScript is strict; generated or hand-maintained bridge types must be versioned and reviewed.
- React components call a typed application client, not Wails runtime globals scattered through the UI.
- Phaser is lazy/feature-scoped and must not block the core cockpit from rendering.
- Frontend commands are expected to be:

```text
npm ci
npm run dev
npm run build
npm run lint
npm run typecheck
npm test
```

The exact package manager and versions are selected in the Phase 1 implementation issue and committed with `package.json`/lockfile. No frontend dependency is added only to populate the scaffold.

## Dependency and license policy

- Prefer standard library and existing platform capabilities when they satisfy the contract.
- Every direct dependency requires a reason, pinned version, transitive review plan, and license record.
- MIT/BSD/Apache-compatible dependencies require retained notices where applicable.
- AGPL/GPL or unclear-license dependencies require explicit Project Lead/legal approval before adoption.
- Harvested repositories are reference material and are not dependencies.
- No provider SDK or Linear MCP client is a runtime dependency of the smoke application.

## Reproducible validation commands

Until Phase 1 creates manifests, the commands below are the target contract:

```powershell
# Repository/documentation checks
& .\scripts\harvest.ps1 -SkipClone

git diff --check

git status --short --ignored

# Go foundation (after go.mod exists)
go test ./...
gofmt -l .

# Frontend foundation (after package.json exists)
npm ci
npm run typecheck
npm run lint
npm test -- --runInBand
npm run build
```

A command that is not applicable because its manifest does not exist is a scaffold gap, not a passing test. Phase 1 must replace the placeholders with executable scripts and record tool versions.

## Phase 1 smoke application acceptance

The first vertical slice should include only:

1. Wails window/application startup.
2. React shell with a stable root route.
3. One typed Go-to-frontend snapshot call.
4. A deterministic health/status response.
5. Clean shutdown and a test that exercises the application service without a live LLM or network.

Out of scope for this contract: project CRUD, worktree mutations, real engine workers, task scheduling, Phaser office simulation, provider authentication, and production packaging.
