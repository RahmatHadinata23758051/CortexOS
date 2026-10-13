# Phase 13 Execution Plan: Advanced Staff Orchestration

**Generated:** 2026-10-13  
**Status:** Approved for Implementation  
**Estimated Complexity:** High  
**Objective:** Extend the Phase 12 Brain cockpit into a governed multi-agent orchestration flow: route Brain plans into Orchestra tasks, preserve Orchestra/Harness authority boundaries, introduce explicit tool approval gates, coordinate multiple Staff avatars, and ground Brain context with bounded Knowledge retrieval.

---

## 1. Architecture Boundaries & Constraints

1. **Authority boundaries**
   - Brain may propose plans, task intents, tool calls, and Staff assignments.
   - Orchestra remains authoritative for task graph state, dependencies, assignment transitions, cancellation, retry, and completion.
   - Harness remains authoritative for execution, resource leases, process lifecycle, and evidence production.
   - Inspector/MergeAuthority remains the only authority allowed to accept a completed task into `TaskSuccess`.

2. **Fail-closed and secret safety**
   - No provider API keys, process handles, raw command output, or absolute filesystem paths cross application or Wails DTOs.
   - Invalid routing requests, unavailable stores, unsupported contracts, and denied approvals fail closed with sanitized stable error codes.
   - Automated tests use `FakeProvider`; live 9router coverage remains opt-in via `CORTEXOS_9ROUTER_LIVE_TEST=1`.

3. **Brain independence**
   - `internal/cortex/brain` remains independent from Wails, persistence, Orchestra stores, Harness process handles, and provider-specific configuration.
   - Integration belongs in application ports and adapters, with the bridge adapter kept outside both domain packages to avoid import cycles.

4. **Concurrency, budgets, and transport**
   - Routing is context-cancelable, bounded by task/step/token budgets, and safe under duplicate or concurrent dispatch attempts.
   - Event subscribers are non-blocking and bounded; no per-token Phaser scene rebuilds.
   - No remote transport, HTTP server, or WebSocket endpoint is introduced.

---

## 2. Sprint Roadmap

### Sprint 13.1: Brain ↔ Orchestra Multi-step Task Routing
**Goal:** Convert a Brain-produced task intent into an Orchestra-owned task graph and dispatch lifecycle without allowing Brain or the UI to mutate authoritative task state directly.

**Demo/Validation:**
- Submit a bounded Brain routing request with `FakeProvider`.
- Observe a sanitized routing result containing the Orchestra task ID and lifecycle correlation data.
- Verify Orchestra owns the persisted task/event transitions and rejects duplicate, invalid, canceled, or unavailable requests.
- Run scoped Go tests with the race detector where supported and confirm no secrets or raw execution details appear in DTOs/events/logs.

#### Task 13.1.1: Define Brain-to-Orchestra application contract
- **Location:** `internal/cortex/application/brain_orchestra.go`, related application contract tests.
- **Description:** Define versioned request/result DTOs and a `BrainOrchestraPort` for submitting a bounded multi-step routing intent. Include correlation IDs, Staff assignment intent, task title/action, acceptance criteria, dependencies, priority, and resource hints only as safe domain values. Add validation and stable sanitized error mapping.
- **Dependencies:** Phase 12 BrainPort and existing `OrchestraPort`.
- **Acceptance Criteria:**
  - Contract has an explicit schema version.
  - Missing task identity/prompt, oversized plans, invalid dependencies, and unsupported versions fail closed.
  - DTO JSON contains no provider credentials, process handles, absolute paths, or raw provider errors.
- **Validation:** Application contract serialization and validation tests with table-driven invalid cases.

#### Task 13.1.2: Implement the Brain-to-Orchestra routing adapter
- **Location:** `internal/cortex/bridge/brain_orchestra_adapter.go`, adapter tests.
- **Description:** Implement an adapter that asks Brain for a bounded structured plan, validates the plan, and submits task creation/dispatch through the existing Orchestra port. Keep Brain output advisory; all authoritative transitions go through Orchestra. Support cancellation, timeout, duplicate correlation protection, and bounded plan size.
- **Dependencies:** Task 13.1.1.
- **Acceptance Criteria:**
  - A valid plan creates or dispatches Orchestra-owned tasks through the port only.
  - Malformed or over-budget Brain output is rejected without partial persistence.
  - Context cancellation prevents further dispatch and returns a sanitized error.
  - Adapter has no direct SQLite, Wails, process, or provider-key dependency.
- **Validation:** Contract tests using `FakeProvider`, fake Orchestra port, cancellation, timeout, malformed plan, and duplicate submission cases.

#### Task 13.1.3: Wire Service, Wails bridge, and lifecycle events
- **Location:** `internal/cortex/application/service.go`, `internal/platform/bridge.go`, `internal/platform/events.go`, `internal/bootstrap/bootstrap.go`.
- **Description:** Expose a typed `RouteBrainTaskToOrchestra` application/bridge operation and bounded lifecycle events for accepted, rejected, queued, and canceled routing. Preserve the existing Brain streaming contract and Orchestra bridge authority.
- **Dependencies:** Task 13.1.2.
- **Acceptance Criteria:**
  - Bridge method validates schema and delegates to the application service.
  - Event payloads contain stable IDs/statuses only and are non-blocking for subscribers.
  - Missing Brain or Orchestra wiring returns sanitized fail-closed errors.
  - Existing Phase 12 methods remain backward compatible.
- **Validation:** Platform bridge tests, event subscription tests, bootstrap wiring test, and scoped `go test`.

#### Task 13.1.4: Add routing integration evidence and regression gates
- **Location:** `internal/cortex/application/brain_orchestra_test.go`, `internal/cortex/bridge/brain_orchestra_adapter_test.go`, `internal/platform/bridge_test.go`, `docs/specs/phase-13-evidence.md`.
- **Description:** Exercise the end-to-end in-process route from Brain intent through Orchestra task creation/dispatch using fakes. Record metrics for routing latency, rejection behavior, bounded plan size, and event delivery without recording secrets.
- **Dependencies:** Tasks 13.1.1–13.1.3.
- **Acceptance Criteria:**
  - Happy path and fail-closed paths are covered.
  - Duplicate requests do not create duplicate authoritative tasks.
  - Cancellation leaves no orphaned active routing operation.
  - Evidence documents exact commands, results, and rollback commit.
- **Validation:** `go test ./internal/cortex/brain ./internal/cortex/application ./internal/cortex/bridge ./internal/platform ./internal/bootstrap -count=1`; `go test -race` for affected packages where available; secret-scan assertions.

---

### Sprint 13.2: Tool Approval Gates and Governed Tool Calls
**Goal:** Introduce an explicit proposal → approval/denial → execution lifecycle for Brain tool calls without granting Brain direct process or filesystem authority.

**Demo/Validation:**
- Brain proposes a tool call; the Cockpit displays the sanitized proposal.
- Operator approval dispatches only an allowlisted tool through the governed Harness path.
- Denial, timeout, cancellation, malformed arguments, and unknown tools remain fail closed.

**Planned tasks:**
- Define versioned tool proposal and approval contracts with argument redaction and bounded payloads.
- Add an approval store/port and lifecycle events with idempotent decisions.
- Connect approved calls to Harness capability checks and evidence capture.
- Add Cockpit approval UI, keyboard/accessibility behavior, and E2E tests.

---

### Sprint 13.3: Multi-Staff Coordination and Office State
**Goal:** Visualize and control multiple Staff roles participating in one Orchestra execution while keeping state updates event-driven and performant.

**Demo/Validation:**
- A routed plan shows assigned Staff, dependencies, active phase, and blocked/awaiting states in the Office and Cockpit.
- Avatars update on lifecycle events rather than token frequency.
- Reassignment, cancellation, and partial failure render consistently.

**Planned tasks:**
- Define a bounded coordination snapshot and event projection.
- Add application/frontend subscriptions for Staff assignment and task graph updates.
- Extend Office scene state mapping and Cockpit coordination timeline.
- Add multi-avatar Playwright performance and correctness coverage.

---

### Sprint 13.4: Knowledge Retrieval Grounding
**Goal:** Ground routed plans and tool proposals with bounded, inspectable Knowledge context without leaking internal paths or untrusted metadata.

**Demo/Validation:**
- Brain routing shows the retrieved context IDs and source labels in the existing inspector.
- Retrieval timeout, empty result, and malformed metadata degrade safely.
- Context budgets remain bounded and prompt assembly remains deterministic.

**Planned tasks:**
- Add a routing-specific Knowledge context contract and budget policy.
- Integrate retrieval through the application port, not directly into Brain.
- Add provenance-safe context inspection and redaction tests.
- Measure latency, context size, and UI rendering impact.

---

### Sprint 13.5: End-to-End Hardening and Release Readiness
**Goal:** Verify the complete multi-agent orchestration path under cancellation, concurrency, failure, and packaging conditions.

**Demo/Validation:**
- Playwright route → approval → execution → evidence → Inspector flow passes with zero page/console errors.
- Go unit, contract, integration, race, frontend, and packaged desktop checks pass.
- Rollback and data-contract compatibility are documented.

**Planned tasks:**
- Add multi-session soak and duplicate-dispatch stress benchmarks.
- Verify bounded memory, event queues, and frontend frame budgets.
- Run security/data-contract/release review and publish final evidence.
- Update version/changelog only after all gates pass.

---

## 3. Testing Strategy

1. **Go unit and contract tests**
   - Use `FakeProvider`, fake Brain ports, fake Orchestra ports, and fake Harness ports.
   - Cover validation, cancellation, timeout, duplicate requests, dependency graphs, approval decisions, and authority boundaries.

2. **Integration and race tests**
   - Exercise the in-process adapter chain with durable Orchestra test stores where appropriate.
   - Run `go test -race` for concurrency-sensitive packages; assert no data races, deadlocks, or leaked goroutines.

3. **Frontend tests**
   - Mock Wails runtime events and verify event cleanup, bounded rendering, approval keyboard behavior, and state recovery.
   - Keep Phaser updates event-driven and verify frame budgets with Playwright.

4. **Opt-in live provider tests**
   - Only run when `CORTEXOS_9ROUTER_LIVE_TEST=1` is explicitly set.
   - Never include provider keys, authorization headers, or raw provider responses in logs, fixtures, snapshots, or evidence.

---

## 4. Risks & Gotchas

1. **Brain plan ambiguity:** free-form model output may produce invalid graphs or unsupported fields. Mitigate with a strict bounded schema and reject-before-persist validation.
2. **Authority bypass:** a shortcut from Brain/UI to SQLite or Harness would undermine Orchestra governance. Keep all mutations behind application ports and assert call paths in tests.
3. **Duplicate routing:** retries can create duplicate tasks. Require an idempotency/correlation key and verify it at the authoritative boundary.
4. **Approval races:** two operator decisions may arrive concurrently. Persist a single terminal decision with compare-and-set semantics.
5. **Event flooding:** multi-agent token and lifecycle events can overload Wails. Batch/coalesce UI updates and never rebuild Phaser scenes per token.
6. **Context injection:** retrieved documents and tool arguments are untrusted input. Bound size, preserve provenance, redact paths/secrets, and test adversarial content.
7. **Partial failure:** one Staff task may fail while others continue. Model explicit blocked/failed states and ensure cancellation propagates without falsely marking success.

---

## 5. Rollback Plan

- Sprint 13.1 is atomic and can be reverted independently to the Phase 12 release point (`937e25d`; Phase 11 fallback remains `bf43689`).
- Do not introduce irreversible schema changes without a forward/backward compatibility window and migration test.
- If routing regresses existing Brain or Orchestra flows, disable the new bridge operation and event subscriptions while preserving the existing Phase 12 methods.
