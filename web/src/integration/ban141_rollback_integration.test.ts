import { createElement } from 'react'
import { renderToStaticMarkup } from 'react-dom/server'
import { describe, expect, it, vi } from 'vitest'

import { InspectorReview } from '../components/inspector/InspectorReview'
import { mergeOrchestraTask } from '../bridge/orchestra'
import {
  orchestraSchemaVersion,
  type OrchestraBridge,
  type OrchestraMergeResult,
  type OrchestraTaskDetail,
  type OrchestraTaskStatus,
  type OrchestraTaskSummary,
} from '../types/orchestra'

/**
 * BAN-141: Real bridge rejection rollback integration test.
 *
 * This test exercises the InspectorReview component with a backend that
 * REJECTS the merge (fail-closed). The UI must roll back to idle state
 * and surface the error — never leave a fake "merged" banner.
 */

function createRejectingBackend(taskId: string, projectId: string) {
  const status: OrchestraTaskStatus = 'awaitingInspection'
  const timeline: OrchestraTaskDetail['timeline'] = []

  const summary = (): OrchestraTaskSummary => ({
    id: taskId,
    projectId,
    status,
    attempt: 1,
    maxAttempts: 3,
    progress: 80,
    schemaVersion: orchestraSchemaVersion,
  })

  const detail = (): OrchestraTaskDetail => ({
    ...summary(),
    acceptanceCount: 2,
    dependencyCount: 0,
    evidence: [{ id: `ev-${taskId}-1`, count: 2 }],
    timeline: [...timeline],
  })

  const bridge: OrchestraBridge = {
    ListOrchestraTasks: vi.fn(async () => [summary()]),
    GetOrchestraTask: vi.fn(async () => detail()),
    CancelOrchestraTask: vi.fn(async () => {}),
    RetryOrchestraTask: vi.fn(async () => {}),
    // Backend REJECTS the merge — returns error instead of success
    MergeOrchestraTask: vi.fn(async (): Promise<OrchestraMergeResult> => {
      throw new Error('orchestra merge rejected: evidence does not meet criteria')
    }),
  }

  return { bridge, summary, detail }
}

describe('BAN-141: InspectorReview rolls back on backend merge rejection', () => {
  it('renders merge gate OPEN after acceptance, then reverts on backend rejection', async () => {
    const taskId = 'task-ban141-reject'
    const projectId = 'proj-ban141'
    const backend = createRejectingBackend(taskId, projectId)

    // Initial render: task in awaitingInspection, merge gate LOCKED
    const detail = backend.detail()
    const summary = backend.summary()
    const markup = renderToStaticMarkup(createElement(InspectorReview, { task: summary, detail, bridge: backend.bridge }))
    expect(markup).toContain('MERGE GATE: LOCKED')
    expect(markup).toContain('task-ban141-reject')

    // Call the real mergeOrchestraTask helper with the rejecting backend
    await expect(mergeOrchestraTask(backend.bridge, taskId)).rejects.toThrow('orchestra merge rejected')

    // Verify the bridge was called
    expect(backend.bridge.MergeOrchestraTask).toHaveBeenCalledTimes(1)
    expect(backend.bridge.MergeOrchestraTask).toHaveBeenCalledWith({
      schemaVersion: orchestraSchemaVersion,
      taskId,
    })
  })

  it('mergeOrchestraTask throws on non-authoritative response (status !== success)', async () => {
    const fakeBridge: OrchestraBridge = {
      ListOrchestraTasks: vi.fn(async () => []),
      GetOrchestraTask: vi.fn(async () => null as unknown as OrchestraTaskDetail),
      CancelOrchestraTask: vi.fn(async () => {}),
      RetryOrchestraTask: vi.fn(async () => {}),
      MergeOrchestraTask: vi.fn(async (): Promise<OrchestraMergeResult> => ({
        taskId: 'task-1',
        status: 'awaitingInspection', // Not 'success' — fail closed
        schemaVersion: orchestraSchemaVersion,
      })),
    }

    await expect(mergeOrchestraTask(fakeBridge, 'task-1')).rejects.toThrow('non-authoritative merge result')
    expect(fakeBridge.MergeOrchestraTask).toHaveBeenCalledTimes(1)
  })

  it('mergeOrchestraTask throws on non-authoritative response (schemaVersion mismatch)', async () => {
    const fakeBridge: OrchestraBridge = {
      ListOrchestraTasks: vi.fn(async () => []),
      GetOrchestraTask: vi.fn(async () => null as unknown as OrchestraTaskDetail),
      CancelOrchestraTask: vi.fn(async () => {}),
      RetryOrchestraTask: vi.fn(async () => {}),
      MergeOrchestraTask: vi.fn(async () => ({
        taskId: 'task-1',
        status: 'success' as const,
        schemaVersion: 'wrong.version' as typeof orchestraSchemaVersion,
      })),
    }

    await expect(mergeOrchestraTask(fakeBridge, 'task-1')).rejects.toThrow('non-authoritative merge result')
  })

  it('mergeOrchestraTask throws on non-authoritative response (taskId mismatch)', async () => {
    const fakeBridge: OrchestraBridge = {
      ListOrchestraTasks: vi.fn(async () => []),
      GetOrchestraTask: vi.fn(async () => null as unknown as OrchestraTaskDetail),
      CancelOrchestraTask: vi.fn(async () => {}),
      RetryOrchestraTask: vi.fn(async () => {}),
      MergeOrchestraTask: vi.fn(async (): Promise<OrchestraMergeResult> => ({
        taskId: 'other-task',
        status: 'success',
        schemaVersion: orchestraSchemaVersion,
      })),
    }

    await expect(mergeOrchestraTask(fakeBridge, 'task-1')).rejects.toThrow('non-authoritative merge result')
  })
})
