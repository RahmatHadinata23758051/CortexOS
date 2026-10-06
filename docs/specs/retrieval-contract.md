# CortexOS Deterministic Retrieval Contract

**Issue:** BAN-50  
**Format:** `cortexos.retrieval.v1`  
**Authority:** Markdown Vault content and SQLite metadata; the retrieval index is disposable.

## Scope

The first retrieval implementation is local, model-free, deterministic, bounded, and network-free. It provides search hints over approved Vault notes. It does not mutate notes, execute commands, or claim that a watcher event was successfully indexed.

## Document identity

Each indexed note produces one document:

```text
sha256:cortexos.retrieval.v1:<project-id>:<note-id>:<relative-path>
```

The implementation hashes the canonical tuple rather than trusting a caller-provided document ID. `ProjectID`, `NoteID`, and canonical Vault-relative path remain attached to every result. `SourceHash` is the Vault body hash observed during ingestion.

## Normalization and tokenization

- Input must be valid UTF-8.
- Unicode is lower-cased using Go Unicode rules.
- Runs of Unicode letters or numbers form tokens; punctuation and separators are boundaries.
- Query and indexed content use the same normalization.
- Empty queries are invalid; repeated whitespace and punctuation do not change token identity.
- The index stores the original human-readable content but scores normalized tokens.

## Ranking

For query tokens `q`, a document receives one point for each distinct query token present in its normalized content, plus one title-weight point for each distinct token present in the title prefix supplied by the adapter. The exact score is an implementation detail of this v1 contract; ordering is not:

1. higher score first;
2. lower document ID first;
3. lower project ID, note ID, and relative path as defensive tie-breakers.

A document appears once at most. Results are capped by the caller's positive limit and by an implementation maximum of 100. A limit above the maximum is clamped; zero or negative limits fail with `workspace.invalid_request`.

## Persistence and rebuild

The JSON index is a rebuildable derivative under the configured index root. Writes use a temporary file in the same directory, flush/close, and atomic rename. The index contains the format version and documents; it never contains secrets or raw process output.

- Missing index: treated as an empty index.
- Malformed JSON, unsupported version, invalid document identity, or invalid source hash: `workspace.retrieval_corrupt`.
- `Rebuild` reads the authoritative Vault store, constructs a complete replacement in memory, then atomically replaces the derivative. A failed rebuild leaves the previous derivative untouched.
- Incremental `Upsert` and `Remove` are explicit operations and persist the derivative after validation.
- Stale source hashes are returned as facts; retrieval never silently rewrites Markdown or SQLite.

## Cancellation and isolation

Every operation checks context before filesystem or source work and before committing a rebuilt index. The index is scoped to the configured root and ProjectID in each document. Querying a different project cannot return another project's document.

## Validation corpus

Tests must prove:

- golden corpus produces stable IDs and ordering;
- punctuation, case, and Unicode normalization are deterministic;
- result limits are bounded;
- attribution and source hash survive round trip;
- incremental upsert/remove;
- missing index and corrupt index recovery through rebuild;
- unsupported version and invalid document rejection;
- cancellation before and during rebuild;
- no network or provider key is required.
