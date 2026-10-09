import { renderToStaticMarkup } from 'react-dom/server'
import { describe, expect, it, vi } from 'vitest'
import { SessionHistory } from './SessionHistory'
import type { TerminalSession } from '../../hooks/useTerminal'

describe('SessionHistory', () => {
  const makeSession = (overrides: Partial<TerminalSession> = {}): TerminalSession => ({
    id: 'session-1',
    taskId: 'task-1',
    workerId: 'worker-1',
    status: 'terminated',
    startedAt: '2026-10-10T00:00:00Z',
    endedAt: '2026-10-10T00:01:00Z',
    lines: [],
    resources: { memoryMB: 0, memoryLimitMB: 2400, cpuPriority: 0, cpuLimit: 4, activeLeases: 0 },
    ...overrides,
  })

  it('renders empty history state', () => {
    const markup = renderToStaticMarkup(
      <SessionHistory history={[]} currentSessionId="current" selectedHistoryId={null} onSelectSession={vi.fn()} />
    )
    expect(markup).toContain('No recorded past sessions')
    expect(markup).toContain('PERSISTED RUNS')
  })

  it('renders history items with status badges and task ids', () => {
    const history = [makeSession({ id: 'sess-a', status: 'running' }), makeSession({ id: 'sess-b', status: 'terminated' })]
    const onSelect = vi.fn()
    const markup = renderToStaticMarkup(
      <SessionHistory history={history} currentSessionId="current" selectedHistoryId="sess-a" onSelectSession={onSelect} />
    )
    expect(markup).toContain('sess-a')
    expect(markup).toContain('sess-b')
    expect(markup).toContain('task-1')
    expect(markup).toContain('running')
    expect(markup).toContain('terminated')
  })

  it('highlights selected and active sessions', () => {
    const history = [makeSession({ id: 'sess-a' })]
    const markup = renderToStaticMarkup(
      <SessionHistory history={history} currentSessionId="sess-a" selectedHistoryId="sess-a" onSelectSession={vi.fn()} />
    )
    expect(markup).toContain('active-run')
    expect(markup).toContain('selected')
  })
})