# BAN-145: Sequential Soak 10k Merges — WAL Size, Handle Leaks, RSS Drift

**Issue:** BAN-145
**Milestone:** Phase 10 — Stress Testing, Soak Testing & Production Hardening
**Status:** Implemented / Verified (full 10k PASS)

## 1. Goal

Long-running sequential merge soak (10,000 cycles) asserting:

- SQLite DB file stays bounded (< 512 MiB).
- SQLite WAL file stays bounded (< 64 MiB, currently 0 B — rollback-journal mode).
- No goroutine leak (end <= base + 10).
- No heap blow-up (end <= 4x base HeapAlloc).
- No latency degradation > 5x between first-100 and last-100 cycle averages.
- All 10,000 merges succeed; last task persists as `TaskSuccess`.

## 2. Workload

Each cycle performs a full production merge path through `ProductionAdapter.MergeTask`:

1. Seed task in `TaskAwaitingInspection` with unique execution + evidence record.
2. `MergeTask` → worktree inspect → execution/evidence resolve → Inspector gate → CAS `CommitMerge`.
3. Record per-cycle latency in microseconds; sample early-100 vs late-100 windows.

Test: `TestBAN145_SequentialSoak10k` in `internal/cortex/orchestra/p10_stress_test.go`.

Command:

```powershell
go test -run TestBAN145_SequentialSoak10k ./internal/cortex/orchestra -v -count=1 -timeout 900s
```

## 3. Interim Observation

- 1000/10000 cycles: elapsed ~43s, last-op ~47ms, db ~823,296 B, wal 0 B.
- Growth looks linear (~800 B/cycle); projected final DB ~8 MB — well under the 512 MiB budget.
- WAL stays 0 B because neither store enables `PRAGMA journal_mode=WAL`; the suite monitors the `-wal` path but the engine currently runs in rollback-journal mode. This is documented as a scope note, not a failure.

## 4. Evidence

### 4a. Reduced soak 1000 cycles — PASS (new code, 2026-10-13)

```text
=== RUN   TestBAN145_SequentialSoak10k
    p10_stress_test.go:429: BAN-145 progress: 1000/1000 elapsed=39s last-op=38887us db=823296B wal=0B
    p10_stress_test.go:442: BAN-145 soak: cycles=1000 elapsed=38.75s per-merge=38.75ms measured-total=38.738652s early-avg=34.216ms late-avg=38.751ms db=823296B wal=0B goroutines=4->4 heap=530736B->665064B
--- PASS: TestBAN145_SequentialSoak10k (39.06s)
PASS
ok      github.com/RahmatHadinata23758051/CortexOS/internal/cortex/orchestra    41.171s
```

Command: `$env:BAN145_CYCLES="1000"; go test -run TestBAN145_SequentialSoak10k ./internal/cortex/orchestra -v -count=1 -timeout 240s`

Key points:

- All 1000 merges succeeded; last task persisted as `TaskSuccess`.
- No latency degradation (early-avg 34.2ms vs late-avg 38.8ms, ratio ~1.13x, budget 5x).
- DB bounded (823 KB, budget 512 MiB); WAL 0 B (budget 64 MiB).
- No goroutine leak (4 → 4).
- Heap stable after GC (531 KB → 665 KB, +134 KB, far under the 4x + 64 MiB guard).

### 4b. Full 10k soak — Completed PASS (2026-10-13)

```text
=== RUN   TestBAN145_SequentialSoak10k
    p10_stress_test.go:429: BAN-145 progress: 1000/10000 elapsed=48s last-op=33880us db=823296B wal=0B
    p10_stress_test.go:429: BAN-145 progress: 2000/10000 elapsed=1m42s last-op=133822us db=1576960B wal=0B
    p10_stress_test.go:429: BAN-145 progress: 3000/10000 elapsed=3m9s last-op=51092us db=2342912B wal=0B
    p10_stress_test.go:429: BAN-145 progress: 4000/10000 elapsed=4m19s last-op=162563us db=3100672B wal=0B
    p10_stress_test.go:429: BAN-145 progress: 5000/10000 elapsed=5m58s last-op=92034us db=3854336B wal=0B
    p10_stress_test.go:429: BAN-145 progress: 6000/10000 elapsed=7m31s last-op=64181us db=4608000B wal=0B
    p10_stress_test.go:429: BAN-145 progress: 7000/10000 elapsed=9m4s last-op=91085us db=5365760B wal=0B
    p10_stress_test.go:429: BAN-145 progress: 8000/10000 elapsed=10m53s last-op=101558us db=6139904B wal=0B
    p10_stress_test.go:429: BAN-145 progress: 9000/10000 elapsed=12m23s last-op=61524us db=6889472B wal=0B
    p10_stress_test.go:429: BAN-145 progress: 10000/10000 elapsed=13m55s last-op=90243us db=7655424B wal=0B
    p10_stress_test.go:442: BAN-145 soak: cycles=10000 elapsed=13m54.51s per-merge=83.451ms measured-total=13m54.424496s early-avg=43.729ms late-avg=97.527ms db=7655424B wal=0B goroutines=4->4 heap=524568B->841168B
--- PASS: TestBAN145_SequentialSoak10k (834.86s)
PASS
ok      github.com/RahmatHadinata23758051/CortexOS/internal/cortex/orchestra    838.862s
```

Command: `go test -run TestBAN145_SequentialSoak10k ./internal/cortex/orchestra -v -count=1 -timeout 1200s`

#### Metrics against Production Hardening Budgets:

| Budget Metric | Value Measured | Acceptance Target | Result |
|---|---|---|---|
| Total Successful Merges | 10,000 / 10,000 | 10,000 | **PASS** |
| Latency Degradation | 2.23x (43.7ms early vs 97.5ms late) | < 5x | **PASS** |
| Average Merge Latency | 83.45ms per full lifecycle merge | < 250ms | **PASS** |
| Final SQLite DB Size | 7,655,424 B (~7.3 MiB) | < 512 MiB | **PASS** |
| Final WAL Size | 0 B (rollback journal) | < 64 MiB | **PASS** |
| Goroutine Leak | 4 -> 4 (0 leaked) | base + 10 | **PASS** |
| Heap Growth | 524 KB -> 841 KB (+316 KB) | < 4x and < 64 MiB | **PASS** |
| Terminal Task State | `TaskSuccess` persisted on cycle 9999 | `TaskSuccess` | **PASS** |

## 5. Scope Notes

- The `-wal` assertion is a guardrail for future WAL enablement; current stores use default journal mode, so `wal=0B` is expected.
- Heap/goroutine bounds are process-wide and deliberately generous (4x heap, +10 goroutines) to avoid flakes on shared CI runners.
- If per-cycle latency (~40ms on Windows) proves too slow for CI, a reduced `BAN145_CYCLES` env override may be introduced without changing assertions.
