# CortexOS Architecture Overview

**Status:** Foundation design
**Issue:** BAN-27
**Scope:** Runtime boundaries for the local-first desktop application

## Product boundary

CortexOS is a local-first desktop system for coordinating multiple projects, logical AI Staff, tools, memory, and validation. It runs on the user's machine and treats the workspace, execution evidence, and local state as primary. Linear is development management only and is not a CortexOS runtime dependency.

The runtime is divided into five conceptual layers plus replaceable engine adapters:

```text
â”Œâ”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”
â”‚ Workspace                                                     â”‚
â”‚ projects Â· worktrees Â· SQLite Â· Markdown Vault Â· retrieval    â”‚
â””â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”¬â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”˜
                               â”‚ durable workspace facts
â”Œâ”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â–¼â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”
â”‚ Orchestra                                                     â”‚
â”‚ plans Â· DAG scheduler Â· dispatch Â· retries Â· inspection       â”‚
â”‚ merge authority Â· execution outcome authority                 â”‚
â””â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”¬â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”˜
                               â”‚ governed task envelopes
â”Œâ”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â–¼â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”
â”‚ Harness                                                       â”‚
â”‚ Tool Broker Â· sandbox Â· routing Â· skills Â· JSONL lifecycle    â”‚
â”‚ resource governor Â· child-process policy                      â”‚
â””â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”¬â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”˜
                               â”‚ capabilities and evidence
â”Œâ”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â–¼â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”
â”‚ Staff                                                         â”‚
â”‚ logical roles Â· permissions Â· memory context Â· assigned task   â”‚
â””â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”¬â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”˜
                               â”‚ selected execution adapter
â”Œâ”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â–¼â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”
â”‚ Engine adapters                                               â”‚
â”‚ native Go Â· Pi Â· OMP Â· future engines                         â”‚
â””â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”˜
```

## Layer responsibilities

### Workspace

Owns project registration, repository/worktree isolation, filesystem boundaries, SQLite persistence, Markdown Vault notes, file watching, and local retrieval indexes. Workspace APIs expose facts and controlled mutations; they do not decide whether a task is successful.

### Orchestra

Owns the plan graph and task lifecycle. It validates dependencies, selects dispatch candidates, applies retry/circuit-breaker policy, requests inspection, records evidence, and controls integration/merge. Orchestra is the only runtime layer allowed to transition an execution to `SUCCESS`.

### Harness

Owns safe execution mechanics: tool allowlists, path and command policy, sandboxing, engine/model routing, skill injection, structured JSONL protocol, process lifecycle, cancellation, timeouts, and resource budgets. Harness reports observations and failures to Orchestra; it cannot approve its own work.

### Staff

Represents logical workers such as Developer, Research, Security, QC, Documentation, and Release. A Staff definition contains role, skills, permissions, memory context, and workspace assignment. Staff is not a process and is not permanently coupled to a particular engine.

### Engine adapters

Provide replaceable implementations behind a stable harness contract:

- Native Go tools for deterministic filesystem, Git, validation, and retrieval operations.
- Pi as the primary general coding engine.
- OMP as specialist/recovery execution where the policy allows it.
- Future engines without changing Workspace, Orchestra, Staff, or product contracts.

## Non-negotiable boundaries

1. Logical Staff must not map one-to-one to CLI processes. Use governed worker pools.
2. Engine output is evidence, never an execution verdict.
3. Inspector/Orchestra owns `SUCCESS`; a worker cannot self-approve.
4. All filesystem and command execution passes through Workspace/Harness policy.
5. Merge authority remains in Orchestra after inspection and validation.
6. Linear MCP is not imported into runtime packages.
7. Third-party harvest material is not source code until explicit license and attribution approval.

## Dependency direction

```text
UI / Wails bindings â†’ application services â†’ Orchestra â†’ Harness â†’ Workspace/platform
                                         â””â”€â”€â”€â”€â”€â”€â”€â”€â†’ Staff contracts
Engine adapters â†’ Harness contracts
Persistence and OS adapters â†’ Workspace/platform ports
```

Lower layers must not import UI concerns. Engine adapters must not import Orchestra internals. Staff definitions must depend on contracts, not process implementations. Cross-layer access should use narrow ports/interfaces and explicit command/query boundaries.

## Runtime state ownership

| State | Owner | Persistence | Notes |
| --- | --- | --- | --- |
| Project and worktree registration | Workspace | SQLite + Git metadata | Paths are validated and normalized |
| Plan, task, dependency, retry state | Orchestra | SQLite | Append execution events where auditability matters |
| Tool policy and process state | Harness | SQLite/local logs as appropriate | Secrets are never written to task output |
| Staff role and skill assignment | Staff/Orchestra | SQLite + versioned definitions | Staff remains logical |
| Notes and durable knowledge | Workspace | Markdown Vault + retrieval index | Source attribution is required |
| Execution evidence and validation | Orchestra | SQLite/artifact references | Evidence is immutable by event, not by worker claim |

## Task lifecycle

```text
Draft plan
  â†’ validated task graph
  â†’ ready queue
  â†’ dispatched with workspace + policy context
  â†’ running
  â†’ worker evidence collected
  â†’ Inspector validates acceptance criteria
  â†’ accepted / retryable failure / blocked / rejected
  â†’ merge or remediation
  â†’ completed with evidence
```

A task may only be marked complete when acceptance criteria, relevant tests, security checks, and Inspector review have passed. An engine exit code of zero is insufficient on its own.

## Resource model

The system manages active engine workers independently from logical Staff. The Resource Governor considers available memory, observed peak/p95 usage, CPU pressure, task priority, engine class, and active child processes. Initial budgets are configuration, not architectural identity: approximately four Pi workers and one OMP worker, subject to telemetry and machine capacity.
