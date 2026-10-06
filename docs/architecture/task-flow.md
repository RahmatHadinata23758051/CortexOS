# CortexOS Task and Validation Flow

**Issue:** BAN-27
**Authority:** Orchestra and Inspector

## Flow

```text
User goal
  |
  v
Plan + acceptance criteria
  |
  v
Task graph (DAG)
  | validate dependencies, scope, permissions
  v
Ready queue ------------------+
  |                           | blocked/canceled
  v                           v
Dispatch envelope       blocked event
  |
  +- Workspace context: project, worktree, allowed paths
  +- Staff context: role, skills, permissions
  +- Harness policy: tools, commands, limits, timeout
  +- Engine route: native / Pi / OMP
  |
  v
Worker execution
  |
  +- structured events and tool evidence
  +- files/tests/command results
  +- cancellation, timeout, or failure
  |
  v
Orchestra collects evidence
  |
  v
Inspector evaluates acceptance criteria
  +- accepted -> merge authority review -> integrated
  +- retryable -> bounded retry / alternate engine
  +- rejected -> remediation task
  +- blocked -> human/project-lead decision
```

## Dispatch envelope

Every execution must carry a durable envelope containing:

- task and plan identifiers;
- project and worktree identity;
- logical Staff identity and role;
- selected engine adapter and model route;
- allowed tools, paths, and commands;
- timeout, retry, and resource budget;
- acceptance criteria hash/version;
- correlation ID for events and artifacts.

The envelope is policy context, not a prompt shortcut. The worker cannot widen its permissions by editing the envelope.

## Evidence contract

Harness events should distinguish at least:

- `started`, `progress`, `tool_call`, `tool_result`;
- `file_changed`, `test_result`, `diagnostic`;
- `completed`, `failed`, `canceled`, `timed_out`.

Evidence must preserve timestamps, task/correlation IDs, exit status, redacted command details, affected paths, and artifact references. Secrets and raw provider credentials must never enter evidence.

## Inspector gate

Inspector evaluates the actual workspace and evidence against acceptance criteria. It should verify, as applicable:

1. expected files and contracts exist;
2. tests, format, lint, and type checks pass;
3. security and path-policy checks pass;
4. no unrelated or prohibited changes were introduced;
5. documentation and attribution requirements are satisfied;
6. the result is reproducible from the recorded command/configuration.

Only after Inspector passes may Orchestra request merge/integration. A Staff worker or engine adapter cannot emit a trusted `SUCCESS` event.

## Failure policy

- **Retryable:** transient engine/process/network failure, bounded by retry budget.
- **Recoverable:** route to another adapter or specialist Staff after recording why.
- **Rejected:** output exists but fails acceptance; create remediation, do not silently retry forever.
- **Blocked:** missing permission, dependency, human decision, license approval, or resource capacity.
- **Canceled:** explicit project/user cancellation; preserve partial evidence and workspace state.

Circuit breakers should stop repeated failures for the same task/route and surface a clear human decision point.
