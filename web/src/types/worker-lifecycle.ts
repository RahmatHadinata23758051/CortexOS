export const workerLifecycleSchemaVersion = 'cortexos.worker-lifecycle.v1' as const

export type WorkerLifecycleState =
  | 'spawned'
  | 'running'
  | 'awaiting_approval'
  | 'reviewing'
  | 'accepted'
  | 'unspawned'
  | 'failed'
  | 'blocked'

export type WorkerCard = {
  workerId: string
  taskId: string
  projectId: string
  callSign?: string
  role?: string
  state: WorkerLifecycleState
  reason?: string
  spawnedAt: string
  updatedAt: string
  unspawnedAt?: string
  schemaVersion: typeof workerLifecycleSchemaVersion
}

export type WorkerLifecycleEvent = {
  eventId: string
  workerId: string
  taskId: string
  projectId: string
  from?: WorkerLifecycleState
  to: WorkerLifecycleState
  reason?: string
  occurredAt: string
  schemaVersion: typeof workerLifecycleSchemaVersion
}

export type WorkerCardsRequest = {
  schemaVersion: typeof workerLifecycleSchemaVersion
  projectId?: string
  includeHistory?: boolean
}

export type WorkerCardsResult = {
  schemaVersion: typeof workerLifecycleSchemaVersion
  projectId?: string
  workers: WorkerCard[]
  events: WorkerLifecycleEvent[]
  generatedAt: string
}

export type WorkerTransitionRequest = {
  workerId: string
  state: WorkerLifecycleState
  reason?: string
  schemaVersion: typeof workerLifecycleSchemaVersion
}

export type WorkerTransitionResult = {
  worker: WorkerCard
  event: WorkerLifecycleEvent
  schemaVersion: typeof workerLifecycleSchemaVersion
}

export type WorkerLifecycleBridge = {
  GetWorkerCards(request: WorkerCardsRequest): Promise<WorkerCardsResult>
  TransitionWorker(request: WorkerTransitionRequest): Promise<WorkerTransitionResult>
}
