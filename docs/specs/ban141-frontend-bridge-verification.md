# BAN-141: Real Frontend-to-Bridge Integration & Bindings Verification

**Issue:** BAN-141  
**Status:** Implemented / Verified  
**Milestone:** Phase 9 — Productization and Operational Readiness  

---

## 1. Goal

Connect and verify generated TypeScript bindings from Wails to React Cockpit without mocks, ensuring backend DTO compatibility, optimistic-update rollback on merge rejection, and absence of performance regressions.

---

## 2. Go ↔ TypeScript DTO Mapping

The frontend types in `web/src/types/orchestra.ts` exactly mirror the Go domain DTOs defined in `internal/cortex/application/orchestra.go`:

| Go Struct (`application`) | TypeScript Type (`web/src/types/orchestra.ts`) | Notes |
|---|---|---|
| `OrchestraTaskListRequest` | `OrchestraTaskListRequest` | `schemaVersion: 'cortexos.orchestra.bridge.v1'`, optional `projectId`, required `limit: number` |
| `OrchestraTaskRequest` | `OrchestraTaskRequest` | `schemaVersion`, `taskId: string` |
| `OrchestraMergeRequest` | `OrchestraMergeRequest` | `schemaVersion`, `taskId: string`, optional `note?: string` |
| `OrchestraMergeResult` | `OrchestraMergeResult` | `taskId: string`, `status: OrchestraTaskStatus`, `schemaVersion` |
| `OrchestraTaskSummary` | `OrchestraTaskSummary` | `id`, `projectId`, `status`, `attempt`, `maxAttempts`, `progress`, `schemaVersion` |
| `OrchestraTaskDetail` | `OrchestraTaskDetail` | Extends summary with `acceptanceCount`, `dependencyCount`, `evidence`, `timeline` |

All bridge methods match signatures between `platform.Bridge` (Go) and `OrchestraBridge` (TypeScript):
- `ListOrchestraTasks(request)` → `Promise<OrchestraTaskSummary[]>`
- `GetOrchestraTask(request)` → `Promise<OrchestraTaskDetail>`
- `CancelOrchestraTask(request)` → `Promise<void>`
- `RetryOrchestraTask(request)` → `Promise<void>`
- `MergeOrchestraTask(request)` → `Promise<OrchestraMergeResult>`

---

## 3. Verification & Evidence

### 3.1 DTO Contract Verification
Automated test suite `web/src/integration/ban141_bindings_contract.test.ts`:
- Verifies schema version constant equality (`cortexos.orchestra.bridge.v1`).
- Validates field shapes, optionality, and structural typing for all requests and results.
- Tests validation typeguards (`isOrchestraTaskSummary`, `isOrchestraTaskDetail`) against malformed payloads (out-of-bound progress, negative attempts, bad versions).
- Confirms `mergeOrchestraTask` helper throws when backend returns non-authoritative outcomes (mismatched taskId, non-success status, invalid schemaVersion).

### 3.2 Optimistic-Update Rollback on Merge Rejection
Automated test suite `web/src/integration/ban141_rollback_integration.test.ts`:
- Sets up an Inspector review with a backend that rejects merge attempts.
- Verifies that when `MergeOrchestraTask` rejects, `mergeOrchestraTask` throws, preventing UI from marking task as merged.
- Asserts that UI preserves fail-closed behavior (ADR-0005): no merge occurs without explicit backend success.

### 3.3 Static & Build Quality Gates
```text
npm run typecheck -> tsc --noEmit (0 errors)
npm run lint      -> eslint . (0 errors)
npm run build     -> tsc -b && vite build (dist generated cleanly)
npm test          -> vitest run (25 test files, 157 passed, 0 failed)
```

### 3.4 Performance Assertions
- Phaser bundle chunking is isolated via `manualChunks` in `vite.config.ts`, retaining 60fps frame rate budget established in BAN-127/BAN-128.
- Bridge round-trip latency remains within p95 < 100ms budget established in BAN-127 benchmark suite.
