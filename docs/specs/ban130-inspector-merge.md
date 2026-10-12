# BAN-130 — Inspector merge command (P7-05)

## Scope

Replace the simulated client-side merge in `useInspector` (a fixed `setTimeout`
that flipped `mergeState` to `merged` on its own) with a typed backend command.
Authority stays with Inspector → Orchestra (ADR-0005). React remains
presentation-only; the Go application layer owns the command contract.

## Contract

| Layer | Artifact | Behavior |
| --- | --- | --- |
| Application | `OrchestraPort.MergeTask(ctx, id, note)` | Only port allowed to move a task to `success`. |
| Application | `Service.MergeOrchestraTask` | Validates schema and task id, fails closed with `orchestra.storage_unavailable` when no port is wired. Maps `ErrInspectionRejected` / `ErrMissingEvidence` / `ErrSelfReportedSuccess` to `orchestra.inspection_rejected`. |
| Platform | `Bridge.MergeOrchestraTask` | Thin pass-through; no business logic. |
| Bridge (TS) | `OrchestraBridge.MergeOrchestraTask` + `mergeOrchestraTask()` | Accepts only `status: success` with matching `taskId` and schema; anything else throws. |
| Hook | `useInspector(task, { mergeCommand, onMerged })` | `merged` only after the command resolves. On error the state returns to `idle` and `mergeError` is shown. |
| Gate | `canRequestMerge(state, hasCommand)` | Pure: requires `accepted`, `idle`, and a wired command. |

## Fail-closed rules

1. No Inspector acceptance → merge disabled, command not called.
2. No bridge / command → merge disabled; attempting it shows an error, no state change to `merged`.
3. Backend rejection or non-success response → `mergeState` back to `idle`, error shown, no success banner, no audit "merged" entry.
4. Mismatched task id or schema in the response → treated as non-authoritative.

## Tests

- Go: `TestOrchestraMergeBridgeReturnsAcceptedTask`, `TestOrchestraMergeBridgeFailsClosed` (unwired, rejected, empty id, wrong schema).
- TS bridge: `merges only through the backend command…`, non-success, mismatched identity, backend rejection.
- TS gate: `src/hooks/useInspector.test.ts` (pure `canRequestMerge`).
- Integration: `ban124_e2e.test.ts` drives the full lifecycle through `mergeOrchestraTask`.

## Residual risk

- `internal/bootstrap/bootstrap.go` does not wire Orchestra. Production merge therefore fails closed. Enabling merge needs an Orchestra adapter that loads the task, execution, worktree state and governed evidence, runs `MergeAuthority.Inspect`, persists `TaskSuccess`, and appends `inspection.accepted`. That adapter is a follow-up, not part of this change.
- The Inspector accept decision is still client-side presentation state. Only merge is backed by the backend command in this slice.
