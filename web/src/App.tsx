import { useEffect, useState, type FormEvent } from 'react'

import { loadRuntimeSnapshot, type RuntimeState } from './bridge/runtime'
import { loadWorkspaceSnapshot, type WorkspaceState } from './bridge/workspace'
import { wailsRuntimeBridge, workspaceSchemaVersion, type WorkspaceProject } from './types'
import { OfficeView } from './office/OfficeView'
import { TaskBoard } from './taskboard/TaskBoard'
import { KnowledgeView } from './office/KnowledgeView'

export type ProjectFormState = {
  id: string
  name: string
  repositoryRoot: string
  vaultRoot: string
  defaultBranch: string
}

export type ShellRoute = 'workspace' | 'operations' | 'activity' | 'office' | 'knowledge' | 'settings'
export type Theme = 'light' | 'dark'

export interface AppProps {
  initialRoute?: ShellRoute
  initialTheme?: Theme
}

const initialState: RuntimeState = { status: 'loading' }
const initialWorkspace: WorkspaceState = { status: 'loading' }
const emptyProject: ProjectFormState = {
  id: '',
  name: '',
  repositoryRoot: '',
  vaultRoot: '',
  defaultBranch: 'main',
}

const navigation: Array<{ route: ShellRoute; label: string; icon: string; detail: string }> = [
  { route: 'workspace', label: 'Workspace', icon: '◈', detail: 'Projects and roots' },
  { route: 'operations', label: 'Operations', icon: '⌁', detail: 'Execution surface' },
  { route: 'activity', label: 'Activity', icon: '▦', detail: 'Recent events' },
  { route: 'office', label: 'Office', icon: '□', detail: 'Virtual office' },
  { route: 'knowledge', label: 'Knowledge', icon: '⌘', detail: 'Vault browser' },
]

export function App({ initialRoute, initialTheme }: AppProps = {}) {
  const [runtime, setRuntime] = useState<RuntimeState>(initialState)
  const [workspace, setWorkspace] = useState<WorkspaceState>(initialWorkspace)
  const [form, setForm] = useState<ProjectFormState>(emptyProject)
  const [mutationState, setMutationState] = useState<'idle' | 'saving' | 'success' | 'error'>('idle')
  const [mutationMessage, setMutationMessage] = useState('')
  const [route, setRoute] = useState<ShellRoute>(() => initialRoute ?? getRouteFromLocation())
  const [theme, setTheme] = useState<Theme>(() => initialTheme ?? getThemePreference())

  const refreshWorkspace = () => {
    setWorkspace({ status: 'loading' })
    void loadWorkspaceSnapshot(wailsRuntimeBridge).then(setWorkspace)
  }

  const submitProject = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault()
    if (!form.id || !form.name || !form.repositoryRoot || !form.vaultRoot) {
      setMutationState('error')
      setMutationMessage('Project ID, name, repository root, and Vault root are required.')
      return
    }
    setMutationState('saving')
    setMutationMessage('')
    try {
      await wailsRuntimeBridge.RegisterWorkspaceProject({ ...form, schemaVersion: workspaceSchemaVersion })
      setMutationState('success')
      setMutationMessage('Project registered in the local Workspace.')
      setForm(emptyProject)
      refreshWorkspace()
    } catch (error) {
      setMutationState('error')
      setMutationMessage(error instanceof Error ? error.message : 'Project registration failed.')
    }
  }

  useEffect(() => {
    let active = true
    void loadRuntimeSnapshot(wailsRuntimeBridge).then((nextState) => {
      if (active) setRuntime(nextState)
    })
    void loadWorkspaceSnapshot(wailsRuntimeBridge).then((nextState) => {
      if (active) setWorkspace(nextState)
    })
    return () => {
      active = false
    }
  }, [])

  useEffect(() => {
    const handleHashChange = () => setRoute(getRouteFromLocation())
    window.addEventListener('hashchange', handleHashChange)
    return () => window.removeEventListener('hashchange', handleHashChange)
  }, [])

  useEffect(() => {
    if (typeof document !== 'undefined') {
      document.documentElement.dataset.theme = theme
    }
    if (typeof window !== 'undefined' && window.localStorage) {
      window.localStorage.setItem('cortexos-theme', theme)
    }
  }, [theme])

  const updateForm = (key: keyof ProjectFormState, value: string) => {
    setForm((current) => ({ ...current, [key]: value }))
    setMutationState('idle')
    setMutationMessage('')
  }

  return (
    <div className="shell" data-theme={theme}>
      <a className="skip-link" href="#main-content">Skip to main content</a>
      <header className="topbar">
        <a className="brand" href="#workspace" aria-label="CortexOS workspace home">
          <span className="brand-mark" aria-hidden="true">C/OS</span>
          <span className="topbar-title"><span>Workspace layer</span><strong>Local control surface</strong></span>
        </a>
        <div className="topbar-actions">
          <div className="topbar-status"><i aria-hidden="true" /> LOCAL / SAFE MODE</div>
          <button className="theme-toggle" type="button" onClick={() => setTheme((current) => current === 'light' ? 'dark' : 'light')} aria-label={`Switch to ${theme === 'light' ? 'dark' : 'light'} theme`}>
            <span aria-hidden="true">{theme === 'light' ? '☾' : '☀'}</span><span className="theme-toggle-label">{theme === 'light' ? 'Dark mode' : 'Light mode'}</span>
          </button>
        </div>
      </header>

      <div className="workspace-grid">
        <aside className="rail" aria-label="Primary navigation">
          <div className="rail-label">Navigation</div>
          <nav className="rail-nav">
            {navigation.map((item) => <a className={`rail-item${route === item.route ? ' active' : ''}`} href={`#${item.route}`} aria-current={route === item.route ? 'page' : undefined} key={item.route}>
              <span aria-hidden="true">{item.icon}</span><span>{item.label}</span>{item.route !== 'workspace' && item.route !== 'office' && item.route !== 'operations' && <em>soon</em>}
            </a>)}
          </nav>
          <a className={`rail-item rail-settings${route === 'settings' ? ' active' : ''}`} href="#settings" aria-current={route === 'settings' ? 'page' : undefined}><span aria-hidden="true">⚙</span><span>Settings</span></a>
          <div className="rail-footnote">
            <span className="eyebrow">Phase 02 / 06</span>
            <p>Facts first. Execution stays outside this layer.</p>
          </div>
        </aside>

        <main id="main-content" className="content" aria-live="polite">
          {route === 'workspace' ? (
            <WorkspaceView
              runtime={runtime}
              workspace={workspace}
              form={form}
              mutationState={mutationState}
              mutationMessage={mutationMessage}
              onRefresh={refreshWorkspace}
              onSubmit={submitProject}
              onUpdateForm={updateForm}
            />
          ) : route === 'operations' ? (
            <TaskBoard bridge={wailsRuntimeBridge} />
          ) : route === 'office' ? (
            <div className="office-section reveal-1">
              <div className="section-heading">
                <div><span className="eyebrow">Presentation layer</span><h2>Virtual office</h2></div>
                <span className="section-index">01 / 01</span>
              </div>
              <OfficeView />
            </div>
          ) : route === 'knowledge' ? (
            <KnowledgeView />
          ) : (
            <RoutePlaceholder route={route} />
          )}
        </main>
      </div>
    </div>
  )
}

export function WorkspaceView({
  runtime,
  workspace,
  form,
  mutationState,
  mutationMessage,
  onRefresh,
  onSubmit,
  onUpdateForm,
}: {
  runtime: RuntimeState
  workspace: WorkspaceState
  form: ProjectFormState
  mutationState: 'idle' | 'saving' | 'success' | 'error'
  mutationMessage: string
  onRefresh: () => void
  onSubmit: (event: FormEvent<HTMLFormElement>) => void
  onUpdateForm: (key: keyof ProjectFormState, value: string) => void
}) {
  return <>
    <div className="page-heading reveal-1">
      <div><span className="eyebrow">CORTEXOS / WORKSPACE</span><h1>Keep the ground<br /><i>reliable.</i></h1><p className="lede">A bounded view of your local projects, isolated worktrees, and durable knowledge.</p></div>
      <div className="schema-stamp"><span>CONTRACT</span><strong>workspace.v1</strong><small>validated boundary</small></div>
    </div>

    <div className="status-strip reveal-2">
      <StatusDot label="Runtime" value={runtime.status === 'success' ? 'Ready' : runtime.status === 'loading' ? 'Connecting' : 'Unavailable'} tone={runtime.status === 'success' ? 'good' : runtime.status === 'error' ? 'bad' : 'pending'} />
      <StatusDot label="Watcher" value={workspace.status === 'success' ? workspace.snapshot.watcherState : 'Loading'} tone={workspace.status === 'success' ? 'good' : 'pending'} />
      <StatusDot label="Retrieval" value={workspace.status === 'success' ? workspace.snapshot.retrievalState : 'Loading'} tone={workspace.status === 'success' ? 'good' : 'pending'} />
      <div className="status-count"><strong>{workspace.status === 'success' ? workspace.snapshot.vaultNoteCount : '—'}</strong><span>Vault notes</span></div>
    </div>

    {workspace.status === 'error' && <div className="error-banner reveal-3" role="alert"><strong>{workspace.code}</strong><span>{workspace.message}</span><button type="button" onClick={onRefresh}>Retry</button></div>}

    <div className="section-heading reveal-3"><div><span className="eyebrow">Registered roots</span><h2>Projects in orbit</h2></div><span className="section-index">01 / 02</span></div>
    <div className="project-list reveal-4">
      {workspace.status === 'loading' && <div className="empty-card skeleton">Reading local Workspace…</div>}
      {workspace.status === 'success' && workspace.snapshot.projects.length === 0 && <div className="empty-card"><span className="empty-glyph" aria-hidden="true">∅</span><div><strong>No projects registered</strong><p>Add the first trusted repository below.</p></div></div>}
      {workspace.status === 'success' && workspace.snapshot.projects.map((project) => <ProjectCard key={project.id} project={project} />)}
    </div>

    <div className="section-heading form-heading reveal-5"><div><span className="eyebrow">Controlled mutation</span><h2>Register a project</h2></div><span className="section-index">02 / 02</span></div>
    <form className="project-form reveal-5" onSubmit={onSubmit}>
      <div className="form-intro"><span className="form-number" aria-hidden="true">02</span><p>Roots are validated and stored by the Workspace boundary. The UI never touches the filesystem directly.</p></div>
      <div className="form-fields">
        <label>Project ID<input required value={form.id} onChange={(event) => onUpdateForm('id', event.target.value)} placeholder="studio-core" /></label>
        <label>Display name<input required value={form.name} onChange={(event) => onUpdateForm('name', event.target.value)} placeholder="Studio Core" /></label>
        <label className="wide">Repository root<input required aria-label="Repository root" value={form.repositoryRoot} onChange={(event) => onUpdateForm('repositoryRoot', event.target.value)} placeholder="C:\\Projects\\studio-core" /></label>
        <label className="wide">Vault root<input required aria-label="Vault root" value={form.vaultRoot} onChange={(event) => onUpdateForm('vaultRoot', event.target.value)} placeholder="C:\\Projects\\studio-core\\vault" /></label>
        <label>Default branch<input value={form.defaultBranch} onChange={(event) => onUpdateForm('defaultBranch', event.target.value)} placeholder="main" /></label>
        <button className="submit-button" type="submit" disabled={mutationState === 'saving'}>{mutationState === 'saving' ? 'Registering…' : 'Register project'} <span aria-hidden="true">↗</span></button>
      </div>
      {mutationMessage && <div className={`form-message ${mutationState}`} role={mutationState === 'error' ? 'alert' : 'status'}>{mutationMessage}</div>}
    </form>
  </>
}

export function RoutePlaceholder({ route }: { route: Exclude<ShellRoute, 'workspace' | 'office' | 'knowledge'> }) {
  const title = route === 'settings' ? 'Settings' : route === 'operations' ? 'Operations' : 'Activity'
  const eyebrow = route === 'settings' ? 'CORTEXOS / PREFERENCES' : `CORTEXOS / ${title.toUpperCase()}`
  return (
    <div className="route-placeholder reveal-1">
      <span className="eyebrow">{eyebrow}</span>
      <h1>{title}<br /><i>is on the way.</i></h1>
      <p className="lede">This surface is reserved for the next boundary. The shell is ready without exposing execution or filesystem authority.</p>
      <div className="coming-soon">
        <span aria-hidden="true">⌁</span>
        <div>
          <strong>Coming soon</strong>
          <p>Only the Workspace contract is active in this release.</p>
        </div>
      </div>
    </div>
  )
}

export function StatusDot({ label, value, tone }: { label: string; value: string; tone: 'good' | 'bad' | 'pending' }) {
  return <div className="status-item"><span>{label}</span><strong><i className={tone} aria-hidden="true" />{value}</strong></div>
}

export function ProjectCard({ project }: { project: WorkspaceProject }) {
  return <article className="project-card"><div className="project-sigil" aria-hidden="true">{project.name.slice(0, 1).toUpperCase()}</div><div className="project-info"><div className="project-title"><h3>{project.name}</h3><span className="badge">{project.status}</span></div><p>{project.id}</p></div><div className="project-meta"><span>BRANCH</span><strong>{project.defaultBranch || 'not set'}</strong></div><div className="project-arrow" aria-hidden="true">↗</div></article>
}

function getRouteFromLocation(): ShellRoute {
  if (typeof window === 'undefined') return 'workspace'
  const route = window.location.hash.slice(1) as ShellRoute
  return ['workspace', 'operations', 'activity', 'office', 'knowledge', 'settings'].includes(route) ? route : 'workspace'
}

function getThemePreference(): Theme {
  if (typeof window === 'undefined') return 'light'
  const stored = window.localStorage.getItem('cortexos-theme')
  return stored === 'dark' ? 'dark' : 'light'
}
