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
import type { WorkspaceBridge } from './workspace'
import type { OfficeBridge } from './office'

export type WailsWindow = Window & {
  go?: {
    platform?: {
      Bridge?: RuntimeBridge & WorkspaceBridge & OfficeBridge
    }
  }
}

function getWailsBridge(): RuntimeBridge & WorkspaceBridge & OfficeBridge {
  const bridge = (window as WailsWindow).go?.platform?.Bridge
  if (!bridge) {
    throw new Error('CortexOS runtime bridge is unavailable')
  }
  return bridge
}

export const wailsRuntimeBridge: RuntimeBridge & WorkspaceBridge & OfficeBridge = {
  GetRuntimeSnapshot: (request) => getWailsBridge().GetRuntimeSnapshot(request),
  GetWorkspaceSnapshot: (request) => getWailsBridge().GetWorkspaceSnapshot(request),
  QueryWorkspace: (request) => getWailsBridge().QueryWorkspace(request),
  RebuildWorkspaceRetrieval: (request) => getWailsBridge().RebuildWorkspaceRetrieval(request),
  RegisterWorkspaceProject: (request) => getWailsBridge().RegisterWorkspaceProject(request),
  ListStaffSummaries: (request) => getWailsBridge().ListStaffSummaries(request),
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
