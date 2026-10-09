export const runtimeSchemaVersion = 'cortexos.runtime.v1' as const

export type SnapshotRequest = {
  schemaVersion: typeof runtimeSchemaVersion
}

export type RuntimeSnapshot = {
  schemaVersion: typeof runtimeSchemaVersion
  status: 'ready'
  environment: 'local'
  provider: 'disabled'
}

export type RuntimeBridge = {
  GetRuntimeSnapshot(request: SnapshotRequest): Promise<RuntimeSnapshot>
}

export * from './workspace'
export * from './office'
export * from './cockpit'
export * from './orchestra'
export * from './knowledge'
import type { WorkspaceBridge } from './workspace'
import type { KnowledgeBridge } from './knowledge'
import type { OfficeBridge } from './office'
import type { CockpitBridge } from './cockpit'
import type { OrchestraBridge } from './orchestra'

export type FullBridge = RuntimeBridge & WorkspaceBridge & OfficeBridge & CockpitBridge & OrchestraBridge & KnowledgeBridge

export type WailsWindow = Window & {
  go?: {
    platform?: {
      Bridge?: FullBridge
    }
  }
}

function getWailsBridge(): FullBridge {
  const bridge = (window as WailsWindow).go?.platform?.Bridge
  if (!bridge) {
    throw new Error('CortexOS runtime bridge is unavailable')
  }
  return bridge
}

export const wailsRuntimeBridge: FullBridge = {
  GetRuntimeSnapshot: (request) => getWailsBridge().GetRuntimeSnapshot(request),
  GetWorkspaceSnapshot: (request) => getWailsBridge().GetWorkspaceSnapshot(request),
  QueryWorkspace: (request) => getWailsBridge().QueryWorkspace(request),
  RebuildWorkspaceRetrieval: (request) => getWailsBridge().RebuildWorkspaceRetrieval(request),
  RegisterWorkspaceProject: (request) => getWailsBridge().RegisterWorkspaceProject(request),
  ListStaffSummaries: (request) => getWailsBridge().ListStaffSummaries(request),
  GetCockpitRuntime: () => getWailsBridge().GetCockpitRuntime(),
  GetCockpitWorkspace: (request) => getWailsBridge().GetCockpitWorkspace(request),
  RegisterCockpitProject: (request) => getWailsBridge().RegisterCockpitProject(request),
  QueryCockpitWorkspace: (request) => getWailsBridge().QueryCockpitWorkspace(request),
  RebuildCockpitRetrieval: (request) => getWailsBridge().RebuildCockpitRetrieval(request),
  ListOrchestraTasks: (request) => getWailsBridge().ListOrchestraTasks(request),
  GetOrchestraTask: (request) => getWailsBridge().GetOrchestraTask(request),
  CancelOrchestraTask: (request) => getWailsBridge().CancelOrchestraTask(request),
  RetryOrchestraTask: (request) => getWailsBridge().RetryOrchestraTask(request),
  ListKnowledgeSources: (request) => getWailsBridge().ListKnowledgeSources(request),
  QueryKnowledge: (request) => getWailsBridge().QueryKnowledge(request),
}

export async function getRuntimeSnapshot(
  bridge: RuntimeBridge,
): Promise<RuntimeSnapshot> {
  const snapshot = await bridge.GetRuntimeSnapshot({
    schemaVersion: runtimeSchemaVersion,
  })
  if (
    snapshot.schemaVersion !== runtimeSchemaVersion ||
    snapshot.status !== 'ready' ||
    snapshot.environment !== 'local' ||
    snapshot.provider !== 'disabled'
  ) {
    throw new Error('CortexOS returned an invalid runtime snapshot')
  }
  return snapshot
}
