import {
  harnessSchemaVersion,
  type HarnessBridge,
  type HarnessCapabilitySummary,
  type HarnessWorkerSummary,
  type HarnessExecutionSummary,
  type HarnessEvidenceSummary,
  isHarnessWorkerSummary,
  isHarnessExecutionSummary,
} from '../types/harness'

export type HarnessCapabilitiesState =
  | { status: 'loading' }
  | { status: 'success'; capabilities: HarnessCapabilitySummary[] }
  | { status: 'error'; code: string; message: string }

export type HarnessWorkersState =
  | { status: 'loading' }
  | { status: 'success'; workers: HarnessWorkerSummary[] }
  | { status: 'error'; code: string; message: string }

export type HarnessExecutionsState =
  | { status: 'loading' }
  | { status: 'success'; executions: HarnessExecutionSummary[] }
  | { status: 'error'; code: string; message: string }

export type HarnessEvidenceState =
  | { status: 'loading' }
  | { status: 'success'; evidence: HarnessEvidenceSummary[] }
  | { status: 'error'; code: string; message: string }

export async function loadHarnessCapabilities(
  bridge: HarnessBridge,
): Promise<HarnessCapabilitiesState> {
  try {
    const capabilities = await bridge.ListHarnessCapabilities({ schemaVersion: harnessSchemaVersion })
    if (!Array.isArray(capabilities)) throw new Error('Invalid capabilities response')
    return { status: 'success', capabilities }
  } catch (error) {
    return normalizeHarnessError(error)
  }
}

export async function loadHarnessWorkers(
  bridge: HarnessBridge,
): Promise<HarnessWorkersState> {
  try {
    const workers = await bridge.ListHarnessWorkers({ schemaVersion: harnessSchemaVersion })
    if (!Array.isArray(workers) || !workers.every(isHarnessWorkerSummary)) {
      throw new Error('CortexOS returned invalid harness workers')
    }
    return { status: 'success', workers }
  } catch (error) {
    return normalizeHarnessError(error)
  }
}

export async function loadActiveHarnessExecutions(
  bridge: HarnessBridge,
): Promise<HarnessExecutionsState> {
  try {
    const executions = await bridge.ListActiveHarnessExecutions({ schemaVersion: harnessSchemaVersion })
    if (!Array.isArray(executions) || !executions.every(isHarnessExecutionSummary)) {
      throw new Error('CortexOS returned invalid harness executions')
    }
    return { status: 'success', executions }
  } catch (error) {
    return normalizeHarnessError(error)
  }
}

export async function loadHarnessEvidence(
  bridge: HarnessBridge,
  executionId: string,
): Promise<HarnessEvidenceState> {
  try {
    if (!executionId) throw new Error('Execution ID required')
    const evidence = await bridge.GetHarnessEvidence({ schemaVersion: harnessSchemaVersion, executionId })
    if (!Array.isArray(evidence)) throw new Error('Invalid evidence response')
    return { status: 'success', evidence }
  } catch (error) {
    return normalizeHarnessError(error)
  }
}

function normalizeHarnessError(error: unknown): { status: 'error'; code: string; message: string } {
  if (error && typeof error === 'object') {
    const candidate = error as Partial<{ code: string; message: string }>
    if (typeof candidate.code === 'string' && typeof candidate.message === 'string') {
      return { status: 'error', code: candidate.code, message: candidate.message }
    }
  }
  return {
    status: 'error',
    code: 'harness.internal',
    message: error instanceof Error ? error.message : 'Harness bridge failed',
  }
}