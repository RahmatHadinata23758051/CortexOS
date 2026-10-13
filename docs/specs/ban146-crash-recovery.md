# BAN-146: Crash Recovery — Abrupt Process Kill Mid-Transaction

**Issue:** BAN-146
**Milestone:** Phase 10 — Stress Testing, Soak Testing & Production Hardening
**Status:** Implemented / Verified

---

## 1. Goal

Verify that an abrupt process termination (SIGKILL / TerminateProcess / power loss) **during an atomic merge transaction** leaves the Orchestra database in a consistent, corruption-free state where:

- Partial mutations are never committed (SQLite auto-rollback).
- `PRAGMA integrity_check` passes.
- Parent task remains `TaskAwaitingInspection` — no leaked `TaskSuccess`.
- Child dependent task remains `TaskDraft` — no phantom unblocking.
- Crashed worker heartbeat locks are recovered via `RecoverInterrupted`.

---

## 2. Threat Model

| Scenario                          | What fails                    | Expected outcome                                  |
|-----------------------------------|-------------------------------|---------------------------------------------------|
| `CommitMerge` serializable tx     | Process killed before `tx.Commit()` | All writes rolled back; DB clean                  |
| Parent `AwaitingInspection` → `Success` + Child `Draft` → `Ready` | Same                          | Both revert to original states                    |
| `orchestra_active_tasks` heartbeat | Worker process dies           | `RecoverInterrupted` promotes to `TaskReady` (retryable) |

---

## 3. Implementation

### New Store API (internal/cortex/orchestra/sqlite/store.go)

```go
// OpenRawForCrashSimulation — test-only helper that opens a second raw connection
// to the same database file. Used to begin a transaction, mutate rows, then
// abandon it (emulating SIGKILL) without the Store's normal transaction wrapper.

// CheckIntegrity — runs `PRAGMA integrity_check`; returns error if any row != "ok".
```

### Test: `TestBAN146_CrashRecovery_AbruptKillMidTransaction`

Located in `internal/cortex/orchestra/p10_stress_test.go`.

Steps:
1. Seed parent task in `TaskAwaitingInspection`, child in `TaskDraft` (dependency on parent).
2. Register active-task heartbeat for parent (simulates worker running).
3. Open raw second connection → begin transaction → mutate parent to `TaskSuccess`, child to `TaskReady`.
4. **Rollback & close** (simulates SIGKILL before commit).
5. Reopen database via `orchestrasqlite.Open`.
6. Assert:
   - `CheckIntegrity() == nil`
   - Parent status == `TaskAwaitingInspection`
   - Child status == `TaskDraft`
   - `RecoverInterrupted` returns parent in recovered list
   - After recovery, parent status == `TaskReady` (retryable)

---

## 4. Evidence

```text
=== RUN   TestBAN146_CrashRecovery_AbruptKillMidTransaction
    p10_stress_test.go:627: BAN-146 crash recovery passed: partial transaction rolled back, integrity OK, retryable recovery OK
--- PASS: TestBAN146_CrashRecovery_AbruptKillMidTransaction (0.19s)
PASS
ok      github.com/RahmatHadinata23758051/CortexOS/internal/cortex/orchestra    2.424s
```

Command: `go test -run TestBAN146 ./internal/cortex/orchestra -v -count=1`

---

## 5. Real Subprocess Kill Evidence

The end-to-end companion test `TestBAN146_CrashRecovery_RealSubprocessKill` now launches the current test binary as a helper process, waits until it has opened an uncommitted transaction and updated the task, then calls `cmd.Process.Kill()` (TerminateProcess on Windows, SIGKILL-equivalent process termination on POSIX).

```text
=== RUN   TestBAN146_CrashRecovery_RealSubprocessKill
    p10_stress_test.go:729: BAN-146 real subprocess kill passed: process terminated mid-tx, 0 phantom mutations, integrity OK
--- PASS: TestBAN146_CrashRecovery_RealSubprocessKill
```

The parent reopens the database after the child is killed and verifies `PRAGMA integrity_check == ok` plus the original `TaskAwaitingInspection` state. This closes the previous in-process-only testing gap.

The lower-level in-process test remains useful because it separately verifies `RecoverInterrupted` promotion of a crashed active worker to retryable `TaskReady`.

---

## 6. Files Changed

- `internal/cortex/orchestra/sqlite/store.go` — added `OpenRawForCrashSimulation`, `CheckIntegrity`
- `internal/cortex/orchestra/p10_stress_test.go` — added `TestBAN146_CrashRecovery_AbruptKillMidTransaction`
- `docs/specs/ban146-crash-recovery.md` — this document