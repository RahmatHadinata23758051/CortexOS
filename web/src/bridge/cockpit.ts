import {
  type CockpitBridge,
  type CockpitQueryResult,
  type CockpitRuntimeSnapshot,
  type CockpitWorkspaceSnapshot,
  type CockpitWorkspaceRequest,
  type CockpitMutationRequest,
  type CockpitProject,
  type CockpitProjectRequest,
  cockpitSchemaVersion,
  isCockpitRuntimeSnapshot,
  isCockpitWorkspaceSnapshot,
} from '../types'

export type CockpitRuntimeState =
  | { status: 'loading' }
  | { status: 'success'; snapshot: CockpitRuntimeSnapshot }
  | { status: 'error'; code: string; message: string }

export type CockpitWorkspaceState =
  | { status: 'loading' }
  | { status: 'success'; snapshot: CockpitWorkspaceSnapshot }
  | { status: 'error'; code: string; message: string }

export async function loadCockpitRuntime(
  bridge: CockpitBridge,
): Promise<CockpitRuntimeState> {
  try {
    const snapshot = await bridge.GetCockpitRuntime()
    if (!isCockpitRuntimeSnapshot(snapshot)) {
      throw new Error('CortexOS returned an invalid cockpit runtime snapshot')
    }
    return { status: 'success', snapshot }
  } catch (error) {
    return normalizeCockpitError(error)
  }
}

export async function loadCockpitWorkspace(
  bridge: CockpitBridge,
  request?: CockpitWorkspaceRequest,
): Promise<CockpitWorkspaceState> {
  try {
    const req: CockpitWorkspaceRequest = request ?? { schemaVersion: cockpitSchemaVersion }
    const snapshot = await bridge.GetCockpitWorkspace(req)
    if (!isCockpitWorkspaceSnapshot(snapshot)) {
      throw new Error('CortexOS returned an invalid cockpit workspace snapshot')
    }
    return { status: 'success', snapshot }
  } catch (error) {
    return normalizeCockpitError(error)
  }
}

export async function registerCockpitProject(
  bridge: CockpitBridge,
  request: Omit<CockpitProjectRequest, 'schemaVersion'>,
): Promise<CockpitProject> {
  return bridge.RegisterCockpitProject({
    ...request,
    schemaVersion: cockpitSchemaVersion,
  })
}

export async function queryCockpitWorkspace(
  bridge: CockpitBridge,
  projectId: string,
  query: string,
  limit = 10,
): Promise<CockpitQueryResult[]> {
  return bridge.QueryCockpitWorkspace({
    schemaVersion: cockpitSchemaVersion,
    projectId,
    query,
    limit,
  })
}

export async function rebuildCockpitRetrieval(
  bridge: CockpitBridge,
  projectId: string,
): Promise<void> {
  const request: CockpitMutationRequest = {
    schemaVersion: cockpitSchemaVersion,
    projectId,
  }
  return bridge.RebuildCockpitRetrieval(request)
}

function normalizeCockpitError(error: unknown): { status: 'error'; code: string; message: string } {
  if (error && typeof error === 'object') {
    const candidate = error as Partial<{ code: string; message: string }>
    if (typeof candidate.code === 'string' && typeof candidate.message === 'string') {
      return { status: 'error', code: candidate.code, message: candidate.message }
    }
  }
  return {
    status: 'error',
    code: 'cockpit.internal',
    message: error instanceof Error ? error.message : 'Cockpit bridge failed',
  }
}

export * from './events'
