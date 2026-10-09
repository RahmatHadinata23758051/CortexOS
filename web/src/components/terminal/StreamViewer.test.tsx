import { renderToStaticMarkup } from 'react-dom/server'
import { describe, expect, it } from 'vitest'
import { StreamViewer } from './StreamViewer'
import type { TerminalLine } from '../../hooks/useTerminal'

describe('StreamViewer', () => {
  it('renders JSONL contract and empty state', () => {
    const markup = renderToStaticMarkup(<StreamViewer lines={[]} />)
    expect(markup).toContain('cortexos.worker.v1')
    expect(markup).toContain('No JSONL worker messages')
    expect(markup).toContain('Raw JSONL')
  })

  it('renders structured progress and malformed records', () => {
    const lines: TerminalLine[] = [
      {
        id: 'line-1',
        raw: '{"version":"cortexos.worker.v1"}',
        error: 'invalid correlation or sequence',
        receivedAt: '2026-10-10T00:00:00Z',
      },
      {
        id: 'line-2',
        raw: '{"version":"cortexos.worker.v1","type":"progress"}',
        message: {
          version: 'cortexos.worker.v1',
          type: 'progress',
          correlationId: 'exec-1',
          sequence: 2,
          payload: { phase: 'inspect', percent: 55, message: 'Reading context' },
        },
        receivedAt: '2026-10-10T00:00:01Z',
      },
    ]
    const markup = renderToStaticMarkup(<StreamViewer lines={lines} />)
    expect(markup).toContain('MALFORMED')
    expect(markup).toContain('Phase:')
    expect(markup).toContain('inspect')
  })
})
