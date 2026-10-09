# Virtual Office Dependency Baseline

**Issue:** BAN-122
**Scope:** Phaser presentation-only office scene

## Dependency review

| Package | Version | Purpose | Owner | License | Transitive risk | Validation | Removal path |
| --- | --- | --- | --- | --- | --- | --- | --- |
| `phaser` (npm) | `3.90.0` | Render the virtual-office room zones and read-only Staff avatars | UI / office presentation | MIT ([npm metadata](https://www.npmjs.com/package/phaser/v/3.90.0)) | Browser-focused rendering runtime; adds a large lazy-loaded bundle; no native binaries | `npm run typecheck`, `npm run lint`, `npm test`, `npm run build` in `web/` | `OfficeScene` and `createOfficeGame` isolate Phaser behind the office view; typed bridge and event contracts remain Phaser-independent |

Phaser is constrained to the office presentation feature. It is dynamically imported so the core Workspace cockpit can render without loading or instantiating the scene. Phaser receives `StaffSummary` and activity snapshots and has no domain/application imports or mutation port.

The version and lockfile are pinned in `web/package.json` and `web/package-lock.json`. Dependency evidence was checked against npm registry metadata (`npm view phaser@3.90.0 version license`), which reports version `3.90.0` and MIT licensing.
