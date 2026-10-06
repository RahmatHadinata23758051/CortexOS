# ADR-0004: Route execution through replaceable engine adapters

- **Status:** Accepted
- **Date:** 2026-10-06
- **Scope:** Harness and engine integration

## Context

Native deterministic tools, Pi, OMP, and future engines have different costs, protocols, and failure modes. Binding Orchestra or Staff directly to one CLI would make replacement and recovery expensive.

## Decision

Harness exposes a stable engine-adapter contract. Native Go tools handle deterministic operations where possible; Pi is the default general coding route; OMP is available for specialist/recovery routes. Adapters return structured events and evidence, not success authority.

## Consequences

- Routing policy and capability discovery are explicit.
- Each adapter needs lifecycle, timeout, cancellation, redaction, and health behavior.
- Engine-specific prompts/protocol details stay behind the adapter boundary.
- Integration tests must exercise the contract independently of a live provider.
