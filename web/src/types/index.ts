import { GetRuntimeSnapshot } from '../../wailsjs/go/platform/Bridge'

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

export const wailsRuntimeBridge: RuntimeBridge = {
  GetRuntimeSnapshot: async (request) => {
    const snapshot = await GetRuntimeSnapshot(request)
    if (
      snapshot.schemaVersion !== runtimeSchemaVersion ||
      snapshot.status !== 'ready' ||
      snapshot.environment !== 'local' ||
      snapshot.provider !== 'disabled'
    ) {
      throw new Error('CortexOS returned an invalid runtime snapshot')
    }
    return snapshot as RuntimeSnapshot
  },
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
