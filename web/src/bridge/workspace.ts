import {
  type WorkspaceBridge,
  type WorkspaceNoteResult,
  type WorkspaceSnapshot,
  type WorkspaceSnapshotRequest,
  type WorkspaceMutationRequest,
  type WorkspaceProject,
  type WorkspaceProjectRequest,
  workspaceSchemaVersion,
  isWorkspaceSnapshot,
} from '../types'

export type WorkspaceState =
  | { status: 'loading' }
  | { status: 'success'; snapshot: WorkspaceSnapshot }
  | { status: 'error'; code: string; message: string }

export async function loadWorkspaceSnapshot(
  bridge: WorkspaceBridge,
): Promise<WorkspaceState> {
  try {
    const request: WorkspaceSnapshotRequest = { schemaVersion: workspaceSchemaVersion }
    const snapshot = await bridge.GetWorkspaceSnapshot(request)
    if (!isWorkspaceSnapshot(snapshot)) {
      throw new Error('CortexOS returned an invalid workspace snapshot')
    }
    return { status: 'success', snapshot }
  } catch (error) {
    return normalizeWorkspaceError(error)
  }
}

export async function queryWorkspace(
  bridge: WorkspaceBridge,
  projectId: string,
  query: string,
  limit = 10,
): Promise<WorkspaceNoteResult[]> {
  return bridge.QueryWorkspace({
    schemaVersion: workspaceSchemaVersion,
    projectId,
    query,
    limit,
  })
}

export async function rebuildWorkspaceRetrieval(
  bridge: WorkspaceBridge,
  projectId: string,
): Promise<void> {
  const request: WorkspaceMutationRequest = {
    schemaVersion: workspaceSchemaVersion,
    projectId,
  }
  return bridge.RebuildWorkspaceRetrieval(request)
}

export async function registerWorkspaceProject(
  bridge: WorkspaceBridge,
  request: Omit<WorkspaceProjectRequest, 'schemaVersion'>,
): Promise<WorkspaceProject> {
  return bridge.RegisterWorkspaceProject({
    ...request,
    schemaVersion: workspaceSchemaVersion,
  })
}

function normalizeWorkspaceError(error: unknown): Extract<WorkspaceState, { status: 'error' }> {
  if (error && typeof error === 'object') {
    const candidate = error as Partial<{ code: string; message: string }>
    if (typeof candidate.code === 'string' && typeof candidate.message === 'string') {
      return { status: 'error', code: candidate.code, message: candidate.message }
    }
  }
  return {
    status: 'error',
    code: 'workspace.internal',
    message: error instanceof Error ? error.message : 'Workspace bridge failed',
  }
}
