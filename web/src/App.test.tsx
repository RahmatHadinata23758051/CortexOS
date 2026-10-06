import { renderToStaticMarkup } from 'react-dom/server'
import { describe, expect, it } from 'vitest'

import { App } from './App'

describe('App', () => {
  it('renders the foundation shell', () => {
    expect(renderToStaticMarkup(<App />)).toContain('CortexOS foundation')
  })
})
