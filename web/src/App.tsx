import { lazy, Suspense, useEffect, useState, type FormEvent, type ReactNode } from 'react'
import { Activity, Brain, Building2, ChevronDown, ChevronsUpDown, Crosshair, FileClock, FolderOpen, Gauge, LayoutGrid, Moon, RefreshCw, Search, Settings, Sparkles, Sun, TerminalSquare, UsersRound, Zap } from 'lucide-react'

import { loadRuntimeSnapshot, type RuntimeState } from './bridge/runtime'
import { loadWorkspaceSnapshot, queryWorkspace, type WorkspaceState } from './bridge/workspace'
import { wailsRuntimeBridge, workspaceSchemaVersion, type WorkspaceNoteResult, type WorkspaceProject } from './types'
const OfficeView = lazy(() => import('./office/OfficeView').then(({ OfficeView }) => ({ default: OfficeView })))
import { TaskBoard } from './taskboard/TaskBoard'
import { KnowledgeView } from './office/KnowledgeView'
import { StaffPanel } from './staff/StaffPanel'
import { TerminalPanel } from './components/terminal/TerminalPanel'
import { BrainPanel } from './components/brain/BrainPanel'
import { ApprovalDrawer } from './office/ApprovalDrawer'

export type ProjectFormState = {
  id: string
  name: string
  repositoryRoot: string
  vaultRoot: string
  defaultBranch: string
}

export type ShellRoute = 'workspace' | 'operations' | 'terminal' | 'activity' | 'office' | 'knowledge' | 'staff' | 'brain' | 'settings'
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

const navigation: Array<{ route: ShellRoute; label: string; icon: ReactNode }> = [
  { route: 'workspace', label: 'Workspace', icon: <LayoutGrid size={18} strokeWidth={1.8} /> },
  { route: 'operations', label: 'Operations', icon: <Activity size={18} strokeWidth={1.8} /> },
  { route: 'terminal', label: 'Terminal', icon: <TerminalSquare size={18} strokeWidth={1.8} /> },
  { route: 'activity', label: 'Activity', icon: <FileClock size={18} strokeWidth={1.8} /> },
  { route: 'office', label: 'Office', icon: <Building2 size={18} strokeWidth={1.8} /> },
  { route: 'knowledge', label: 'Knowledge', icon: <Gauge size={18} strokeWidth={1.8} /> },
  { route: 'staff', label: 'Staff', icon: <UsersRound size={18} strokeWidth={1.8} /> },
  { route: 'brain', label: 'Brain', icon: <Brain size={18} strokeWidth={1.8} /> },
]

const routeGroups: Record<ShellRoute, string> = {
  workspace: 'Workspace', operations: 'Execution', terminal: 'Execution', activity: 'Execution', office: 'Execution',
  knowledge: 'Intelligence', staff: 'Intelligence', brain: 'Intelligence', settings: 'System',
}

type NavItemData = { route: ShellRoute; label: string; icon: ReactNode }

function NavItem({ item, activeRoute, standalone = false }: { item: NavItemData; activeRoute: ShellRoute; standalone?: boolean }) {
  return <a className={`nav-item${standalone ? ' nav-item-standalone' : ' nav-child-item'}${activeRoute === item.route ? ' active' : ''}`} href={`#${item.route}`} aria-current={activeRoute === item.route ? 'page' : undefined}>
    <span className="nav-icon" aria-hidden="true">{item.icon}</span><span className="nav-label">{item.label}</span>
  </a>
}

function NavGroup({ title, routes, activeRoute }: { title: string; routes: ShellRoute[]; activeRoute: ShellRoute }) {
  const [expanded, setExpanded] = useState(true)
  const items = navigation.filter((item) => routes.includes(item.route))
  const id = `nav-group-${title.toLowerCase()}`
  return <div className={`nav-group${expanded ? '' : ' collapsed'}`}>
    <button className="nav-group-header" type="button" aria-expanded={expanded} aria-controls={id} onClick={() => setExpanded((value) => !value)}>
      <span className="nav-group-title">{title}</span><ChevronDown className="nav-group-chevron" size={14} aria-hidden="true" />
    </button>
    <nav className="nav-group-children" id={id} aria-label={title}>
      {items.map((item) => <NavItem key={item.route} item={item} activeRoute={activeRoute} />)}
      {title === 'System' && <NavItem item={{ route: 'settings', label: 'Settings', icon: <Settings size={18} strokeWidth={1.8} /> }} activeRoute={activeRoute} />}
    </nav>
  </div>
}

export function App({ initialRoute, initialTheme }: AppProps = {}) {
  const [runtime, setRuntime] = useState<RuntimeState>(initialState)
  const [workspace, setWorkspace] = useState<WorkspaceState>(initialWorkspace)
  const [form, setForm] = useState<ProjectFormState>(emptyProject)
  const [mutationState, setMutationState] = useState<'idle' | 'saving' | 'success' | 'error'>('idle')
  const [mutationMessage, setMutationMessage] = useState('')
  const [selectedProjectId, setSelectedProjectId] = useState('')
  const [route, setRoute] = useState<ShellRoute>(() => initialRoute ?? getRouteFromLocation())
  const [theme, setTheme] = useState<Theme>(() => {
    const nextTheme = initialTheme ?? getThemePreference()
    if (typeof document !== 'undefined') document.documentElement.dataset.theme = nextTheme
    return nextTheme
  })
  const [approvalDrawerOpen, setApprovalDrawerOpen] = useState(false)
  const [pendingApprovalCount, setPendingApprovalCount] = useState(0)
  const [lastCheckedAt, setLastCheckedAt] = useState<number | null>(null)

  const refreshRuntime = async () => {
    setRuntime({ status: 'loading' })
    const [nextRuntime, nextWorkspace] = await Promise.all([
      loadRuntimeSnapshot(wailsRuntimeBridge),
      loadWorkspaceSnapshot(wailsRuntimeBridge),
    ])
    setRuntime(nextRuntime)
    setWorkspace(nextWorkspace)
    setLastCheckedAt(Date.now())
  }

  const refreshWorkspace = () => {
    setWorkspace({ status: 'loading' })
    void loadWorkspaceSnapshot(wailsRuntimeBridge).then(setWorkspace)
  }

  const submitProject = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault()
    if (runtime.status !== 'success') {
      setMutationState('error')
      setMutationMessage('Registration requires an active runtime bridge.')
      return
    }
    if (!form.id.trim() || !form.name.trim() || !form.repositoryRoot.trim() || !form.vaultRoot.trim()) {
      setMutationState('error')
      setMutationMessage('Project ID, name, repository root, and Vault root are required.')
      return
    }
    setMutationState('saving')
    setMutationMessage('')
    try {
      const project = await wailsRuntimeBridge.RegisterWorkspaceProject({
        ...form,
        id: form.id.trim(),
        name: form.name.trim(),
        repositoryRoot: form.repositoryRoot.trim(),
        vaultRoot: form.vaultRoot.trim(),
        defaultBranch: form.defaultBranch.trim(),
        schemaVersion: workspaceSchemaVersion,
      })
      setSelectedProjectId(project.id)
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
    let active = true
    const refreshPendingApprovals = async () => {
      try {
        const pending = await wailsRuntimeBridge.ListPendingToolApprovals()
        if (active) setPendingApprovalCount(pending.length)
      } catch {
        if (active) setPendingApprovalCount(0)
      }
    }
    void refreshPendingApprovals()
    const interval = window.setInterval(refreshPendingApprovals, 5000)
    return () => {
      active = false
      window.clearInterval(interval)
    }
  }, [])

  useEffect(() => {
    document.documentElement.dataset.theme = theme
    window.localStorage.setItem('cortexos-theme', theme)
  }, [theme])

  const updateForm = (key: keyof ProjectFormState, value: string) => {
    setForm((current) => ({ ...current, [key]: value }))
    setMutationState('idle')
    setMutationMessage('')
  }

  const activeNavigation = navigation.find((item) => item.route === route)
  const currentGroup = routeGroups[route]
  const runtimeLabel = runtime.status === 'success' ? 'Connected' : runtime.status === 'loading' ? 'Connecting' : 'Unavailable'

  return <div className="shell" data-theme={theme}>
    <a className="skip-link" href="#main-content">Skip to main content</a>
    <div className="shell-grid">
      <aside className="rail" aria-label="Primary navigation">
        <a className="sidebar-switcher" href="#workspace" aria-label="CortexOS workspace home">
          <span className="switcher-logo" aria-hidden="true">C</span>
          <span className="switcher-info"><strong className="switcher-title">CortexOS</strong><small className="switcher-subtitle">Local workspace</small></span>
          <ChevronsUpDown className="switcher-chevron" size={16} aria-hidden="true" />
        </a>
        <div className="sidebar-scroll">
          <nav aria-label="Workspace navigation">
            <NavItem item={navigation[0]} activeRoute={route} standalone />
          </nav>
          <NavGroup title="Execution" routes={['operations', 'terminal', 'activity', 'office']} activeRoute={route} />
          <NavGroup title="Intelligence" routes={['knowledge', 'staff', 'brain']} activeRoute={route} />
          <NavGroup title="System" routes={[]} activeRoute={route} />
        </div>
        <div className="rail-runtime" role="status">
          <span className={`runtime-indicator runtime-${runtime.status}`} aria-hidden="true" />
          <span className="sidebar-footer-text">{runtime.status === 'success' ? 'Bridge connected' : runtime.status === 'loading' ? 'Connecting to bridge' : 'Bridge unavailable'}</span>
        </div>
      </aside>

      <header className="topbar">
        <div className="topbar-left">
          <div className="topbar-breadcrumb" aria-label="Breadcrumb">
            <span className="breadcrumb-segment">{currentGroup === 'Workspace' ? 'Control room' : currentGroup}</span>
            <span className="breadcrumb-separator" aria-hidden="true">›</span>
            <strong className="breadcrumb-current">{activeNavigation?.label ?? 'Settings'}</strong>
          </div>
        </div>
        <div className="topbar-actions">
          <div className={`topbar-status runtime-${runtime.status}`} role="status" aria-label={`Runtime ${runtimeLabel.toLowerCase()}`}>
            <span className="status-dot" aria-hidden="true" /><span>Runtime {runtimeLabel.toLowerCase()}</span>
          </div>
          <button type="button" className={`approval-badge-btn${approvalDrawerOpen ? ' active' : ''}${pendingApprovalCount > 0 ? ' has-pending' : ''}`} onClick={() => setApprovalDrawerOpen((value) => !value)} aria-label={`Tool approval queue: ${pendingApprovalCount} pending`} aria-expanded={approvalDrawerOpen}>
            <Zap size={14} strokeWidth={1.8} aria-hidden="true" /><span className="approval-label">Approvals</span>
            {pendingApprovalCount > 0 && <span className="approval-count" aria-live="polite">{pendingApprovalCount}</span>}
          </button>
          <button className="theme-toggle" type="button" onClick={() => setTheme((current) => current === 'light' ? 'dark' : 'light')} aria-label={`Switch to ${theme === 'light' ? 'dark' : 'light'} theme`}>
            <span aria-hidden="true">{theme === 'light' ? <Moon size={16} /> : <Sun size={16} />}</span><span className="theme-toggle-label">{theme === 'light' ? 'Dark' : 'Light'}</span>
          </button>
        </div>
      </header>

      <main id="main-content" className="content" aria-live="polite">
        {route === 'workspace' ? <WorkspaceView
          runtime={runtime}
          workspace={workspace}
          form={form}
          mutationState={mutationState}
          mutationMessage={mutationMessage}
          selectedProjectId={selectedProjectId}
          onSelectProject={setSelectedProjectId}
          onRefresh={refreshWorkspace}
          onReconnect={refreshRuntime}
          lastCheckedAt={lastCheckedAt}
          onSubmit={submitProject}
          onUpdateForm={updateForm}
          onResetForm={() => { setForm(emptyProject); setMutationState('idle'); setMutationMessage('') }}
        /> : route === 'operations' ? <TaskBoard bridge={wailsRuntimeBridge} /> : route === 'terminal' ? <TerminalPanel bridge={wailsRuntimeBridge} /> : route === 'office' ? (
          <div className="office-section reveal-1">
            <header className="section-heading"><div className="tool-heading"><span className="eyebrow">CORTEXOS / OFFICE</span><h1>Virtual office</h1></div></header>
            <Suspense fallback={<div className="empty-card" role="status">Loading virtual office…</div>}><OfficeView /></Suspense>
          </div>
        ) : route === 'knowledge' ? <KnowledgeView /> : route === 'staff' ? <StaffPanel /> : route === 'brain' ? <BrainPanel /> : <RoutePlaceholder route={route} />}
      </main>
    </div>
    <ApprovalDrawer isOpen={approvalDrawerOpen} onClose={() => setApprovalDrawerOpen(false)} />
  </div>
}

export function WorkspaceView({
  runtime,
  workspace,
  form,
  mutationState,
  mutationMessage,
  selectedProjectId,
  onSelectProject,
  onRefresh,
  onReconnect,
  lastCheckedAt,
  onSubmit,
  onUpdateForm,
  onResetForm,
}: {
  runtime: RuntimeState
  workspace: WorkspaceState
  form: ProjectFormState
  mutationState: 'idle' | 'saving' | 'success' | 'error'
  mutationMessage: string
  selectedProjectId: string
  onSelectProject: (projectId: string) => void
  onRefresh: () => void
  onReconnect: () => Promise<void>
  lastCheckedAt: number | null
  onSubmit: (event: FormEvent<HTMLFormElement>) => void
  onUpdateForm: (key: keyof ProjectFormState, value: string) => void
  onResetForm: () => void
}) {
  const projects = workspace.status === 'success' ? workspace.snapshot.projects : []
  const selectedProject = projects.find((project) => project.id === selectedProjectId) ?? projects[0]
  const [searchQuery, setSearchQuery] = useState('')
  const [searchState, setSearchState] = useState<{ status: 'idle' | 'loading' | 'success' | 'error'; results: WorkspaceNoteResult[]; message?: string }>({ status: 'idle', results: [] })
  const [rebuildingProjectId, setRebuildingProjectId] = useState('')
  const [actionError, setActionError] = useState('')
  const [isReconnecting, setIsReconnecting] = useState(false)
  const isOffline = runtime.status === 'error'
  const canRegister = runtime.status === 'success'
  const hasMetrics = canRegister && workspace.status === 'success'
  const lastChecked = lastCheckedAt === null ? 'Checking…' : new Date(lastCheckedAt).toLocaleTimeString()

  const reconnect = async () => {
    setIsReconnecting(true)
    try {
      await onReconnect()
    } finally {
      setIsReconnecting(false)
    }
  }

  const runRebuild = async (projectId: string) => {
    setActionError('')
    setRebuildingProjectId(projectId)
    try {
      await wailsRuntimeBridge.RebuildWorkspaceRetrieval({ schemaVersion: workspaceSchemaVersion, projectId })
      onRefresh()
    } catch (error) {
      setActionError(error instanceof Error ? error.message : 'Retrieval rebuild failed.')
    } finally {
      setRebuildingProjectId('')
    }
  }

  const runSearch = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault()
    if (!selectedProject || !searchQuery.trim()) return
    setSearchState({ status: 'loading', results: [] })
    try {
      const results = await queryWorkspace(wailsRuntimeBridge, selectedProject.id, searchQuery.trim())
      setSearchState({ status: 'success', results })
    } catch (error) {
      setSearchState({ status: 'error', results: [], message: error instanceof Error ? error.message : 'Workspace search failed.' })
    }
  }

  const runtimeTone = runtime.status === 'success' ? 'ok' : runtime.status === 'error' ? 'err' : 'neutral'
  const watcherTone = workspace.status === 'success' ? statusTone(workspace.snapshot.watcherState) : 'neutral'
  const retrievalTone = workspace.status === 'success' ? statusTone(workspace.snapshot.retrievalState) : 'neutral'
  const runtimeValue = 'connected'

  return <div className="workspace-container">
    <header className="workspace-header">
      <div className="workspace-header-main">
        <h1>Workspace</h1>
        <div className="workspace-context-row"><span>Active project</span>
          {selectedProject ? <><span className="badge-mono">{selectedProject.id}</span><span className="badge-mono">{selectedProject.defaultBranch || '—'}</span></> : <span className="badge-mono">—</span>}
        </div>
      </div>
      <div className="workspace-header-actions">
        <button className="btn-secondary" type="button" onClick={() => selectedProject && void runRebuild(selectedProject.id)} disabled={!selectedProject || rebuildingProjectId !== '' || !canRegister}>
          <RefreshCw size={14} className={rebuildingProjectId === selectedProject?.id ? 'spinning' : ''} aria-hidden="true" />Rebuild retrieval
        </button>
        <button className="btn-secondary" type="button" disabled={!selectedProject} aria-label="Open project folder (not supported by bridge)" title="Opening a local folder is not available in the runtime bridge">
          <FolderOpen size={14} aria-hidden="true" />Open folder
        </button>
      </div>
    </header>

    {isOffline && <section className="runtime-offline-banner" role="alert" aria-labelledby="runtime-offline-title">
      <div className="offline-banner-body">
        <div className="offline-banner-header"><span className="offline-badge">Bridge offline</span><strong className="offline-banner-title" id="runtime-offline-title">Runtime bridge is disconnected</strong></div>
        <p className="offline-banner-desc"><span>CortexOS runtime bridge is unavailable. Start the backend with <code>wails dev</code> or verify the local IPC port.</span><code className="offline-technical-reason">{runtime.message}</code></p>
        <div className="offline-banner-meta"><span>Last checked: {lastChecked}</span><span className="meta-sep" aria-hidden="true">·</span><code>wailsRuntimeBridge</code></div>
      </div>
      <div className="offline-banner-action"><button className="btn-secondary reconnect-btn" type="button" onClick={() => void reconnect()} disabled={isReconnecting}><RefreshCw size={13} className={isReconnecting ? 'spinning' : ''} aria-hidden="true" />{isReconnecting ? 'Connecting…' : 'Reconnect'}</button></div>
    </section>}

    {hasMetrics ? <section className="status-strip-panel" aria-label="Workspace metrics">
      <StatusCell label="Runtime" value={runtimeValue} tone={runtimeTone} />
      <StatusCell label="Watcher" value={workspace.snapshot.watcherState} tone={watcherTone} />
      <StatusCell label="Retrieval" value={workspace.snapshot.retrievalState} tone={retrievalTone} />
      <div className="status-strip-cell tone-neutral"><span className="cell-label">Vault notes</span><strong className="cell-value cell-value-number">{workspace.snapshot.vaultNoteCount.toLocaleString('en-US')}</strong></div>
    </section> : <div className="status-compact-row" role="status" aria-label="Workspace status">
      <span className="status-compact-dot" aria-hidden="true" />
      <span className="status-compact-text">{isOffline ? 'Runtime offline — telemetry, watcher, and note indexes will synchronize once connected.' : runtime.status === 'loading' ? 'Connecting to runtime — workspace telemetry will synchronize shortly.' : 'Workspace telemetry is loading from the runtime bridge.'}</span>
    </div>}

    {canRegister && (workspace.status === 'error' || actionError) && <div className="workspace-error" role="alert"><code>{workspace.status === 'error' ? `${workspace.code}: ${workspace.message}` : actionError}</code>{workspace.status === 'error' && <button className="btn-secondary" type="button" onClick={onRefresh}>Retry</button>}</div>}

    <div className="workspace-two-col">
      <div className="workspace-left-col">
        <section className="console-panel projects-panel" aria-labelledby="projects-title">
          <header className="panel-header"><h2 id="projects-title">Projects{workspace.status === 'success' ? ` (${projects.length})` : ''}</h2><button className="icon-btn" type="button" onClick={onRefresh} aria-label="Refresh projects" title="Refresh projects"><Activity size={15} aria-hidden="true" /></button></header>
          {isOffline ? <div className="empty-state-card" role="status">Connect the runtime bridge to view registered projects.</div> : runtime.status === 'loading' ? <StateBlock kind="loading" label="Connecting to runtime bridge" /> : workspace.status === 'loading' ? <StateBlock kind="loading" label="Loading projects" /> : workspace.status === 'error' ? <StateBlock kind="error" label="Projects unavailable" /> : projects.length === 0 ? <StateBlock kind="empty" label="No projects registered. Add a local repository to begin." /> : (
            <div className="table-container"><table className="projects-table"><colgroup><col /><col style={{ width: '110px' }} /><col style={{ width: '110px' }} /><col style={{ width: '128px' }} /></colgroup><thead><tr><th scope="col">Project</th><th scope="col">Status</th><th scope="col">Branch</th><th scope="col"><span className="sr-only">Actions</span></th></tr></thead>
              <tbody>{projects.map((project) => <ProjectRow key={project.id} project={project} selected={selectedProject?.id === project.id} rebuilding={rebuildingProjectId === project.id} onSelect={() => onSelectProject(project.id)} onRebuild={() => void runRebuild(project.id)} />)}</tbody>
            </table></div>
          )}
        </section>

        <section className="console-panel knowledge-panel" aria-labelledby="knowledge-search-title">
          <header className="panel-header"><h2 id="knowledge-search-title">Knowledge search · {selectedProject?.id ?? '—'}</h2></header>
          <form className="search-form-row" onSubmit={runSearch}>
            <input className="search-input" aria-label="Search workspace knowledge" value={searchQuery} onChange={(event) => setSearchQuery(event.target.value)} placeholder={selectedProject ? 'Search project notes' : 'Select a project first'} disabled={!selectedProject || searchState.status === 'loading'} />
            <button className="btn-primary" type="submit" disabled={!selectedProject || !searchQuery.trim() || searchState.status === 'loading'}><Search size={14} aria-hidden="true" />Search</button>
          </form>
          {!selectedProject ? <div className="empty-state-card">Select a project to search its indexed notes.</div> : searchState.status === 'idle' ? <div className="empty-state-card">Search notes in the selected project.</div> : searchState.status === 'loading' ? <StateBlock kind="loading" label="Searching workspace knowledge" /> : searchState.status === 'error' ? <div className="empty-state-card" role="alert">{searchState.message}</div> : <>
            <div className="search-results-meta">{searchState.results.length} results</div>
            {searchState.results.length === 0 ? <div className="empty-state-card">No matching notes found.</div> : <div className="search-results-list">{searchState.results.map((result) => <SearchResult key={result.id} result={result} />)}</div>}
          </>}
        </section>
      </div>

      <div className="workspace-right-col">
        <section className="console-panel register-panel" id="register-project" aria-labelledby="register-project-title">
          <header className="panel-header">
            <h2 id="register-project-title">Register project</h2>
            {!canRegister && <span className="panel-badge-offline">Offline</span>}
          </header>
          <form className={`register-form${!canRegister ? ' form-disabled' : ''}`} onSubmit={onSubmit}>
            {!canRegister && <div className="register-bridge-notice" role="note">Registration requires an active runtime bridge.</div>}
            <fieldset disabled={!canRegister} className="register-fieldset">
              <FormField label="Project ID" required value={form.id} placeholder="project-id" onChange={(value) => onUpdateForm('id', value)} disabled={!canRegister} />
              <FormField label="Display Name" required value={form.name} placeholder="Project name" onChange={(value) => onUpdateForm('name', value)} disabled={!canRegister} />
              <FormField label="Repository Root" required value={form.repositoryRoot} placeholder="/path/to/repo" onChange={(value) => onUpdateForm('repositoryRoot', value)} disabled={!canRegister} />
              <FormField label="Vault Root" required value={form.vaultRoot} placeholder="/path/to/vault" onChange={(value) => onUpdateForm('vaultRoot', value)} disabled={!canRegister} />
              <FormField label="Default Branch" value={form.defaultBranch} placeholder="main" onChange={(value) => onUpdateForm('defaultBranch', value)} disabled={!canRegister} />
              <div className="form-actions-row">
                <button className="btn-secondary" type="button" onClick={onResetForm} disabled={!canRegister}>Reset</button>
                <button className="btn-primary" type="submit" disabled={!canRegister || mutationState === 'saving'}><span aria-hidden="true">+</span>{mutationState === 'saving' ? 'Registering…' : 'Register'}</button>
              </div>
            </fieldset>
            {mutationMessage && <div className={`form-alert ${mutationState === 'error' ? 'error' : 'success'}`} role={mutationState === 'error' ? 'alert' : 'status'}>{mutationMessage}</div>}
          </form>
        </section>
      </div>
    </div>
  </div>
}

function FormField({ label, required = false, value, placeholder, onChange, disabled = false }: { label: string; required?: boolean; value: string; placeholder: string; onChange: (value: string) => void; disabled?: boolean }) {
  const id = `register-${label.toLowerCase().replaceAll(' ', '-')}`
  return <div className="form-group"><label htmlFor={id}>{label}{required ? ' *' : ''}</label><input id={id} required={required} value={value} onChange={(event) => onChange(event.target.value)} placeholder={placeholder} disabled={disabled} /></div>
}

function StatusCell({ label, value, tone }: { label: string; value: string; tone: 'ok' | 'warn' | 'err' | 'neutral' }) {
  return <div className={`status-strip-cell tone-${tone}`}><span className="cell-label">{label}</span><strong className="cell-value">{tone !== 'neutral' && <i className={`cell-dot dot-${tone}`} aria-hidden="true" />}{value}</strong></div>
}

function StateBlock({ kind, label }: { kind: 'loading' | 'empty' | 'error'; label: string }) {
  return kind === 'loading' ? <div className="state-skeleton" role="status" aria-label={label}><span /><span /><span /></div> : <div className={`empty-state-card state-${kind}`} role={kind === 'error' ? 'alert' : undefined}>{label}</div>
}

function ProjectRow({ project, selected, rebuilding, onSelect, onRebuild }: { project: WorkspaceProject; selected: boolean; rebuilding: boolean; onSelect: () => void; onRebuild: () => void }) {
  const tone = statusTone(project.status)
  return <tr className={selected ? 'row-selected' : ''} aria-selected={selected}>
    <td><div className="project-cell-name"><strong>{project.name}</strong><span>{project.id}</span></div></td>
    <td><span className={`project-status-pill status-${tone}`}><i className="pill-dot" aria-hidden="true" />{project.status}</span></td>
    <td><code className="branch-value">{project.defaultBranch || '—'}</code></td>
    <td><div className="actions-cell">
      <button className="icon-btn" type="button" onClick={onSelect} aria-label={`Select project ${project.name}`} title={`Select project ${project.name}`}><Crosshair size={15} aria-hidden="true" /></button>
      <button className="icon-btn" type="button" onClick={onRebuild} disabled={rebuilding} aria-label={`Rebuild retrieval for ${project.name}`} title={`Rebuild retrieval for ${project.name}`}><RefreshCw className={rebuilding ? 'spinning' : ''} size={15} aria-hidden="true" /></button>
      <button className="icon-btn" type="button" disabled aria-label={`Open location for ${project.name} (not supported by bridge)`} title="Opening a local folder is not available in the runtime bridge"><FolderOpen size={15} aria-hidden="true" /></button>
    </div></td>
  </tr>
}

function SearchResult({ result }: { result: WorkspaceNoteResult }) {
  return <article className="result-card"><code className="result-path">{result.relativePath}</code><p>{result.excerpt}</p></article>
}

function statusTone(status: string): 'ok' | 'warn' | 'err' | 'neutral' {
  const normalized = status.trim().toLowerCase()
  if (['ready', 'healthy', 'connected', 'running', 'success', 'complete', 'available'].includes(normalized)) return 'ok'
  if (['stale', 'idle', 'pending', 'waiting', 'degraded', 'warning'].includes(normalized)) return 'warn'
  if (['error', 'failed', 'unavailable', 'disconnected'].includes(normalized)) return 'err'
  return 'neutral'
}

export function RoutePlaceholder({ route }: { route: Exclude<ShellRoute, 'workspace' | 'office' | 'knowledge' | 'terminal'> }) {
  const title = route === 'settings' ? 'Settings' : route === 'operations' ? 'Operations' : 'Activity'
  const eyebrow = route === 'settings' ? 'CORTEXOS / PREFERENCES' : `CORTEXOS / ${title.toUpperCase()}`
  return <section className="route-placeholder reveal-1" aria-labelledby="placeholder-title"><header className="tool-heading"><span className="eyebrow">{eyebrow}</span><h1 id="placeholder-title">{title}</h1></header><div className="coming-soon"><Sparkles size={16} aria-hidden="true" /><div><strong>Coming soon</strong><p>Only the Workspace contract is active in this release.</p></div></div></section>
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
  return ['workspace', 'operations', 'terminal', 'activity', 'office', 'knowledge', 'staff', 'brain', 'settings'].includes(route) ? route : 'workspace'
}

function getThemePreference(): Theme {
  if (typeof window === 'undefined') return 'dark'
  const stored = window.localStorage.getItem('cortexos-theme')
  return stored === 'light' ? 'light' : 'dark'
}
