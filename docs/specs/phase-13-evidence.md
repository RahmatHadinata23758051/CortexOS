# Phase 13 Evidence: Brain ↔ Orchestra Routing and Tool Approval Gates

**Parent Epic:** BAN-155  
**Sprint:** BAN-156  
**Date:** 2026-10-13  
**Status:** Completed and verified  

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
- **BAN-162**: Sprint 13.3 Multi-Staff Coordination queued (`Todo`)
- **BAN-163**: Sprint 13.4 Knowledge Retrieval Grounding queued (`Todo`)
- **BAN-164**: Sprint 13.5 End-to-End Hardening queued (`Todo`)

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

Sprint 13.2 rollback target: `4abe8bc` (Phase 13 Sprint 13.1); Phase 12 fallback: `937e25d`.

