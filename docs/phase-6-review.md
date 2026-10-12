# Phase 6 — Cockpit and Virtual Office Review & Phase 7 Approval

**Milestone:** Phase 6 — Cockpit and Virtual Office
**Parent Epic:** BAN-115
**Review Issue:** BAN-125
**Date:** 2026-10-11
**Status:** **GO — Phase 7 entry approved with tracked follow-up gates**

---

## 1. Executive Summary

Phase 6 delivers the Wails-facing Cockpit and Virtual Office presentation layer: typed Go ↔ TypeScript bridge contracts, the React Cockpit shell, Orchestra task board, Staff panel, Knowledge browser, Inspector review and merge dialog, Phaser virtual office, and governed Harness terminal workflow UI. BAN-124 adds the provider-free integration fixtures that verify the cross-layer lifecycle from task dispatch through Staff assignment, Harness evidence, Inspector acceptance, Orchestra merge, and plan progression.

The phase preserves the architecture established by ADR-0003, ADR-0004, and ADR-0005:

- React and Phaser consume typed bridge/event state; they do not own business transitions.
- Staff remains a logical identity and capability boundary, not a process identity.
- Engine output is evidence only; Inspector and Orchestra own acceptance and `TaskSuccess`.
- Terminal sessions are represented through Harness broker contracts rather than raw shell access.
- The Phaser scene renders read-only Staff/activity state and emits selection intent only.

**Decision:** **GO for Phase 7.** All nine implementation child issues BAN-116 through BAN-124 are Done, the Phase 6 integration fixtures pass, the full Go and frontend functional gates pass, and the repository is clean on `main`. Two follow-up items remain explicit Phase 7 entry work: Linux/macOS race-detector CI and measured performance evidence for the 60fps / bridge-latency budgets. Neither finding invalidates the Phase 6 architectural acceptance, but both must be closed before production release readiness.

---

## 2. Deliverables and Issue Evidence

| Issue | Deliverable | Status | Commit | Evidence |
|---|---|---:|---|---|
| **BAN-116** | Wails bridge contracts, Go DTOs, TypeScript types | **Done** | `3a1b4db` | `internal/platform/*`, `internal/cortex/application/*`, `web/src/types/*`, bridge tests |
| **BAN-117** | React Cockpit shell, layout, navigation, design tokens, theme, responsive breakpoints | **Done** | `02fc594` | `web/src/App.tsx`, `web/src/styles.css`, `web/src/App.test.tsx` |
| **BAN-118** | Orchestra task-board Kanban and event-driven refresh seam | **Done** | `c3bd54a` | `web/src/taskboard/TaskBoard.tsx`, `web/src/bridge/orchestra.ts`, `TaskBoard.test.tsx` |
| **BAN-119** | Staff management panel and logical Staff observability | **Done** | `dd32b38` | `web/src/staff/*`, Staff bridge/types, Staff tests |
| **BAN-120** | Knowledge Vault browser, search, retrieval, provenance, ingestion status | **Done** | `6a45ee2` | `web/src/office/KnowledgeView.tsx`, Knowledge bridge/types/tests |
| **BAN-121** | Inspector evidence review, accept/reject, merge dialog | **Done** | `cdb1791` | `web/src/components/inspector/*`, `useInspector.ts`, 16 Inspector tests |
| **BAN-122** | Phaser 3 Virtual Office scene, zones, avatars, activity visualization | **Done** | `6a4f45f` | `web/src/office/OfficeScene.ts`, `OfficeView.tsx`, Office types/tests |
| **BAN-123** | Harness terminal workflow, JSONL stream, cancellation, resource monitor | **Done** | `2ef0846` | `web/src/components/terminal/*`, `useTerminal.ts`, terminal tests |
| **BAN-124** | Phase 6 end-to-end fixture and quality gates | **Done** | `8abe318` | `internal/cortex/orchestra/ban124_integration_test.go`, `web/src/integration/ban124_e2e.test.ts`, workflow gate update |
| **BAN-125** | Phase 6 review and Phase 7 approval | **In review** | this commit | This document |

All predecessor deliverables are atomic, issue-labelled commits already integrated into `main`. `git worktree list` and `git branch -a` show only the retained `main` worktree/branch and `origin/main`; completed topic worktrees and branches were removed under the documented cleanup policy.

---

## 3. Architecture and Boundary Verification

### 3.1 Layer responsibilities

| Boundary | Verification | Result |
|---|---|---|
| Wails/platform → application | `internal/platform` exposes thin bridge methods and forwards typed requests to application services. | **PASS** |
| Application → domain services | Cockpit, Orchestra, Harness, Staff, Knowledge, and Workspace ports remain behind Go services. | **PASS** |
| React → bridge | UI bridge modules validate schema versions and DTO shapes before rendering. | **PASS** |
| Phaser → state | `OfficeScene.setOfficeState` accepts read-only Staff/activity data; rendering is derived from that state. Avatar interaction emits `onStaffSelect` only. | **PASS** |
| Staff ≠ process | Staff UI exposes role, capabilities, availability, skills, and memory metadata; it does not expose PIDs or engine process handles. | **PASS** |
| Terminal → Harness | Terminal UI consumes structured session/JSONL/resource contracts and does not execute a raw shell command from the browser layer. | **PASS** |
| Real-time activity | Typed Orchestra event subscriptions and Wails event seams are used for changes; no new polling loop was introduced for Orchestra lifecycle state. | **PASS** |

### 3.2 ADR-0003 — logical Staff and governed worker pools

**PASS.** The UI presents logical Staff definitions and availability. Backend assignment remains in `staff.Router`, `staff.Scheduler`, `StaffTaskAssigner`, and Harness adapter routing. No UI or Phaser code maps a Staff member to a CLI process. This preserves the separation between durable logical actors and replaceable governed engine workers.

### 3.3 ADR-0004 — replaceable engine adapters

**PASS.** The Cockpit does not bind to a provider-specific CLI. Terminal and task views consume Harness bridge DTOs/events, while engine selection remains behind Harness capabilities and adapter routing. BAN-124 verifies a native adapter through the broker without requiring a live external provider; existing Phase 5 coverage exercises the adapter contract and policy boundary.

### 3.4 ADR-0005 — Inspector/Orchestra success authority

**PASS.** The Inspector review surface starts with `MERGE GATE: LOCKED`. `useInspector` enables merge only for an accepted decision, and the backend integration fixture verifies that a worker's successful tool result leaves the task in `TaskAwaitingInspection`. Only the authoritative inspection/merge path persists `TaskSuccess`; evidence IDs and historical events remain unchanged after merge.

---

## 4. Security Review

| Control | Evidence | Result |
|---|---|---|
| UI XSS surface | Repository scan of `web/src` found no `dangerouslySetInnerHTML`, `innerHTML`, or `eval`. React renders structured values and code snippets as text. | **PASS** |
| Bridge validation | TypeScript bridge modules attach schema versions and validate returned Orchestra DTOs; malformed task data fails closed into an error state. Go bridge/application tests cover invalid requests and canceled contexts. | **PASS** |
| Command boundary | No browser-side raw shell execution path was added. Terminal state is a Harness contract; execution policy, sandboxing, path checks, redaction, and resource limits remain in Go. | **PASS** |
| CSRF/native boundary | The desktop app uses Wails bindings/events rather than a cookie-authenticated browser HTTP mutation API. No CSRF-bearing web session was introduced. Any future remote/web transport must add origin, token, and replay protection before exposure. | **PASS for current desktop scope; follow-up for remote transport** |
| Secret handling | Existing Harness, Staff, Knowledge, and bridge contracts retain redaction and fail-closed policy behavior. BAN-124 uses disposable fixtures and no provider credentials. | **PASS** |
| Untrusted worker output | Worker completion is evidence, not approval. BAN-124 and prior BAN-113/Inspector tests verify the acceptance gate. | **PASS** |
| Dependency/source hygiene | No generated build output, dependency directory, harvest clone, or credentials were staged. | **PASS** |

No secret values, credential prefixes, or unsafe UI HTML sinks were found in the Phase 6 source changes.

---

## 5. Data Contract Compliance

The bridge contracts are versioned and mirrored across the Go application/platform packages and `web/src/types`. The frontend bridge modules validate the returned DTOs before state is exposed to components.

| Contract area | Representative contract | Verification |
|---|---|---|
| Cockpit runtime/workspace | `cortexos.cockpit.bridge.v1` | Go platform/application bridge tests plus TypeScript cockpit tests |
| Orchestra task state | `cortexos.orchestra.bridge.v1` | `web/src/types/orchestra.ts`, bridge tests, TaskBoard tests, BAN-124 frontend fixture |
| Harness sessions and evidence | Harness bridge schema/types | Harness bridge/type tests and terminal component tests |
| Staff observability | Staff bridge schema/types | Staff API/types and panel tests |
| Knowledge/Vault | Knowledge and workspace bridge schemas | Knowledge view and bridge tests |
| Office activity | Typed Staff/activity office state | Office types/tests and `OfficeScene` read-only state boundary |

The UI does not invent lifecycle states: the task board renders the finite Orchestra status union (`draft`, `ready`, `running`, `awaitingInspection`, `success`, `failed`, `canceled`). Unknown or malformed bridge data is rejected rather than silently coerced.

---

## 6. BAN-124 End-to-End Evidence

### 6.1 Go fixture

`internal/cortex/orchestra/ban124_integration_test.go` creates disposable in-memory Workspace, Knowledge, Staff, Orchestra, and Harness services. It then:

1. registers a project and ingests a Vault note into Knowledge;
2. creates a Staff definition with coding capabilities and matching workspace scope;
3. registers a native Harness tool/engine and an allow-coding policy;
4. creates a parent/child plan with a dependency;
5. simulates the task-board draft → ready → dispatch path;
6. verifies Staff assignment and native broker execution produce evidence;
7. verifies the worker result stops at `TaskAwaitingInspection`;
8. rejects dirty-worktree and invalid-merge-capability inspection requests;
9. accepts valid Inspector evidence and persists the authoritative success state;
10. verifies evidence IDs remain immutable and the dependent child becomes ready after merge.

### 6.2 Frontend fixture

`web/src/integration/ban124_e2e.test.ts` uses a typed mocked bridge and exercises create → assign/run → inspect → accept/merge. It asserts:

- the initial board state is `draft`;
- execution ends in `awaitingInspection`, not `success`;
- evidence is present before acceptance;
- the rendered Inspector contains `MERGE GATE: LOCKED` and a disabled merge button;
- the UI re-read reflects backend `success` only after Inspector acceptance and merge;
- terminal commands cannot bypass a terminal state;
- invalid bridge schema data becomes an error state instead of unverified UI state.

---

## 7. Quality Gate Results

| Gate | Command | Result | Notes |
|---|---|---|---|
| Diff hygiene | `git diff --check` | **PASS** | No whitespace errors |
| Go tests | `go test ./...` | **PASS** | All Go packages pass, including BAN-124 |
| BAN-124 Go fixture | `go test ./internal/cortex/orchestra/ -run TestBAN124 -count=1` | **PASS** | End-to-end Cockpit bridge chain passes |
| Go vet | `go vet ./...` | **PASS** | No warnings/errors in the repository command |
| Edited Go formatting | `gofmt -d internal/cortex/orchestra/ban124_integration_test.go` | **PASS** | BAN-124 Go fixture is formatted |
| Frontend typecheck | `cd web; npm run typecheck` | **PASS** | No TypeScript errors |
| Frontend lint | `cd web; npm run lint` | **PASS** | No ESLint errors |
| Frontend tests | `cd web; npm test` | **PASS** | 22 files, 128 tests passed, including BAN-124 |
| BAN-124 frontend fixture | `cd web; npm test -- --run src/integration/ban124_e2e.test.ts` | **PASS** | 3 tests passed |
| Frontend build | `cd web; npm run build` | **PASS** | Production bundle built; Vite reports the existing Phaser chunk-size warning |
| Windows race detector | `go test -race ./...` | **KNOWN LIMITATION** | Windows `cgo.exe` toolchain failure; Linux/macOS CI remains required |
| Whole-tree `gofmt -l` | `gofmt -l internal/` | **ENVIRONMENT NOTE** | Existing tracked Go files are checked out as CRLF under Windows `core.autocrlf`; the raw command lists those legacy files. No unrelated line-ending normalization was introduced; edited BAN-124 Go source is formatted. |

The Vite bundle warning is not a functional failure, but Phaser code splitting should be addressed before a production performance sign-off.

---

## 8. Performance Budget Review

The Phase 6 acceptance target is Phaser at 60fps and bridge latency below 100ms p95. BAN-127 now provides an automated browser performance harness and documented Windows baseline; the benchmark's browser-local bridge fixture does not yet prove packaged Wails/Go service latency. Therefore:

- **Architecture risk:** acceptable for Phase 7 entry; Phaser state rendering is bounded and read-only, and the bridge is typed and local to the Wails desktop process.
- **Evidence status:** **PARTIALLY MEASURED**; BAN-127 has repeatable browser-local evidence, while packaged Wails/Go service latency remains a release follow-up.
- **Required Phase 7 gate:** **implemented in BAN-127.** `cd web; npm run benchmark` runs the production Vite bundle through Playwright, measures OfficeScene frame intervals and typed `ListStaffSummaries` round trips, reports p50/p95/p99, and fails on the 60fps / 100ms p95 budgets. The workload, Windows baseline, limitations, and evidence format are documented in `docs/specs/ban127-performance-benchmark.md`. The current fixture is browser-local; a future packaged Wails run is still required for production Go/service latency evidence.
- **Known bundle concern:** Vite reports the Phaser `OfficeScene` chunk above 500 kB after minification. **Resolved in BAN-128:** `OfficeView` is now lazy-loaded and Phaser is emitted as a deferred `vendor-phaser` chunk, so it is removed from the initial payload. The remaining 1,208 kB (332 kB gzip) chunk is inherent to Phaser 3 and is only fetched when the Virtual Office route opens; startup and frame behavior are measured under the P7-02 benchmark.

---

## 9. Residual Risks and Phase 7 Mitigations

| ID | Severity | Residual risk | Phase 7 mitigation / exit criterion |
|---|---|---|---|
| **P7-01** | Medium | Windows cannot execute the race detector because of the local cgo toolchain. | **Closed (BAN-126).** GitHub Actions `ubuntu-latest` and `macos-latest` run `go test -race ./...` plus gofmt/vet and frontend gates (`.github/workflows/ci.yml`, commits `93432a2`, `b20a00c`, `a748383`, `6f5c7a5`, `e079179`). Evidence: CI run `38215509096` success across all three jobs (Frontend 26s, Linux race 1m38s, macOS race 2m53s), zero data races. |
| **P7-02** | Medium | Browser benchmark does not cross the packaged Wails/Go service boundary. | **Harness closed (BAN-127).** `cd web; npm run benchmark` measures the production OfficeScene and typed bridge fixture with p50/p95/p99 output and budget failures; see `docs/specs/ban127-performance-benchmark.md`. **Residual:** run the same reporting contract against a packaged Wails bridge before production release sign-off. |
| **P7-03** | Medium | Phaser production chunk is above Vite's 500 kB warning threshold. | **Closed (BAN-128).** `OfficeView` is lazy-loaded via `React.lazy`/`Suspense` and `manualChunks` emits `vendor-phaser` as a deferred chunk (commits in `web/src/App.tsx`, `web/vite.config.ts`). Phaser is off the initial critical path; startup/frame impact is measured by the P7-02 benchmark. **Residual:** the deferred `vendor-phaser` chunk is 1,208 kB minified (332 kB gzip) — inherent to Phaser 3, which cannot be tree-shaken below 500 kB without a custom engine build. Accepted because it loads only when the Virtual Office route is opened. |
| **P7-04** | Medium | The current frontend integration fixture mocks the bridge rather than launching Wails dev mode. | **Closed (BAN-129).** Added `docs/specs/ban129-wails-smoke.md`, a documented Wails dev-mode harness that invokes the generated `window.go.platform.Bridge` binding and verifies the corresponding runtime event. Bootstrap now wires the production bridge to Wails `OnStartup`, enabling real event delivery. The interactive desktop run remains an operator release check; mocked Vitest fixtures are not presented as desktop evidence. |
| **P7-05** | Low | `useInspector` owns presentation timing/state while authoritative acceptance is backend-owned. | Replace simulated client-side merge delay with a typed Inspector/Orchestra command when the production merge endpoint is exposed; keep UI optimistic state fail-closed. **Status (BAN-130):** harness-closed. The simulated `setTimeout` merge is removed; merge calls `MergeOrchestraTask` through the typed bridge, `merged` is reported only on backend `success`, and failures revert to locked with a visible error. Residual: the Orchestra adapter is not yet wired in `bootstrap.go`, so production merge currently fails closed with `orchestra.storage_unavailable`; a real `MergeTask` adapter that runs `MergeAuthority.Inspect` is required before merge can succeed end-to-end. See `docs/specs/ban130-inspector-merge.md`. |
| **P7-06** | Low | No remote HTTP transport is in scope, so CSRF controls are not exercised. | If remote access is introduced, add origin checks, CSRF/replay protection, authentication, and authorization tests before exposing mutation methods. |
| **P7-07** | Low | Native desktop event delivery is tested through bridge hooks but not a packaged Wails binary on every OS. | Add packaged smoke tests for Windows and CI coverage for supported desktop targets. |

No residual risk grants the UI success authority or bypasses Harness policy. The risks are release-hardening and measurement work for Phase 7.

---

## 10. Phase 7 Entry Criteria

Phase 7 may begin after this review with the following explicit gates:

1. **CI concurrency gate:** Linux and macOS workflows run `go test -race ./...` and publish artifacts.
2. **Performance gate:** **Harness implemented in BAN-127.** Playwright/browser benchmark demonstrates Phaser frame performance and typed bridge p95 latency against the named Windows baseline; packaged Wails/Go latency remains a release follow-up.
3. **Bundle gate:** Phaser is lazy-loaded or split; build warnings and startup impact are reviewed.
4. **Desktop integration gate:** generated Wails bindings/events are smoke-tested in a dev or packaged desktop run, not only mocked in Vitest.
5. **Authority gate:** any production merge command remains disabled until a backend Inspector acceptance result is received; no client-only success transition is accepted.
6. **Security gate:** any new remote transport has explicit authentication, authorization, origin/CSRF, replay, and input-validation tests.
7. **Documentation gate:** update this review with measured performance and CI links before a production release decision.

---

## 11. Recommendation and Sign-Off

**Recommendation: GO for Phase 7.** Phase 6 acceptance is complete for the desktop Cockpit/Virtual Office architecture and implementation scope. The integration fixtures demonstrate the required UI → bridge → Orchestra → Harness → evidence → Inspector → merge chain, and the repository is pushed at `8abe318` with a clean retained `main` worktree.

BAN-115 should be marked **Done** after this review is committed and the Linear milestone is updated to 100%. BAN-125 should be marked **Done** with this document attached as its evidence. The Phase 7 hardening gates above remain required before production release approval.

| Role | Decision | Date | Evidence |
|---|---|---|---|
| Automated engineering review | **GO — Phase 7 entry approved** | 2026-10-11 | `8abe318`, full Go/frontend gates, BAN-124 integration fixtures |

*This review is evidence-based. Where a numeric performance budget or cross-platform race result was not measured in the current Windows environment, it is explicitly recorded as a Phase 7 gate rather than represented as a false pass.*
