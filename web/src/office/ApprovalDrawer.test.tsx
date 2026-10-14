import { describe, expect, it } from 'vitest'
import { renderToStaticMarkup } from 'react-dom/server'
import { ApprovalDrawer } from './ApprovalDrawer'

describe('ApprovalDrawer', () => {
  it('renders nothing when closed', () => {
    const markup = renderToStaticMarkup(<ApprovalDrawer isOpen={false} onClose={() => {}} />)
    expect(markup).toBe('')
  })

  it('renders drawer structure when open', () => {
    const markup = renderToStaticMarkup(<ApprovalDrawer isOpen={true} onClose={() => {}} />)
    expect(markup).toContain('Pending approvals')
    expect(markup).toContain('TOOL APPROVAL')
  })
})
