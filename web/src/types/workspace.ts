export const workspaceSchemaVersion = 'cortexos.workspace.v1' as const

export type WorkspaceSnapshotRequest = {
  schemaVersion: typeof workspaceSchemaVersion
}

export type WorkspaceProject = {
  id: string
  name: string
  defaultBranch?: string
  status: string
  createdAt: string
  updatedAt: string
  schemaVersion: string
}

export type WorkspaceWorktree = {
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

export type WorkspaceSnapshot = {
  schemaVersion: typeof workspaceSchemaVersion
  projects: WorkspaceProject[]
  worktrees: WorkspaceWorktree[]
  vaultNoteCount: number
  watcherState: string
  watcherErrorCode?: string
  retrievalVersion: string
  retrievalState: string
}

export type WorkspaceNoteQueryRequest = {
  schemaVersion: typeof workspaceSchemaVersion
  projectId: string
  query: string
  limit: number
}

export type WorkspaceNoteResult = {
  id: string
  noteId: string
  projectId: string
  relativePath: string
  sourceHash: string
  attribution: string
  indexedAt: string
  excerpt: string
}

export type WorkspaceProjectRequest = {
  schemaVersion: typeof workspaceSchemaVersion
  id: string
  name: string
  repositoryRoot: string
  vaultRoot: string
  defaultBranch?: string
}

export type WorkspaceMutationRequest = {
  schemaVersion: typeof workspaceSchemaVersion
  projectId: string
}

export type WorkspaceBridgeError = {
  code: string
  message: string
}

export type WorkspaceBridge = {
  GetWorkspaceSnapshot(request: WorkspaceSnapshotRequest): Promise<WorkspaceSnapshot>
  QueryWorkspace(request: WorkspaceNoteQueryRequest): Promise<WorkspaceNoteResult[]>
  RebuildWorkspaceRetrieval(request: WorkspaceMutationRequest): Promise<void>
  RegisterWorkspaceProject(request: WorkspaceProjectRequest): Promise<WorkspaceProject>
}

export function isWorkspaceSnapshot(value: unknown): value is WorkspaceSnapshot {
  if (!value || typeof value !== 'object') return false
  const snapshot = value as Partial<WorkspaceSnapshot>
  return (
    snapshot.schemaVersion === workspaceSchemaVersion &&
    Array.isArray(snapshot.projects) &&
    snapshot.projects.every(isWorkspaceProject) &&
    Array.isArray(snapshot.worktrees) &&
    snapshot.worktrees.every(isWorkspaceWorktree) &&
    typeof snapshot.vaultNoteCount === 'number' &&
    Number.isInteger(snapshot.vaultNoteCount) &&
    snapshot.vaultNoteCount >= 0 &&
    typeof snapshot.watcherState === 'string' &&
    typeof snapshot.retrievalVersion === 'string' &&
    typeof snapshot.retrievalState === 'string'
  )
}

function isWorkspaceProject(value: unknown): value is WorkspaceProject {
  if (!value || typeof value !== 'object') return false
  const project = value as Partial<WorkspaceProject>
  return typeof project.id === 'string' && typeof project.name === 'string' && typeof project.status === 'string'
}

function isWorkspaceWorktree(value: unknown): value is WorkspaceWorktree {
  if (!value || typeof value !== 'object') return false
  const worktree = value as Partial<WorkspaceWorktree>
  return typeof worktree.id === 'string' && typeof worktree.projectId === 'string' && typeof worktree.status === 'string' && typeof worktree.dirty === 'boolean'
}
