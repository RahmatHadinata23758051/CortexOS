# Phase 13 Evidence: Brain ↔ Orchestra Routing, Tool Approval, and Staff Coordination

**Parent Epic:** BAN-155  
**Sprint:** 13.3 Multi-Staff Coordination
**Date:** 2026-10-13
**Status:** Sprint 13.3 completed and verified; full-suite version baseline remains pre-existing

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

- **BAN-155**: Phase 13 Epic created
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
- **BAN-163**: Sprint 13.4 Knowledge Retrieval Grounding queued (`Todo`)
- **BAN-164**: Sprint 13.5 End-to-End Hardening queued (`Todo`)

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

