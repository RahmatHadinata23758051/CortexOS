import { renderToStaticMarkup } from 'react-dom/server'
import { describe, expect, it, vi } from 'vitest'
import { TerminalPanel } from './TerminalPanel'
import { harnessSchemaVersion, type HarnessBridge } from '../../types/harness'

describe('TerminalPanel workflow integration', () => {
  const mockBridge: HarnessBridge = {
    ListHarnessCapabilities: vi.fn().mockResolvedValue([
      { capability: 'shell', requiresAsk: true, schemaVersion: harnessSchemaVersion },
    ]),
    ListHarnessWorkers: vi.fn().mockResolvedValue([]),
    ListActiveHarnessExecutions: vi.fn().mockResolvedValue([
      {
        id: 'exec-1',
        taskId: 'task-test-01',
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
    GetHarnessEvidence: vi.fn().mockResolvedValue([]),
  }

  it('renders Terminal Workflow panel with contract stamp', () => {
    const markup = renderToStaticMarkup(<TerminalPanel bridge={mockBridge} initialTaskId="task-demo-42" />)
    expect(markup).toContain('Terminal Workflow')
    expect(markup).toContain('cortexos.worker.v1')
    expect(markup).toContain('task-demo-42')
    expect(markup).toContain('New Session')
    expect(markup).toContain('Load sample stream')
  })

  it('renders without bridge in safe standalone mode', () => {
    const markup = renderToStaticMarkup(<TerminalPanel initialTaskId="task-offline" />)
    expect(markup).toContain('Terminal Workflow')
    expect(markup).toContain('task-offline')
    expect(markup).toContain('JSONL Wire Stream')
    expect(markup).toContain('Resource &amp; Lease Monitor')
    expect(markup).toContain('Session History')
  })
})
