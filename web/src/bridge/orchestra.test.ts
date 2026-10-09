import { describe, expect, it, vi } from 'vitest'

import {
  cancelOrchestraTask,
  getOrchestraTask,
  loadOrchestraTasks,
  onOrchestraEvent,
  onOrchestraTaskChanged,
  retryOrchestraTask,
} from './orchestra'
import {
  orchestraSchemaVersion,
  OrchestraEventNames,
  type OrchestraBridge,
  type OrchestraTaskDetail,
  type OrchestraTaskSummary,
} from '../types/orchestra'

const validTask: OrchestraTaskSummary = {
  id: 'task-1',
  projectId: 'proj-1',
  status: 'ready',
  attempt: 0,
  maxAttempts: 3,
  progress: 20,
  schemaVersion: orchestraSchemaVersion,
}

const validDetail: OrchestraTaskDetail = {
  ...validTask,
  acceptanceCount: 2,
  dependencyCount: 0,
  evidence: [],
  timeline: [
    {
      sequence: 1,
      type: 'task.validated',
      from: 'draft',
      to: 'ready',
      occurredAt: '2026-10-10T00:00:00Z',
    },
  ],
}

const mockBridge = (): OrchestraBridge => ({
  ListOrchestraTasks: vi.fn().mockResolvedValue([validTask]),
  GetOrchestraTask: vi.fn().mockResolvedValue(validDetail),
  CancelOrchestraTask: vi.fn().mockResolvedValue(undefined),
  RetryOrchestraTask: vi.fn().mockResolvedValue(undefined),
})

describe('orchestra bridge operations', () => {
  it('loads valid tasks with schemaVersion', async () => {
    const bridge = mockBridge()
    const state = await loadOrchestraTasks(bridge, { projectId: 'proj-1', limit: 50 })
    expect(state).toEqual({
      status: 'success',
      tasks: [validTask],
    })
    expect(bridge.ListOrchestraTasks).toHaveBeenCalledWith({
      schemaVersion: orchestraSchemaVersion,
      projectId: 'proj-1',
      limit: 50,
    })
  })

  it('handles empty task lists gracefully', async () => {
    const bridge = mockBridge()
    vi.mocked(bridge.ListOrchestraTasks).mockResolvedValue([])
    const state = await loadOrchestraTasks(bridge)
    expect(state).toEqual({
      status: 'success',
      tasks: [],
    })
  })

  it('rejects invalid task items with structured error code', async () => {
    const bridge = mockBridge()
    vi.mocked(bridge.ListOrchestraTasks).mockResolvedValue([{ id: 'bad-task' }] as never)
    const state = await loadOrchestraTasks(bridge)
    expect(state).toEqual({
      status: 'error',
      code: 'orchestra.internal',
      message: 'CortexOS returned invalid orchestra tasks',
    })
  })

  it('normalizes bridge error responses', async () => {
    const bridge = mockBridge()
    vi.mocked(bridge.ListOrchestraTasks).mockRejectedValue({
      code: 'orchestra.storage_unavailable',
      message: 'database offline',
    })
    const state = await loadOrchestraTasks(bridge)
    expect(state).toEqual({
      status: 'error',
      code: 'orchestra.storage_unavailable',
      message: 'database offline',
    })
  })

  it('fetches single task detail with validated structure', async () => {
    const bridge = mockBridge()
    const result = await getOrchestraTask(bridge, 'task-1')
    expect(result.id).toBe('task-1')
    expect(result.acceptanceCount).toBe(2)
    expect(bridge.GetOrchestraTask).toHaveBeenCalledWith({
      schemaVersion: orchestraSchemaVersion,
      taskId: 'task-1',
    })
  })

  it('throws error when task detail fails validation', async () => {
    const bridge = mockBridge()
    vi.mocked(bridge.GetOrchestraTask).mockResolvedValue({ id: 'invalid' } as never)
    await expect(getOrchestraTask(bridge, 'task-1')).rejects.toThrow(
      'CortexOS returned invalid orchestra task detail',
    )
  })

  it('dispatches cancel command through the typed boundary', async () => {
    const bridge = mockBridge()
    await cancelOrchestraTask(bridge, 'task-1')
    expect(bridge.CancelOrchestraTask).toHaveBeenCalledWith({
      schemaVersion: orchestraSchemaVersion,
      taskId: 'task-1',
    })
  })

  it('dispatches retry command through the typed boundary', async () => {
    const bridge = mockBridge()
    await retryOrchestraTask(bridge, 'task-1')
    expect(bridge.RetryOrchestraTask).toHaveBeenCalledWith({
      schemaVersion: orchestraSchemaVersion,
      taskId: 'task-1',
    })
  })
})

describe('orchestra runtime event subscription seam', () => {
  it('returns no-op unsubscribe when runtime is unavailable', () => {
    const originalRuntime = (globalThis as { runtime?: unknown }).runtime
    delete (globalThis as { runtime?: unknown }).runtime

    const unsub = onOrchestraEvent(OrchestraEventNames.TaskChanged, () => {})
    expect(typeof unsub).toBe('function')
    unsub()

    ;(globalThis as { runtime?: unknown }).runtime = originalRuntime
  })

  it('subscribes and receives event callbacks via runtime.EventsOn', () => {
    const offFn = vi.fn()
    const eventsOn = vi.fn().mockReturnValue(offFn)
    ;(globalThis as { runtime?: unknown }).runtime = { EventsOn: eventsOn }

    const handler = vi.fn()
    const unsub = onOrchestraTaskChanged(handler)

    expect(eventsOn).toHaveBeenCalledWith(OrchestraEventNames.TaskChanged, expect.any(Function))

    const callback = eventsOn.mock.calls[0][1]
    callback({ taskId: 'task-1' })
    expect(handler).toHaveBeenCalledWith({ taskId: 'task-1' })

    unsub()
    expect(offFn).toHaveBeenCalled()
  })
})
