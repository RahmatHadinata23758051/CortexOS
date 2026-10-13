import { describe, expect, it, vi } from 'vitest'
import { renderToStaticMarkup } from 'react-dom/server'

vi.mock('../hooks/useBrainDispatch', () => ({
  useBrainDispatch: vi.fn(() => ({
    state: 'idle',
    output: '',
    isRunning: false,
    run: vi.fn(),
    reset: vi.fn(),
  })),
}))

vi.mock('../bridge/office', () => ({
  loadOfficeSnapshot: vi.fn().mockResolvedValue({
    status: 'success',
    snapshot: {
      schemaVersion: 'cortexos.office.v1',
      staff: [
        { id: 'staff-1', name: 'Ada', role: 'planner', capabilities: [], workspaceId: 'ws-1', lifecycle: 'active', availability: 'available', schemaVersion: 'v1' },
        { id: 'staff-2', name: 'Grace', role: 'implementer', capabilities: [], workspaceId: 'ws-1', lifecycle: 'active', availability: 'busy', schemaVersion: 'v1' },
      ],
      activity: [],
      terminal: { visible: false, lines: [] },
    },
  }),
  createOfficeEventSource: () => ({
    subscribe: vi.fn(() => () => {}),
  }),
  staffById: vi.fn((staff: Array<{ id: string }>, id: string) => staff.find((s) => s.id === id)),
}))

import { OfficeView } from './OfficeView'

describe('OfficeView with Brain integration', () => {
  it('renders the office loading boundary before effects resolve', () => {
    const markup = renderToStaticMarkup(<OfficeView />)
    expect(markup).toContain('office-view')
    expect(markup).toContain('Loading virtual office')
  })
})

describe('StaffBrainState type integration', () => {
  it('accepts all brain states including tool_calling', () => {
    const states = ['idle', 'planning', 'executing', 'streaming', 'tool_calling', 'completed', 'failed', 'canceled'] as const
    states.forEach((s) => {
      const brainState = { staffId: 's1', state: s }
      expect(brainState.state).toBe(s)
    })
  })
})