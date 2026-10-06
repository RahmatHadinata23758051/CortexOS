# CortexOS Markdown Vault Contract

**Issue:** BAN-47  
**Status:** Phase 2 contract  
**Owner:** Workspace Vault layer  
**Format:** `cortexos.vault.v1`

## Purpose

The Markdown Vault is the human-readable authoritative content layer for durable Workspace notes. SQLite stores structured metadata and synchronization facts; retrieval indexes are rebuildable derivatives. A retrieval or database failure must never silently replace or delete Markdown content.

## File layout

A note is stored as a Markdown file below the registered Project Vault root:

```text
<vault-root>/<approved-relative-path>.md
```

The API accepts only a validated note-relative path. Absolute paths, traversal, volume-qualified paths, symlink escapes, and paths outside the registered Vault root are rejected by the Workspace path policy.

The `.md` extension is required for persisted notes. Directory names and filenames must be non-empty, must not be `.` or `..`, and must not contain control characters. Slug generation is deterministic but callers may choose nested approved relative paths.

## Document format

Every durable note starts with a strict YAML-like front matter block delimited by `---` on its own line, followed by a blank line and the human-editable Markdown body:

```markdown
---
id: note-01
project_id: project-01
worktree_id: worktree-01
relative_path: decisions/storage.md
title: Storage Decision
format_version: cortexos.vault.v1
source: manual
author: Rahmat HadiNata
created_at: 2026-10-06T12:00:00Z
updated_at: 2026-10-06T12:00:00Z
content_hash: sha256:...
---

# Storage Decision

The body remains ordinary Markdown.
```

The parser accepts one scalar `key: value` per metadata line. Values may be plain UTF-8 scalars or double-quoted strings with JSON escaping. Empty values are invalid for required fields. Multiline YAML, aliases, anchors, arbitrary nested objects, duplicate keys, tabs for indentation, and unrecognized keys are rejected rather than guessed.

The body begins after the closing delimiter and the required blank separator. Body bytes are preserved as UTF-8 content; the parser does not interpret Markdown.

## Required metadata

| Field | Rule |
| --- | --- |
| `id` | Stable non-empty note identity; unique within the Workspace state. |
| `project_id` | Stable owning Project identity; required even for global-looking notes. |
| `worktree_id` | Optional stable Worktree identity; omitted for project-wide notes. |
| `relative_path` | Canonical approved Vault-relative path, including `.md`. |
| `title` | Non-empty human-readable title. |
| `format_version` | Exactly `cortexos.vault.v1`; unknown versions fail closed. |
| `source` | Non-empty attribution such as `manual`, `worker`, or `import`. |
| `author` | Non-empty attribution identity. |
| `created_at` | RFC3339 timestamp. |
| `updated_at` | RFC3339 timestamp, not earlier than `created_at`. |
| `content_hash` | `sha256:<lowercase-hex>` hash of the canonical Markdown body bytes. |

The parser returns typed metadata and body. It does not silently infer missing attribution, identity, timestamps, or hashes.

## Canonical hashing

`content_hash` is calculated over the exact UTF-8 body bytes after the required metadata separator. Newline normalization is performed only by the writer before hashing and writing. A reader compares the declared hash with the body hash and returns a conflict/corrupt-content error when they differ.

A metadata rewrite must preserve the body bytes. A body edit changes `updated_at` and `content_hash` together.

## Attribution and scope

Every note is attributed by `source` and `author`. Workspace services may restrict allowed values for a particular command, but the file format preserves both values for human review and future migration.

`project_id` is authoritative ownership. `worktree_id`, when present, must reference a Worktree belonging to the same Project. The Vault adapter must not use a front matter project ID to widen filesystem authority; the requested Project and approved Vault root remain the security boundary.

## Unknown and malformed input

The following are explicit errors:

- missing opening/closing delimiter;
- missing required field;
- unknown field;
- duplicate field;
- malformed scalar or timestamp;
- unsupported format version;
- invalid identity or relative path;
- invalid `.md` path;
- body hash mismatch;
- invalid UTF-8;
- `updated_at` earlier than `created_at`.

A malformed file is never silently converted into an empty note, partially parsed note, or retrieval document. The original bytes remain untouched for remediation.

## Conflict policy

The Vault adapter uses the last known `content_hash` as an optimistic concurrency token:

1. Read and parse the current file.
2. Compare its body hash with the caller's expected hash.
3. If they differ, return `workspace.conflict` and do not overwrite.
4. Write a complete temporary file in the same directory.
5. Flush/close it and atomically rename it over the target where supported.
6. Re-read and validate the resulting file before reporting success.

Delete follows the same expected-hash rule when a caller has a known version. A missing file is an explicit `workspace.not_found`, not a successful delete unless the service command is explicitly idempotent.

## SQLite synchronization

SQLite records note metadata, path, source hash, and synchronization state. Markdown body remains authoritative and human-editable. Synchronization is transactional for metadata but cannot claim file success until the atomic filesystem operation and validation complete. A failed metadata update must not delete Markdown; a failed Markdown write must not claim synchronized metadata.

## Migration

Format changes require a new `format_version` or a documented, tested migration. Unknown versions fail closed. Migrations must preserve note ID, attribution, body, and source hash. No migration may silently drop unknown metadata.

## Validation fixtures

The contract test corpus must include:

- valid project-scoped note;
- valid worktree-scoped note;
- missing required field;
- unknown field;
- duplicate field;
- malformed delimiter;
- unsupported version;
- malformed timestamp;
- invalid path/traversal;
- body hash mismatch;
- invalid UTF-8;
- deterministic parse/serialize round trip;
- external-edit conflict token.
