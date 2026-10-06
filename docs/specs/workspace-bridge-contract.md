# CortexOS Workspace Bridge Contract

**Issue:** BAN-52
**Format:** `cortexos.workspace.v1`

## Boundary

The Wails bridge exposes only typed Workspace application operations. It does not expose raw filesystem handles, absolute roots, SQL, shell commands, process arguments, provider credentials, or generated frontend internals.

The application layer validates the contract version and maps domain errors to stable `WorkspaceBridgeError` codes with safe, path-free messages. Unknown versions fail closed. Cancellation is returned as `workspace.canceled`; it is never presented as a successful snapshot or mutation.

## Operations

- `GetWorkspaceSnapshot`: bounded redacted projects/worktrees and watcher/retrieval status.
- `QueryWorkspace`: project-scoped retrieval results with note IDs, relative paths, source hashes, attribution, timestamps, and bounded excerpts.
- `RegisterWorkspaceProject`: explicit project mutation with approved roots supplied only to the application boundary; responses omit machine-specific roots.
- `RebuildWorkspaceRetrieval`: explicit project-scoped derivative rebuild.

Every request includes `schemaVersion`. DTO fields are mirrored in `web/src/types/workspace.ts`; frontend validation rejects malformed snapshots before rendering.

## Error safety

Bridge messages are stable and intentionally generic. Detailed local paths, SQL, raw process output, and underlying causes remain outside the Wails/TypeScript response. Structured error codes include `workspace.invalid_request`, `workspace.unsupported_version`, `workspace.path_denied`, `workspace.conflict`, `workspace.canceled`, and storage/retrieval health codes.
