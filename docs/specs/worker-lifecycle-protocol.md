# Worker Lifecycle Protocol

**Version:** `cortexos.worker.v1`  
**Issue:** BAN-88  
**Transport:** one UTF-8 JSON object per line over worker stdin/stdout

## Ownership

The worker protocol transports observations and lifecycle requests between Harness and a long-lived worker. Worker claims, including `terminal.status = "success"`, are **untrusted evidence**. Inspector/Orchestra remains the only authority that can accept task success.

## Envelope

Every record has exactly these fields:

```json
{
  "version": "cortexos.worker.v1",
  "type": "progress",
  "correlationId": "execution-id",
  "sequence": 5,
  "payload": {}
}
```

`correlationId` identifies one execution stream. `sequence` starts at 1 for the handshake and increases by exactly one. Records larger than 1 MiB, malformed JSON, duplicate keys, unknown envelope/payload fields, unknown message types, wrong correlation IDs, or out-of-order sequences are rejected before use.

## Message types

| Type | Direction / purpose |
| --- | --- |
| `handshake` | Worker identifies itself and negotiates the protocol version. Must be first. |
| `capabilities` | Worker capability discovery. Informational only; does not bypass ToolBroker policy. |
| `health` | Readiness/health discovery. |
| `task` | Harness delivers a versioned task envelope scoped to execution, task, worktree, project, tool, input, timeout, and trace IDs. |
| `progress` | Worker reports phase and bounded percentage. |
| `evidence` | Worker emits an evidence reference, digest, and optional redacted record. |
| `diagnostic` | Structured redacted diagnostic (`debug`, `info`, `warn`, or `error`). |
| `heartbeat` | Worker liveness timestamp. Silence is observable as `ErrProtocolHeartbeatLost`. |
| `terminal` | Worker claims `success`, `failure`, or `canceled`; the claim is not a verdict. |
| `cancel` | Harness requests cancellation with a reason. |
| `shutdown` | Graceful process/session close. |

## Lifecycle

```text
awaiting_handshake -> ready -> running -> cancel_requested -> terminated -> closed
                         \-> closed (shutdown)
```

A task can only be delivered in `ready`. Streaming progress/evidence is allowed while running or after cancellation is requested. A terminal record is required before a normal task session can close. A process EOF while `running` is an observable crash; callers must map it to an adapter failure and never infer success.

The Go implementation is in `internal/cortex/harness/protocol.go`. Tests use deterministic fixtures in `internal/cortex/harness/testdata/`, including normal streaming, cancellation, and failure lifecycle records, plus fuzz coverage for decoder safety.
