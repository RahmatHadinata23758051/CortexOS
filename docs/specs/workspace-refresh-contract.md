# CortexOS Workspace Refresh Contract

**Issue:** BAN-51
**Format:** `cortexos.workspace-refresh.v1`

## Boundary

The refresh coordinator is an invalidation bridge between the policy-scoped file watcher and the rebuildable retrieval index. A watcher event is not an indexing success verdict, and a refresh request is not a mutation command. Markdown Vault and SQLite remain the authoritative sources.

## Request semantics

Each accepted request carries:

- project identity supplied when the coordinator starts;
- the approved watcher root identity;
- a canonical Vault-relative Markdown path, or an empty path for a rescan/error request;
- the watcher operation and observed timestamp;
- the source content hash when the file was readable at observation time.

Non-Markdown paths, empty ordinary paths, paths outside the configured root, and events from another root are discarded. Create/modify/delete/rename events are coalesced by relative path for the debounce window. Rescan and error events remain explicit and are never silently converted into success.

## Refresh behavior

The handler is invoked after a request is published. Handler failures are placed on a bounded failure channel and do not become successful watcher events. The default handler requests a project-scoped retrieval rebuild from the authoritative Vault store; it does not trust event content as the index source.

A full queue is observable loss of an audit hint, not a reason to block filesystem delivery. Callers can request an explicit rebuild after an overflow, error, or failed refresh.

## Shutdown

`Stop` cancels the coordinator, stops the underlying watcher, waits for the worker to finish, and is idempotent. A stopped coordinator can be started again only after the previous stop returns. No channel or goroutine is exposed as an authoritative persistence handle.
