export const cockpitSchemaVersion = 'cortexos.cockpit.bridge.v1' as const

export type CockpitRuntimeSnapshot = {
  schemaVersion: typeof cockpitSchemaVersion
  status: 'ready' | 'degraded' | 'unavailable'
  environment: 'local' | 'production'
  provider: 'disabled' | 'configured'
}

export type CockpitWorkspaceRequest = {
  schemaVersion: typeof cockpitSchemaVersion
  projectId?: string
}

export type CockpitProject = {
  id: string
  name: string
  defaultBranch?: string
  status: string
  createdAt: string
  updatedAt: string
  schemaVersion: string
}

export type CockpitWorktree = {
  id: string
  projectId: string
  branch?: string
  revision?: string
  status: string
  dirty: boolean
  activeReference?: string
  createdAt: string
  updatedAt: string
}

export type CockpitWorkspaceSnapshot = {
  schemaVersion: typeof cockpitSchemaVersion
  projects: CockpitProject[]
  worktrees: CockpitWorktree[]
  vaultNoteCount: number
  watcherState: string
  watcherErrorCode?: string
  retrievalVersion: string
  retrievalState: string
}

export type CockpitProjectRequest = {
  schemaVersion: typeof cockpitSchemaVersion
  id: string
  name: string
  repositoryRoot: string
  vaultRoot: string
  defaultBranch?: string
}

export type CockpitQueryRequest = {
  schemaVersion: typeof cockpitSchemaVersion
  projectId: string
  query: string
  limit?: number
}

export type CockpitQueryResult = {
  id: string
  noteId: string
  projectId: string
  relativePath: string
  sourceHash: string
  attribution: string
  indexedAt: string
  excerpt: string
  schemaVersion: string
}

export type CockpitMutationRequest = {
  schemaVersion: typeof cockpitSchemaVersion
  projectId: string
}

export type CockpitBridgeError = {
  code: string
  message: string
}

export type CockpitBridge = {
  GetCockpitRuntime(): Promise<CockpitRuntimeSnapshot>
  GetCockpitWorkspace(request: CockpitWorkspaceRequest): Promise<CockpitWorkspaceSnapshot>
  RegisterCockpitProject(request: CockpitProjectRequest): Promise<CockpitProject>
  QueryCockpitWorkspace(request: CockpitQueryRequest): Promise<CockpitQueryResult[]>
  RebuildCockpitRetrieval(request: CockpitMutationRequest): Promise<void>
}

export function isCockpitRuntimeSnapshot(value: unknown): value is CockpitRuntimeSnapshot {
  if (!value || typeof value !== 'object') return false
  const s = value as Partial<CockpitRuntimeSnapshot>
  return (
    s.schemaVersion === cockpitSchemaVersion &&
    ['ready', 'degraded', 'unavailable'].includes(s.status ?? '') &&
    ['local', 'production'].includes(s.environment ?? '') &&
    ['disabled', 'configured'].includes(s.provider ?? '')
  )
}

export function isCockpitWorkspaceSnapshot(value: unknown): value is CockpitWorkspaceSnapshot {
  if (!value || typeof value !== 'object') return false
  const s = value as Partial<CockpitWorkspaceSnapshot>
  return (
    s.schemaVersion === cockpitSchemaVersion &&
    Array.isArray(s.projects) &&
    s.projects.every(isCockpitProject) &&
    Array.isArray(s.worktrees) &&
    s.worktrees.every(isCockpitWorktree) &&
    typeof s.vaultNoteCount === 'number' &&
    Number.isInteger(s.vaultNoteCount) &&
    s.vaultNoteCount >= 0 &&
    typeof s.watcherState === 'string' &&
    typeof s.retrievalVersion === 'string' &&
    typeof s.retrievalState === 'string'
  )
}

function isCockpitProject(value: unknown): value is CockpitProject {
  if (!value || typeof value !== 'object') return false
  const p = value as Partial<CockpitProject>
  return typeof p.id === 'string' && typeof p.name === 'string' && typeof p.status === 'string'
}

function isCockpitWorktree(value: unknown): value is CockpitWorktree {
  if (!value || typeof value !== 'object') return false
  const w = value as Partial<CockpitWorktree>
  return typeof w.id === 'string' && typeof w.projectId === 'string' && typeof w.status === 'string' && typeof w.dirty === 'boolean'
}

// Bridge runtime event names.
export const CockpitEventNames = {
  RuntimeChanged: 'cortexos:cockpit:runtime:changed',
  WorkspaceChanged: 'cortexos:cockpit:workspace:changed',
  ProjectRegistered: 'cortexos:cockpit:project:registered',
  RetrievalRebuilt: 'cortexos:cockpit:retrieval:rebuilt',
} as const