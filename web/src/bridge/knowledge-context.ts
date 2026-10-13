import {
  isKnowledgeContextBundle,
  knowledgeContextSchemaVersion,
  type KnowledgeContextBridge,
  type KnowledgeContextBundle,
  type KnowledgeContextBudget,
} from '../types'
import { onCockpitEvent } from './events'

export const KnowledgeContextEventNames = {
  Grounded: 'cortexos:knowledge:context:grounded',
} as const

export function defaultKnowledgeContextBudget(): KnowledgeContextBudget {
  return { maxItems: 5, maxSnippetChars: 400, maxTotalChars: 2000, timeoutMs: 2000 }
}

export async function groundKnowledgeContext(
  bridge: KnowledgeContextBridge,
  projectId: string,
  query: string,
  budget = defaultKnowledgeContextBudget(),
): Promise<KnowledgeContextBundle> {
  if (!projectId.trim()) throw new Error('projectId is required')
  if (!query.trim()) throw new Error('query is required')

  const result = await bridge.GroundRoutingContext({
    projectId,
    query,
    budget,
    schemaVersion: knowledgeContextSchemaVersion,
  })
  if (!isKnowledgeContextBundle(result)) throw new Error('CortexOS returned invalid knowledge context')
  return result
}

export function onKnowledgeContextGrounded(handler: (payload: unknown) => void): () => void {
  return onCockpitEvent(KnowledgeContextEventNames.Grounded, handler)
}
