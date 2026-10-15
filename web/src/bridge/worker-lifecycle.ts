import {
  workerLifecycleSchemaVersion,
  type WorkerCardsRequest,
  type WorkerCardsResult,
  type WorkerLifecycleBridge,
} from '../types/worker-lifecycle'

export async function loadWorkerCardsSnapshot(
  bridge: WorkerLifecycleBridge,
  projectId?: string,
  includeHistory = false,
): Promise<WorkerCardsResult> {
  const request: WorkerCardsRequest = {
    schemaVersion: workerLifecycleSchemaVersion,
    projectId,
    includeHistory,
  }
  return bridge.GetWorkerCards(request)
}
