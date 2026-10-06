# ADR-0002: Isolate project execution with Git worktrees

- **Status:** Accepted
- **Date:** 2026-10-06
- **Scope:** Workspace isolation

## Context

Multiple tasks and logical Staff may work on related repositories concurrently. Direct mutation of one shared checkout makes changes difficult to attribute, review, cancel, or roll back.

## Decision

CortexOS uses Git worktrees as the default isolation boundary for task execution. A task envelope identifies its project, worktree, branch, and allowed path scope. Integration into the canonical branch is controlled by Orchestra after inspection.

## Consequences

- Parallel work becomes reviewable and independently discardable.
- Worktree lifecycle, cleanup, disk quotas, and detached-state recovery require explicit services.
- Uncommitted changes and generated artifacts must be reported before cleanup.
- Non-Git projects need a documented fallback policy rather than silent shared mutation.
