# Orchestra Inspection and Merge Authority Contract

**Version:** `cortexos.orchestra.inspection.v1`
**Scope:** Phase 3 inspection boundary

Workers and engines provide observations and evidence only. They do not own task success. The Orchestra merge authority is the only component allowed to accept a task into `success`.

Inspection requires:

- a valid task and matching execution;
- execution status that is not worker-reported `success`;
- a valid worktree with the expected worktree identity;
- a clean worktree;
- at least one typed, passing evidence item;
- no duplicate changed paths.

The authority applies `awaitingInspection -> success` only after all checks pass. Rejections are typed and fail closed. No raw process output, absolute paths, or secrets are accepted as inspection authority.
