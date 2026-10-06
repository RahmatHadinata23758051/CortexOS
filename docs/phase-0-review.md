# Phase 0 Review — Foundation and Repository Readiness

**Linear:** BAN-31
**Project:** CortexOS
**Review date:** 2026-10-06
**Decision:** Ready for Project Lead approval to enter Phase 1

## Completed checkpoints

| Issue | Checkpoint | Evidence | Status |
| --- | --- | --- | --- |
| BAN-24 | Repository hygiene and ignore policy | `.gitignore`, ignore checks, diff checks | Done |
| BAN-32 | Multi-agent delivery operating model | `RULES.md`, Linear operating-model comment | Done |
| BAN-26 | Harvest provenance and license audit | `docs/harvest-audit.md`, `_harvest/harvest-manifest.json`, pinned SHAs | Done |
| BAN-27 | Architecture boundaries and ADRs | `docs/architecture/`, `docs/adr/` | Done |
| BAN-28 | Go/Wails/frontend foundation contracts | `docs/specs/foundation-contracts.md`, `docs/specs/dependency-policy.md` | Done |
| BAN-29 | Configuration and secrets policy | `docs/specs/configuration-secrets-policy.md`, `configs/runtime.example.yaml` | Done |
| BAN-30 | Validation and developer workflow | `docs/development-workflow.md`, `scripts/validate-repository.ps1` | Done |

## Gate validation

The currently executable repository gate passed:

```text
& .\scripts\validate-repository.ps1
Repository validation passed for currently available gates.
Go/frontend/build gates remain pending until Phase 1 manifests exist.
```

Additional checks:

- `git diff --check` passed.
- `.mcp.json` is ignored.
- `_harvest/raw/` is ignored.
- `_harvest/harvest-manifest.json` is explicitly trackable.
- Safe example configuration is trackable; real environment and credential files remain ignored.
- Raw third-party source was not committed.
- No known plaintext credential was introduced by Phase 0 work.
- Architecture docs identify Inspector/Orchestra as the only success authority.
- Logical Staff remain decoupled from CLI process count.

## Accepted risks and blockers

1. The repository has not yet received Go/Wails/frontend manifests. Those are Phase 1 deliverables, so Go/frontend/build checks are not claimed as passing.
2. `agent-teams-ai` is AGPL-3.0 and remains blocked from adoption pending legal/product approval.
3. `opencode-harness` has no repository-wide declared license in GitHub metadata and remains blocked from adoption.
4. Nested package, dependency, font, audio, tileset, and asset licenses require review before any material is promoted from reference-only harvest.
5. The current Linear team has no `In Review` status. Implementation-complete issues were kept in `In Progress` until Project Lead approval, then moved to `Done`.
6. No commit has been pushed yet. Local atomic commits are present; remote publication remains a separate controlled action.

These are either explicitly accepted Phase 0 boundaries or deliberate adoption blockers. No critical security, licensing, or repository-hygiene blocker prevents Phase 1 from starting, provided blocked sources are not adopted and Phase 1 creates the missing executable manifests before claiming build readiness.

## Phase 1 entry recommendation

**GO**, with these conditions:

- Start Phase 1 from a new Linear issue/branch and keep implementation scoped to the Go + Wails + frontend smoke application.
- Do not add blocked or unreviewed harvested source as a dependency.
- Create `go.mod`, frontend package/lock manifests, and executable build/test commands before feature work.
- Preserve the current architecture boundaries and configuration/secrets policy.
- Continue atomic checkpoint commits and record validation evidence in Linear.
