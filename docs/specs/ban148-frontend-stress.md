# BAN-148: Frontend Bundle & Runtime Stress — 50+ Staff Avatars & 100 Concurrent Bridge Calls

**Issue:** BAN-148
**Milestone:** Phase 10 — Stress Testing, Soak Testing & Production Hardening
**Status:** Implemented / Verified

---

## 1. Goal

Extend the Playwright benchmark to stress the Virtual Office with:

- **60 concurrent Staff avatars** rendered in Phaser scene (strictly exceeding 50+ requirement)
- **100 concurrent bridge calls** (`ListStaffSummaries`) hammering the Wails bridge simultaneously
- Assertions:
  - ≥ 60 FPS steady-state rendering with 60 avatars on screen
  - Bridge p95 latency ≤ 100ms under massive parallel contention
  - Zero console errors, zero page errors
  - Frame budget compliance: p95 frame interval ≤ 16.67ms (1 frame @ 60 FPS) + small tolerance

---

## 2. Workload

| Dimension | Value | Notes |
|---|---|---|
| Staff avatars | 60 | 15 per zone × 4 zones (Coordination, Planning, Delivery, Review) |
| Bridge concurrency | 100 | `Promise.all` of 100 simultaneous `ListStaffSummaries` calls |
| Frame sample window | 3,000ms | ~180 animation frames at 60 FPS |
| Bridge p95 budget | 100ms | Generous headroom for WASM/bridge overhead |

Test: `web/benchmarks/ban148-frontend-stress.spec.ts`

Command:

```bash
cd web && npm run benchmark -- benchmarks/ban148-frontend-stress.spec.ts
```

---

## 3. Evidence (2026-10-13)

```
BAN-148 performance result {
  "baseline": {
    "platform": "win32",
    "arch": "x64",
    "os": "10.0.26200",
    "cpu": "AMD Ryzen 5 PRO 5650U with Radeon Graphics",
    "browser": "Chrome/140.0.7339.16",
    "viewport": "1280x720"
  },
  "workload": {
    "staffCount": 60,
    "frameSampleMs": 3000,
    "concurrentBridgeCalls": 100
  },
  "frames": {
    "count": 180,
    "p50Ms": 16.70,
    "p95Ms": 16.80,
    "p99Ms": 16.80,
    "meanMs": 16.59,
    "minMs": 2.20,
    "maxMs": 16.80,
    "effectiveFps": 60.29,
    "overBudget": 0
  },
  "concurrentBridge": {
    "count": 100,
    "p50Ms": 0.20,
    "p95Ms": 0.20,
    "p99Ms": 0.20,
    "meanMs": 0.19,
    "minMs": 0.10,
    "maxMs": 0.20
  }
}
```

```
  ok 1 benchmarks\ban148-frontend-stress.spec.ts:140:1 › BAN-148 Frontend runtime stress — 60 Staff avatars in Phaser scene and 100 concurrent bridge calls (6.8s)
  1 passed (48.7s)
```

---

## 4. Metrics vs. Budget

| Metric | Measured | Budget | Result |
|---|---|---|---|
| Effective FPS | **60.29** | ≥ 58 (allowing Windows CI margin) | **PASS** |
| Frame p95 interval | **16.80ms** | ≤ 16.67ms + 1.5ms tolerance | **PASS** |
| Over-budget frames | **0** | 0 | **PASS** |
| Bridge p95 latency | **0.20ms** | ≤ 100ms | **PASS** |
| Bridge p99 latency | **0.20ms** | — | **PASS** |
| Console errors | **0** | 0 | **PASS** |
| Page errors | **0** | 0 | **PASS** |
| Staff rendered | **60** | ≥ 50 | **PASS** |

---

## 5. Files Added

- `web/benchmarks/ban148-frontend-stress.spec.ts` — dedicated stress benchmark
- `docs/specs/ban148-frontend-stress.md` — this document
- Artifact: `test-results/ban148-performance.json` (auto-attached by Playwright)