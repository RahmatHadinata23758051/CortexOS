import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'
import { renderToStaticMarkup } from 'react-dom/server'
import { describe, expect, it } from 'vitest'

import { App } from './App'

describe('App shell', () => {
  it('renders the workspace cockpit shell', () => {
    const markup = renderToStaticMarkup(<App />)
    expect(markup).toContain('Keep the ground')
    expect(markup).toContain('Register a project')
    expect(markup).toContain('Repository root')
  })

  it('includes skip link for accessibility', () => {
    const markup = renderToStaticMarkup(<App />)
    expect(markup).toContain('Skip to main content')
    expect(markup).toContain('id="main-content"')
  })

  it('includes theme toggle button with icon and label', () => {
    const markup = renderToStaticMarkup(<App />)
    expect(markup).toContain('theme-toggle')
    expect(markup).toContain('Dark mode')
    expect(markup).toContain('☾')
  })

  it('includes navigation rail with workspace, operations, activity, settings', () => {
    const markup = renderToStaticMarkup(<App />)
    expect(markup).toContain('Workspace')
    expect(markup).toContain('Operations')
    expect(markup).toContain('Activity')
    expect(markup).toContain('Settings')
    expect(markup).toContain('Primary navigation')
  })

  it('defines focus-visible outline styles and focus token', () => {
    const css = readFileSync(resolve(import.meta.dirname, 'styles.css'), 'utf8')
    expect(css).toContain('--focus-ring')
    expect(css).toContain(':focus-visible')
  })

  it('applies light theme by default', () => {
    const markup = renderToStaticMarkup(<App initialTheme="light" />)
    expect(markup).toContain('data-theme="light"')
  })

  it('includes proper ARIA attributes on navigation items', () => {
    const markup = renderToStaticMarkup(<App />)
    expect(markup).toContain('aria-current="page"')
    expect(markup).toContain('aria-label')
  })

  it('renders route placeholder content for non-workspace routes', () => {
    const markup = renderToStaticMarkup(<App initialRoute="activity" />)
    expect(markup).toContain('is on the way')
    expect(markup).toContain('Coming soon')
  })

  it('renders the task board on operations route', () => {
    const markup = renderToStaticMarkup(<App initialRoute="operations" />)
    expect(markup).toContain('Task board')
    expect(markup).toContain('orchestra.bridge.v1')
  })
})

describe('Theme state', () => {
  it('renders the light-mode control when dark theme is active', () => {
    const markup = renderToStaticMarkup(<App initialTheme="dark" />)
    expect(markup).toContain('Light mode')
    expect(markup).toContain('☀')
  })
})

describe('Routing', () => {
  it('exposes shell route state', () => {
    const markup = renderToStaticMarkup(<App />)
    expect(markup).toContain('Workspace')
    expect(markup).toContain('Operations')
    expect(markup).toContain('Activity')
    expect(markup).toContain('Settings')
  })
})
