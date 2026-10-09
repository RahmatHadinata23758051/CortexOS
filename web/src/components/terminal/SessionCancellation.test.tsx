import { renderToStaticMarkup } from 'react-dom/server'
import { describe, expect, it, vi } from 'vitest'
import { SessionCancellation } from './SessionCancellation'

describe('SessionCancellation', () => {
  it('renders enabled cancel seam for running sessions', () => {
    const markup = renderToStaticMarkup(
      <SessionCancellation sessionId="sess-1" sessionStatus="running" onCancel={vi.fn()} />
    )
    expect(markup).toContain('Cancel Session')
    expect(markup).not.toContain('disabled=""')
  })

  it('disables cancellation for terminal sessions', () => {
    const markup = renderToStaticMarkup(
      <SessionCancellation sessionId="sess-1" sessionStatus="terminated" onCancel={vi.fn()} />
    )
    expect(markup).toContain('disabled=""')
  })

  it('renders cancellation errors safely', () => {
    const markup = renderToStaticMarkup(
      <SessionCancellation sessionId="sess-1" sessionStatus="running" onCancel={vi.fn()} error="harness.canceled: cancellation unavailable" />
    )
    expect(markup).toContain('Cancellation failed')
    expect(markup).toContain('harness.canceled')
  })
})
