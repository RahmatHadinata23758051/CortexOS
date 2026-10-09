import {
  orchestraSchemaVersion,
  type OrchestraBridge,
  type OrchestraTaskDetail,
  type OrchestraTaskListRequest,
  type OrchestraTaskRequest,
  type OrchestraTaskSummary,
  isOrchestraTaskDetail,
  isOrchestraTaskSummary,
} from '../types/orchestra'

export type OrchestraTaskState =
  | { status: 'loading' }
  | { status: 'success'; tasks: OrchestraTaskSummary[] }
  | { status: 'error'; code: string; message: string }

export async function loadOrchestraTasks(
  bridge: OrchestraBridge,
  options: { projectId?: string; limit?: number } = {},
): Promise<OrchestraTaskState> {
  try {
    const request: OrchestraTaskListRequest = {
      schemaVersion: orchestraSchemaVersion,
      projectId: options.projectId,
      limit: options.limit ?? 100,
    }
    const tasks = await bridge.ListOrchestraTasks(request)
    if (!Array.isArray(tasks) || !tasks.every(isOrchestraTaskSummary)) {
      throw new Error('CortexOS returned invalid orchestra tasks')
    }
    return { status: 'success', tasks }
  } catch (error) {
    return normalizeOrchestraError(error)
  }
}

export async function getOrchestraTask(bridge: OrchestraBridge, taskId: string): Promise<OrchestraTaskDetail> {
  const request: OrchestraTaskRequest = { schemaVersion: orchestraSchemaVersion, taskId }
  const task = await bridge.GetOrchestraTask(request)
  if (!isOrchestraTaskDetail(task)) throw new Error('CortexOS returned invalid orchestra task detail')
  return task
}

/** Commands remain seams: the UI asks Orchestra to act and owns no lifecycle logic. */
export function cancelOrchestraTask(bridge: OrchestraBridge, taskId: string): Promise<void> {
  return bridge.CancelOrchestraTask({ schemaVersion: orchestraSchemaVersion, taskId })
}

export function retryOrchestraTask(bridge: OrchestraBridge, taskId: string): Promise<void> {
  return bridge.RetryOrchestraTask({ schemaVersion: orchestraSchemaVersion, taskId })
}

function normalizeOrchestraError(error: unknown): { status: 'error'; code: string; message: string } {
  if (error && typeof error === 'object') {
    const candidate = error as Partial<{ code: string; message: string }>
    if (typeof candidate.code === 'string' && typeof candidate.message === 'string') {
      return { status: 'error', code: candidate.code, message: candidate.message }
    }
  }
  return { status: 'error', code: 'orchestra.internal', message: error instanceof Error ? error.message : 'Orchestra bridge failed' }
}

export * from './orchestra-events'
