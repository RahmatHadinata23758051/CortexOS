# BAN-131 Remote transport security guard (P7-06)

## Status

CortexOS has no remote HTTP or TCP transport. All IPC runs in-process through
the Wails bridge. The P7-06 requirement is conditional: security tests for CSRF,
origin checks, replay, authentication, and authorization apply only if remote
access is introduced. This document defines the trigger and the regression
guard that enforces it.

## Guard

`internal/platform/security_guard_test.go`

- `TestNoUnreviewedRemoteTransport` walks `internal/` and `cmd/` production Go
  files (test files excluded) and fails when any file:
  - imports `net/http`, or
  - contains `net.Listen(`, `net.ListenTCP(`, `http.ListenAndServe(`, or
    `http.ListenAndServeTLS(`.

`internal/platform/security_guard_probe_test.go`

- `TestTransportGuardDetectsViolations` feeds synthetic violating and compliant
  files through the same scanner. It proves the guard is not vacuous.

## Trigger: what must happen before a remote transport merges

A change that adds any of the patterns above must, in the same PR:

1. Add a threat model using `security-threat-model` (assets, trust boundaries,
   entry points).
2. Implement and test origin validation, CSRF or equivalent request binding,
   replay protection, authentication, authorization, and input validation.
3. Update the guard so the new transport is explicitly allowlisted by file path,
   with the review reference in a comment.

Mutation methods must not be exposed on the remote transport until (2) passes.

## Verification

```powershell
go test ./internal/platform/ -run "TestTransportGuard|TestNoUnreviewed" -count=1 -v
```

## Residual risk

- The scanner is pattern-based. It does not detect transports pulled in through
  third-party packages (for example a WebSocket library). Dependency review
  under `docs/specs/dependency-policy.md` covers that gap.
- The guard runs on Windows locally and on CI Linux and macOS through the
  existing `go test ./...` jobs.
