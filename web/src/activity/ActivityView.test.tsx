import { describe, expect, it } from 'vitest'
import { renderToStaticMarkup } from 'react-dom/server'
import { ActivityView } from './ActivityView'

describe('ActivityView', () => {
  it('renders header, filter controls, and empty state when no events exist', () => {
    const markup = renderToStaticMarkup(<ActivityView />)
    expect(markup).toContain('CORTEXOS / AUDIT STREAM')
    expect(markup).toContain('Activity')
    expect(markup).toContain('Filter activity')
    expect(markup).toContain('All')
    expect(markup).toContain('Task')
    expect(markup).toContain('Approval')
    expect(markup).toContain('System')
    expect(markup).toContain('NO EVENTS YET')
  })
})
