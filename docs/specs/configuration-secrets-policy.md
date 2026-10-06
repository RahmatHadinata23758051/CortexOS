# CortexOS Configuration and Secrets Policy

**Issue:** BAN-29
**Status:** Phase 0 security contract
**Owner:** Project Lead / future Platform owner

## Scope and ownership

CortexOS is local-first. Configuration is split into safe, versioned defaults and machine-specific runtime state. Linear MCP is a development-management integration used by OpenCode; it is not a CortexOS runtime service and must not be imported into the application.

| Configuration area | Owner | Source | Commit policy |
| --- | --- | --- | --- |
| Safe feature defaults and limits | Platform | `configs/*.example.*` and versioned defaults | Trackable |
| Machine paths and local runtime state | Platform | OS application-data directory | Ignored, never in repository |
| Provider credentials | Harness/Engine | OS keychain or explicitly injected process environment | Never committed or logged |
| User-provided/BYOK credentials | User + Harness | OS keychain, scoped by provider/project | Never mixed with platform credentials |
| Development Linear MCP connection | Development tooling | `opencode.json` plus local auth flow | Config is trackable; credentials are not |
| Test fixtures | Test | Redacted deterministic fixture files | Trackable only when synthetic |

## Configuration inventory

| Key | Local | Test | Staging | Production | Secret? | Owner | Rotation/revocation |
| --- | --- | --- | --- | --- | --- | --- | --- |
| `CORTEXOS_DATA_DIR` | Optional safe OS default | Temporary isolated directory | Unknown; deployment-defined | Unknown; deployment-defined | No | Platform | Change through deployment/config management |
| `CORTEXOS_LOG_LEVEL` | `info` | `warn` or test override | `info` | `info`/structured policy | No | Platform | Config release |
| `CORTEXOS_ENV` | `local` | `test` | Explicit required value | Explicit required value | No | Platform | Config release |
| `CORTEXOS_PROVIDER_DEFAULT` | Explicit user choice or disabled | Fake provider only | Unknown | Unknown | No | Harness | Config release; fail closed if unsupported |
| `CORTEXOS_PROVIDER_KEY_REF` | OS keychain reference | Synthetic test reference | Secret manager reference unknown | Secret manager/keychain reference unknown | Indirectly | Harness | Revoke at provider/keychain/secret manager |
| `CORTEXOS_USER_KEY_REF` | Per-user OS keychain reference | Synthetic test reference | Unknown | Unknown | Indirectly | Harness | User revokes/replaces through credential settings |
| `LINEAR_MCP_URL` | Development tooling only | Development tooling only if explicitly enabled | Not applicable | Not applicable | No | Development tooling | Remove/disable integration |
| `LINEAR_API_KEY` | Never read by CortexOS runtime | Never read by CortexOS runtime | Not applicable | Not applicable | Yes | Development tooling | Revoke and replace in Linear; never put in project files |

Unknown staging/production values are intentional. They must be resolved by a deployment decision before those environments exist; local defaults must not silently become production configuration.

## Secret flows

### Provider/platform keys

1. User or deployment operator supplies a provider credential through the supported setup flow.
2. CortexOS validates the provider and scope without echoing the value.
3. The credential is stored in the OS keychain where available, referenced by opaque ID, and passed to an engine adapter only for the active task.
4. The Harness redacts credential values from structured events, child-process arguments, environment snapshots, diagnostics, and persisted task evidence.
5. Revocation removes the keychain entry and invalidates any in-memory session on the next safe boundary.

### User-provided/BYOK keys

BYOK credentials are tenant/user-owned, even in a single-user desktop. They must be stored under a provider-and-user scope, never copied into project files, and never used as a fallback for another project or Staff role. The UI must show which provider/account a task will use without showing the secret.

### Linear MCP

The Linear MCP configuration in `opencode.json` is for development orchestration. `.mcp.json` may contain credentials and machine-local paths and remains ignored. CortexOS runtime packages must not import Linear SDK/MCP code, read Linear credentials, or require network access to start a local project.

## Validation and startup wiring

- Parse configuration into a typed schema before starting the runtime.
- Reject unknown critical environment values and invalid paths rather than broadening permissions.
- Fail closed when a requested provider credential is missing, revoked, or mismatched with the task/project scope.
- Refuse to start production mode with development-only integrations enabled.
- Resolve local data directories to an approved application-data root; reject repository-relative secrets and path traversal.
- Keep build-time configuration separate from runtime secret resolution.
- Do not silently use a default provider or shared credential after a lookup failure.

The Phase 1 startup implementation must prove that configuration is actually read and validated at startup. A declaration in an example file alone is not evidence of wiring.

## Redaction policy

Redact before persistence or display:

- API keys, OAuth tokens, cookies, refresh tokens, private keys, certificates, signed URLs, and authorization headers;
- secret-looking environment variable values and command-line arguments;
- provider request headers and raw authentication errors;
- keychain payloads and credential lookup responses.

Use stable placeholders such as `[REDACTED]` and preserve only safe metadata (provider name, key reference ID, scope, and rotation timestamp). Redaction must be applied to errors as well as successful event payloads.

## Rotation and revocation

- Rotation is additive first: create and validate the replacement, switch new tasks to it, then revoke the old credential.
- Record only metadata: credential reference, provider, scope, created/rotated/revoked timestamps, and last validation result.
- On suspected exposure, revoke immediately, invalidate active engine sessions, rotate the credential, and record an incident without copying the secret into the report.
- A failed rotation must not delete the last known-good credential until the replacement passes validation.
- Project Lead approval is required before changing production secret storage or provider policy.

## Drift checks

Before a release or environment promotion, check:

1. no tracked file matches secret-bearing filename patterns or contains known credential prefixes;
2. all required config keys have a documented owner and source;
3. example files contain names/placeholders only;
4. development-only MCP settings are excluded from runtime manifests;
5. provider allowlists, redaction rules, and keychain namespaces match the current schema;
6. staging/production values are explicit and were not inherited from local defaults.

## Verification commands

Current repository checks:

```powershell
git diff --check
git status --short --ignored
git check-ignore -v .mcp.json .env .env.example
Select-String -Path .\configs\*.example.* -Pattern 'sk-|token|password|secret' -CaseSensitive:$false
```

The last command is expected to find only safe key names/placeholders, never real values. Phase 1 adds a repository secret scanner and automated schema validation to the standard gate.

## Failure behavior

Missing non-secret optional configuration uses a documented safe default. Missing critical configuration, invalid secret references, unsupported providers, unsafe paths, and production/development mode conflicts are hard errors with actionable diagnostics. Diagnostics must not include the rejected secret or full local credential path.
