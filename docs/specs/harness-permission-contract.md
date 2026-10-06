# Harness Permission Contract

**Version:** `cortexos.harness.policy.v1`
**Scope:** Phase 3 contract; runtime Tool Broker is Phase 4

## Decision model

```text
action + resource -> allow | ask | deny
```

Rules are evaluated in declaration order. The last matching rule wins. If no rule matches, the result is `deny`. Invalid policies and requests fail closed with typed errors.

Resource matching uses whole-value `*` and `?` wildcards. Backslashes are normalized to `/` before matching. Action matching is case-insensitive; resource text is otherwise preserved to avoid changing command semantics.

## Example

```text
shell + git status * -> allow
shell + npm install * -> ask
shell + git push * -> deny
read  + .env* -> deny
```

This contract does not grant process, filesystem, or network authority. A later Tool Broker must enforce worktree containment, timeouts, cancellation, output limits, redaction, and audit evidence around every execution.
