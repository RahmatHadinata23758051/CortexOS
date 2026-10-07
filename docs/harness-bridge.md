# Harness observability application bridge (BAN-99)

The Wails bridge exposes only typed, metadata-only Harness observability:

- `ListHarnessCapabilities`: capability names, descriptions, ask/default policy metadata.
- `ListHarnessWorkers`: worker identity, engine metadata, status, and heartbeat times.
- `ListActiveHarnessExecutions`: execution/task identifiers, resource accounting, trace and worker identifiers.
- `GetHarnessEvidence`: evidence identifiers, relationship identifiers, kind, digest, and collection time.

All requests include `cortexos.harness.bridge.v1`. Every call receives the bridge context and stops before work when it is canceled; downstream application ports must honor that context. Cancellation and deadline failures map to the stable `harness.canceled` code.

The application mapper is the security boundary. Paths, SQL/database handles, process IDs and handles, secrets, raw input, raw provider output, raw payload, and raw audit metadata are not fields in public DTOs and are never serialized. Errors use stable `harness.*` codes and generic user-facing messages; internal causes do not cross Wails.
