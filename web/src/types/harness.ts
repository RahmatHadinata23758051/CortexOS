export const harnessSchemaVersion = 'cortexos.harness.bridge.v1' as const

export type HarnessCapabilitySummary = {
  capability: string
  description?: string
  requiresAsk?: boolean
  defaultEffect?: string
  schemaVersion: typeof harnessSchemaVersion
}

export type HarnessWorkerSummary = {
  id: string
  instanceId: string
  kind: string
  class: string
  version: string
  workerSchema: string
  status: string
  registeredAt: string
  lastHeartbeatAt: string
  schemaVersion: typeof harnessSchemaVersion
}

export type HarnessExecutionSummary = {
  id: string
  taskId: string
  engineClass: string
  priority: string
  memoryMB: number
  cpuPriority: number
  traceId: string
  workerId: string
  createdAt: string
  schemaVersion: typeof harnessSchemaVersion
}

export type HarnessEvidenceSummary = {
  id: string
  executionId: string
  taskId: string
  worktreeId: string
  kind: string
  digest: string
  collectedAt: string
  schemaVersion: typeof harnessSchemaVersion
}

export type HarnessBridge = {
  ListHarnessCapabilities(request: { schemaVersion: typeof harnessSchemaVersion }): Promise<HarnessCapabilitySummary[]>
  ListHarnessWorkers(request: { schemaVersion: typeof harnessSchemaVersion }): Promise<HarnessWorkerSummary[]>
  ListActiveHarnessExecutions(request: { schemaVersion: typeof harnessSchemaVersion }): Promise<HarnessExecutionSummary[]>
  GetHarnessEvidence(request: { schemaVersion: typeof harnessSchemaVersion; executionId: string }): Promise<HarnessEvidenceSummary[]>
}

export function isHarnessWorkerSummary(value: unknown): value is HarnessWorkerSummary {
  if (!value || typeof value !== 'object') return false
  const worker = value as Partial<HarnessWorkerSummary>
  return typeof worker.id === 'string' && typeof worker.status === 'string' && worker.schemaVersion === harnessSchemaVersion
}

export function isHarnessExecutionSummary(value: unknown): value is HarnessExecutionSummary {
  if (!value || typeof value !== 'object') return false
  const execution = value as Partial<HarnessExecutionSummary>
  return typeof execution.id === 'string' && typeof execution.taskId === 'string' &&
    typeof execution.memoryMB === 'number' && typeof execution.cpuPriority === 'number' &&
    execution.schemaVersion === harnessSchemaVersion
}
