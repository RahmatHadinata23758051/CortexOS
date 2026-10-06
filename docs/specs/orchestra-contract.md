# Orchestra Contract

**Version:** `cortexos.orchestra.v1`
**Scope:** Phase 3 — task planning and execution authority

## Ownership

Orchestra owns plans, task dependencies, task lifecycle, execution attempts, retry decisions, inspection requests, evidence references, and the final execution outcome. It is independent of Wails, React, Linear, Staff implementations, and engine adapters.

Workers and engines provide observations only. They cannot transition a task to `success` or approve a merge.

## Task lifecycle

```text
draft -> ready -> running -> awaitingInspection -> success
                         |                    |
                         v                    v
                      canceled              failed -> ready (explicit retry)
```

Cancellation is allowed from `draft`, `ready`, and `running`. Terminal `success` and `canceled` tasks cannot be mutated. Repeated commands must be handled idempotently by the application layer; state transitions themselves reject illegal transitions.

## Contract rules

- Every task has a stable ID, project/worktree scope, title, acceptance criteria, bounded attempts, and schema version.
- A task must be validated before dispatch.
- Dispatch increments the attempt count exactly once.
- Evidence collection moves execution to inspection; evidence is not a success verdict.
- Only inspection acceptance moves a task to `success`.
- Retry is explicit and bounded; retry and circuit-breaker policy is implemented in later Orchestra work.
- Persistence stores immutable events and redacted evidence references, not arbitrary process output.
