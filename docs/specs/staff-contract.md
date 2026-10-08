# CortexOS Staff Contract

**Status:** Phase 5 contract
**Issue:** BAN-103
**Owner:** Staff layer
**Version:** `cortexos.staff.v1`

## Purpose

Staff is the durable, logical identity and policy contract for a product role. A Staff member may represent a coordinator, planner, implementer, reviewer, or specialist, but it is not an AI CLI process, worker handle, engine adapter, operating-system identity, or execution result.

The contract is intentionally Wails-independent and contains only versioned, JSON-safe domain data. Persistence, routing, Harness enforcement, and bridge adapters consume this contract through narrow ports.

## Identity and independence

A Staff definition contains:

- stable logical `id` and human-facing `name`;
- one governed role;
- product capabilities;
- declarative permission rules;
- references to governed skills and memory (never their contents);
- workspace, optional project, and optional worktree assignment;
- Staff-owned lifecycle and observed availability;
- creation/update timestamps and schema version.

It must not contain `workerID`, `processID`, `pid`, CLI identity, process handles, engine names, credentials, provider output, execution IDs, success verdicts, merge authority, or raw filesystem paths. Worker pools can schedule work for Staff and workers can be replaced without changing Staff identity or lifecycle. Availability is an observation, not a process reservation.

## Roles, capabilities, and scope

The v1 roles are:

- `coordinator`
- `planner`
- `implementer`
- `reviewer`
- `specialist`

Capabilities are product-level tokens such as `coding`, `review`, or `test_run`; they are not direct OS grants. Skills and memory are references with IDs and versions. Workspace assignment is opaque ID scope and does not grant authority by itself.

Unknown schema versions, roles, lifecycle states, availability states, malformed IDs, duplicate collections, invalid timestamps, and unsafe references fail closed with typed Staff errors.

## Permission model

A permission is a declarative rule:

```json
{
  "id": "read-worktree",
  "action": "read",
  "resource": "src/*",
  "effect": "allow",
  "priority": 10
}
```

Effects are `allow`, `ask`, and `deny`. Action matching is case-insensitive and resource separators are normalized from `\\` to `/`. `*` and `?` are whole-value wildcard operators; resource patterns may not contain control characters or traversal (`..`) segments.

Evaluation is deterministic:

1. validate the complete PermissionSet before matching;
2. normalize the request;
3. consider matching rules by descending priority;
4. break equal-priority ties by ascending permission ID;
5. return the first match;
6. return deny when no rule matches.

Invalid permission sets and invalid requests return errors and never produce execution authority. A matching `ask` is not an allow. A deny, missing permission ID, wrong contract version, mismatched action/resource, or any incomplete decision is rejected by `ValidateExecutionPermission`.

## Harness compatibility

`PolicyTranslator` maps Staff permissions to Harness rules without importing Harness into the Staff model. `EnvelopeAdapter` maps only an unconditional Staff allow to `harness.PolicyDecision{Allowed:true, Effect:allow}`. Ask, deny, malformed, unknown-version, or mismatched decisions become an explicit denied Harness decision; they never become an execution envelope grant. Harness and Orchestra remain the enforcement and execution authorities.

## Bridge and JSON contract

`Definition.MarshalJSON` validates before serialization. `Definition.UnmarshalJSON` rejects unknown/forbidden process, secret, provider, and execution-authority keys before validation. `JSONSafe` produces a `Summary` containing only identity, role, capabilities, assignment IDs, lifecycle, availability, timestamps, and schema version; it omits permission rules and skill/memory references.

Bridge consumers should use `Summary`/`JSONSafe` rather than exposing complete definitions. Summary JSON is also validated on decode and rejects forbidden fields and unsupported enum/version values. No bridge DTO includes raw paths, process identity, credentials, provider output, or execution evidence.

## Ports and non-goals

The Staff package owns contracts, validation, policy evaluation, translation boundaries, and narrow interfaces for stores, routers, assignment, memory metadata, and bridge summaries. It does not implement:

- worker pools or process lifecycle;
- task planning, orchestration, retries, or merge;
- skill execution or memory retrieval/content storage;
- persistence adapters;
- Wails/UI behavior;
- provider SDKs or cloud synchronization.

Those concerns belong to later or neighboring contracts and must not be added as fields to Staff definitions.

## Required contract tests

The v1 suite must cover model validation and collection-copy behavior; unsupported values and malformed references; permission validation; deterministic priority and tie-breaking; default deny and fail-closed invalid decisions; Harness translation; JSON round trips; forbidden-key rejection including casing/separator variants; JSON-safe Summary omission; and logical Staff independence from process identity.
