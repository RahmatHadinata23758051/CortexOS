# ADR-0001: Keep primary workspace state local-first

- **Status:** Accepted
- **Date:** 2026-10-06
- **Scope:** Workspace and application state

## Context

CortexOS manages repositories, worktrees, task execution, local memory, and validation evidence. These must remain useful when network access or external management services are unavailable. Sensitive workspace metadata should not require a hosted control plane.

## Decision

Primary runtime state is stored locally. SQLite owns structured application state, Git owns repository history/worktrees, and the Markdown Vault owns human-readable durable knowledge. Retrieval indexes are local derivatives that can be rebuilt. External services may be optional integrations, never a runtime prerequisite for core execution.

## Consequences

- Offline operation is a first-class path.
- Backup/export and migration become explicit product responsibilities.
- Local encryption/keychain policy must be designed before storing sensitive credentials.
- Rebuildable indexes must have versioned ingestion contracts.
