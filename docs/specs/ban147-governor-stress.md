# BAN-147: Governor Resource Exhaustion & Recovery Under Load

**Issue:** BAN-147
**Milestone:** Phase 10 — Stress Testing, Soak Testing & Production Hardening
**Status:** Implemented / Verified

---

## 1. Goal

Hammer the `Governor` with extreme concurrency to verify:

- Zero deadlock under massive burst admission.
- Strict budget bounding: `MaxWorkers` and `MaxMemoryMB` never exceeded at any instant.
- Priority queue ordering with out-of-order dispatch (no head-of-line blocking).
- Clean lease release on context cancellation/timeout — zero resource leaks.
- No starvation of feasible tasks behind oversized waiters.

---

## 2. Test Suite (`internal/cortex/harness/p10_governor_stress_test.go`)

| Test | Scenario | Key Assertions |
|---|---|---|
| `TestBAN147_GovernorBurstAdmissionAndOrderStress` | 200 concurrent tasks (mixed priority) against `MaxWorkers=4`, `MaxMemoryMB=1000` | Peak workers ≤ 4, peak memory ≤ 1000 MB, all 200 admitted + completed, telemetry `Released == Admitted` |
| `TestBAN147_GovernorCancellationUnderHeavyQueue` | 50 tasks queued on full pool, 30 cancelled mid-queue, then 2 holders released | Exactly 30 cancellations, 20 drain cleanly, zero leak (`CurrentActive == 0`) |
| `TestBAN147_GovernorHeadOfLineBlockingAvoidance` | Base 600MB held → 800MB waiter queued → 200MB task admitted before large waiter | Small task bypasses stuck large waiter via out-of-order dispatch; large completes after base release |

---

## 3. Evidence

```text
=== RUN   TestBAN147_GovernorBurstAdmissionAndOrderStress
    p10_governor_stress_test.go:155: BAN-147 burst stress passed: 200 tasks, peakWorkers=4/4, peakMem=1000/1000MB, 0 leaks
--- PASS: TestBAN147_GovernorBurstAdmissionAndOrderStress (0.14s)

=== RUN   TestBAN147_GovernorCancellationUnderHeavyQueue
    p10_governor_stress_test.go:256: BAN-147 cancellation stress passed: 50 queued, 30 canceled cleanly, 20 admitted and released
--- PASS: TestBAN147_GovernorCancellationUnderHeavyQueue (0.15s)

=== RUN   TestBAN147_GovernorHeadOfLineBlockingAvoidance
    p10_governor_stress_test.go:336: BAN-147 head-of-line blocking avoidance passed: small task admitted while large waited
--- PASS: TestBAN147_GovernorHeadOfLineBlockingAvoidance (0.18s)

PASS
ok      github.com/RahmatHadinata23758051/CortexOS/internal/cortex/harness    0.582s
```

Command: `go test -run TestBAN147 ./internal/cortex/harness -v -count=1`

---

## 4. Metrics Table

| Budget Metric | Measured | Target | Result |
|---|---|---|---|
| Burst admission (200 tasks) | 200/200 admitted | 100% | **PASS** |
| Peak concurrent workers | 4 | ≤ 4 | **PASS** |
| Peak concurrent memory | 1000 MB | ≤ 1000 MB | **PASS** |
| Cancellation leak test | 0 leaked | 0 | **PASS** |
| Priority ordering (implicit) | Verified via wait-time stats | Critical < High < Normal < Low | **PASS** (from existing `TestPriorityOrderAdmission`) |
| Head-of-line blocking avoidance | 200MB task admitted while 800MB waited | No HOL blocking | **PASS** |
| Idempotent Release | Tested in existing suite | No double-decrement | **PASS** |

---

## 5. Files Added

- `internal/cortex/harness/p10_governor_stress_test.go` — 3 new stress tests.
- `docs/specs/ban147-governor-stress.md` — this document.