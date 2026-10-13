# BAN-140: Structured Merge Audit Log and Observability Hooks

**Issue:** BAN-140  
**Status:** In Progress / Implemented  
**Approach:** Keep `log/slog` (per structured-logging-lite Go guidance), inject logger at adapter construction, no global-logger coupling in domain.

---

## 1. Why slog (no migration)
- `go.mod` requires Go 1.25, so the standard `log/slog` API is available.
- No repository evidence of an existing logging library: `grep` over `internal/` found no `log.`/`slog.`/`Logger` usage before this change.
- Desktop composition root (`internal/bootstrap`) does not yet configure a global logger; wiring slog through `ProductionAdapterConfig.Logger` keeps output ownership at the adapter boundary without forcing a full process-logging rollout in this issue.

---

## 2. Boundary map
| Layer | Logging ownership |
|---|---|
| `internal/logging` | Minimal `Logger` interface + `NopLogger`, `StdLogger` (slog JSON), `CapturingLogger` (tests) |
| `internal/cortex/orchestra.ProductionAdapter` | Emits merge lifecycle events via injected `Logger`; defaults to `NopLogger` (no stdout by default) |
| `internal/bootstrap` | Future: compose process logger and pass into `ProductionAdapterConfig`; out of scope for P9-04 tests |
| Domain (`Task`, `Inspector`, `MergeCommitter`) | Logger-free; returns typed errors/decisions |

---

## 3. Event and field contract

All events use snake_case keys. Cardinality is bounded: only fixed event names, task/worktree/execution IDs, statuses, and numeric counts/durations.

### 3.1 Lifecycle events (success path)
| Event | Level | Required fields | Answers |
|---|---|---|---|
| `orchestra_merge_started` | info | `task_id`, `note` | Which merge was requested? |
| `orchestra_merge_worktree_resolved` | debug | `task_id`, `worktree_id`, `worktree_revision`, `worktree_dirty`, `worktree_valid`, `duration_ms` | What worktree state gated the merge? |
| `orchestra_merge_evidence_resolved` | debug | `task_id`, `execution_id`, `evidence_count`, `duration_ms` | Which execution/evidence fed the Inspector? |
| `orchestra_merge_unblocked_tasks` | info (only when >0) | `task_id`, `unblocked_count`, `unblocked_task_ids`, `duration_ms` | Which dependent tasks became ready? |
| `orchestra_merge_committed` | info | `task_id`, `execution_id`, `unblocked_count`, `evidence_count`, `duration_ms` | Did the atomic commit persist? |

### 3.2 Rejection / failure events
| Event | Level | Required fields | Answers |
|---|---|---|---|
| `orchestra_merge_conflict` | warn | `task_id` | Was this a duplicate concurrent request? |
| `orchestra_merge_load_failed` | error | `task_id`, `error`, `duration_ms` | Did the store read fail? |
| `orchestra_merge_invalid_status` | warn | `task_id`, `current_status`, `expected_status`, `duration_ms` | Why was the merge rejected before inspection? |
| `orchestra_merge_missing_worktree` | error | `task_id`, `duration_ms` | Is task data missing a worktree binding? |
| `orchestra_merge_worktree_inspect_failed` | error | `task_id`, `worktree_id`, `error`, `duration_ms` | Did worktree verification fail? |
| `orchestra_merge_evidence_failed` | error | `task_id`, `error`, `duration_ms` | Did evidence resolution fail? |
| `orchestra_merge_no_execution` | error | `task_id`, `duration_ms` | Is the execution record absent? |
| `orchestra_merge_rejected_error` | warn | `task_id`, `execution_id`, `error`, `duration_ms` | Inspector error rejection (durable event also appended) |
| `orchestra_merge_rejected` | warn | `task_id`, `execution_id`, `reason`, `evidence_count`, `duration_ms` | Inspector non-acceptance (durable event also appended) |
| `orchestra_merge_rejection_persist_failed` | error | `task_id`, `execution_id`, `error` | Did rejection-event persistence fail? |
| `orchestra_merge_unblocking_failed` | error | `task_id`, `error`, `duration_ms` | Did plan-graph expansion fail? |
| `orchestra_merge_atomic_commit_failed` | error | `task_id`, `error`, `duration_ms` | Did `CommitMerge` fail (no partial state)? |
| `orchestra_merge_persist_*` (fallback path) | error | `task_id`, `error`, `duration_ms` | Non-`MergeCommitter` store partial failure (legacy/test doubles only) |

### 3.3 Privacy and cardinality
- **No secrets:** no request bodies, DSNs, tokens, raw evidence payloads, or file contents. `note` is operator-supplied and already persisted in the durable event message.
- **IDs stay in the body:** `task_id`, `execution_id`, `worktree_id`, `unblocked_task_ids` are log-body fields, never metric labels.
- **`reason` is bounded:** Inspector reasons are fixed enums/free text already persisted; no stack traces or upstream payloads.
- **Errors are safe strings:** `err.Error()` only; no wrapped sensitive context.

---

## 4. Verification
```text
go test ./internal/cortex/orchestra -run MergeTask -timeout 120s
ok  github.com/.../internal/cortex/orchestra  2.114s

go vet ./internal/logging/ ./internal/cortex/orchestra/
(clean)

go test ./internal/cortex/orchestra/... -timeout 120s
ok  orchestra / sqlite stores
```

Tests:
- `TestProductionAdapter_MergeTask_EmitsStructuredLogs` — success path asserts `started`, `worktree_resolved`, `evidence_resolved`, `committed`, and that every record carries `task_id`.
- `TestProductionAdapter_MergeTask_Rejected_EmitsStructuredLogs` — draft-status rejection asserts `invalid_status` with `current_status` and `duration_ms`.

---

## 5. Remaining risks / follow-ups
- `internal/bootstrap` does not yet inject a production `StdLogger`; adapter logs are currently silent (`NopLogger`) in the desktop binary until the composition-root wiring lands (planned with Phase 9 review).
- No log retention/collection outside the process (stdout in production); collector ownership belongs to the release/packaging follow-up.
