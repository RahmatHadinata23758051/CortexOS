export const orchestraSchemaVersion = 'cortexos.orchestra.bridge.v1' as const

/** The only lifecycle states the task board may render. Orchestra owns transitions. */
export type OrchestraTaskStatus =
  | 'draft'
  | 'ready'
  | 'running'
  | 'awaitingInspection'
  | 'success'
  | 'failed'
  | 'canceled'

export type OrchestraTaskSummary = {
  id: string
  projectId: string
  status: OrchestraTaskStatus
  attempt: number
  maxAttempts: number
  progress: number
  schemaVersion: typeof orchestraSchemaVersion
}

export type OrchestraTimelineEntry = {
  sequence: number
  type: string
  from?: OrchestraTaskStatus
  to?: OrchestraTaskStatus
  occurredAt: string
  evidenceIds?: string[]
}

export type OrchestraTaskDetail = OrchestraTaskSummary & {
  acceptanceCount: number
  dependencyCount: number
  evidence: Array<{ id: string; count: number }>
  timeline: OrchestraTimelineEntry[]
}

export type OrchestraTaskListRequest = {
  schemaVersion: typeof orchestraSchemaVersion
  projectId?: string
  limit: number
}

export type OrchestraTaskRequest = {
  schemaVersion: typeof orchestraSchemaVersion
  taskId: string
}

export type OrchestraBridge = {
  ListOrchestraTasks(request: OrchestraTaskListRequest): Promise<OrchestraTaskSummary[]>
  GetOrchestraTask(request: OrchestraTaskRequest): Promise<OrchestraTaskDetail>
  CancelOrchestraTask(request: OrchestraTaskRequest): Promise<void>
  RetryOrchestraTask(request: OrchestraTaskRequest): Promise<void>
}

export function isOrchestraTaskStatus(value: unknown): value is OrchestraTaskStatus {
  return typeof value === 'string' && [
    'draft', 'ready', 'running', 'awaitingInspection', 'success', 'failed', 'canceled',
  ].includes(value)
}

export function isOrchestraTaskSummary(value: unknown): value is OrchestraTaskSummary {
  if (!value || typeof value !== 'object') return false
  const task = value as Partial<OrchestraTaskSummary>
  const attempt = task.attempt
  const maxAttempts = task.maxAttempts
  const progress = task.progress
  return (
    typeof task.id === 'string' &&
    typeof task.projectId === 'string' &&
    isOrchestraTaskStatus(task.status) &&
    typeof attempt === 'number' && Number.isInteger(attempt) && attempt >= 0 &&
    typeof maxAttempts === 'number' && Number.isInteger(maxAttempts) && maxAttempts >= 0 &&
    typeof progress === 'number' && Number.isInteger(progress) && progress >= 0 && progress <= 100 &&
    task.schemaVersion === orchestraSchemaVersion
  )
}

export function isOrchestraTaskDetail(value: unknown): value is OrchestraTaskDetail {
  if (!isOrchestraTaskSummary(value)) return false
  const detail = value as Partial<OrchestraTaskDetail>
  const acceptanceCount = detail.acceptanceCount
  const dependencyCount = detail.dependencyCount
  return (
    typeof acceptanceCount === 'number' && Number.isInteger(acceptanceCount) && acceptanceCount >= 0 &&
    typeof dependencyCount === 'number' && Number.isInteger(dependencyCount) && dependencyCount >= 0 &&
    Array.isArray(detail.evidence) && Array.isArray(detail.timeline)
  )
}

export const OrchestraEventNames = {
  TaskChanged: 'cortexos:orchestra:task:changed',
} as const
