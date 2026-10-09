import { describe, expect, it, vi } from 'vitest'
import {
  loadHarnessCapabilities,
  loadHarnessWorkers,
  loadActiveHarnessExecutions,
  loadHarnessEvidence,
} from './harness'
import {
  harnessSchemaVersion,
  type HarnessBridge,
} from '../types/harness'

describe('harness bridge wrappers', () => {
  it('loads capabilities successfully', async () => {
    const bridge: HarnessBridge = {
      ListHarnessCapabilities: vi.fn().mockResolvedValue([
        { capability: 'shell', requiresAsk: true, schemaVersion: harnessSchemaVersion },
      ]),
      ListHarnessWorkers: vi.fn(),
      ListActiveHarnessExecutions: vi.fn(),
      GetHarnessEvidence: vi.fn(),
    }
    const result = await loadHarnessCapabilities(bridge)
    expect(result.status).toBe('success')
    if (result.status === 'success') {
      expect(result.capabilities).toHaveLength(1)
      expect(result.capabilities[0].capability).toBe('shell')
    }
  })

  it('normalizes bridge error gracefully', async () => {
    const bridge: HarnessBridge = {
      ListHarnessCapabilities: vi.fn().mockRejectedValue({ code: 'harness.store_unavailable', message: 'Store offline' }),
      ListHarnessWorkers: vi.fn(),
      ListActiveHarnessExecutions: vi.fn(),
      GetHarnessEvidence: vi.fn(),
    }
    const result = await loadHarnessCapabilities(bridge)
    expect(result.status).toBe('error')
    if (result.status === 'error') {
      expect(result.code).toBe('harness.store_unavailable')
      expect(result.message).toBe('Store offline')
    }
  })

  it('loads workers successfully', async () => {
    const bridge: HarnessBridge = {
      ListHarnessCapabilities: vi.fn(),
      ListHarnessWorkers: vi.fn().mockResolvedValue([
        {
          id: 'w-1',
          instanceId: 'inst-1',
          kind: 'pi',
          class: 'pi',
          version: '1.0',
          workerSchema: 'cortexos.worker.v1',
          status: 'healthy',
          registeredAt: '2026-10-10T00:00:00Z',
          lastHeartbeatAt: '2026-10-10T00:00:00Z',
          schemaVersion: harnessSchemaVersion,
        },
      ]),
      ListActiveHarnessExecutions: vi.fn(),
      GetHarnessEvidence: vi.fn(),
    }
    const result = await loadHarnessWorkers(bridge)
    expect(result.status).toBe('success')
  })

  it('loads active executions successfully', async () => {
    const bridge: HarnessBridge = {
      ListHarnessCapabilities: vi.fn(),
      ListHarnessWorkers: vi.fn(),
      ListActiveHarnessExecutions: vi.fn().mockResolvedValue([
        {
          id: 'e-1',
          taskId: 't-1',
          engineClass: 'pi',
          priority: '1',
          memoryMB: 400,
          cpuPriority: 2,
          traceId: 'tr-1',
          workerId: 'w-1',
          createdAt: '2026-10-10T00:00:00Z',
          schemaVersion: harnessSchemaVersion,
        },
      ]),
      GetHarnessEvidence: vi.fn(),
    }
    const result = await loadActiveHarnessExecutions(bridge)
    expect(result.status).toBe('success')
  })

  it('loads evidence successfully', async () => {
    const bridge: HarnessBridge = {
      ListHarnessCapabilities: vi.fn(),
      ListHarnessWorkers: vi.fn(),
      ListActiveHarnessExecutions: vi.fn(),
      GetHarnessEvidence: vi.fn().mockResolvedValue([
        {
          id: 'ev-1',
          executionId: 'e-1',
          taskId: 't-1',
          worktreeId: 'wt-1',
          kind: 'diff',
          digest: 'abc123',
          collectedAt: '2026-10-10T00:00:00Z',
          schemaVersion: harnessSchemaVersion,
        },
      ]),
    }
    const result = await loadHarnessEvidence(bridge, 'e-1')
    expect(result.status).toBe('success')
  })
})
