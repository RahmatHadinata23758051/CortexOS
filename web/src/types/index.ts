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
export * from './coordination'
export * from './knowledge'
export * from './knowledge-context'
export * from './harness'
export * from './brain'
export * from './phase13'
export * from './orchestra-plan'
export * from './files'
export * from './worker-lifecycle'
import type { WorkspaceBridge } from './workspace'
import type { Phase13Bridge } from './phase13'
import type { KnowledgeBridge } from './knowledge'
import type { KnowledgeContextBridge } from './knowledge-context'
import type { HarnessBridge } from './harness'
import type { OfficeBridge } from './office'
import type { CockpitBridge } from './cockpit'
import type { OrchestraBridge } from './orchestra'
import type { CoordinationBridge } from './coordination'
import type { BrainBridge } from './brain'
import type { OrchestraPlanBridge } from './orchestra-plan'
import type { FilesBridge } from './files'
import type { WorkerLifecycleBridge } from './worker-lifecycle'

export type FullBridge = RuntimeBridge & WorkspaceBridge & OfficeBridge & CockpitBridge & OrchestraBridge & CoordinationBridge & KnowledgeBridge & KnowledgeContextBridge & HarnessBridge & BrainBridge & Phase13Bridge & OrchestraPlanBridge & FilesBridge & WorkerLifecycleBridge

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
  PickDirectory: () => getWailsBridge().PickDirectory(),
  GetWorkspaceSnapshot: (request) => getWailsBridge().GetWorkspaceSnapshot(request),
  QueryWorkspace: (request) => getWailsBridge().QueryWorkspace(request),
  RebuildWorkspaceRetrieval: (request) => getWailsBridge().RebuildWorkspaceRetrieval(request),
  RegisterWorkspaceProject: (request) => getWailsBridge().RegisterWorkspaceProject(request),
  ListFiles: (request) => getWailsBridge().ListFiles(request),
  ReadFile: (request) => getWailsBridge().ReadFile(request),
  GetWorkerCards: (request) => getWailsBridge().GetWorkerCards(request),
  TransitionWorker: (request) => getWailsBridge().TransitionWorker(request),
  ListStaffSummaries: (request) => getWailsBridge().ListStaffSummaries(request),
  GetCockpitRuntime: () => getWailsBridge().GetCockpitRuntime(),
  GetCockpitWorkspace: (request) => getWailsBridge().GetCockpitWorkspace(request),
  RegisterCockpitProject: (request) => getWailsBridge().RegisterCockpitProject(request),
  QueryCockpitWorkspace: (request) => getWailsBridge().QueryCockpitWorkspace(request),
  RebuildCockpitRetrieval: (request) => getWailsBridge().RebuildCockpitRetrieval(request),
  ListOrchestraTasks: (request) => getWailsBridge().ListOrchestraTasks(request),
  GetCoordinationSnapshot: (request) => getWailsBridge().GetCoordinationSnapshot(request),
  GetOrchestraTask: (request) => getWailsBridge().GetOrchestraTask(request),
  CancelOrchestraTask: (request) => getWailsBridge().CancelOrchestraTask(request),
  RetryOrchestraTask: (request) => getWailsBridge().RetryOrchestraTask(request),
  MergeOrchestraTask: (request) => getWailsBridge().MergeOrchestraTask(request),
  ListKnowledgeSources: (request) => getWailsBridge().ListKnowledgeSources(request),
  QueryKnowledge: (request) => getWailsBridge().QueryKnowledge(request),
  GroundRoutingContext: (request) => getWailsBridge().GroundRoutingContext(request),
  ListHarnessCapabilities: (request) => getWailsBridge().ListHarnessCapabilities(request),
  ListHarnessWorkers: (request) => getWailsBridge().ListHarnessWorkers(request),
  ListActiveHarnessExecutions: (request) => getWailsBridge().ListActiveHarnessExecutions(request),
  GetHarnessEvidence: (request) => getWailsBridge().GetHarnessEvidence(request),
  DispatchBrainTask: (request) => getWailsBridge().DispatchBrainTask(request),
  DispatchBrainTaskStream: (request) => getWailsBridge().DispatchBrainTaskStream(request),
  RouteBrainTaskToOrchestra: (request) => getWailsBridge().RouteBrainTaskToOrchestra(request),
  ProposeOrchestraPlan: (request) => getWailsBridge().ProposeOrchestraPlan(request),
  ConfirmOrchestraPlan: (request) => getWailsBridge().ConfirmOrchestraPlan(request),
  RejectOrchestraPlan: (request) => getWailsBridge().RejectOrchestraPlan(request),
  GetOrchestraPlan: (request) => getWailsBridge().GetOrchestraPlan(request),
  ProposeToolCall: (proposal) => getWailsBridge().ProposeToolCall(proposal),
  DecideToolCall: (decision) => getWailsBridge().DecideToolCall(decision),
  GetToolApproval: (proposalId) => getWailsBridge().GetToolApproval(proposalId),
  ListPendingToolApprovals: () => getWailsBridge().ListPendingToolApprovals(),
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
