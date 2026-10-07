# Harness Contract

**Version:** `cortexos.harness.v1`
**Issue:** BAN-93
**Scope:** Phase 4 Harness contracts and Tool Broker boundary

## Purpose and ownership

Harness owns safe execution mechanics. It defines the typed boundary between Orchestra/Staff dispatch and replaceable execution engines without importing Orchestra internals or binding to Wails, React, Linear, concrete CLIs, or third-party AI runtimes.

Harness output is **untrusted evidence**. A successful process, adapter result, or `ToolResult.Status == "success"` is not a task-success verdict. Inspector evaluates workspace state and evidence; Orchestra/Inspector is the sole authority that may accept a task into `success` or approve merge readiness.

## Contract surface

The Go contracts live in `internal/cortex/harness/`:

| Contract | Responsibility |
| --- | --- |
| `ToolDefinition` | Versioned metadata, capabilities, schemas, and timeout for a registered tool. |
| `ToolRequest` | Execution request scoped to execution, task, worktree, tool, input, timeout, and trace IDs. |
| `ToolResult` | Structured untrusted observation with status, redacted output, error, timing, and evidence references. |
| `EvidenceRecord` | Immutable, redacted, digest-addressed audit evidence. |
| `ToolBroker` | Registration, policy check, engine selection, sandbox execution boundary, and evidence result boundary. |
| `EngineAdapter` | Stable engine-neutral interface for native Go, Pi, OMP, and future adapters. |
| `PolicyEngine` | Permission evaluation and versioned policy loading. |
| `CodedError` | Stable sanitized error code, message, retryability, and non-serialized cause. |

All serialized DTOs are JSON-compatible and use explicit contract/schema version fields where they cross a package or process boundary.

## Tool Broker lifecycle

```text
register ToolDefinition
        |
        v
validate request -> resolve tool -> evaluate policy
        |                    |
        | denied/ask          +--> typed policy error, no adapter call
        v
select EngineAdapter -> derive timeout context -> execute in sandbox
        |
        v
redacted ToolResult + EvidenceRecord references
        |
        v
Inspector/Orchestra inspection (sole success authority)
```

`Broker.Execute` must fail closed. It does not execute an unregistered tool, bypass a policy decision, select an unavailable adapter, or claim task success. It routes only through the adapter registered for the tool kind.

## Tool and engine contracts

Supported engine kinds are:

- `native`: deterministic Go-implemented operations;
- `pi`: general coding execution;
- `omp`: specialist/recovery execution.

`EngineAdapter` exposes only `Kind`, `Capabilities`, `Execute`, `Health`, and `Describe`. Engine-specific process handles, prompts, protocols, command lines, environment variables, and provider SDK types remain behind the adapter. `ExecutionEnvelope` carries stable IDs, policy decision, input, timeout, and trace metadata; adapter implementations must not use ambient state to escape the assigned sandbox.

**Engine Adapter Router (BAN-89)**  
The `AdapterRouter` (`internal/cortex/harness/router.go`) provides a stable, engine-neutral registry and deterministic selection layer:

| Component | Responsibility |
| --- | --- |
| `ExtendedAdapterDescriptor` | Augments `AdapterDescriptor` with `AdapterIdentity` (name, instance ID, kind, class, version, schema version), declared `ResourceRequirement` (min/peak memory, CPU priority), and optional `VersionConstraint` for compatibility. |
| `RoutingPolicy` | Controls `AllowedClasses`/`DeniedClasses`, `AllowedAdapters`/`DeniedAdapters`, `PreferredClasses` (priority order), `MaxFallbackAttempts` (bounded retries), and `RequireHealthEnforcement`. |
| `RoutingConstraints` | Request-scoped constraints: `ExcludeClasses`, `MinMemoryMB`, `PreferInstanceID`, `RequiredSchemaVersion`, `RequiredAdapterVersion`. |
| `AdapterSelection` | Returns selected adapter, reason, tie-break key, and a bounded `RetryOrder` of alternative adapters for fallback. |
| `HealthStatus` | Tracks `Healthy`, check counts, fail counts, consecutive failures, last check time, and reason. |

Routing algorithm (deterministic):
1. Filter registered adapters by required capabilities, policy class/adapter allow/deny lists, minimum memory, schema/version constraints.
2. Sort by `PreferredClasses` rank, then `FailCount` (ascending), then lexicographic `name|instanceId` tie-break.
3. If health enforcement enabled, probe candidates in order (bounded by `MaxFallbackAttempts + 1`); return first healthy.
4. On no eligible adapter: typed error `ErrNoEligibleAdapter`, `ErrPolicyDeniedEngine`, `ErrVersionIncompatible`, or `ErrAdapterUnhealthy` with retryable flag for health.

Tools declare capabilities such as `shell`, `file_read`, `file_write`, `git`, `search`, `test_run`, and `build`. The broker uses the tool's declared capabilities to query the router; the router selects the best matching adapter. Engine class and resource requirements (`EngineClassNative`, `EngineClassPi`, `EngineClassOMP`) enable resource-aware scheduling.

## Timeout and cancellation

Every `ToolRequest` has a positive timeout. The broker derives a child context with that timeout and propagates parent cancellation to the adapter through `ExecutionContext`. Adapters must stop or terminate their underlying work according to their own sandbox/process implementation and return `context.Canceled` or `context.DeadlineExceeded` (or the equivalent Harness sentinel). These map to stable `harness.canceled` and `harness.timeout` errors.

`DefaultTimeoutPolicy` and `DefaultCancellationPolicy` define baseline configuration only; policy enforcement and process cleanup remain adapter responsibilities. The configured cancellation grace period and force-kill behavior must be honored by concrete process adapters when they are added.

## Redaction and evidence

Evidence is created with `NewEvidenceRecord`, which:

1. marshals the payload;
2. redacts recognized secrets and absolute Windows paths;
3. computes a SHA-256 digest of the redacted bytes;
4. assigns a deterministic evidence ID prefix and records execution/task/worktree scope.

Public error messages are sanitized and do not expose causes. `CodedError.Cause` is excluded from JSON. `ToolResult.Redacted` is always set by the broker before returning an adapter result. Raw command output, credentials, environment values, absolute paths, process handles, and provider-specific output must not cross a future application/UI bridge.

Redaction is a safety boundary, not a guarantee that arbitrary untrusted content is safe. Future adapters must redact before constructing evidence and should prefer structured summaries over raw output.

## Error taxonomy

| Sentinel / code | Meaning | Default retryability |
| --- | --- | --- |
| `ErrPolicyDenied` / `harness.policy_denied` | Policy explicitly denied execution. | No |
| `ErrPolicyAsk` / `harness.policy_approval_required` | Approval is required and was not supplied. | No |
| `ErrTimeout` / `harness.timeout` | Execution exceeded its deadline. | Yes |
| `ErrCanceled` / `harness.canceled` | Parent or caller canceled execution. | Yes |
| `ErrWorktreeViolation` / `harness.worktree_violation` | Operation attempted to leave or violate the assigned worktree. | No |
| `ErrSandbox` / `harness.sandbox_error` | Sandbox setup or enforcement failed. | Yes |
| `ErrAdapterFailure` / `harness.adapter_failure` | Adapter unavailable or returned an unknown execution failure. | No |
| `ErrNoEligibleAdapter` / `harness.no_eligible_adapter` | No registered adapter satisfies requested capabilities and constraints. | No |
| `ErrPolicyDeniedEngine` / `harness.policy_denied_engine` | Matching adapters were excluded by routing policy. | No |
| `ErrAdapterUnhealthy` / `harness.adapter_unhealthy` | Matching adapters failed health checks after bounded fallback. | Yes |
| `ErrVersionIncompatible` / `harness.version_incompatible` | Matching adapters do not satisfy the requested version range. | No |
| `ErrToolNotFound` / `harness.tool_not_found` | Request referenced an unregistered tool. | No |
| `ErrInvalidContract` / `harness.invalid_contract` | DTO, definition, or version failed validation. | No |

Errors returned across this boundary use `CodedError`; internal causes are available for local diagnostics only and are never serialized.

## Dependency rules

Harness may depend on the Go standard library and its own policy contract. It must not import:

- Wails or React/application bridge packages;
- Linear or MCP clients;
- Orchestra internals or task persistence;
- Pi, OMP, or other concrete execution runtimes;
- process-specific or UI-specific DTOs.

Concrete adapters depend inward on Harness contracts. Orchestra depends on the Harness port and consumes results as observations. Harness never mutates Orchestra task state and never transitions a task to success.

## Contract tests

`internal/cortex/harness/` tests cover:

- DTO validation and JSON round trips;
- tool and adapter registration, duplicate handling, and lookup;
- policy allow, ask, and deny behavior;
- missing adapters and missing tools;
- timeout and cancellation propagation;
- stable error mapping and retry classification;
- evidence IDs, digests, serialization, and required scope IDs;
- secret and Windows absolute-path redaction;
- engine-neutral fake adapter behavior.

These tests are deterministic and do not require Pi, OMP, a shell, Wails, Linear, or network access.

## Follow-on boundaries

BAN-93 defines the contract only. Subsequent issues own implementation details:

- BAN-95: Pi adapter and JSONL process lifecycle;
- BAN-98: integration with Orchestra Dispatcher;
- BAN-99: safe application/Wails observability DTOs;
- BAN-100: Harness persistence for policy/process/evidence state.
