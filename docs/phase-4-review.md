# Phase 4 — Harness Layer Review & Milestone Evidence

**Milestone:** Phase 4 — Harness & Execution Boundary Layer  
**Parent Epic:** BAN-87  
**Issue:** BAN-96: "Add Phase 4 end-to-end fixture and quality gates"  
**Status:** IMPLEMENTED — RECOMMENDATION: GO FOR PHASE 5  
**Date:** 2026-10-08  

---

## 1. Executive Summary

Phase 4 implements the complete, hardened, engine-neutral Harness execution boundary, Tool Broker, Resource Governor, Sandbox isolation, and engine adapter layer (Native, Pi, OMP) for CortexOS.

Every tool execution in CortexOS is subject to:
1. **Tool Broker Policy Enforcement**: Fail-closed permission evaluation (`allow`, `ask`, `deny`) with explicit approval grants, parameter validation, and capability binding.
2. **Resource Governor Admission**: Strict capacity budgets per engine class (Native, Pi, OMP) preventing laptop starvation; deterministic priority/FIFO admission queues; idempotent capacity reservation leases.
3. **Sandbox Execution Boundary**: Mandatory worktree containment rejecting path traversals and escapes, sanitized environment variables, strictly bounded stdout/stderr streams, and process-group termination on cancellation or timeout.
4. **Untrusted Evidence Boundary**: Execution adapters and worker processes never emit trusted success; they emit immutable `EvidenceRecord` artifacts collected by the Broker and streamed to Orchestra.
5. **Orchestra Sovereignty & Inspector Merge Authority**: Inspector (`MergeAuthority`) evaluates governed evidence against acceptance criteria; Orchestra alone transitions tasks to `TaskSuccess`.

All 13 Phase 4 engineering deliverables (BAN-93 through BAN-96) have been implemented, tested, verified, and integrated into the disposable end-to-end fixture suite.

---

## 2. Issues and Deliverables Matrix

| Issue | Title | Status | Key Artifacts |
|---|---|---|---|
| **BAN-93** | Define harness contracts and tool broker boundary | **Done** | `internal/cortex/harness/contract.go`, `errors.go` |
| **BAN-88** | Define JSONL worker lifecycle protocol | **Done** | `internal/cortex/harness/protocol.go`, `protocol_test.go` |
| **BAN-89** | Define replaceable engine adapter interface and routing policy | **Done** | `internal/cortex/harness/router.go`, `router_test.go` |
| **BAN-90** | Implement Tool Broker policy enforcement and audit evidence | **Done** | `internal/cortex/harness/broker.go`, `evidence.go` |
| **BAN-91** | Implement sandbox execution boundary | **Done** | `internal/cortex/harness/sandbox.go`, `process_group_*.go` |
| **BAN-94** | Implement Resource Governor and governed worker pools | **Done** | `internal/cortex/harness/governor.go`, `governor_test.go` |
| **BAN-92** | Implement native deterministic tool adapter | **Done** | `internal/cortex/harness/adapter_native.go`, `adapter_native_test.go` |
| **BAN-95** | Implement Pi engine adapter | **Done** | `internal/cortex/harness/adapter_pi.go`, `adapter_pi_test.go` |
| **BAN-97** | Implement OMP specialist and recovery adapter | **Done** | `internal/cortex/harness/adapter_omp.go`, `adapter_omp_test.go` |
| **BAN-100**| Persist harness policy process and evidence state | **Done** | `internal/cortex/harness/sqlite/` migrations & store |
| **BAN-98** | Integrate Orchestra dispatcher with Harness broker | **Done** | `internal/cortex/orchestra/harness_bridge.go` |
| **BAN-99** | Expose safe Harness observability bridge | **Done** | `internal/cortex/application/harness.go`, `harness_adapter.go`, `bridge_harness_test.go` |
| **BAN-96** | Add Phase 4 end-to-end fixture and quality gates | **Done** | `internal/cortex/harness/integration_test.go`, this review document |

---

## 3. Architecture & Boundary Verification

1. **Provider Independence**:
   - The end-to-end integration fixture in `internal/cortex/harness/integration_test.go` relies strictly on deterministic test fakes and local filesystem tempdirs (`t.TempDir()`).
   - No external AI providers, network APIs, or cloud services are invoked.
2. **Untrusted Worker Principle**:
   - Workers emit JSONL wire messages (`cortexos.worker.v1`) containing status claims, progress reports, diagnostics, and evidence references.
   - Worker claims of "success" are treated strictly as untrusted evidence. Inspector (`MergeAuthority`) evaluates cryptographic evidence digests against task criteria before any task can transition towards success.
3. **Execution Sandbox Boundary**:
   - `ExecutionSandbox` canonicalizes all paths and rejects path traversal (`../`, `..\`, null bytes, symlinks) escaping the assigned worktree.
   - Process environment variables are filtered against a strict allowlist; secret variables (`KEY`, `TOKEN`, `PASSWORD`, `SECRET`) are unconditionally scrubbed.
   - Stdout and stderr outputs are capped by buffer limits, truncated gracefully, and scrubbed through regex-based credential redaction.
   - Child processes run in isolated process groups and are guaranteed to terminate upon context deadline or cancellation.
4. **Tool Broker Gatekeeping**:
   - Direct process execution outside the Tool Broker is prohibited.
   - Tool requests are evaluated against declared capability bindings and policy rules (`allow/ask/deny`).
   - Capability bypass attempts (requesting actions not declared in tool definitions) fail closed with `policy_denied`.
   - Sensitive actions requiring explicit approval (`requiresAsk`) block until authorized via `WithApproval`.
5. **Resource Governor Accounting**:
   - Worker pool limits are enforced independently for Native, Pi, and OMP engine classes.
   - Admitted tasks receive a capacity `Reservation` lease with idempotent release semantics (`reservation.Release()`).
   - Excess concurrency is safely queued in priority/FIFO order or timed out without capacity leakage.
6. **Import & Architecture Purity**:
   - Compile-time type assertions verify interface compliance across `ToolBroker`, `PolicyEngine`, `EngineAdapter`, `Sandbox`, `Executor`, and `Inspector`.
   - External test packaging (`package harness_test`) prevents illegal import cycles between the Harness and Orchestra packages.

---

## 4. Security & Concurrency Verification

- **Path Traversal Tests**: Paths outside the worktree root (`../outside`, `..\outside`, absolute host paths) fail with `ErrWorktreeViolation`.
- **Secret Redaction Tests**: Sensitive tokens in execution output and environment variables are filtered out and redacted.
- **Capability Bypass Tests**: Requesting undeclared tool actions is intercepted and rejected with `ToolStatusPolicyDenied`.
- **Output Limit Tests**: Execution output exceeding `StdoutLimitBytes` is cleanly truncated with `Truncated = true`.
- **Process Cleanup Tests**: Context cancellation guarantees runner process cleanup without dangling or zombie processes.
- **Terminal Race Tests**: Concurrent `Cancel` and `Complete` calls on the Dispatcher resolve deterministically to either `TaskCanceled` or `TaskAwaitingInspection` without corrupted states or panics.
- **Capacity Reservation Tests**: Concurrent admissions up to maximum workers observe budget bounds; all resources are returned upon lease release.
- **Recovery Ownership Tests**: `VerifyWorkerIdentity` checks ensure that only the active worker assigned to an execution can participate in recovery.

---

## 5. Test & Quality Gate Results

All test suites and verification gates pass cleanly across the repository:

- `go test ./...`: **PASS** (100% across all packages including `harness`, `orchestra`, `workspace`, `application`, `platform`, `bootstrap`)
- `go test -v -run TestHarnessFullPathFixture ./internal/cortex/harness/`: **PASS** (100% pass rate)
- `go vet ./...`: **PASS** (0 warnings or errors)
- `gofmt`: **PASS** (all Go source files formatted)
- `git diff --check`: **PASS** (0 whitespace or syntax errors)
- Frontend Vitest suite (`npm test` in `web/`): **PASS** (4 test files, 8 tests passed)
- Frontend TypeScript check (`npm run typecheck` in `web/`): **PASS** (0 errors)

---

## 6. Recommendation

**GO for Phase 5.**  
All Phase 4 Harness requirements and BAN-96 acceptance criteria are fully met, verified by disposable fixtures, documented, and enforced by automated quality gates.
