import { describe, expect, it, vi } from 'vitest'
import { renderToStaticMarkup } from 'react-dom/server'
import { brainBridgeSchemaVersion, type BrainBridge } from '../../types'
import { BrainPanel } from './BrainPanel'
import { BrainStreamViewer } from './StreamViewer'
import { ReasoningTimeline } from './ReasoningTimeline'
import { ContextInspector } from './ContextInspector'

const mockBridge: BrainBridge = {
  DispatchBrainTask: vi.fn().mockResolvedValue({
    taskId: 't1',
    staffId: '',
    state: 'completed',
    output: 'done',
    steps: 1,
    tokens: 10,
    durationMs: 100,
    error: '',
    schemaVersion: brainBridgeSchemaVersion,
  }),
  DispatchBrainTaskStream: vi.fn().mockResolvedValue({
    taskId: 't1',
    staffId: '',
    state: 'completed',
    output: 'done',
    steps: 1,
    tokens: 10,
    durationMs: 100,
    error: '',
    schemaVersion: brainBridgeSchemaVersion,
  }),
}

describe('BrainPanel component', () => {
  it('renders initial form and idle state', () => {
    const markup = renderToStaticMarkup(<BrainPanel bridge={mockBridge} />)
    expect(markup).toContain('Reasoning cockpit')
    expect(markup).toContain('idle')
    expect(markup).toContain('Run Brain')
  })

  it('renders with context snippets', () => {
    const snippets = [
      { id: '1', source: 'docs/arch.md', title: 'Architecture', content: 'Local-first architecture' },
    ]
    const markup = renderToStaticMarkup(<BrainPanel bridge={mockBridge} contextSnippets={snippets} />)
    expect(markup).toContain('Context Snippets')
    expect(markup).toContain('1 loaded')
  })
})

describe('BrainStreamViewer component', () => {
  it('renders empty placeholder when no output', () => {
    const markup = renderToStaticMarkup(<BrainStreamViewer output="" chunks={[]} isRunning={false} />)
    expect(markup).toContain('Enter a prompt and run')
    expect(markup).toContain('0 chunks')
  })

  it('renders markdown stream content', () => {
    const markup = renderToStaticMarkup(
      <BrainStreamViewer
        output="Hello world reasoning"
        chunks={[{ taskId: 't1', type: 'token', data: 'Hello', sequence: 1, timestamp: '' }]}
        isRunning={true}
      />,
    )
    expect(markup).toContain('Hello world reasoning')
    expect(markup).toContain('1 chunks')
    expect(markup).toContain('streaming')
  })
})

describe('ReasoningTimeline component', () => {
  it('renders phase list and active state', () => {
    const markup = renderToStaticMarkup(
      <ReasoningTimeline state="streaming" steps={3} tokens={120} durationMs={1500} />,
    )
    expect(markup).toContain('Streaming')
    expect(markup).toContain('3')
    expect(markup).toContain('120')
    expect(markup).toContain('1s')
  })
})

describe('ContextInspector component', () => {
  it('renders closed by default', () => {
    const markup = renderToStaticMarkup(<ContextInspector snippets={[]} isOpen={false} />)
    expect(markup).toContain('closed')
  })

  it('renders snippet list when open', () => {
    const snippets = [
      { id: '1', source: 'rules.md', title: 'Security Rules', content: 'Never expose API keys' },
    ]
    const markup = renderToStaticMarkup(<ContextInspector snippets={snippets} isOpen={true} />)
    expect(markup).toContain('open')
    expect(markup).toContain('Security Rules')
    expect(markup).toContain('rules.md')
  })
})
