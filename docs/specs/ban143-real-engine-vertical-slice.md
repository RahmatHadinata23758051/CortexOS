# BAN-143: End-to-End Orchestration Vertical Slice with Real Subprocess Engines (Pi/OMP)

**Issue:** BAN-143  
**Status:** Implemented / Verified  
**Milestone:** Phase 9 — Productization and Operational Readiness  

---

## 1. Goal & Architectural Boundary

Establish an end-to-end integration test demonstrating the complete CortexOS execution pipeline across all architectural tiers:
1. **Workspace**: Project and worktree boundaries registered in memory/sqlite workspace.
2. **Policy & Broker**: Static policy with explicit capability binding and allow/ask/deny fail-closed boundary.
3. **Sandbox & Subprocess**: Real compiled worker executable (`cmd/testworker`) executed via `PiAdapter` and `DefaultProcessRunner`, communicating via strict JSONL protocol.
4. **Harness & Governor**: Capability matching, correlation and monotonically increasing sequence tracking, heartbeat age, and bounded resource limits (EngineClassPi: max workers, memory, CPU priority).
5. **Orchestra Dispatcher & Bridge**: Dispatches task, tracks state transition from `ready` -> `running` -> `awaiting_inspection`, collects command execution evidence.
6. **Inspector & Authority (ADR-0005)**: MergeAuthority inspects collected evidence. Requires valid evidence, rejects failed evidence (fail-closed), and produces authoritative accepted task with `mergeReady: true`.
7. **Merge Persistence**: Authoritative transition to `TaskSuccess` written to store and verified.

---

## 2. Real Worker Protocol Boundary (`cmd/testworker`)

Because third-party `pi` or `omp` binaries may be absent on local development or CI worker machines, `cmd/testworker` acts as a zero-mock, real-process worker adhering strictly to the CortexOS worker protocol specification (`internal/cortex/harness/protocol.go`):
- Supports `--handshake` command emitting `harness.MessageHandshake` with `WorkerID` and `WorkerProtocolVersion`.
- Supports `--worker` command:
  - Validates sanitized task context from environment variables (`CORTEXOS_EXECUTION_ID`, `CORTEXOS_TASK_ID`, etc.) without exposing private host paths or unredacted policies.
  - Emits handshake response matching the execution correlation ID.
  - Emits task echo (`harness.MessageTask`) with monotonically advancing sequence number.
  - Emits progress events (`harness.MessageProgress`) reflecting worker phases.
  - Emits real evidence record (`harness.MessageEvidence`) with SHA-256 digest and command metadata.
  - Emits terminal outcome (`harness.MessageTerminal`) indicating success or controlled failure.

---

## 3. Tool Alias Normalization Fix

During implementation of BAN-143, tool name aliases were resolved:
- In `internal/cortex/harness/adapter_pi.go` and `internal/cortex/harness/adapter_omp.go`, tool names ending with `_agent` (e.g. `coding_agent`) now normalize cleanly to base capabilities (`coding`).
- This allows Orchestra's default tool selector (`coding_agent`) to route directly to `PiAdapter` without manual aliasing or bypass.

---

## 4. Resource Governance & Governor Admission

Verified that real `EngineClassPi` execution conforms to laptop-safe bounds:
- Extended descriptor declares `MinMemoryMB: 200`, `PeakMemoryMB: 600`, `CPUPriority: 1`.
- Governor rejects dispatch exceeding budget (`PeakMemoryMB + 1`) and admits dispatch within budget.
- Governor leases are strictly released via `reservation.Release()` after execution.

---

## 5. Verification & Test Evidence

Run targeted integration test:
```text
go test -run TestBAN143_RealEngineVerticalSlice ./internal/cortex/orchestra -v
```
Result:
```text
=== RUN   TestBAN143_RealEngineVerticalSlice
    ban143_integration_test.go:139: PiAdapter descriptor: kind=pi caps=[coding analysis refactor debug review test_gen doc_gen]
    ban143_integration_test.go:141: PiExtendedDescriptor resource: minMem=200 peakMem=600 cpuPrio=1
--- PASS: TestBAN143_RealEngineVerticalSlice (2.43s)
PASS
ok      github.com/RahmatHadinata23758051/CortexOS/internal/cortex/orchestra    3.503s
```

All 7 assertions pass:
1. Real worker subprocess builds and passes handshake health check.
2. Orchestra dispatches task and awaits inspection.
3. Subprocess emits valid JSONL evidence records without protocol violations.
4. MergeAuthority (Inspector) accepts valid evidence and declares merge readiness.
5. Merge outcome persists with final status `TaskSuccess`.
6. Negative inspection test rejects failing evidence fail-closed (`ErrInspectionRejected`).
7. Governor enforces memory and worker concurrency bounds.
