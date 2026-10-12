import { createElement } from 'react'
import { renderToStaticMarkup } from 'react-dom/server'
import { describe, expect, it, vi } from 'vitest'

import { InspectorReview } from '../components/inspector/InspectorReview'
import { getOrchestraTask, loadOrchestraTasks, mergeOrchestraTask } from '../bridge/orchestra'
import {
  orchestraSchemaVersion,
  type OrchestraBridge,
  type OrchestraMergeResult,
  type OrchestraTaskDetail,
  type OrchestraTaskStatus,
  type OrchestraTaskSummary,
} from '../types/orchestra'

/**
 * BAN-124 end-to-end seam: a fake Orchestra backend stands in for the Go
 * services behind the Wails bridge. The UI never mutates lifecycle state; it
 * only reads through the bridge. Every status the UI renders must match the
 * backend state at that step, and merge must stay locked until the backend
 * has recorded Inspector acceptance (ADR-0005).
 */
function createFakeBackend(taskId: string, projectId: string) {
  let status: OrchestraTaskStatus = 'draft'
  let attempt = 0
  let inspectorAccepted = false
  const timeline: OrchestraTaskDetail['timeline'] = []
  const record = (type: string, from?: OrchestraTaskStatus, to?: OrchestraTaskStatus, evidenceIds?: string[]) => {
    timeline.push({
      sequence: timeline.length + 1,
      type,
      from,
      to,
      occurredAt: '2026-10-11T00:00:00Z',
      ...(evidenceIds ? { evidenceIds } : {}),
    })
  }
  const summary = (): OrchestraTaskSummary => ({
    id: taskId,
    projectId,
    status,
    attempt,
    maxAttempts: 3,
    progress: status === 'success' ? 100 : status === 'awaitingInspection' ? 80 : 20,
    schemaVersion: orchestraSchemaVersion,
  })
  const detail = (): OrchestraTaskDetail => ({
    ...summary(),
    acceptanceCount: 2,
    dependencyCount: 0,
    evidence: status === 'draft' || status === 'ready' ? [] : [{ id: `ev-${taskId}-1`, count: 2 }],
    timeline: [...timeline],
  })

  // Backend authority helpers: the UI never drives these transitions itself;
  // it only reads state and requests a governed merge through the typed command.
  const dispatch = () => {
    record('task.validated', 'draft', 'ready')
    status = 'ready'
    record('task.dispatched', 'ready', 'running')
    status = 'running'
    attempt = 1
    record('evidence.collected', 'running', 'running', [`ev-${taskId}-1`])
    record('execution.completed', 'running', 'awaitingInspection')
    status = 'awaitingInspection'
  }
  const inspectorAcceptAndMerge = () => {
    inspectorAccepted = true
    record('inspector.accepted', 'awaitingInspection', 'awaitingInspection')
    record('task.merged', 'awaitingInspection', 'success')
    status = 'success'
  }

  const bridge: OrchestraBridge = {
    ListOrchestraTasks: vi.fn(async () => [summary()]),
    GetOrchestraTask: vi.fn(async () => detail()),
    CancelOrchestraTask: vi.fn(async () => {
      if (status === 'success' || status === 'failed' || status === 'canceled') throw new Error('task is terminal')
      status = 'canceled'
    }),
    RetryOrchestraTask: vi.fn(async () => {
      if (status !== 'failed') throw new Error('only failed tasks can be retried')
      status = 'ready'
    }),
    MergeOrchestraTask: vi.fn(async (request): Promise<OrchestraMergeResult> => {
      if (request.taskId !== taskId) throw new Error('task not found')
      if (status !== 'awaitingInspection') {
        throw new Error('orchestra merge requires Inspector acceptance')
      }
      inspectorAcceptAndMerge()
      return { taskId, status: 'success', schemaVersion: orchestraSchemaVersion }
    }),
  }

  return {
    bridge,
    dispatch,
    inspectorAcceptAndMerge,
    get status() { return status },
    get inspectorAccepted() { return inspectorAccepted },
  }
}

function renderInspector(summary: OrchestraTaskSummary, detail: OrchestraTaskDetail) {
  return renderToStaticMarkup(createElement(InspectorReview, { task: summary, detail }))
}

describe('BAN-124 Phase 6 end-to-end cockpit chain', () => {
  it('keeps UI state in lockstep with backend lifecycle from create to merge', async () => {
    const taskId = 'task-ban124-e2e'
    const projectId = 'proj-ban124'
    const backend = createFakeBackend(taskId, projectId)

    // create → visible on the board as draft
    let state = await loadOrchestraTasks(backend.bridge, { projectId })
    expect(state).toMatchObject({ status: 'success', tasks: [{ id: taskId, status: 'draft' }] })

    // assign → run → worker evidence; backend moves to awaitingInspection
    backend.dispatch()
    expect(backend.status).toBe('awaitingInspection')
    state = await loadOrchestraTasks(backend.bridge, { projectId })
    expect(state.status === 'success' && state.tasks[0].status).toBe('awaitingInspection')

    // inspect: UI reads detail and must show the merge gate LOCKED
    const summary = state.status === 'success' ? state.tasks[0] : undefined
    const detail = await getOrchestraTask(backend.bridge, taskId)
    expect(detail.status).toBe('awaitingInspection')
    expect(detail.evidence.length).toBeGreaterThan(0)
    expect(detail.timeline.some((entry) => entry.type === 'evidence.collected')).toBe(true)
    if (!summary) throw new Error('expected a summary')
    const beforeAccept = renderInspector(summary, detail)
    expect(beforeAccept).toContain('MERGE GATE: LOCKED')
    expect(beforeAccept).toMatch(/<button[^>]*class="merge-button"[^>]*disabled/)

    // Worker self-reports success: UI must not treat this as success
    expect(backend.status).not.toBe('success')
    const selfReported = await getOrchestraTask(backend.bridge, taskId)
    expect(selfReported.status).toBe('awaitingInspection')

    // Inspector acceptance + Orchestra merge happen behind the typed command.
    const mergeResult = await mergeOrchestraTask(backend.bridge, taskId)
    expect(mergeResult).toEqual({ taskId, status: 'success', schemaVersion: orchestraSchemaVersion })
    expect(backend.inspectorAccepted).toBe(true)

    // UI re-reads and matches backend: success, timeline ordered, merge after accept
    state = await loadOrchestraTasks(backend.bridge, { projectId })
    expect(state.status === 'success' && state.tasks[0].status).toBe('success')
    const finalDetail = await getOrchestraTask(backend.bridge, taskId)
    expect(finalDetail.status).toBe('success')
    const sequences = finalDetail.timeline.map((entry) => entry.sequence)
    expect(sequences).toEqual([...sequences].sort((a, b) => a - b))
    const acceptIndex = finalDetail.timeline.findIndex((entry) => entry.type === 'inspector.accepted')
    const mergeIndex = finalDetail.timeline.findIndex((entry) => entry.type === 'task.merged')
    expect(acceptIndex).toBeGreaterThanOrEqual(0)
    expect(mergeIndex).toBeGreaterThan(acceptIndex)
  })

  it('does not allow cancel or retry to bypass the lifecycle', async () => {
    const backend = createFakeBackend('task-ban124-guard', 'proj-ban124')
    backend.dispatch()
    backend.inspectorAcceptAndMerge()

    await expect(backend.bridge.CancelOrchestraTask({ schemaVersion: orchestraSchemaVersion, taskId: 'task-ban124-guard' }))
      .rejects.toThrow('terminal')
    await expect(backend.bridge.RetryOrchestraTask({ schemaVersion: orchestraSchemaVersion, taskId: 'task-ban124-guard' }))
      .rejects.toThrow('failed')
    expect(backend.status).toBe('success')
  })

  it('surfaces bridge contract violations instead of rendering unverified state', async () => {
    const bridge: OrchestraBridge = {
      ListOrchestraTasks: vi.fn(async () => [{ id: 'x', projectId: 'p', status: 'success', attempt: 0, maxAttempts: 1, progress: 100, schemaVersion: 'bogus' } as unknown as OrchestraTaskSummary]),
      GetOrchestraTask: vi.fn(),
      CancelOrchestraTask: vi.fn(),
      RetryOrchestraTask: vi.fn(),
      MergeOrchestraTask: vi.fn(),
    }
    const state = await loadOrchestraTasks(bridge)
    expect(state.status).toBe('error')
  })
})
