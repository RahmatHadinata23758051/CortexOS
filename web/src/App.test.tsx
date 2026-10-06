import { renderToStaticMarkup } from 'react-dom/server'
import { describe, expect, it } from 'vitest'

import { App } from './App'

describe('App', () => {
  it('renders the workspace cockpit shell', () => {
    const markup = renderToStaticMarkup(<App />)
    expect(markup).toContain('Keep the ground')
    expect(markup).toContain('Register a project')
    expect(markup).toContain('Repository root')
  })
})
