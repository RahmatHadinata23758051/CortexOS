# Orchestra Bridge Contract

**Version:** `cortexos.orchestra.bridge.v1`
**Scope:** Phase 3 typed application and platform bridge

## Ownership & Security Principles

1. **Path-Free and Shell-Free DTOs**: Frontend and Wails layers never receive filesystem paths, repository roots, raw SQL, shell handles, child processes, or secret tokens.
2. **Stable Redacted Error Codes**:
   - `orchestra.unsupported_version`
   - `orchestra.invalid_request`
   - `orchestra.not_found`
   - `orchestra.conflict`
   - `orchestra.storage_unavailable`
   - `orchestra.terminal`
   - `orchestra.invalid_graph`
   - `orchestra.canceled`
   - `orchestra.internal`
3. **Structured Timelines**: Timeline entries contain monotonic sequences, event types, state transitions, timestamps, and evidence summaries. Evidence IDs are redacted and bounded.
4. **Governed Mutations**: Cancel and retry requests are checked by schema version and dispatched through typed domain ports.
