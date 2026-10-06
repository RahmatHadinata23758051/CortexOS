# CortexOS Dependency Policy

**Issue:** BAN-28
**Status:** Foundation policy

## Review record required for each direct dependency

| Field | Required value |
| --- | --- |
| Package/module | Exact name and ecosystem |
| Version | Exact version or commit, never an unbounded range |
| Purpose | One sentence tied to a CortexOS contract |
| Owner layer | Workspace, Orchestra, Harness, Staff, Engine, Platform, or UI |
| License | SPDX identifier plus source URL |
| Transitive risk | Known notices, native binaries, assets, or copyleft dependencies |
| Validation | Build/test/security command that exercises it |
| Removal path | What local abstraction prevents lock-in |

## Compatibility rules

- Core runtime prefers Go standard library and narrow internal ports.
- Wails is isolated in `internal/platform` and generated/binding glue.
- React/TypeScript packages are isolated from Go domain packages through DTOs.
- Phaser belongs only to the office presentation feature.
- Provider and engine SDKs belong behind Harness/Engine adapters.
- Linear tooling stays in development configuration and cannot be imported by runtime packages.
- AGPL/GPL or unclear-license material is blocked until Project Lead/legal approval.
- Copied third-party code/assets require license text and attribution in the distribution plan.

## Locking and update policy

Lockfiles and `go.sum` are committed. Dependency upgrades are separate atomic changes, include release notes/security advisories where relevant, and rerun the complete foundation validation. A dependency upgrade must not be hidden inside an unrelated feature commit.
