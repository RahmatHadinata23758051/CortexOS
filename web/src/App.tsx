import { useEffect, useState, type FormEvent } from 'react'

import { loadRuntimeSnapshot, type RuntimeState } from './bridge/runtime'
import { loadWorkspaceSnapshot, type WorkspaceState } from './bridge/workspace'
import { wailsRuntimeBridge, workspaceSchemaVersion, type WorkspaceProject } from './types'

type ProjectFormState = {
  id: string
  name: string
  repositoryRoot: string
  vaultRoot: string
  defaultBranch: string
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

export function App() {
  const [runtime, setRuntime] = useState<RuntimeState>(initialState)
  const [workspace, setWorkspace] = useState<WorkspaceState>(initialWorkspace)
  const [form, setForm] = useState<ProjectFormState>(emptyProject)
  const [mutationState, setMutationState] = useState<'idle' | 'saving' | 'success' | 'error'>('idle')
  const [mutationMessage, setMutationMessage] = useState('')

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

  const updateForm = (key: keyof ProjectFormState, value: string) => {
    setForm((current) => ({ ...current, [key]: value }))
    setMutationState('idle')
    setMutationMessage('')
  }

  return (
    <main className="shell">
      <header className="topbar">
        <div className="brand-mark" aria-hidden="true">C/OS</div>
        <div className="topbar-title">
          <span>Workspace layer</span>
          <strong>Local control surface</strong>
        </div>
        <div className="topbar-status"><i /> LOCAL / SAFE MODE</div>
      </header>

      <div className="workspace-grid">
        <aside className="rail">
          <div className="rail-label">Navigation</div>
          <button className="rail-item active"><span>◈</span> Workspace</button>
          <button className="rail-item" disabled><span>⌁</span> Operations <em>soon</em></button>
          <button className="rail-item" disabled><span>▦</span> Activity <em>soon</em></button>
          <div className="rail-footnote">
            <span className="eyebrow">Phase 02 / 06</span>
            <p>Facts first. Execution stays outside this layer.</p>
          </div>
        </aside>

        <section className="content" aria-live="polite">
          <div className="page-heading reveal-1">
            <div>
              <span className="eyebrow">CORTEXOS / WORKSPACE</span>
              <h1>Keep the ground<br /><i>reliable.</i></h1>
              <p className="lede">A bounded view of your local projects, isolated worktrees, and durable knowledge.</p>
            </div>
            <div className="schema-stamp"><span>CONTRACT</span><strong>workspace.v1</strong><small>validated boundary</small></div>
          </div>

          <div className="status-strip reveal-2">
            <StatusDot label="Runtime" value={runtime.status === 'success' ? 'Ready' : runtime.status === 'loading' ? 'Connecting' : 'Unavailable'} tone={runtime.status === 'success' ? 'good' : runtime.status === 'error' ? 'bad' : 'pending'} />
            <StatusDot label="Watcher" value={workspace.status === 'success' ? workspace.snapshot.watcherState : 'Loading'} tone={workspace.status === 'success' ? 'good' : 'pending'} />
            <StatusDot label="Retrieval" value={workspace.status === 'success' ? workspace.snapshot.retrievalState : 'Loading'} tone={workspace.status === 'success' ? 'good' : 'pending'} />
            <div className="status-count"><strong>{workspace.status === 'success' ? workspace.snapshot.vaultNoteCount : '—'}</strong><span>Vault notes</span></div>
          </div>

          {workspace.status === 'error' && <div className="error-banner reveal-3" role="alert"><strong>{workspace.code}</strong><span>{workspace.message}</span><button onClick={refreshWorkspace}>Retry</button></div>}

          <div className="section-heading reveal-3"><div><span className="eyebrow">Registered roots</span><h2>Projects in orbit</h2></div><span className="section-index">01 / 02</span></div>
          <div className="project-list reveal-4">
            {workspace.status === 'loading' && <div className="empty-card skeleton">Reading local Workspace…</div>}
            {workspace.status === 'success' && workspace.snapshot.projects.length === 0 && <div className="empty-card"><span className="empty-glyph">∅</span><div><strong>No projects registered</strong><p>Add the first trusted repository below.</p></div></div>}
            {workspace.status === 'success' && workspace.snapshot.projects.map((project) => <ProjectCard key={project.id} project={project} />)}
          </div>

          <div className="section-heading form-heading reveal-5"><div><span className="eyebrow">Controlled mutation</span><h2>Register a project</h2></div><span className="section-index">02 / 02</span></div>
          <form className="project-form reveal-5" onSubmit={submitProject}>
            <div className="form-intro"><span className="form-number">02</span><p>Roots are validated and stored by the Workspace boundary. The UI never touches the filesystem directly.</p></div>
            <div className="form-fields">
              <label>Project ID<input value={form.id} onChange={(event) => updateForm('id', event.target.value)} placeholder="studio-core" /></label>
              <label>Display name<input value={form.name} onChange={(event) => updateForm('name', event.target.value)} placeholder="Studio Core" /></label>
              <label className="wide">Repository root<input aria-label="Repository root" value={form.repositoryRoot} onChange={(event) => updateForm('repositoryRoot', event.target.value)} placeholder="C:\\Projects\\studio-core" /></label>
              <label className="wide">Vault root<input aria-label="Vault root" value={form.vaultRoot} onChange={(event) => updateForm('vaultRoot', event.target.value)} placeholder="C:\\Projects\\studio-core\\vault" /></label>
              <label>Default branch<input value={form.defaultBranch} onChange={(event) => updateForm('defaultBranch', event.target.value)} placeholder="main" /></label>
              <button className="submit-button" disabled={mutationState === 'saving'}>{mutationState === 'saving' ? 'Registering…' : 'Register project'} <span>↗</span></button>
            </div>
            {mutationMessage && <div className={`form-message ${mutationState}`} role={mutationState === 'error' ? 'alert' : 'status'}>{mutationMessage}</div>}
          </form>
        </section>
      </div>
    </main>
  )
}

function StatusDot({ label, value, tone }: { label: string; value: string; tone: 'good' | 'bad' | 'pending' }) {
  return <div className="status-item"><span>{label}</span><strong><i className={tone} />{value}</strong></div>
}

function ProjectCard({ project }: { project: WorkspaceProject }) {
  return <article className="project-card"><div className="project-sigil">{project.name.slice(0, 1).toUpperCase()}</div><div className="project-info"><div className="project-title"><h3>{project.name}</h3><span className="badge">{project.status}</span></div><p>{project.id}</p></div><div className="project-meta"><span>BRANCH</span><strong>{project.defaultBranch || 'not set'}</strong></div><div className="project-arrow" aria-hidden="true">↗</div></article>
}
