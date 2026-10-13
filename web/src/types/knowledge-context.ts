export const knowledgeContextSchemaVersion = 'cortexos.knowledge.context.v1' as const

export type KnowledgeContextBudget = {
  maxItems: number
  maxSnippetChars: number
  maxTotalChars: number
  timeoutMs: number
}

export type KnowledgeContextRequest = {
  projectId: string
  query: string
  budget: KnowledgeContextBudget
  schemaVersion: typeof knowledgeContextSchemaVersion
}

export type KnowledgeContextItem = {
  id: string
  title: string
  sourceLabel: string
  snippet: string
  score: number
}

export type GroundedContextRef = {
  id: string
  title: string
  sourceLabel: string
}

export type KnowledgeContextBundle = {
  items: KnowledgeContextItem[]
  totalChars: number
  truncated: boolean
  degraded: boolean
  degradedReason?: string
  schemaVersion: typeof knowledgeContextSchemaVersion
}

export type KnowledgeContextBridge = {
  GroundRoutingContext(request: KnowledgeContextRequest): Promise<KnowledgeContextBundle>
}

export function isKnowledgeContextItem(value: unknown): value is KnowledgeContextItem {
  if (!value || typeof value !== 'object') return false
  const item = value as Partial<KnowledgeContextItem>
  return typeof item.id === 'string' &&
    typeof item.title === 'string' &&
    typeof item.sourceLabel === 'string' &&
    typeof item.snippet === 'string' &&
    typeof item.score === 'number'
}

export function isKnowledgeContextBundle(value: unknown): value is KnowledgeContextBundle {
  if (!value || typeof value !== 'object') return false
  const bundle = value as Partial<KnowledgeContextBundle>
  return bundle.schemaVersion === knowledgeContextSchemaVersion &&
    Array.isArray(bundle.items) && bundle.items.every(isKnowledgeContextItem) &&
    typeof bundle.totalChars === 'number' &&
    typeof bundle.truncated === 'boolean' &&
    typeof bundle.degraded === 'boolean'
}
