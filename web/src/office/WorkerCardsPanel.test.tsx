import { describe, expect, it, vi } from 'vitest'
import { renderToStaticMarkup } from 'react-dom/server'
import { workerLifecycleSchemaVersion, type WorkerLifecycleBridge } from '../types/worker-lifecycle'
import { WorkerCardsPanel } from './WorkerCardsPanel'

function mockBridge(): WorkerLifecycleBridge {
  return {
    GetWorkerCards: vi.fn().mockResolvedValue({
      schemaVersion: workerLifecycleSchemaVersion,
      projectId: 'proj-1',
      generatedAt: '2026-10-15T10:00:00Z',
      events: [],
      workers: [
        {
          workerId: 'worker-active', taskId: 'task-active', projectId: 'proj-1', callSign: 'Scout', role: 'reviewer', state: 'running',
          spawnedAt: '2026-10-15T09:00:00Z', updatedAt: '2026-10-15T10:00:00Z', schemaVersion: workerLifecycleSchemaVersion,
        },
        {
          workerId: 'worker-accepted', taskId: 'task-accepted', projectId: 'proj-1', state: 'accepted',
          spawnedAt: '2026-10-15T09:00:00Z', updatedAt: '2026-10-15T10:00:00Z', schemaVersion: workerLifecycleSchemaVersion,
        },
      ],
    }),
    TransitionWorker: vi.fn(),
  }
}

describe('WorkerCardsPanel', () => {
  it('renders the live active worker surface without inventing cards', () => {
    const markup = renderToStaticMarkup(<WorkerCardsPanel projectId="proj-1" bridge={mockBridge()} />)
    expect(markup).toContain('Ephemeral Workers')
    expect(markup).toContain('Loading worker cards')
    expect(markup).not.toContain('Scout')
  })
})
