import { describe, expect, it, vi } from 'vitest'
import { loadWorkerCardsSnapshot } from './worker-lifecycle'
import {
  workerLifecycleSchemaVersion,
  type WorkerLifecycleBridge,
  type WorkerCardsResult,
} from '../types/worker-lifecycle'

describe('loadWorkerCardsSnapshot', () => {
  it('passes schemaVersion, projectId, and includeHistory to the bridge', async () => {
    const expectedResult: WorkerCardsResult = {
      schemaVersion: workerLifecycleSchemaVersion,
      projectId: 'proj-omega',
      workers: [
        {
          workerId: 'worker-1',
          taskId: 'task-1',
          projectId: 'proj-omega',
          state: 'running',
          spawnedAt: '2026-10-15T00:00:00Z',
          updatedAt: '2026-10-15T00:00:00Z',
          schemaVersion: workerLifecycleSchemaVersion,
        },
      ],
      events: [],
      generatedAt: '2026-10-15T00:00:00Z',
    }

    const bridge: WorkerLifecycleBridge = {
      GetWorkerCards: vi.fn().mockResolvedValue(expectedResult),
      TransitionWorker: vi.fn(),
    }

    const result = await loadWorkerCardsSnapshot(bridge, 'proj-omega', false)
    expect(bridge.GetWorkerCards).toHaveBeenCalledWith({
      schemaVersion: workerLifecycleSchemaVersion,
      projectId: 'proj-omega',
      includeHistory: false,
    })
    expect(result).toBe(expectedResult)
  })
})
