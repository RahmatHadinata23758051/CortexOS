# BAN-135: Phase 8 P8-02 Atomic Merge Commit & Concurrency Hardening

## Overview
BAN-135 resolves the residual risk of non-atomic merge persistence identified in BAN-134. Previously, the production adapter updated tasks, appended events, and modified dependent tasks in separate database calls, leaving windows where partial failures could leave an accepted task without an audit event or leave dependent tasks blocked.

This implementation establishes an authoritative, atomic merge persistence pipeline across in-memory and SQLite stores.

## Architecture & Data Contracts

### 1. Atomic Merge Commit Contract (`internal/cortex/orchestra/merge_commit.go`)
- `MergeCommit`:
  - `MergedTask`: Transitioned to `TaskSuccess`.
  - `AcceptedEvent`: Timeline record of type `EventInspectionAccepted`.
  - `UnblockedTasks`: Dependent tasks transitioned from `TaskDraft` to `TaskReady`.
  - `ValidatedEvents`: Timeline records of type `EventTaskValidated`.
- `MergeCommitter`:
  - Optional storage interface implemented by stores that can commit the complete payload inside a single transaction.

### 2. SQLite Atomic Persistence (`internal/cortex/orchestra/sqlite/tasks.go`)
- Executed inside a `sql.LevelSerializable` transaction.
- Compare-and-Set updates:
  - Merged task: `UPDATE orchestra_tasks ... WHERE id = ? AND status = 'awaitingInspection'`
    - Fails with `ErrMergeConflict` if rows affected is not exactly 1.
  - Dependent tasks: `UPDATE orchestra_tasks ... WHERE id = ? AND status = 'draft'`
    - Fails with `ErrMergeConflict` if rows affected is not exactly 1.
- Event sequence generation occurs inside the transaction using `MAX(sequence) + 1`.
- Any error triggers `tx.Rollback()`, guaranteeing all-or-nothing semantics.

### 3. Production Adapter Integration (`internal/cortex/orchestra/adapter.go`)
- Merges are protected by:
  1. Per-task in-process lock (`activeMerge` map) to reject concurrent calls in the same process with immediate feedback.
  2. Transactional compare-and-set in SQLite for cross-process and distributed safety.
- `MergeTask`:
  - Validates worktree cleanliness and immutable evidence.
  - Passes request to `MergeAuthority.Inspect`.
  - Computes dependent plan unblocking in-memory.
  - Dispatches `MergeCommit` payload to `MergeCommitter`.
  - Fallback logic remains present for testing doubles without transaction capabilities.

## Verification & Test Matrix

1. **Unit & Store Atomicity (`internal/cortex/orchestra/sqlite/merge_test.go`)**
   - `TestCommitMerge_AtomicSuccess`: Verifies parent and multiple child tasks persist together.
   - `TestCommitMerge_ConflictRejectsConcurrentStateChange`: Proves stale tasks reject out-of-order commits.
   - `TestCommitMerge_RollsBackOnDependentTaskFailure`: Injected bad dependent task status rolls back parent and prevents events.
   - `TestCommitMerge_MemoryStoreRoundTrip`: Validates in-memory store adherence to the contract.

2. **Concurrency & Restart Persistence (`internal/cortex/orchestra/adapter_atomic_test.go`)**
   - `TestProductionAdapter_MergeTask_ConcurrentOnlyOneSucceeds`: 4 concurrent goroutines; exactly 1 succeeds, exactly 1 event created.
   - `TestCommitMerge_PersistsAcrossReopen`: Ensures all updates, events, and ready states survive complete database closure and reopen.
   - `TestProductionAdapter_MergeTask_NonAtomicFallbackReportsFailure`: Injected task save error propagates error cleanly.

3. **End-to-End Suite (`internal/cortex/orchestra/adapter_integration_test.go`)**
   - Verified bridge-to-SQLite end-to-end execution remains 100% green.
