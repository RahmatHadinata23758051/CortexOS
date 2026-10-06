# ADR-0003: Decouple logical Staff from engine processes

- **Status:** Accepted
- **Date:** 2026-10-06
- **Scope:** Staff and resource governance

## Context

CortexOS may expose 10â€“20 logical Staff, but starting one AI CLI process per Staff would exceed consumer-laptop memory budgets and make lifecycle control fragile.

## Decision

Staff are durable logical actors. Harness schedules their tasks onto replaceable, governed engine worker pools. Concurrency is adaptive using memory, CPU, task priority, engine class, and observed usage telemetry; worker counts are configuration limits, not Staff identity.

## Consequences

- Staff state survives worker replacement.
- Queueing, fairness, cancellation, and context handoff are required.
- Resource telemetry becomes part of scheduling correctness.
- Initial Pi/OMP worker budgets may be tuned without changing the domain model.
