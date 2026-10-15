import { useState, type FormEvent, type ReactNode } from 'react'
import { FolderPlus, GitBranch, AlertTriangle, ArrowRight, RefreshCw, FolderGit2, Search, MapPin } from 'lucide-react'
import type { WorkspaceProject } from '../types/workspace'
import type { WorkspaceState } from '../bridge/workspace'
import type { RuntimeState } from '../bridge/runtime'

export interface ProjectFormState {
  id: string
  name: string
  repositoryRoot: string
  defaultBranch: string
}

export interface ProjectPickerProps {
  runtime: RuntimeState
  workspace: WorkspaceState
  form: ProjectFormState
  mutationState: 'idle' | 'saving' | 'success' | 'error'
  mutationMessage: string
  onSelectProject: (projectId: string) => void
  onRefresh: () => void
  onReconnect: () => Promise<void>
  onSubmit: (event: FormEvent<HTMLFormElement>) => void
  onUpdateForm: (key: keyof ProjectFormState, value: string) => void
  onResetForm: () => void
  onPickFolder: () => Promise<void>
}

export function ProjectPicker({
  runtime,
  workspace,
  form,
  mutationState,
  mutationMessage,
  onSelectProject,
  onRefresh,
  onReconnect,
  onSubmit,
  onUpdateForm,
  onResetForm,
  onPickFolder,
}: ProjectPickerProps): ReactNode {
  const [showRegisterForm, setShowRegisterForm] = useState(false)
  const [isReconnecting, setIsReconnecting] = useState(false)

  const isOffline = runtime.status === 'error'
  const isConnecting = runtime.status === 'loading'
  const canRegister = runtime.status === 'success'
  const projects = workspace.status === 'success' ? workspace.snapshot.projects : []

  const handleReconnect = async () => {
    setIsReconnecting(true)
    try {
      await onReconnect()
    } finally {
      setIsReconnecting(false)
    }
  }

  return (
    <div className="project-picker-shell" role="region" aria-label="Project and Office Selection">
      <div className="picker-container">
        <header className="picker-header">
          <div className="picker-brand">
            <span className="picker-logo" aria-hidden="true">C</span>
            <div>
              <span className="eyebrow">CORTEXOS / ENTRY</span>
              <h1 className="picker-title">Select Project or Office</h1>
            </div>
          </div>
          <div className="picker-header-actions">
            <div className={`runtime-status-pill runtime-${runtime.status}`} role="status">
              <span className="status-dot" aria-hidden="true" />
              <span>
                {runtime.status === 'success'
                  ? 'Runtime connected'
                  : isConnecting
                  ? 'Connecting to runtime'
                  : 'Runtime bridge unavailable'}
              </span>
            </div>
            {isOffline && (
              <button
                type="button"
                className="btn-secondary btn-sm"
                onClick={handleReconnect}
                disabled={isReconnecting}
                aria-label="Retry runtime bridge connection"
              >
                <RefreshCw size={13} className={isReconnecting ? 'spin' : ''} aria-hidden="true" />
                <span>{isReconnecting ? 'Connecting…' : 'Reconnect'}</span>
              </button>
            )}
            <button
              type="button"
              className="btn-primary btn-sm"
              onClick={() => {
                setShowRegisterForm((prev) => !prev)
                onResetForm()
              }}
              disabled={!canRegister}
              aria-expanded={showRegisterForm}
              aria-controls="register-project-drawer"
            >
              <FolderPlus size={14} aria-hidden="true" />
              <span>Register new project</span>
            </button>
          </div>
        </header>

        {isOffline && (
          <div className="picker-banner offline-banner" role="alert">
            <AlertTriangle size={16} aria-hidden="true" />
            <div className="banner-copy">
              <strong>Runtime bridge is disconnected</strong>
              <p>
                CortexOS needs a live Wails runtime bridge to query and initialize project workspaces.
                Make sure the native runtime is running.
              </p>
            </div>
          </div>
        )}

        {workspace.status === 'error' && (
          <div className="picker-banner error-banner" role="alert">
            <AlertTriangle size={16} aria-hidden="true" />
            <div className="banner-copy">
              <strong>Workspace error</strong>
              <p>
                <code>{workspace.code}: {workspace.message}</code>
              </p>
            </div>
            <button type="button" className="btn-secondary btn-sm" onClick={onRefresh}>
              Retry
            </button>
          </div>
        )}

        {showRegisterForm && (
          <section id="register-project-drawer" className="picker-register-card" aria-label="Register Project Form">
            <header className="card-header">
              <h3>Register Project</h3>
              <button
                type="button"
                className="btn-ghost btn-sm"
                onClick={() => {
                  setShowRegisterForm(false)
                  onResetForm()
                }}
                aria-label="Cancel registration"
              >
                Cancel
              </button>
            </header>
            <form onSubmit={onSubmit} className="register-form">
              <div className="form-grid">
                <label className="field-group">
                  <span>Project ID *</span>
                  <input
                    type="text"
                    value={form.id}
                    onChange={(e) => onUpdateForm('id', e.target.value)}
                    placeholder="proj-omega"
                    required
                  />
                </label>
                <label className="field-group">
                  <span>Display Name *</span>
                  <input
                    type="text"
                    value={form.name}
                    onChange={(e) => onUpdateForm('name', e.target.value)}
                    placeholder="Project Omega"
                    required
                  />
                </label>
                <div className="field-group project-location-field">
                  <span>Project Location *</span>
                  <button
                    type="button"
                    className="project-location-picker"
                    onClick={() => void onPickFolder()}
                    disabled={mutationState === 'saving'}
                  >
                    <MapPin size={15} aria-hidden="true" />
                    <span className="project-location-path">{form.repositoryRoot || 'Explore folders'}</span>
                    <Search size={14} aria-hidden="true" />
                  </button>
                  <small>
                    Choose the repository folder. CortexOS keeps the Vault inside <code>.cortexos/vault</code>.
                  </small>
                </div>
                <label className="field-group">
                  <span>Default Branch</span>
                  <input
                    type="text"
                    value={form.defaultBranch}
                    onChange={(e) => onUpdateForm('defaultBranch', e.target.value)}
                    placeholder="main"
                  />
                </label>
              </div>

              {mutationMessage && (
                <div
                  className={`form-message ${mutationState === 'error' ? 'is-error' : 'is-success'}`}
                  role={mutationState === 'error' ? 'alert' : 'status'}
                >
                  {mutationMessage}
                </div>
              )}

              <footer className="form-actions">
                <button
                  type="submit"
                  className="btn-primary"
                  disabled={mutationState === 'saving' || !canRegister}
                >
                  {mutationState === 'saving' ? 'Registering…' : 'Save and Open Workspace'}
                </button>
              </footer>
            </form>
          </section>
        )}

        <section className="projects-grid-section" aria-label="Available Projects">
          <div className="section-title-row">
            <h2>Local Workspaces ({projects.length})</h2>
            <button
              type="button"
              className="btn-ghost btn-sm"
              onClick={onRefresh}
              aria-label="Refresh project list"
            >
              <RefreshCw size={13} aria-hidden="true" />
              <span>Refresh</span>
            </button>
          </div>

          {workspace.status === 'loading' ? (
            <div className="picker-loading" role="status">
              <span className="spin-dot" aria-hidden="true" />
              <span>Loading projects…</span>
            </div>
          ) : projects.length === 0 ? (
            <div className="picker-empty-state">
              <FolderGit2 size={36} strokeWidth={1.5} className="empty-icon" aria-hidden="true" />
              <h3>No projects registered yet</h3>
              <p>Choose one project folder to enter the Orchestra workspace. CortexOS keeps project knowledge inside it.</p>
              <button
                type="button"
                className="btn-primary"
                onClick={() => setShowRegisterForm(true)}
                disabled={!canRegister}
              >
                <FolderPlus size={15} aria-hidden="true" />
                <span>Register your first project</span>
              </button>
            </div>
          ) : (
            <div className="projects-cards-grid" role="list">
              {projects.map((project: WorkspaceProject) => (
                <button
                  key={project.id}
                  type="button"
                  className="project-card"
                  role="listitem"
                  onClick={() => onSelectProject(project.id)}
                  aria-label={`Open project ${project.name}`}
                >
                  <div className="project-card-header">
                    <span className="project-avatar" aria-hidden="true">
                      {project.name.slice(0, 1).toUpperCase()}
                    </span>
                    <div className="project-card-title-wrap">
                      <strong className="project-name">{project.name}</strong>
                      <span className="project-id">{project.id}</span>
                    </div>
                    <span className={`project-status-badge status-${project.status.toLowerCase()}`}>
                      {project.status}
                    </span>
                  </div>

                  <div className="project-card-meta">
                    {project.defaultBranch && (
                      <span className="meta-branch">
                        <GitBranch size={12} aria-hidden="true" />
                        <span>{project.defaultBranch}</span>
                      </span>
                    )}
                    <span className="meta-updated">
                      Updated {new Date(project.updatedAt).toLocaleDateString()}
                    </span>
                  </div>

                  <div className="project-card-footer">
                    <span>Enter Workspace</span>
                    <ArrowRight size={14} aria-hidden="true" />
                  </div>
                </button>
              ))}
            </div>
          )}
        </section>
      </div>
    </div>
  )
}
