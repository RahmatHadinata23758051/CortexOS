import { describe, expect, it, vi } from 'vitest'
import { renderToStaticMarkup } from 'react-dom/server'
import { SettingsView } from './SettingsView'

vi.mock('../types', () => ({
  wailsRuntimeBridge: {},
  workspaceSchemaVersion: 'cortexos.workspace.v1',
}))

describe('SettingsView', () => {
  it('renders preference controls, diagnostics, and maintenance section', () => {
    const markup = renderToStaticMarkup(<SettingsView
      theme="dark"
      onThemeChange={() => undefined}
      runtime={{ status: 'success', snapshot: { schemaVersion: 'cortexos.runtime.v1', status: 'ready', environment: 'local', provider: 'disabled' } }}
      workspace={{ status: 'success', snapshot: { schemaVersion: 'cortexos.workspace.v1', projects: [], worktrees: [], vaultNoteCount: 0, watcherState: 'ready', retrievalVersion: 'fts5', retrievalState: 'ready' } }}
    />)
    expect(markup).toContain('Settings &amp; Diagnostics')
    expect(markup).toContain('Obsidian Dark')
    expect(markup).toContain('Drafting Paper Light')
    expect(markup).toContain('Runtime Diagnostics')
    expect(markup).toContain('local')
    expect(markup).toContain('Workspace Retrieval Index')
    expect(markup).toContain('Register a project in Workspace')
  })
})
