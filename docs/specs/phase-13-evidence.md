# Phase 13 Evidence: Brain ↔ Orchestra Routing, Tool Approval, and Staff Coordination

**Parent Epic:** BAN-155  
**Sprint:** 13.5 End-to-End Hardening and Release Readiness
**Date:** 2026-10-14
**Status:** Sprint 13.5 hardening slice implemented and verified; packaged desktop and race gates remain CI-owned

## Implemented slices

| Slice | Files | Status |
|---|---|---|
| Versioned routing contracts | `internal/cortex/application/brain_orchestra.go` | Implemented |
| Contract validation tests | `internal/cortex/application/brain_orchestra_test.go` | Implemented |
| Brain → Orchestra adapter | `internal/cortex/bridge/brain_orchestra_adapter.go` | Implemented |
| Adapter fail-closed tests | `internal/cortex/bridge/brain_orchestra_adapter_test.go` | Implemented |
| Orchestra authoritative task creation | `internal/cortex/orchestra/adapter.go`, `internal/cortex/application/orchestra.go` | Implemented |
| Service/Wails route and lifecycle events | `internal/cortex/application/service.go`, `internal/platform/bridge.go`, `internal/platform/events.go` | Implemented |
| Bootstrap wiring | `internal/bootstrap/bootstrap.go` | Implemented |
| Platform event test | `internal/platform/brain_orchestra_test.go` | Implemented |

## Contract and authority guarantees

- Brain output is advisory and must pass a bounded structured-plan contract before submission.
- `OrchestraPort.CreateTask` is the only task-creation boundary used by routing.
- Orchestra rejects duplicate task IDs and persists the `task.created` audit event.
- Route correlation IDs deterministically derive task IDs, making retries idempotent.
- Missing Brain/Orchestra wiring, canceled contexts, malformed plans, unsupported schema versions, and storage failures fail closed.
- Bridge lifecycle events contain only correlation/task IDs, sanitized status, schema version, and timestamp.
- No provider keys, process handles, absolute paths, raw provider output, or raw storage errors are included in bridge DTOs/events.

## Verification

```text
go test ./internal/cortex/application ./internal/cortex/bridge -count=1  PASS
go test ./internal/platform -count=1                                  PASS
go test ./internal/bootstrap -run TestOpenOrchestra -count=1          PASS
go test ./internal/cortex/orchestra -run "TestProductionAdapter|TestValidate|TestCreate" -count=1 PASS
go test ./internal/platform -run TestNoUnreviewedRemoteTransport -count=1 PASS
go vet ./internal/cortex/application ./internal/cortex/bridge ./internal/cortex/orchestra PASS
```

## Linear status

- **BAN-155**: Phase 13 Epic Advanced Staff Orchestration (`Done`)
- **BAN-156**: Phase 13 Sprint 13.1 completed (`Done`)
- **BAN-157**: 13.1.1 Define Brain-to-Orchestra application contract (`Done`)
- **BAN-158**: 13.1.2 Implement Brain-to-Orchestra routing adapter (`Done`)
- **BAN-159**: 13.1.3 Wire routing service, Wails bridge, and lifecycle events (`Done`)
- **BAN-160**: 13.1.4 Add routing integration evidence and regression gates (`Done`)
- **BAN-161**: Sprint 13.2 Tool Approval Gates and Governed Tool Calls completed (`Done`)
- **BAN-165**: 13.2.1 Define versioned tool proposal and approval contracts (`Done`)
- **BAN-166**: 13.2.2 Implement approval store and governed execution adapter (`Done`)
- **BAN-167**: 13.2.3 Wire approval service, Wails bridge, and lifecycle events (`Done`)
- **BAN-168**: 13.2.4 Add approval integration evidence and regression gates (`Done`)
- **BAN-162**: Sprint 13.3 Multi-Staff Coordination (`Done`)
- **BAN-169**: 13.3.1 Define bounded coordination snapshot and event contracts (`Done`)
- **BAN-170**: 13.3.2 Implement coordination projection and application service (`Done`)
- **BAN-171**: 13.3.3 Wire coordination Wails bridge, events, and Office/Cockpit mapping (`Done`)
- **BAN-172**: 13.3.4 Add multi-Staff integration, performance, and security evidence (`Done`)
- **BAN-163**: Sprint 13.4 Knowledge Retrieval Grounding (`Done`)
- **BAN-173**: 13.4.1 Define routing Knowledge grounding contract and budget policy (`Done`)
- **BAN-174**: 13.4.2 Implement retrieval grounding adapter via application port (`Done`)
- **BAN-175**: 13.4.3 Wire routing context into Brain→Orchestra, Wails bridge, and events (`Done`)
- **BAN-176**: 13.4.4 Add context inspection, provenance redaction tests, and evidence (`Done`)
- **BAN-164**: Sprint 13.5 End-to-End Hardening and Release Readiness (`Done`)
- **BAN-177**: 13.5.1 Backend E2E authority chain fixture & stress test (`Done`)
- **BAN-178**: 13.5.2 Frontend Phase 13 route & approval contracts and bridge helpers (`Done`)
- **BAN-179**: 13.5.3 Multi-route concurrency, duplicate dispatch, and cancellation stress tests (`Done`)
- **BAN-180**: 13.5.4 Rollback compatibility matrix, evidence documentation, and release review (`Done`)

## Sprint 13.5: End-to-End Hardening and Release Readiness Implemented Slices

| Slice | Files | Status |
|---|---|---|
| Complete authority chain integration fixture | `internal/cortex/orchestra/ban164_integration_test.go` | Implemented |
| Concurrency stress, duplicate dispatch & backpressure tests | `internal/cortex/orchestra/ban164_integration_test.go` | Implemented |
| Context cancellation & malformed fail-closed tests | `internal/cortex/orchestra/ban164_integration_test.go` | Implemented |
| Frontend Phase 13 route and approval contracts | `web/src/types/phase13.ts`, `web/src/types/index.ts` | Implemented |
| Frontend Phase 13 bridge helpers and tests | `web/src/bridge/phase13.ts`, `web/src/bridge/phase13.test.ts` | Implemented |
| Version baseline alignment | `internal/version/version.go`, `internal/version/version_test.go` | Resolved |

## Sprint 13.5 Contract and authority guarantees

- Complete authority chain proven: Brain (advisory) → Knowledge grounding (bounded) → Orchestra (authoritative task creation in `TaskDraft`) → Governed tool proposal → Compare-and-Set operator approval → Harness execution under policy → Evidence collection → Inspector / MergeAuthority gating (`TaskSuccess` only via Inspector).
- High-concurrency duplicate routing (50 concurrent calls, 10 keys x 5 duplicates) proven idempotent: exactly 10 tasks created, all duplicate calls return matching task IDs.
- Concurrent operator approvals race safely under CAS semantics: exactly one execution proceeds, all other attempts return idempotent/terminal state.
- Bounded event hooks deliver events reliably without deadlocks, blocking, or secret/absolute-path leakage.
- Context cancellation stops routing before task persistence; malformed/unstructured Brain output fails closed.
- Version baseline in `internal/version/version.go` aligned with `VERSION` (`v0.10.0-phase10`), resolving pre-existing test mismatch.

## Sprint 13.5 Verification

```text
go test ./internal/cortex/orchestra -run BAN164 -count=1 PASS (0.6s)
go test ./internal/cortex/application ./internal/cortex/bridge ./internal/platform ./internal/version -count=1 PASS
go vet ./internal/cortex/application ./internal/cortex/bridge ./internal/cortex/orchestra ./internal/platform ./internal/version PASS
npm --prefix web test PASS (35 files / 212 tests)
npm --prefix web run build PASS
npm --prefix web run lint PASS
npm --prefix web run typecheck PASS
```

## Rollback & Data-Contract Compatibility

- Sprint 13.5 rollback target: `ebc049b` (Phase 13 Sprint 13.4).
- Sprint 13.4 rollback target: `6a8e514` (Phase 13 Sprint 13.3).
- Sprint 13.3 rollback target: `0e57687` (Phase 13 Sprint 13.2).
- Sprint 13.1 fallback: `4abe8bc`; Phase 12 fallback: `937e25d`.
- Data-contract backward compatibility: all new schemas (`cortexos.brain-orchestra.bridge.v1`, `cortexos.tool-approval.proposal.v1`, `cortexos.tool-approval.decision.v1`, `cortexos.coordination.*`, `cortexos.knowledge.context.v1`) are additive and versioned. Pre-Phase 13 stores and endpoints continue functioning without migration blockers.


## Sprint 13.4: Knowledge Retrieval Grounding Implemented Slices

| Slice | Files | Status |
|---|---|---|
| Versioned context DTOs and budget policy | `internal/cortex/application/knowledge_context.go` | Implemented |
| Budget, deterministic ordering, timeout, empty, and redaction tests | `internal/cortex/application/knowledge_context_test.go`, `internal/cortex/bridge/knowledge_grounding_adapter_test.go` | Implemented |
| Application grounding port and adapter | `internal/cortex/application/service.go`, `internal/cortex/bridge/knowledge_grounding_adapter.go` | Implemented |
| Workspace Knowledge bridge adapter and tests | `internal/cortex/bridge/workspace_knowledge_adapter.go`, `internal/cortex/bridge/workspace_knowledge_adapter_test.go` | Implemented |
| Brain→Orchestra prompt grounding and route references | `internal/cortex/bridge/brain_orchestra_adapter.go`, `internal/cortex/application/brain_orchestra.go` | Implemented |
| Wails inspection method and grounding lifecycle event | `internal/platform/bridge.go`, `internal/platform/events.go` | Implemented |
| Frontend contract, bridge helper, inspector provenance/score display | `web/src/types/knowledge-context.ts`, `web/src/bridge/knowledge-context.ts`, `web/src/components/brain/ContextInspector.tsx` | Implemented |
| Go ↔ TypeScript context binding and latency/size checks | `web/src/integration/ban163_grounding_bindings.test.ts` | Implemented |

## Sprint 13.4 Contract and authority guarantees

- Context retrieval is advisory; unavailable, empty, malformed, or timed-out retrieval degrades without mutating Orchestra authority.
- Context is bounded by item, snippet, total-character, and timeout ceilings (`20`, `2000`, `8000`, `10000ms`).
- Results are deterministic: descending score, then document ID; route DTOs expose only opaque IDs, titles, and sanitized source labels.
- Absolute paths and credential-like text are replaced before prompt assembly or UI/event exposure.
- Prompt assembly is deterministic and uses the application grounding port rather than direct Knowledge store coupling.

## Sprint 13.4 Verification

```text
go test ./internal/cortex/application ./internal/cortex/bridge ./internal/platform ./internal/bootstrap -count=1 PASS
go vet ./internal/cortex/application ./internal/cortex/bridge ./internal/cortex/orchestra ./internal/platform ./internal/bootstrap PASS
npm --prefix web test PASS (34 files / 209 tests)
npm --prefix web run build PASS
npm --prefix web run lint PASS
npm --prefix web run typecheck PASS
```

Note: `go test -race` was attempted but the local Windows cgo toolchain failed before package compilation (`cgo.exe`); no runtime race result is claimed. CI Linux/macOS race jobs remain the authoritative gate. Packaged desktop smoke remains startup-only and is also CI-owned; functional Phase 13 behavior is covered by the backend fixture and frontend contract tests.

## Sprint 13.3: Multi-Staff Coordination Implemented Slices

| Slice | Files | Status |
|---|---|---|
| Bounded coordination snapshot & event contracts | `internal/cortex/application/coordination.go` | Implemented |
| Contract validation, sanitization & phase tests | `internal/cortex/application/coordination_test.go` | Implemented |
| Thread-safe coordination projection | `internal/cortex/application/coordination_projection.go` | Implemented |
| Projection refresh/snapshot & fail-closed tests | `internal/cortex/application/coordination_projection_test.go` | Implemented |
| Application service coordination port | `internal/cortex/application/service.go` | Implemented |
| Orchestra/Staff coordination source adapter | `internal/cortex/bridge/coordination_source.go` | Implemented |
| Source adapter tests | `internal/cortex/bridge/coordination_source_test.go` | Implemented |
| Wails bridge methods & lifecycle events | `internal/platform/bridge.go`, `internal/platform/events.go` | Implemented |
| Platform event mapping & leak tests | `internal/platform/coordination_test.go` | Implemented |
| Bootstrap coordination wiring | `internal/bootstrap/bootstrap.go` | Implemented |
| Frontend coordination contract | `web/src/types/coordination.ts` | Implemented |
| Frontend coordination bridge/subscription | `web/src/bridge/coordination.ts`, `web/src/bridge/coordination.test.ts` | Implemented |
| Office coordination panel | `web/src/office/CoordinationPanel.tsx`, `web/src/office/CoordinationPanel.test.tsx` | Implemented |
| Office scene avatar coordination mapping | `web/src/office/OfficeView.tsx`, `web/src/office/OfficeScene.ts` | Implemented |
| Go ↔ TypeScript binding contract test | `web/src/integration/ban162_coordination_bindings.test.ts` | Implemented |

## Sprint 13.3 Contract and authority guarantees

- Coordination is a read-only projection over authoritative Orchestra tasks and safe Staff summaries; the projection owns no domain mutations.
- Snapshot and event payloads are bounded (`MaxCoordinationTasks` 100, `MaxCoordinationStaff` 50, label/summary length caps) and rejected when exceeded.
- Absolute paths, credential-like tokens, and unsupported schema versions fail closed during validation and are never emitted.
- Dependency phases are computed deterministically with cycle protection; blocked state derives only from Orchestra-owned task statuses.
- Lifecycle events (`cortexos:coordination:changed|assigned|reassigned|progress|blocked|completed|canceled`) carry opaque IDs and sanitized status only.
- The Office scene updates avatars on governed lifecycle events (assignment state + current task signature), not on per-token output frequency.
- Unknown event types and unwired coordination ports fail closed without emission.

## Sprint 13.3 Verification

```text
go test ./internal/cortex/application ./internal/cortex/bridge -count=1  PASS
go test ./internal/platform ./internal/bootstrap -count=1               PASS
go vet ./internal/...                                                   PASS
npm --prefix web test                                                   PASS (32 files / 201 tests)
npm --prefix web run build                                              PASS
npm --prefix web run lint                                               PASS
npm --prefix web run typecheck                                          PASS
npx playwright test benchmarks/ban162-coordination.spec.ts              PASS
```

Note: `go test ./...` still reports the pre-existing `internal/version`
baseline mismatch (`VERSION` = `v0.10.0-phase10` vs `version.Version` =
`0.9.0-phase9`), which is unrelated to Sprint 13.3 and unchanged by this work.

## Sprint 13.2: Tool Approval Gates Implemented Slices

| Slice | Files | Status |
|---|---|---|
| Proposal & decision versioned contracts | `internal/cortex/application/tool_approval.go` | Implemented |
| Contract validation & secret rejection tests | `internal/cortex/application/tool_approval_test.go` | Implemented |
| In-process approval gate adapter | `internal/cortex/bridge/tool_approval_adapter.go` | Implemented |
| Approval gate fail-closed tests | `internal/cortex/bridge/tool_approval_adapter_test.go` | Implemented |
| Harness governed tool executor | `internal/cortex/bridge/harness_tool_executor.go` | Implemented |
| Harness tool executor tests | `internal/cortex/bridge/harness_tool_executor_test.go` | Implemented |
| Service & Wails approval methods | `internal/cortex/application/service.go`, `internal/platform/bridge.go` | Implemented |
| Lifecycle event constants & tests | `internal/platform/events.go`, `internal/platform/tool_approval_test.go` | Implemented |
| Bootstrap wiring with allowlist | `internal/bootstrap/bootstrap.go` | Implemented |

## Sprint 13.2 Verification

```text
go test ./internal/cortex/application ./internal/cortex/bridge -count=1  PASS
go test ./internal/platform -count=1                                  PASS
go test ./internal/bootstrap -count=1                                 PASS
go test ./internal/platform -run TestNoUnreviewedRemoteTransport -count=1 PASS
go vet ./internal/cortex/application ./internal/cortex/bridge ./internal/cortex/harness ./internal/platform ./internal/bootstrap PASS
```

## Rollback

Sprint 13.3 rollback target: `0e57687` (Phase 13 Sprint 13.2); Sprint 13.1 fallback: `4abe8bc`; Phase 12 fallback: `937e25d`.
Sprint 13.2 commit: `0e57687` (feat: add governed tool approval gates).

