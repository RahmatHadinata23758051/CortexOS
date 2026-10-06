# CortexOS Workspace Smoke Cockpit

**Issue:** BAN-53
**Scope:** Phase 2 validation surface only

The Workspace screen is a read-oriented smoke cockpit, not the production cockpit. It renders the typed runtime and Workspace snapshot, project/worktree facts, Vault/index health, loading state, and structured backend errors. The only mutation shown is project registration through the versioned Wails bridge.

The React layer has no filesystem, shell, SQL, Git, or process authority. It submits `cortexos.workspace.v1` DTOs to the bridge; path validation, persistence, duplicate detection, and error classification remain in Go. Empty, loading, malformed-response, duplicate, cancellation, and backend-error states are explicit.

The screen intentionally excludes Orchestra, Harness, Staff, Phaser, cloud sync, provider configuration, arbitrary commands, and production execution controls.
