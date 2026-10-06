# Phase 1 Dependency Baseline

**Issue:** BAN-36
**Status:** Approved baseline
**Scope:** Go + Wails desktop shell and React/TypeScript frontend

## Selected toolchain

| Tool | Pinned baseline | Evidence and compatibility note |
| --- | --- | --- |
| Go | 1.25.0 minimum; local validation uses 1.26.3 | Wails v2.16.0 declares Go 1.25.0 in its module metadata. |
| Wails | CLI and Go module v2.16.0 | Wails v2 is the stable desktop boundary selected for Phase 1. Wails v3 remains beta and is out of scope. |
| Node.js | 22.12.0 or newer; local validation uses 22.23.3 | Required by the Vite 7 toolchain and compatible with the Wails frontend workflow. |
| npm | 10.9.0 or newer; local validation uses 10.9.9 | npm is the package manager for the committed `web/package-lock.json`. |
| React | 19.2.0 | Exact runtime dependency for the first frontend shell. |
| TypeScript | 5.9.3 | Strict TypeScript baseline; no unbounded version ranges. |
| Vite | 7.3.7 | Exact frontend build/dev tool. This version is selected because its current security audit is clean and its Node engine supports the pinned Node baseline. |
| Vitest | 5.0.3 | Exact unit-test runner. The selected version has no reported vulnerabilities in the current npm audit. |
| ESLint | 10.12.0 | Exact lint tool using flat configuration. |

The repository records the frontend versions in `web/package.json` and
`web/package-lock.json`. The Go module path and Wails requirement are recorded
in `go.mod`. `go.sum` is currently empty because no Go package imports Wails
until BAN-37 creates the platform shell; the module metadata is still pinned and
`go mod tidy` remains a required gate after the first import is added.

## Direct dependency review

| Package | Version | Owner layer | License | Purpose | Removal path |
| --- | --- | --- | --- | --- | --- |
| `github.com/wailsapp/wails/v2` | `v2.16.0` | Platform | MIT | Desktop window, lifecycle, and typed binding boundary. | Keep all Wails imports in `internal/platform` and generated binding glue. |
| `react`, `react-dom` | `19.2.0` | UI | MIT | Declarative cockpit presentation. | UI components remain behind the frontend application entrypoint. |
| `typescript` | `5.9.3` | UI | Apache-2.0 | Strict static type checking and bridge DTO authoring. | DTOs remain plain serializable types and can be consumed by another UI toolchain. |
| `vite`, `@vitejs/plugin-react` | `7.3.7`, `5.0.4` | UI | MIT | Frontend development server and production asset build. | Wails consumes the built asset directory; no runtime Vite dependency is required. |
| `vitest` | `5.0.3` | UI/test | MIT | Fast deterministic frontend unit tests. | Tests target components and clients rather than the runner API. |
| `eslint`, `@eslint/js`, `typescript-eslint` | `10.12.0`, `10.0.1`, `8.71.1` | UI/test | MIT | Static analysis for JavaScript and TypeScript source. | Rules are project configuration; source does not depend on ESLint APIs. |
| `@types/node`, `@types/react`, `@types/react-dom`, `globals`, React ESLint plugins | pinned in `web/package.json` | UI/test | MIT | Compile-time declarations and lint environment rules only. | These packages are development-only and are not shipped in the desktop runtime. |

License sources:

- Wails: <https://github.com/wailsapp/wails/blob/v2.16.0/LICENSE>
- React: <https://github.com/facebook/react/blob/v19.2.0/LICENSE>
- TypeScript: <https://github.com/microsoft/TypeScript/blob/v5.9.3/LICENSE.txt>
- Vite: <https://github.com/vitejs/vite/blob/v7.3.7/LICENSE>
- Vitest: <https://github.com/vitest-dev/vitest/blob/v5.0.3/LICENSE>
- ESLint: <https://github.com/eslint/eslint/blob/v10.12.0/LICENSE>
- typescript-eslint: <https://github.com/typescript-eslint/typescript-eslint/blob/v8.71.1/LICENSE>

## Risk and policy decisions

- No provider SDK, Linear client, Phaser package, database driver, or engine
  adapter is included in this baseline.
- Wails carries platform-native WebView integration and must remain isolated
  from domain/application packages. Windows smoke validation requires the
  WebView2 runtime.
- Frontend packages are development/build dependencies except React and
  ReactDOM, which are bundled into the UI assets; none receives credentials or
  filesystem authority.
- `npm audit` is run against the committed lockfile. The current full audit
  reports zero vulnerabilities; production-only audit is also clean.
- GPL, AGPL, unclear-license, copied source, and harvested material are not
  adopted by this baseline.
- Dependency upgrades require a separate issue, exact version review, license
  review, and rerun of all Phase 1 gates.

## Reproducible validation

From the repository root:

```powershell
# Tool versions
 go version
 node --version
 npm --version
 wails version

# Go module and source checks
 go mod download
 go test ./...
 gofmt -l .

# Frontend checks
 Push-Location web
 npm ci
 npm run typecheck
 npm run lint
 npm test
 npm run build
 npm audit
 Pop-Location

# Repository safety checks
 git diff --check
 git check-ignore -v .mcp.json RULES.md opencode.json _harvest/raw build/bin web/node_modules web/dist
```

The Wails application and bridge smoke checks are deferred to BAN-37 and later
issues. This issue establishes manifests, versions, dependency ownership, and
review evidence only.
