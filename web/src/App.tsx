import { lazy, Suspense, useEffect, useState, type FormEvent, type ReactNode } from 'react'
import { ChevronsLeft, ChevronsRight, Files, Menu, Moon, Network, Settings, Sun, X } from 'lucide-react'

import { loadRuntimeSnapshot, type RuntimeState } from './bridge/runtime'
import { loadWorkspaceSnapshot, type WorkspaceState } from './bridge/workspace'
import { wailsRuntimeBridge, workspaceSchemaVersion } from './types'
import { ProjectPicker, type ProjectFormState } from './picker/ProjectPicker'
import { FilesView } from './files/FilesView'
import { ApprovalDrawer } from './office/ApprovalDrawer'
import { SettingsView } from './settings/SettingsView'

const OfficeView = lazy(() => import('./office/OfficeView').then(({ OfficeView }) => ({ default: OfficeView })))

export type ShellRoute = 'orchestra' | 'files' | 'settings'
type LegacyRoute = 'workspace' | 'operations' | 'terminal' | 'activity' | 'office' | 'knowledge' | 'staff' | 'brain'
type RouteInput = ShellRoute | LegacyRoute
export type Theme = 'light' | 'dark'

export interface AppProps {
  initialRoute?: RouteInput
  initialTheme?: Theme
  initialProjectId?: string
  initialWorkspace?: WorkspaceState
}

const PROJECT_STORAGE_KEY = 'cortexos-project-id'
const initialRuntime: RuntimeState = { status: 'loading' }
const initialWorkspaceState: WorkspaceState = { status: 'loading' }
const emptyProject: ProjectFormState = {
  id: '',
  name: '',
  repositoryRoot: '',
  vaultRoot: '',
  defaultBranch: 'main',
}

const navigation: Array<{ route: ShellRoute; label: string; icon: ReactNode; description: string }> = [
  { route: 'orchestra', label: 'Orchestra', icon: <Network size={18} strokeWidth={1.8} />, description: 'Active work graph' },
  { route: 'files', label: 'Files', icon: <Files size={18} strokeWidth={1.8} />, description: 'Project context' },
  { route: 'settings', label: 'Settings', icon: <Settings size={18} strokeWidth={1.8} />, description: 'Runtime preferences' },
]

export function App({ initialRoute, initialTheme, initialProjectId, initialWorkspace }: AppProps = {}) {
  const [runtime, setRuntime] = useState<RuntimeState>(initialRuntime)
  const [workspace, setWorkspace] = useState<WorkspaceState>(initialWorkspace ?? initialWorkspaceState)
  const [form, setForm] = useState<ProjectFormState>(emptyProject)
  const [mutationState, setMutationState] = useState<'idle' | 'saving' | 'success' | 'error'>('idle')
  const [mutationMessage, setMutationMessage] = useState('')
  const [selectedProjectId, setSelectedProjectId] = useState(() => initialProjectId ?? getStoredProjectId())
  const [route, setRoute] = useState<ShellRoute>(() => normalizeRoute(initialRoute ?? getRouteFromLocation()))
  const [theme, setTheme] = useState<Theme>(() => {
    const nextTheme = initialTheme ?? getThemePreference()
    if (typeof document !== 'undefined') document.documentElement.dataset.theme = nextTheme
    return nextTheme
  })
  const [sidebarExpanded, setSidebarExpanded] = useState(true)
  const [mobileMenuOpen, setMobileMenuOpen] = useState(false)
  const [approvalDrawerOpen, setApprovalDrawerOpen] = useState(false)
  const [pendingApprovalCount, setPendingApprovalCount] = useState(0)

  const refreshRuntime = async () => {
    setRuntime({ status: 'loading' })
    const [nextRuntime, nextWorkspace] = await Promise.all([
      loadRuntimeSnapshot(wailsRuntimeBridge),
      loadWorkspaceSnapshot(wailsRuntimeBridge),
    ])
    setRuntime(nextRuntime)
    setWorkspace(nextWorkspace)
  }

  const refreshWorkspace = async () => {
    setWorkspace({ status: 'loading' })
    const nextWorkspace = await loadWorkspaceSnapshot(wailsRuntimeBridge)
    setWorkspace(nextWorkspace)
    return nextWorkspace
  }

  const updateForm = (key: keyof ProjectFormState, value: string) => {
    setForm((current) => ({ ...current, [key]: value }))
    setMutationState('idle')
    setMutationMessage('')
  }

  const resetForm = () => {
    setForm(emptyProject)
    setMutationState('idle')
    setMutationMessage('')
  }

  const selectProject = (projectId: string) => {
    setSelectedProjectId(projectId)
    persistProjectId(projectId)
    navigateTo('orchestra')
  }

  const switchProject = () => {
    setSelectedProjectId('')
    persistProjectId('')
    setMobileMenuOpen(false)
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
      const nextWorkspace = await refreshWorkspace()
      const projectExists = nextWorkspace.status === 'success' && nextWorkspace.snapshot.projects.some((candidate) => candidate.id === project.id)
      if (projectExists) {
        selectProject(project.id)
        resetForm()
      } else {
        setMutationState('error')
        setMutationMessage('Project was registered, but the refreshed workspace snapshot does not include it yet.')
      }
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
    const handleHashChange = () => setRoute(normalizeRoute(getRouteFromLocation()))
    window.addEventListener('hashchange', handleHashChange)
    return () => window.removeEventListener('hashchange', handleHashChange)
  }, [])

  useEffect(() => {
    if (workspace.status !== 'success' || !selectedProjectId) return
    const projectExists = workspace.snapshot.projects.some((project) => project.id === selectedProjectId)
    if (!projectExists) {
      persistProjectId('')
    }
  }, [selectedProjectId, workspace])

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

  useEffect(() => {
    const handleKeyDown = (event: KeyboardEvent) => {
      if (event.key === 'Escape') setMobileMenuOpen(false)
      if ((event.ctrlKey || event.metaKey) && event.key === '\\') {
        event.preventDefault()
        setSidebarExpanded((value) => !value)
      }
    }
    window.addEventListener('keydown', handleKeyDown)
    return () => window.removeEventListener('keydown', handleKeyDown)
  }, [])

  const projects = workspace.status === 'success' ? workspace.snapshot.projects : []
  const activeProject = projects.find((project) => project.id === selectedProjectId)
  const isPickerVisible = !activeProject
  const hasStaleStoredProject = workspace.status === 'success' && Boolean(selectedProjectId) && !activeProject
  const activeNavigation = navigation.find((item) => item.route === route) ?? navigation[0]
  const runtimeLabel = runtime.status === 'success' ? 'connected' : runtime.status === 'loading' ? 'connecting' : 'unavailable'

  if (isPickerVisible) {
    return <>
      <a className="skip-link" href="#project-picker-main">Skip to main content</a>
      <main id="project-picker-main" data-theme={theme} data-stale-project={hasStaleStoredProject ? 'true' : undefined}>
        <ProjectPicker
          runtime={runtime}
          workspace={workspace}
          form={form}
          mutationState={mutationState}
          mutationMessage={mutationMessage}
          onSelectProject={selectProject}
          onRefresh={() => void refreshWorkspace()}
          onReconnect={refreshRuntime}
          onSubmit={submitProject}
          onUpdateForm={updateForm}
          onResetForm={resetForm}
        />
      </main>
    </>
  }

  return <div className={`shell workspace-shell${sidebarExpanded ? ' sidebar-expanded' : ' sidebar-collapsed'}${mobileMenuOpen ? ' mobile-menu-open' : ''}`} data-theme={theme}>
    <a className="skip-link" href="#main-content">Skip to main content</a>
    <div className="shell-grid">
      <button type="button" className="mobile-sidebar-backdrop" aria-label="Close navigation" onClick={() => setMobileMenuOpen(false)} />
      <Sidebar
        activeRoute={route}
        expanded={sidebarExpanded}
        mobileOpen={mobileMenuOpen}
        projectName={activeProject.name}
        runtime={runtime}
        onNavigate={(nextRoute) => { navigateTo(nextRoute); setMobileMenuOpen(false) }}
        onSwitchProject={switchProject}
        onToggle={() => setSidebarExpanded((value) => !value)}
        onClose={() => setMobileMenuOpen(false)}
      />

      <header className="topbar">
        <div className="topbar-left">
          <button type="button" className="mobile-menu-toggle" onClick={() => setMobileMenuOpen(true)} aria-label="Open navigation">
            <Menu size={19} aria-hidden="true" />
          </button>
          <div className="topbar-breadcrumb" aria-label="Breadcrumb">
            <span className="breadcrumb-segment">{activeProject.name}</span>
            <span className="breadcrumb-separator" aria-hidden="true">›</span>
            <strong className="breadcrumb-current">{activeNavigation.label}</strong>
          </div>
        </div>
        <div className="topbar-actions">
          <div className={`topbar-status runtime-${runtime.status}`} role="status" aria-label={`Runtime ${runtimeLabel}`}>
            <span className="status-dot" aria-hidden="true" /><span>Runtime {runtimeLabel}</span>
          </div>
          <button type="button" className={`approval-badge-btn${approvalDrawerOpen ? ' active' : ''}${pendingApprovalCount > 0 ? ' has-pending' : ''}`} onClick={() => setApprovalDrawerOpen((value) => !value)} aria-label={`Tool approval queue: ${pendingApprovalCount} pending`} aria-expanded={approvalDrawerOpen}>
            <span className="approval-label">Approvals</span>
            {pendingApprovalCount > 0 && <span className="approval-count" aria-live="polite">{pendingApprovalCount}</span>}
          </button>
          <button className="theme-toggle" type="button" onClick={() => setTheme((current) => current === 'light' ? 'dark' : 'light')} aria-label={`Switch to ${theme === 'light' ? 'dark' : 'light'} theme`}>
            <span aria-hidden="true">{theme === 'light' ? <Moon size={16} /> : <Sun size={16} />}</span><span className="theme-toggle-label">{theme === 'light' ? 'Dark' : 'Light'}</span>
          </button>
        </div>
      </header>

      <main id="main-content" className="content" aria-live="polite">
        {route === 'orchestra' && <section className="orchestra-section reveal-1" aria-labelledby="orchestra-title">
          <header className="workspace-context-header">
            <div className="tool-heading"><span className="eyebrow">CORTEXOS / ORCHESTRA</span><h1 id="orchestra-title">Orchestra</h1><p>Active project work graph and execution context.</p></div>
            <span className="project-context-badge">{activeProject.id} · {activeProject.defaultBranch || 'branch unavailable'}</span>
          </header>
          <Suspense fallback={<div className="empty-card" role="status">Loading Orchestra workspace…</div>}><OfficeView projectId={activeProject.id} /></Suspense>
        </section>}
        {route === 'files' && <FilesView workspace={workspace} selectedProjectId={activeProject.id} onRefresh={() => void refreshWorkspace()} />}
        {route === 'settings' && <SettingsView theme={theme} onThemeChange={setTheme} runtime={runtime} workspace={workspace} />}
      </main>
    </div>
    <ApprovalDrawer isOpen={approvalDrawerOpen} onClose={() => setApprovalDrawerOpen(false)} />
  </div>
}

function Sidebar({ activeRoute, expanded, mobileOpen, projectName, runtime, onNavigate, onSwitchProject, onToggle, onClose }: {
  activeRoute: ShellRoute
  expanded: boolean
  mobileOpen: boolean
  projectName: string
  runtime: RuntimeState
  onNavigate: (route: ShellRoute) => void
  onSwitchProject: () => void
  onToggle: () => void
  onClose: () => void
}): ReactNode {
  return <aside className={`app-sidebar${mobileOpen ? ' is-open' : ''}`} aria-label="Primary navigation">
    <div className="sidebar-topline">
      <button type="button" className="sidebar-brand" onClick={onSwitchProject} aria-label="Switch project">
        <span className="switcher-logo" aria-hidden="true">C</span>
        <span className="switcher-info"><strong className="switcher-title">{projectName}</strong><small className="switcher-subtitle">Project workspace</small></span>
      </button>
      <button type="button" className="mobile-sidebar-close" onClick={onClose} aria-label="Close navigation"><X size={18} aria-hidden="true" /></button>
    </div>
    <div className="sidebar-scroll">
      <nav className="workspace-nav" aria-label="Workspace navigation">
        {navigation.map((item) => <button key={item.route} type="button" className={`workspace-nav-item${activeRoute === item.route ? ' active' : ''}`} onClick={() => onNavigate(item.route)} aria-current={activeRoute === item.route ? 'page' : undefined} title={expanded ? undefined : item.label}>
          <span className="workspace-nav-icon" aria-hidden="true">{item.icon}</span><span className="workspace-nav-copy"><strong>{item.label}</strong><small>{item.description}</small></span>
        </button>)}
      </nav>
    </div>
    <div className="sidebar-bottom">
      <div className="sidebar-runtime" role="status"><span className={`runtime-indicator runtime-${runtime.status}`} aria-hidden="true" /><span className="sidebar-footer-text">{runtime.status === 'success' ? 'Bridge connected' : runtime.status === 'loading' ? 'Connecting to bridge' : 'Bridge unavailable'}</span></div>
      <button type="button" className="sidebar-collapse-toggle" onClick={onToggle} aria-label={expanded ? 'Collapse sidebar' : 'Expand sidebar'} aria-pressed={!expanded}>
        {expanded ? <ChevronsLeft size={17} aria-hidden="true" /> : <ChevronsRight size={17} aria-hidden="true" />}<span>{expanded ? 'Collapse sidebar' : 'Expand sidebar'}</span>
      </button>
    </div>
  </aside>
}

function navigateTo(route: ShellRoute) {
  if (typeof window !== 'undefined') window.location.hash = route
}

function normalizeRoute(route: RouteInput): ShellRoute {
  if (route === 'files' || route === 'settings' || route === 'orchestra') return route
  if (route === 'knowledge') return 'files'
  return 'orchestra'
}

function getRouteFromLocation(): RouteInput {
  if (typeof window === 'undefined') return 'orchestra'
  const route = window.location.hash.slice(1) as RouteInput
  return ['orchestra', 'files', 'settings', 'workspace', 'operations', 'terminal', 'activity', 'office', 'knowledge', 'staff', 'brain'].includes(route) ? route : 'orchestra'
}

function getStoredProjectId(): string {
  if (typeof window === 'undefined') return ''
  return window.localStorage.getItem(PROJECT_STORAGE_KEY) ?? ''
}

function persistProjectId(projectId: string) {
  if (typeof window === 'undefined') return
  if (projectId) window.localStorage.setItem(PROJECT_STORAGE_KEY, projectId)
  else window.localStorage.removeItem(PROJECT_STORAGE_KEY)
}

function getThemePreference(): Theme {
  if (typeof window === 'undefined') return 'dark'
  return window.localStorage.getItem('cortexos-theme') === 'light' ? 'light' : 'dark'
}
