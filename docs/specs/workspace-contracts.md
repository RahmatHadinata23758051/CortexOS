# CortexOS Workspace Contracts

**Status:** Phase 2 entry contract  
**Issue:** BAN-41  
**Owner:** Workspace layer  
**Scope:** Wails-independent contracts for local projects, Git worktrees, SQLite state, Markdown Vault, file watching, and rebuildable retrieval

## Purpose

Workspace owns durable local workspace facts and controlled workspace mutations. It is the authority for project registration, repository/worktree isolation, local structured state, human-readable Markdown knowledge, file-change observation, and local retrieval derivatives.

Workspace does not decide whether execution succeeded, dispatch workers, run engines, approve task output, or merge changes. Orchestra, Harness, Staff, and engine adapters remain outside this contract.

## Dependency boundary

The Workspace contract package may depend on Go standard-library primitives and narrow shared value types only. It must not import:

- Wails or generated Wails bindings;
- React, TypeScript, or frontend assets;
- Linear SDK, MCP, or development-management clients;
- Orchestra, Harness, Staff, or engine adapter implementations;
- unrestricted OS command or shell helpers exposed to callers.

Platform adapters may implement Workspace ports, but the contract and service packages must not depend on platform lifecycle or UI packages.

## Contract version

The first persisted/bridge-visible Workspace contract is:

```text
cortexos.workspace.v1
```

Breaking changes require a new version or an explicit migration. Unknown versions fail closed; they must not silently downgrade to an older interpretation.

## Core entities

### Project

A registered local repository/workspace root.

Required facts:

- stable project ID;
- human-readable name;
- canonical repository root;
- canonical Vault root;
- default branch metadata when known;
- lifecycle status;
- created and updated timestamps;
- contract/schema version.

A Project is not a task and does not contain execution success state. Repository and Vault roots are canonicalized before persistence and must satisfy the path policy.

### Worktree

An isolated Git working directory associated with a registered Project.

Required facts:

- stable worktree ID;
- parent project ID;
- canonical worktree path under the configured worktree root;
- branch or detached revision identity;
- lifecycle state;
- dirty/clean observation;
- active reference metadata;
- created and updated timestamps.

A worktree is an isolation fact, not a permission grant. Allowed paths and command policy are owned by future Harness/Orchestra contracts.

### VaultNote

A human-readable Markdown document stored below an approved Vault root.

Required facts:

- stable note ID;
- project and optional worktree scope;
- canonical note-relative path, never an unrestricted absolute path at the API boundary;
- title and Markdown body;
- format version;
- source/author attribution;
- created and updated timestamps;
- content hash;
- lifecycle state.

Malformed or unknown metadata is an error. It must not be silently dropped during read or rewrite.

### FileChange

A policy-scoped observation that a file may have changed.

Required facts:

- event ID and correlation ID;
- approved root identity;
- canonical relative path;
- operation (`created`, `modified`, `removed`, `renamed`, `rescanRequired`, or `error`);
- observed timestamp;
- optional source content hash;
- overflow/error detail code where applicable.

A FileChange is an invalidation hint. It is not proof that a write completed, an index refresh succeeded, or an execution task passed.

### RetrievalDocument

A rebuildable derivative of authoritative Vault content.

Required facts:

- deterministic document ID;
- source note ID and canonical relative path;
- project/worktree scope;
- source content hash;
- retrieval index version;
- normalized searchable content or adapter-owned representation;
- attribution metadata;
- indexed timestamp.

RetrievalDocument can always be discarded and rebuilt from Markdown plus authoritative metadata. It must never be the only copy of a note or attribution.

### WorkspaceSnapshot

A bounded read-only summary for application and bridge consumers.

It may include:

- contract version;
- registered project summaries;
- worktree summaries;
- Vault note counts and last indexed state;
- watcher state and last observed error code;
- retrieval index version/status;
- redacted health and diagnostic codes.

It must not include raw database handles, unrestricted filesystem paths, shell commands, provider credentials, raw process output, or unredacted local secrets.

## Path policy

All path-bearing operations pass through one canonical policy before read, write, watch, delete, Git, or persistence mutation.

1. Accept only explicitly designated root or relative-path fields. Never accept a raw path plus an implicit authority level.
2. Resolve relative paths against a registered and approved root.
3. Canonicalize separators, volume/UNC syntax, case behavior where the platform requires it, and existing path components before containment checks.
4. Reject empty roots, relative roots where an absolute root is required, traversal components that escape the root, and paths outside the approved root.
5. Do not use a string-prefix comparison as the containment check; component boundaries and canonical paths must be checked.
6. Re-check containment immediately before mutation and after resolving existing links/reparse points where supported. A path that resolves outside the root is rejected.
7. Never follow a symlink/junction/reparse point into an unapproved root without an explicit, tested policy decision.
8. Database files, temporary files, worktrees, Vault content, and retrieval derivatives each have separate approved roots and cannot be substituted for one another.
9. Return safe relative identifiers to bridge consumers. Redact or omit machine-specific absolute paths unless an explicitly approved diagnostic contract requires them.
10. Tests must use temporary disposable roots and must cover `..`, mixed separators, missing roots, outside-root paths, root-prefix collisions, symlink/junction cases where supported, and cancellation.

Invalid or ambiguous paths fail closed with a typed error. No operation silently broadens the root.

## Query and mutation boundary

Queries are read-only and may include:

- project/worktree lookup and bounded listing;
- Vault note metadata/content retrieval by stable ID or approved relative identifier;
- watcher status;
- retrieval query and index health;
- WorkspaceSnapshot generation.

Mutations are explicit commands and must be cancellable:

- register/update/archive a project;
- create/inspect/remove a controlled worktree;
- create/update/delete a Vault note;
- start/stop a policy-scoped watcher;
- refresh/rebuild retrieval derivatives.

No query or mutation exposes arbitrary SQL, raw filesystem handles, shell commands, process arguments, provider credentials, or unvalidated absolute paths.

## Adapter ports

Ports are narrow interfaces owned by the Workspace contract/application package:

- `ProjectRegistry`: register, update, archive, get, and list projects;
- `WorktreeManager`: inspect, create, list, and remove controlled worktrees;
- `StateStore`: versioned transactions for authoritative structured metadata;
- `VaultStore`: note CRUD and bounded listing with hash/conflict semantics;
- `FileWatcher`: start/stop and typed change observation;
- `RetrievalIndex`: upsert/remove/query/rebuild derivative documents.

Every method accepts `context.Context` where it can block or perform I/O. Implementations must honor cancellation at safe boundaries and return a typed cancellation error rather than partially claiming success.

Ports must not require Wails, React, Linear, or a specific SQLite/Git/watcher/index implementation. Production adapters can be swapped behind these ports and must pass the shared contract tests.

## Error taxonomy

Errors are classified and mapped at the application boundary without leaking secrets or sensitive absolute paths.

| Code | Meaning | Required behavior |
| --- | --- | --- |
| `workspace.invalid_request` | Missing, malformed, unsupported, or conflicting request | Reject before mutation |
| `workspace.unsupported_version` | Unknown contract/schema/index version | Fail closed; require migration or rebuild |
| `workspace.path_denied` | Traversal, outside-root, unsafe link/reparse, or disallowed root | No filesystem mutation |
| `workspace.not_found` | Project, worktree, note, or derivative is absent | Return explicit absence |
| `workspace.conflict` | External note edit, duplicate identity, dirty cleanup, or concurrent mutation conflict | Preserve state; require caller decision |
| `workspace.not_git_repository` | Requested Git operation targets a non-Git root | Use documented fallback or reject explicitly |
| `workspace.migration_failed` | Schema migration cannot complete safely | Preserve prior state; do not claim readiness |
| `workspace.storage_unavailable` | Database or authoritative filesystem state unavailable | Surface hard error; no silent memory fallback |
| `workspace.watcher_overflow` | Watcher lost event fidelity | Emit rescan-required state |
| `workspace.retrieval_stale` | Derivative is behind source content | Return stale status; offer refresh/rebuild |
| `workspace.retrieval_corrupt` | Derivative cannot be read or validated | Discard/rebuild explicitly |
| `workspace.canceled` | Caller canceled or context expired | Preserve known state; no success verdict |
| `workspace.internal` | Unexpected adapter/application failure | Redacted diagnostic, correlation ID, no raw secrets |

Errors may wrap internal causes for logs/tests, but bridge-safe messages must use stable code plus safe detail. Full local paths, credentials, SQL, and raw process output are not bridge-safe details.

## Persistence ownership

- SQLite is authoritative for structured Project, Worktree, note metadata, watcher checkpoints, and retrieval metadata.
- Git is authoritative for repository history and worktree references.
- Markdown Vault content is authoritative for human-readable durable notes.
- Retrieval indexes are local derivatives and can be rebuilt.

A migration or index failure must never silently delete or replace authoritative content. Database migrations are versioned, transactional where supported, and fail closed on unknown or partial state.

## File watching semantics

Watchers are bounded, cancellable, and scoped to approved roots. Native events may be duplicated, reordered, or lost. The contract therefore requires:

- debouncing/coalescing according to a documented interval;
- explicit rename/delete handling;
- bounded delivery with an overflow event;
- a rescan/rebuild path after overflow or watcher failure;
- idempotent stop and no goroutine/resource leak;
- refresh consumers that re-read authoritative state before indexing.

A successful event delivery means only that an observation was delivered. It does not mean the file is valid Markdown or that retrieval refresh succeeded.

## Retrieval semantics

The first retrieval implementation is deterministic and local. It has no network or provider-key requirement.

- Document IDs and source hashes are stable for the same source and contract version.
- Query limits are bounded.
- Ranking ties use a documented stable ordering.
- Results include source note identity, attribution, and source hash.
- Missing, stale, or corrupt derivatives are explicit states.
- Rebuild reads authoritative Markdown and metadata, allowing complete derivative recovery.

## Cancellation and atomicity

Blocking operations observe context cancellation. Filesystem mutations use temporary files plus atomic rename where supported. SQLite mutations use transactions and migration records. Git cleanup inspects dirty/active state before destructive action. Watcher shutdown is idempotent. Retrieval refresh does not delete authoritative notes when indexing fails.

A canceled operation may have completed an indivisible underlying system call; its result must be reconciled and reported as an explicit state, never represented as an unverified success.

## Bridge exposure rules

The Wails bridge may expose versioned, structured Workspace queries and narrowly defined mutations only after application services validate them. React receives typed DTOs and stable error codes. The bridge must not expose:

- arbitrary absolute path reads/writes;
- arbitrary shell/Git commands;
- SQL statements or database files;
- watcher handles or OS event channels;
- raw index storage;
- secrets, provider keys, or raw process diagnostics.

## Validation checklist

- Contract package import scan contains no UI, Wails, Linear, Orchestra, Harness, Staff, or engine dependency.
- Path tests cover traversal, prefix collision, separator normalization, missing roots, links/reparse points, and cancellation.
- Adapter contract tests run against memory and production adapters.
- Persistence tests cover fresh install, reopen, migration, failed migration, uniqueness, and concurrent access.
- Vault tests cover malformed metadata, atomic writes, external edit conflicts, attribution, and deterministic listing.
- Watcher tests cover debounce, rename/delete, overflow, stop, cancellation, and failure.
- Retrieval tests cover deterministic ordering, source hashes, stale/corrupt state, rebuild, bounded results, and cancellation.
- Bridge tests prove no unrestricted filesystem or shell operation is exposed.
