# BAN-144: High-Concurrency Merge Race Stress Suite (100 Tasks, 16 Racers Each)

**Issue:** BAN-144  
**Milestone:** Phase 10 — Stress Testing, Soak Testing & Production Hardening  
**Status:** Implemented / Verified  

---

## 1. Goal & Scale

Verify that the CortexOS Orchestra merge pipeline and SQLite compare-and-set (CAS) persistence engine (`internal/cortex/orchestra/adapter.go` and `internal/cortex/orchestra/sqlite/tasks.go`) maintain zero-leak, zero-corruption, and exactly-one-winner guarantees under high concurrent contention:
- **100 independent tasks** seeded concurrently into `TaskAwaitingInspection`.
- **16 racing goroutines per task** (total **1,600 concurrent merge attempts**).
- Single-connection SQLite WAL store behind serializable CAS transactions.

---

## 2. Invariants Checked

1. **Exactly One Winner per Task**:
   Across 1,600 attempts, exactly **100** succeeded and exactly **1,500** were rejected fail-closed.
2. **Deterministic State Transition**:
   All 100 tasks transitioned to `TaskSuccess` without orphan locks or partial rollbacks.
3. **Timeline Event Integrity**:
   Every merged task recorded exactly one `EventInspectionAccepted` entry in its immutable timeline.
4. **Latency & Throughput**:
   All 1,600 racing attempts completed in **~1.04 seconds** on Windows workstation hardware (~1,540 ops/sec under contention).

---

## 3. Test Command & Evidence

```powershell
go test -run TestBAN144 ./internal/cortex/orchestra -v -timeout 180s
```

Output:
```text
=== RUN   TestBAN144_HighConcurrencyMergeRace
    p10_stress_test.go:222: BAN-144 stress: tasks=100 racers/task=16 attempts=1600 successes=100 rejected=1500 elapsed=1.039s
--- PASS: TestBAN144_HighConcurrencyMergeRace (2.77s)
PASS
ok      github.com/RahmatHadinata23758051/CortexOS/internal/cortex/orchestra    3.743s
```
