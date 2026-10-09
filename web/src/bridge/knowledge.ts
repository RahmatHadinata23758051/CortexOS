import {
  knowledgeSchemaVersion,
  type KnowledgeBridge,
  type KnowledgeResult,
  type KnowledgeSource,
  isKnowledgeResult,
  isKnowledgeSource,
} from '../types'
import { onCockpitEvent } from './events'
import { KnowledgeEventNames, type KnowledgeEventName as KnowledgeEvent } from '../types'

export type KnowledgeState<T> =
  | { status: 'idle' }
  | { status: 'loading' }
  | { status: 'success'; items: T[] }
  | { status: 'error'; code: string; message: string }

export async function loadKnowledgeSources(bridge: KnowledgeBridge, projectId: string): Promise<KnowledgeState<KnowledgeSource>> {
  try {
    const sources = await bridge.ListKnowledgeSources({ schemaVersion: knowledgeSchemaVersion, projectId })
    if (!Array.isArray(sources) || !sources.every(isKnowledgeSource)) throw new Error('CortexOS returned invalid knowledge sources')
    return { status: 'success', items: sources }
  } catch (error) {
    return normalizeKnowledgeError(error)
  }
}

export async function queryKnowledge(bridge: KnowledgeBridge, projectId: string, query: string, limit = 20): Promise<KnowledgeState<KnowledgeResult>> {
  try {
    const results = await bridge.QueryKnowledge({ schemaVersion: knowledgeSchemaVersion, projectId, query, limit })
    if (!Array.isArray(results) || !results.every(isKnowledgeResult)) throw new Error('CortexOS returned invalid knowledge results')
    return { status: 'success', items: results }
  } catch (error) {
    return normalizeKnowledgeError(error)
  }
}

export type KnowledgeEventName = KnowledgeEvent
export function onKnowledgeEvent(eventName: KnowledgeEventName, handler: (payload: unknown) => void): () => void {
  return onCockpitEvent(eventName, handler)
}
export function onKnowledgeSourcesChanged(handler: (payload: unknown) => void): () => void {
  return onKnowledgeEvent(KnowledgeEventNames.SourcesChanged, handler)
}
export function onKnowledgeRetrievalChanged(handler: (payload: unknown) => void): () => void {
  return onKnowledgeEvent(KnowledgeEventNames.RetrievalChanged, handler)
}

function normalizeKnowledgeError(error: unknown): { status: 'error'; code: string; message: string } {
  if (error && typeof error === 'object') {
    const candidate = error as Partial<{ code: string; message: string }>
    if (typeof candidate.code === 'string' && typeof candidate.message === 'string') return { status: 'error', code: candidate.code, message: candidate.message }
  }
  return { status: 'error', code: 'knowledge.internal', message: error instanceof Error ? error.message : 'Knowledge bridge failed' }
}
