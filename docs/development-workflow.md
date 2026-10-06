# CortexOS Validation Gates and Developer Workflow

**Issue:** BAN-30
**Status:** Phase 0 engineering policy

## Required work loop

```text
Linear plan
  → issue scope/acceptance/dependencies
  → issue In Progress
  → branch + contract checkpoint
  → implementation
  → tests and static checks
  → security/diff review
  → Project Lead review
  → Done only with evidence
```

Every change must belong to a Linear issue. If a new requirement appears, stop and create or update the issue before widening scope. Development-management integrations are not runtime dependencies.

## Branch and commit policy

Branch format:

```text
<type>/<LINEAR_ID>-<short-kebab-case-title>
```

Use Conventional Commits with the issue ID:

```text
<type>(<scope>): <imperative subject> [BAN-XXX]
```

Commit after validated logical checkpoints, not only at phase completion. Examples include contract, implementation, tests, QC fix, documentation, and integration. Never create empty or artificial commits. Main must remain buildable; no direct feature work on main.

Before each commit:

```powershell
git status --short --ignored
git diff --check
git diff --cached --check
```

Inspect staged paths and confirm no secret, raw harvest clone, build output, dependency directory, or unrelated change is staged.

## Validation matrix

| Gate | Current repository command | Pass condition |
| --- | --- | --- |
| Formatting/diff hygiene | `git diff --check` | No whitespace errors |
| Ignore policy | `git check-ignore -v .mcp.json _harvest/raw build/bin` | Sensitive/generated paths are ignored |
| Provenance | `& .\scripts\harvest.ps1 -SkipClone` | Manifest regenerates without changing raw clones |
| Secret review | `Select-String` over tracked config/docs | No real secret values or credential prefixes |
| Go format/test | `gofmt -l .`, `go test ./...` | Enabled after Go manifests exist |
| Frontend checks | `npm ci`, `npm run typecheck`, `npm run lint`, `npm test`, `npm run build` | Enabled after frontend manifests exist |
| Documentation review | Manual link/scope/ADR check | Contract agrees with current architecture |

A command is not considered passing merely because it is documented. Until a manifest and implementation exist, the command is recorded as a planned Phase 1 gate and the gap remains visible.

## Push checklist

Before pushing a branch:

1. Confirm the branch contains the Linear issue ID.
2. Review `git diff main...HEAD` and staged/untracked paths.
3. Run all applicable validation commands and record exact results.
4. Check `git check-ignore` for `.mcp.json`, environment files, local databases, raw harvest, build output, and dependencies.
5. Scan tracked content for API-key patterns, private keys, credentials, and machine-local absolute paths.
6. Confirm third-party material has not crossed from ignored/reference-only harvest into source without attribution/license evidence.
7. Confirm commit subjects are atomic and explainable.
8. Push periodically after a coherent checkpoint; never force-push public history without Project Lead approval.

## Linear status workflow

The intended lifecycle is:

```text
Todo → In Progress → In Review → Done
```

The current team does not have an `In Review` state. Until it is added, keep implementation-complete issues in `In Progress`, add a review comment with evidence, and ask the Project Lead for approval. Do not use `Done` as a substitute for unreviewed work.

`Done` requires:

- acceptance criteria satisfied;
- relevant tests/static/security checks run;
- diff and scope reviewed;
- docs/attribution updated;
- no known bug, error, blocker, or acceptance gap;
- validation evidence recorded in Linear;
- Project Lead approval where the issue requires it.

## Blockers and escalation

Classify blockers explicitly:

- **Build/test blocker:** cannot validate until tooling or code is repaired.
- **Security blocker:** secret exposure, unsafe path/command, or unreviewed credential flow.
- **License blocker:** unclear, copyleft, or asset-specific rights without approval.
- **Dependency blocker:** missing upstream, incompatible version, or unavailable tool.
- **Product decision blocker:** requirement or acceptance ambiguity requiring Project Lead input.
- **Resource blocker:** worker/process/memory budget prevents safe execution.

Record the blocker, impact, evidence, owner, and next decision in Linear. Do not hide a blocker by marking the issue Done.

## Inspector review gate

Inspector/Orchestra is the final technical authority for execution success. A worker's completion message or zero exit code is evidence only. Review must compare actual workspace state and artifacts to acceptance criteria, check unrelated changes, run relevant tests, and verify reproducibility.

## Release boundary

Phase 0 establishes policy and documentation. Phase 1 must make the Go/Wails/frontend gates executable and add automation for secret scanning, schema validation, tests, and build packaging. Missing automation is a tracked gap, not silently waived quality.
