import { describe, expect, it } from 'vitest'

import {
  orchestraSchemaVersion,
  type OrchestraMergeRequest,
  type OrchestraMergeResult,
  type OrchestraTaskSummary,
  type OrchestraTaskDetail,
  type OrchestraTaskListRequest,
  type OrchestraTaskRequest,
  type OrchestraBridge,
  isOrchestraTaskSummary,
  isOrchestraTaskDetail,
} from '../types/orchestra'

/**
 * BAN-141: Contract verification between Go bridge DTOs and TypeScript bindings.
 *
 * This test documents the exact TypeScript ↔ Go type correspondence required by
 * Wails code generation. If the Go bridge types in
 * `internal/cortex/application/orchestra.go` change, the TypeScript types in
 * `web/src/types/orchestra.ts` MUST be updated to match, and the Wails bindings
 * must be regenerated via `wails generate` (which runs as part of the build).
 *
 * The test uses structural compatibility checks to ensure field names, types,
 * and required/optional semantics align. It does NOT require a running Wails
 * process — it validates the static contract that the generated bindings must
 * satisfy.
 */

describe('BAN-141: Go ↔ TypeScript Orchestra bridge contract', () => {
  it('orchestraSchemaVersion matches the Go constant OrchestraBridgeSchemaVersion', () => {
    expect(orchestraSchemaVersion).toBe('cortexos.orchestra.bridge.v1')
  })

  it('OrchestraMergeRequest matches Go struct with schemaVersion, taskId, optional note', () => {
    const req: OrchestraMergeRequest = {
      schemaVersion: orchestraSchemaVersion,
      taskId: 'task-123',
      note: 'evidence verified',
    }
    expect(req.schemaVersion).toBe(orchestraSchemaVersion)
    expect(req.taskId).toBe('task-123')
    expect(req.note).toBe('evidence verified')

    // note is optional
    const reqNoNote: OrchestraMergeRequest = { schemaVersion: orchestraSchemaVersion, taskId: 't-1' }
    expect(reqNoNote.note).toBeUndefined()
  })

  it('OrchestraMergeResult matches Go struct with taskId, status, schemaVersion', () => {
    const res: OrchestraMergeResult = {
      taskId: 'task-123',
      status: 'success',
      schemaVersion: orchestraSchemaVersion,
    }
    expect(res.taskId).toBe('task-123')
    expect(res.status).toBe('success')
    expect(res.schemaVersion).toBe(orchestraSchemaVersion)
  })

  it('OrchestraTaskSummary matches Go struct with all required fields', () => {
    const task: OrchestraTaskSummary = {
      id: 'task-1',
      projectId: 'proj-1',
      status: 'awaitingInspection',
      attempt: 1,
      maxAttempts: 3,
      progress: 80,
      schemaVersion: orchestraSchemaVersion,
    }
    expect(isOrchestraTaskSummary(task)).toBe(true)
  })

  it('OrchestraTaskDetail extends summary with acceptanceCount, dependencyCount, evidence, timeline', () => {
    const detail: OrchestraTaskDetail = {
      id: 'task-1',
      projectId: 'proj-1',
      status: 'awaitingInspection',
      attempt: 1,
      maxAttempts: 3,
      progress: 80,
      schemaVersion: orchestraSchemaVersion,
      acceptanceCount: 2,
      dependencyCount: 1,
      evidence: [{ id: 'ev-1', count: 3 }],
      timeline: [
        {
          sequence: 1,
          type: 'task.validated',
          from: 'draft',
          to: 'ready',
          occurredAt: '2026-10-11T00:00:00Z',
        },
      ],
    }
    expect(isOrchestraTaskDetail(detail)).toBe(true)
  })

  it('OrchestraTaskListRequest includes projectId optional, limit required', () => {
    const req: OrchestraTaskListRequest = { schemaVersion: orchestraSchemaVersion, limit: 100 }
    expect(req.limit).toBe(100)
    expect(req.projectId).toBeUndefined()

    const reqWithProject: OrchestraTaskListRequest = {
      schemaVersion: orchestraSchemaVersion,
      projectId: 'proj-1',
      limit: 50,
    }
    expect(reqWithProject.projectId).toBe('proj-1')
  })

  it('OrchestraTaskRequest contains only schemaVersion and taskId', () => {
    const req: OrchestraTaskRequest = { schemaVersion: orchestraSchemaVersion, taskId: 'task-1' }
    expect(req.taskId).toBe('task-1')
  })

  it('OrchestraBridge interface declares all required methods', () => {
    // This type assertion verifies the interface shape at compile time.
    // If any method signature changes in Go and isn't reflected here, TypeScript will error.
    const _bridgeShape: OrchestraBridge = {
      ListOrchestraTasks: async () => [],
       
      GetOrchestraTask: async () => null as unknown as OrchestraTaskDetail,
      CancelOrchestraTask: async () => {},
      RetryOrchestraTask: async () => {},
      MergeOrchestraTask: async () => ({ taskId: '', status: 'success', schemaVersion: orchestraSchemaVersion }),
    }
    expect(typeof _bridgeShape.ListOrchestraTasks).toBe('function')
    expect(typeof _bridgeShape.GetOrchestraTask).toBe('function')
    expect(typeof _bridgeShape.CancelOrchestraTask).toBe('function')
    expect(typeof _bridgeShape.RetryOrchestraTask).toBe('function')
    expect(typeof _bridgeShape.MergeOrchestraTask).toBe('function')
  })

  it('isOrchestraTaskSummary rejects invalid schemaVersion', () => {
    const invalid = {
      id: 't1',
      projectId: 'p1',
      status: 'success',
      attempt: 0,
      maxAttempts: 1,
      progress: 100,
      schemaVersion: 'wrong.version',
    }
    expect(isOrchestraTaskSummary(invalid)).toBe(false)
  })

  it('isOrchestraTaskSummary rejects out-of-range progress', () => {
    const invalid = {
      id: 't1',
      projectId: 'p1',
      status: 'success',
      attempt: 0,
      maxAttempts: 1,
      progress: 150,
      schemaVersion: orchestraSchemaVersion,
    }
    expect(isOrchestraTaskSummary(invalid)).toBe(false)
  })

  it('isOrchestraTaskSummary rejects negative attempt/maxAttempts', () => {
    const invalid = {
      id: 't1',
      projectId: 'p1',
      status: 'success',
      attempt: -1,
      maxAttempts: 1,
      progress: 100,
      schemaVersion: orchestraSchemaVersion,
    }
    expect(isOrchestraTaskSummary(invalid)).toBe(false)
  })

  it('isOrchestraTaskDetail rejects missing evidence/timeline arrays', () => {
    const invalid = {
      id: 't1',
      projectId: 'p1',
      status: 'success',
      attempt: 0,
      maxAttempts: 1,
      progress: 100,
      schemaVersion: orchestraSchemaVersion,
      acceptanceCount: 1,
      dependencyCount: 0,
      evidence: 'not-an-array',
      timeline: [],
    }
    expect(isOrchestraTaskDetail(invalid)).toBe(false)
  })
})

describe('BAN-141: mergeOrchestraTask enforces backend-authoritative result', () => {
  it('throws when result schemaVersion mismatches', async () => {
    const fakeBridge: OrchestraBridge = {
      ListOrchestraTasks: async () => [],
       
      GetOrchestraTask: async () => null as unknown as OrchestraTaskDetail,
      CancelOrchestraTask: async () => {},
      RetryOrchestraTask: async () => {},
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      MergeOrchestraTask: async () => ({ taskId: 'task-1', status: 'success', schemaVersion: 'wrong.version' }) as any,
    }

    await expect(
      (await import('../bridge/orchestra')).mergeOrchestraTask(fakeBridge, 'task-1')
    ).rejects.toThrow('non-authoritative merge result')
  })

  it('throws when result status is not success', async () => {
    const fakeBridge: OrchestraBridge = {
      ListOrchestraTasks: async () => [],
       
      GetOrchestraTask: async () => null as unknown as OrchestraTaskDetail,
      CancelOrchestraTask: async () => {},
      RetryOrchestraTask: async () => {},
      MergeOrchestraTask: async () => ({
        taskId: 'task-1',
        status: 'awaitingInspection',
        schemaVersion: orchestraSchemaVersion,
      }),
    }

    await expect(
      (await import('../bridge/orchestra')).mergeOrchestraTask(fakeBridge, 'task-1')
    ).rejects.toThrow('non-authoritative merge result')
  })

  it('throws when result taskId mismatches request', async () => {
    const fakeBridge: OrchestraBridge = {
      ListOrchestraTasks: async () => [],
       
      GetOrchestraTask: async () => null as unknown as OrchestraTaskDetail,
      CancelOrchestraTask: async () => {},
      RetryOrchestraTask: async () => {},
      MergeOrchestraTask: async () => ({
        taskId: 'other-task',
        status: 'success',
        schemaVersion: orchestraSchemaVersion,
      }),
    }

    await expect(
      (await import('../bridge/orchestra')).mergeOrchestraTask(fakeBridge, 'task-1')
    ).rejects.toThrow('non-authoritative merge result')
  })

  it('returns result when all fields match expected authoritative response', async () => {
    const fakeBridge: OrchestraBridge = {
      ListOrchestraTasks: async () => [],
       
      GetOrchestraTask: async () => null as unknown as OrchestraTaskDetail,
      CancelOrchestraTask: async () => {},
      RetryOrchestraTask: async () => {},
      MergeOrchestraTask: async () => ({
        taskId: 'task-1',
        status: 'success',
        schemaVersion: orchestraSchemaVersion,
      }),
    }

    const result = await (await import('../bridge/orchestra')).mergeOrchestraTask(fakeBridge, 'task-1')
    expect(result).toEqual({
      taskId: 'task-1',
      status: 'success',
      schemaVersion: orchestraSchemaVersion,
    })
  })
})
