import { useState } from 'react'
import { type RuntimeState } from '../bridge/runtime'
import { type WorkspaceState } from '../bridge/workspace'
import { type Theme } from '../App'
import { wailsRuntimeBridge, workspaceSchemaVersion } from '../types'

export type SettingsViewProps = {
  theme: Theme
  onThemeChange: (theme: Theme) => void
  runtime: RuntimeState
  workspace: WorkspaceState
}

export function SettingsView({ theme, onThemeChange, runtime, workspace }: SettingsViewProps) {
  const [rebuilding, setRebuilding] = useState(false)
  const [rebuildStatus, setRebuildStatus] = useState<string>('')
  const projects = workspace.status === 'success' ? workspace.snapshot.projects : []
  const [selectedProject, setSelectedProject] = useState(projects[0]?.id ?? '')

  const handleRebuild = async () => {
    const targetProject = selectedProject || projects[0]?.id
    if (!targetProject) {
      setRebuildStatus('No project selected to rebuild.')
      return
    }
    setRebuilding(true)
    setRebuildStatus('Rebuilding retrieval index...')
    try {
      await wailsRuntimeBridge.RebuildWorkspaceRetrieval({
        schemaVersion: workspaceSchemaVersion,
        projectId: targetProject,
      })
      setRebuildStatus(`Index rebuild requested for project ${targetProject}.`)
    } catch (err) {
      setRebuildStatus(`Rebuild failed: ${err instanceof Error ? err.message : String(err)}`)
    } finally {
      setRebuilding(false)
    }
  }

  const snapshot = runtime.status === 'success' ? runtime.snapshot : null
  const wsSnapshot = workspace.status === 'success' ? workspace.snapshot : null

  return (
    <section className="settings-page reveal-1" aria-labelledby="settings-title">
      <header className="tool-heading">
        <span className="eyebrow">CORTEXOS / PREFERENCES</span>
        <h1 id="settings-title">Settings & Diagnostics</h1>
        <p>Operational diagnostics, telemetry contract baseline, and interface theme preference.</p>
      </header>

      <div className="settings-grid">
        <div className="settings-card">
          <span className="eyebrow">PREFERENCES</span>
          <h2>Visual Theme</h2>
          <p className="settings-desc">Choose between drafting paper high contrast light mode and obsidian console dark mode.</p>
          <div className="theme-options" role="radiogroup" aria-label="Visual theme">
            <button
              type="button"
              className={`theme-option-btn${theme === 'dark' ? ' selected' : ''}`}
              role="radio"
              aria-checked={theme === 'dark'}
              onClick={() => onThemeChange('dark')}
            >
              <strong>Obsidian Dark</strong>
              <small>Low-emission technical palette for extended execution</small>
            </button>
            <button
              type="button"
              className={`theme-option-btn${theme === 'light' ? ' selected' : ''}`}
              role="radio"
              aria-checked={theme === 'light'}
              onClick={() => onThemeChange('light')}
            >
              <strong>Drafting Paper Light</strong>
              <small>Warm archival paper contrast with crisp technical ink</small>
            </button>
          </div>
        </div>

        <div className="settings-card">
          <span className="eyebrow">SYSTEM HEALTH</span>
          <h2>Runtime Diagnostics</h2>
          <div className="diag-rows">
            <div className="diag-row">
              <span className="diag-label">Runtime Contract</span>
              <code className="diag-val">{snapshot?.schemaVersion ?? 'cortexos.runtime.v1'}</code>
            </div>
            <div className="diag-row">
              <span className="diag-label">Bridge Status</span>
              <span className={`badge status-${runtime.status}`}>
                {runtime.status.toUpperCase()}
              </span>
            </div>
            <div className="diag-row">
              <span className="diag-label">Environment</span>
              <code className="diag-val">{snapshot?.environment ?? 'local'}</code>
            </div>
            <div className="diag-row">
              <span className="diag-label">Provider Subsystem</span>
              <span className="badge">{snapshot?.provider ?? 'disabled'}</span>
            </div>
            <div className="diag-row">
              <span className="diag-label">Retrieval Engine</span>
              <code className="diag-val">{wsSnapshot?.retrievalVersion ?? 'v1.sqlite-fts5'} ({wsSnapshot?.retrievalState ?? 'ready'})</code>
            </div>
            <div className="diag-row">
              <span className="diag-label">Vault Notes</span>
              <span className="diag-val">{wsSnapshot?.vaultNoteCount ?? 0} indexed</span>
            </div>
          </div>
        </div>

        <div className="settings-card settings-card-wide">
          <span className="eyebrow">MAINTENANCE</span>
          <h2>Workspace Retrieval Index</h2>
          <p className="settings-desc">Reconstruct local vector and full-text search indices directly from disk notes.</p>
          <div className="maintenance-form">
            {projects.length > 0 ? (
              <div className="m-input-group">
                <label htmlFor="rebuild-project-select">Project:</label>
                <select
                  id="rebuild-project-select"
                  value={selectedProject || projects[0]?.id}
                  onChange={(e) => setSelectedProject(e.target.value)}
                  className="settings-select"
                >
                  {projects.map((p) => (
                    <option key={p.id} value={p.id}>{p.name} ({p.id})</option>
                  ))}
                </select>
              </div>
            ) : (
              <p className="settings-notice">Register a project in Workspace before running maintenance.</p>
            )}
            <button
              type="button"
              className="action-btn"
              disabled={rebuilding || projects.length === 0}
              onClick={handleRebuild}
            >
              {rebuilding ? 'Rebuilding...' : 'Rebuild Retrieval Index'}
            </button>
          </div>
          {rebuildStatus && <div className="maintenance-status" role="status"><small>{rebuildStatus}</small></div>}
        </div>
      </div>
    </section>
  )
}
