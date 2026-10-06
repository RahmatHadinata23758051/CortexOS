# ADR-0005: Make Inspector/Orchestra the only success authority

- **Status:** Accepted
- **Date:** 2026-10-06
- **Scope:** Validation and integration

## Context

An engine can exit successfully while producing incomplete, unsafe, or incorrect changes. Staff are incentivized to report progress, not to self-certify acceptance.

## Decision

Inspector evaluates workspace state and execution evidence against acceptance criteria. Orchestra owns the final task outcome and merge authority. Worker and engine messages are untrusted evidence until inspected.

## Consequences

- Acceptance criteria must be machine-checkable where possible.
- Tests, diff review, security checks, and documentation checks become explicit gates.
- The system can distinguish process success from solution correctness.
- Inspector failures must produce actionable remediation or a human blocker, never silent completion.
