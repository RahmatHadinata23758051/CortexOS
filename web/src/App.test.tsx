import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'
import { describe, expect, it } from 'vitest'
import { renderToStaticMarkup } from 'react-dom/server'

import { App } from './App'
import { FilesView } from './files/FilesView'
import { ProjectPicker } from './picker/ProjectPicker'

const form = { id: '', name: '', repositoryRoot: '', vaultRoot: '', defaultBranch: 'main' }
const emptyWorkspace = {
  status: 'success' as const,
  snapshot: {
    schemaVersion: 'cortexos.workspace.v1' as const,
    projects: [],
    worktrees: [],
    watcherState: 'ready',
    retrievalVersion: '1',
    retrievalState: 'ready',
    vaultNoteCount: 0,
  },
}
const projectWorkspace = {
  status: 'success' as const,
  snapshot: {
    ...emptyWorkspace.snapshot,
    projects: [{ schemaVersion: 'cortexos.workspace.v1', id: 'prj-1', name: 'Alpha', defaultBranch: 'main', status: 'ready', createdAt: '2026-10-14T00:00:00Z', updatedAt: '2026-10-14T00:00:00Z' }],
    worktrees: [{ id: 'wt-1', projectId: 'prj-1', branch: 'main', status: 'ready', dirty: false, createdAt: '2026-10-14T00:00:00Z', updatedAt: '2026-10-14T00:00:00Z' }],
  },
}

const connectedRuntime = { status: 'success' as const, snapshot: { schemaVersion: 'cortexos.runtime.v1' as const, status: 'ready' as const, environment: 'local' as const, provider: 'disabled' as const } }

const pickerProps = {
  runtime: connectedRuntime,
  workspace: emptyWorkspace,
  form,
  mutationState: 'idle' as const,
  mutationMessage: '',
  onSelectProject: () => {},
  onRefresh: () => {},
  onReconnect: async () => {},
  onSubmit: () => {},
  onUpdateForm: () => {},
  onResetForm: () => {},
}

describe('Phase A app shell', () => {
  it('opens with the project picker instead of the retired dashboard', () => {
    const markup = renderToStaticMarkup(<App />)
    expect(markup).toContain('Select Project or Office')
    expect(markup).toContain('Register new project')
    expect(markup).not.toContain('Workspace metrics')
  })

  it('renders honest empty and offline picker states', () => {
    const empty = renderToStaticMarkup(<ProjectPicker {...pickerProps} />)
    expect(empty).toContain('No projects registered yet')

    const offline = renderToStaticMarkup(<ProjectPicker {...pickerProps} runtime={{ status: 'error', message: 'Bridge unavailable' }} />)
    expect(offline).toContain('Runtime bridge is disconnected')
    expect(offline).toContain('Reconnect')
  })

  it('exposes only Orchestra, Files, and Settings in the workspace sidebar', () => {
    const markup = renderToStaticMarkup(<App initialRoute="settings" initialProjectId="prj-1" initialWorkspace={projectWorkspace} />)
    expect(markup).toContain('Orchestra')
    expect(markup).toContain('Files')
    expect(markup).toContain('Settings')
    expect(markup).not.toContain('Operations')
    expect(markup).not.toContain('Staff')
    expect(markup).toContain('workspace-nav-item')
    expect(markup).toContain('aria-current="page"')
  })

  it('maps the legacy office route to Orchestra', () => {
    const markup = renderToStaticMarkup(<App initialRoute="office" />)
    expect(markup).toContain('Select Project or Office')
  })

  it('renders project-scoped Files browser and viewer panes', () => {
    const markup = renderToStaticMarkup(<FilesView workspace={projectWorkspace} selectedProjectId="prj-1" onRefresh={() => {}} />)
    expect(markup).toContain('Repository')
    expect(markup).toContain('Support')
    expect(markup).toContain('Vault')
    expect(markup).toContain('main')
    expect(markup).toContain('Viewer')
    expect(markup).toContain('Select a file to read its current snapshot.')
  })

  it('keeps accessibility skip link and theme control in the workspace shell', () => {
    const markup = renderToStaticMarkup(<App initialTheme="light" initialProjectId="prj-1" initialWorkspace={projectWorkspace} />)
    expect(markup).toContain('Skip to main content')
    expect(markup).toContain('data-theme="light"')
    expect(markup).toContain('Switch to dark theme')
    expect(markup).toContain('Collapse sidebar')
    expect(markup).toContain('Open navigation')
  })

  it('keeps the picker reachable with a skip link when no project is selected', () => {
    const markup = renderToStaticMarkup(<App initialTheme="light" />)
    expect(markup).toContain('Skip to main content')
    expect(markup).toContain('id="project-picker-main"')
    expect(markup).toContain('data-theme="light"')
  })

  it('includes responsive and reduced-motion sidebar rules', () => {
    const css = readFileSync(resolve(import.meta.dirname, 'styles.css'), 'utf8')
    expect(css).toContain('.app-sidebar')
    expect(css).toContain('.mobile-sidebar-backdrop')
    expect(css).toContain('@media (max-width: 768px)')
    expect(css).toContain('prefers-reduced-motion')
    expect(css).toContain('.files-area-tab')
    expect(css).toContain('.project-card')
  })
})
