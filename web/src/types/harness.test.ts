import { describe, expect, it } from 'vitest'
import {
  harnessSchemaVersion,
  isHarnessWorkerSummary,
  isHarnessExecutionSummary,
} from './harness'

describe('harness types and predicates', () => {
  it('identifies valid HarnessWorkerSummary', () => {
    const valid = {
      id: 'worker-1',
      instanceId: 'inst-1',
      kind: 'pi',
      class: 'pi',
      version: '1.0.0',
      workerSchema: 'cortexos.worker.v1',
      status: 'healthy',
      registeredAt: '2026-10-10T00:00:00Z',
      lastHeartbeatAt: '2026-10-10T00:00:00Z',
      schemaVersion: harnessSchemaVersion,
    }
    expect(isHarnessWorkerSummary(valid)).toBe(true)
    expect(isHarnessWorkerSummary(null)).toBe(false)
    expect(isHarnessWorkerSummary({ ...valid, schemaVersion: 'wrong.version' })).toBe(false)
  })

  it('identifies valid HarnessExecutionSummary', () => {
    const valid = {
      id: 'exec-1',
      taskId: 'task-1',
      engineClass: 'pi',
      priority: '1',
      memoryMB: 400,
      cpuPriority: 2,
      traceId: 'tr-1',
      workerId: 'w-1',
      createdAt: '2026-10-10T00:00:00Z',
      schemaVersion: harnessSchemaVersion,
    }
    expect(isHarnessExecutionSummary(valid)).toBe(true)
    expect(isHarnessExecutionSummary(null)).toBe(false)
    expect(isHarnessExecutionSummary({ ...valid, memoryMB: 'not a number' })).toBe(false)
  })
})
