# CortexOS ADR Index

Architecture Decision Records are short, immutable decision checkpoints. A new ADR supersedes an old decision; it does not silently rewrite history.

| ADR | Decision | Status |
| --- | --- | --- |
| [ADR-0001](./ADR-0001-local-first-storage.md) | Keep primary workspace state local-first | Accepted |
| [ADR-0002](./ADR-0002-worktree-isolation.md) | Isolate project execution with Git worktrees | Accepted |
| [ADR-0003](./ADR-0003-governed-worker-pools.md) | Decouple logical Staff from engine processes | Accepted |
| [ADR-0004](./ADR-0004-engine-adapters.md) | Route execution through replaceable engine adapters | Accepted |
| [ADR-0005](./ADR-0005-validation-authority.md) | Make Inspector/Orchestra the only success authority | Accepted |

## ADR format

Each ADR records context, decision, consequences, rejected alternatives, and validation implications. Implementation issues must reference the relevant ADR rather than duplicating architectural policy.
