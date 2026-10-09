import { renderToStaticMarkup } from 'react-dom/server'
import { describe, expect, it } from 'vitest'
import { ResourceMonitor } from './ResourceMonitor'
import type { TerminalResourceSnapshot } from '../../hooks/useTerminal'
import { harnessSchemaVersion, type HarnessExecutionSummary } from '../../types/harness'

describe('ResourceMonitor', () => {
  it('renders memory and CPU gauges with default budgets', () => {
    const resources: TerminalResourceSnapshot = {
      memoryMB: 400,
      memoryLimitMB: 2400,
      cpuPriority: 2,
      cpuLimit: 4,
      activeLeases: 1,
    }
    const markup = renderToStaticMarkup(<ResourceMonitor resources={resources} />)
    expect(markup).toContain('Memory Lease')
    expect(markup).toContain('400 / 2400 MB')
    expect(markup).toContain('CPU Priority Slots')
    expect(markup).toContain('2 / 4')
    expect(markup).toContain('Active Worker Leases')
  })

  it('aggregates active executions when provided', () => {
    const resources: TerminalResourceSnapshot = {
      memoryMB: 100,
      memoryLimitMB: 2400,
      cpuPriority: 1,
      cpuLimit: 4,
      activeLeases: 0,
    }
    const executions: HarnessExecutionSummary[] = [
      {
        id: 'exec-1',
        taskId: 'task-1',
        engineClass: 'pi',
        priority: '1',
        memoryMB: 500,
        cpuPriority: 2,
        traceId: 'tr-1',
        workerId: 'w-1',
        createdAt: '2026-10-10T00:00:00Z',
        schemaVersion: harnessSchemaVersion,
      },
      {
        id: 'exec-2',
        taskId: 'task-2',
        engineClass: 'native',
        priority: '2',
        memoryMB: 300,
        cpuPriority: 1,
        traceId: 'tr-2',
        workerId: 'w-2',
        createdAt: '2026-10-10T00:00:00Z',
        schemaVersion: harnessSchemaVersion,
      },
    ]
    const markup = renderToStaticMarkup(<ResourceMonitor resources={resources} activeExecutions={executions} />)
    expect(markup).toContain('800 / 2400 MB')
    expect(markup).toContain('3 / 4')
    expect(markup).toContain('Active Execution Leases')
    expect(markup).toContain('exec-1')
    expect(markup).toContain('exec-2')
  })
})
