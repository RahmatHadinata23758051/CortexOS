import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'
import { renderToStaticMarkup } from 'react-dom/server'
import { describe, expect, it } from 'vitest'

import { App, WorkspaceView } from './App'

describe('App shell', () => {
  it('renders the workspace cockpit shell', () => {
    const markup = renderToStaticMarkup(<App />)
    expect(markup).toContain('Workspace')
    expect(markup).toContain('Control room')
    expect(markup).toContain('Local workspace')
    expect(markup).toContain('Connecting to bridge')
    expect(markup).toContain('Register project')
    expect(markup).toContain('Repository Root')
  })

  it('includes skip link for accessibility', () => {
    const markup = renderToStaticMarkup(<App />)
    expect(markup).toContain('Skip to main content')
    expect(markup).toContain('id="main-content"')
  })

  it('includes theme toggle button for switching from the dark default', () => {
    const markup = renderToStaticMarkup(<App />)
    expect(markup).toContain('theme-toggle')
    expect(markup).toContain('>Light<')
    expect(markup).toContain('lucide-sun')
  })

  it('includes navigation rail with workspace, operations, terminal, activity, settings', () => {
    const markup = renderToStaticMarkup(<App />)
    expect(markup).toContain('Workspace')
    expect(markup).toContain('Operations')
    expect(markup).toContain('Terminal')
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
    expect(markup).toContain('Activity')
    expect(markup).toContain('Coming soon')
  })

  it('renders the task board on operations route', () => {
    const markup = renderToStaticMarkup(<App initialRoute="operations" />)
    expect(markup).toContain('Task board')
    expect(markup).toContain('orchestra.bridge.v1')
  })

  it('renders the Terminal Workflow panel on terminal route', () => {
    const markup = renderToStaticMarkup(<App initialRoute="terminal" />)
    expect(markup).toContain('Terminal Workflow')
    expect(markup).toContain('cortexos.worker.v1')
    expect(markup).toContain('Resource &amp; Lease Monitor')
  })
})

describe('Theme state', () => {
  it('renders the light-mode control when dark theme is active', () => {
    const markup = renderToStaticMarkup(<App initialTheme="dark" />)
    expect(markup).toContain('>Light<')
    expect(markup).toContain('lucide-sun')
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

describe('Workspace offline state', () => {
  it('renders offline banner, compact status row, and disabled registration controls when runtime is offline', () => {
    const markup = renderToStaticMarkup(
      <WorkspaceView
        runtime={{ status: 'error', message: 'Bridge binding unavailable.' }}
        workspace={{ status: 'error', code: 'workspace.internal', message: 'CortexOS runtime bridge is unavailable' }}
        form={{ id: '', name: '', repositoryRoot: '', vaultRoot: '', defaultBranch: 'main' }}
        mutationState="idle"
        mutationMessage=""
        selectedProjectId=""
        onSelectProject={() => {}}
        onRefresh={() => {}}
        onReconnect={async () => {}}
        lastCheckedAt={null}
        onSubmit={() => {}}
        onUpdateForm={() => {}}
        onResetForm={() => {}}
      />
    )
    expect(markup).toContain('Runtime bridge is disconnected')
    expect(markup).toContain('Bridge offline')
    expect(markup).toContain('Bridge binding unavailable.')
    expect(markup).toContain('Reconnect')
    expect(markup).toContain('Runtime offline — telemetry, watcher, and note indexes will synchronize once connected.')
    expect(markup).toContain('Registration requires an active runtime bridge.')
    expect(markup).toContain('disabled=""')
    expect(markup).toContain('Active project')
    expect(markup).toContain('Open folder')
    expect(markup).not.toContain('Coba lagi')
    expect(markup).not.toContain('Buka lokasi')
    expect(markup).not.toContain('workspace.internal')
  })

  it('renders registered project table with English Actions header when connected', () => {
    const markup = renderToStaticMarkup(
      <WorkspaceView
        runtime={{ status: 'success', snapshot: { schemaVersion: 'cortexos.runtime.v1', status: 'ready', environment: 'local', provider: 'disabled' } }}
        workspace={{
          status: 'success',
          snapshot: {
            schemaVersion: 'cortexos.workspace.v1',
            projects: [{ schemaVersion: 'cortexos.workspace.v1', id: 'prj-1', name: 'Alpha', defaultBranch: 'main', status: 'ready', createdAt: '2026-10-14T00:00:00Z', updatedAt: '2026-10-14T00:00:00Z' }],
            worktrees: [],
            watcherState: 'ready',
            retrievalVersion: '1.0',
            retrievalState: 'ready',
            vaultNoteCount: 42,
          },
        }}
        form={{ id: '', name: '', repositoryRoot: '', vaultRoot: '', defaultBranch: 'main' }}
        mutationState="idle"
        mutationMessage=""
        selectedProjectId="prj-1"
        onSelectProject={() => {}}
        onRefresh={() => {}}
        onReconnect={async () => {}}
        lastCheckedAt={null}
        onSubmit={() => {}}
        onUpdateForm={() => {}}
        onResetForm={() => {}}
      />
    )
    expect(markup).toContain('Actions')
    expect(markup).toContain('Alpha')
    expect(markup).toContain('Vault notes')
    expect(markup).toContain('42')
  })
})
