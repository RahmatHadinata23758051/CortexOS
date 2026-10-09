export const knowledgeSchemaVersion = 'cortexos.knowledge.bridge.v1' as const

export type KnowledgeSourcesRequest = {
  schemaVersion: typeof knowledgeSchemaVersion
  projectId: string
}

export type KnowledgeQueryRequest = {
  schemaVersion: typeof knowledgeSchemaVersion
  projectId: string
  query: string
  limit?: number
}

export type KnowledgeSource = {
  id: string
  projectId: string
  title: string
  relativePath: string
  sourceKind: string
  status: string
  contentHash: string
  updatedAt: string
  schemaVersion: string
}

export type KnowledgeResult = {
  documentId: string
  title: string
  relativePath: string
  snippet: string
  score: number
  schemaVersion: string
}

export type KnowledgeBridge = {
  ListKnowledgeSources(request: KnowledgeSourcesRequest): Promise<KnowledgeSource[]>
  QueryKnowledge(request: KnowledgeQueryRequest): Promise<KnowledgeResult[]>
}

export function isKnowledgeSource(value: unknown): value is KnowledgeSource {
  if (!value || typeof value !== 'object') return false
  const source = value as Partial<KnowledgeSource>
  return typeof source.id === 'string' &&
    typeof source.projectId === 'string' &&
    typeof source.title === 'string' &&
    typeof source.relativePath === 'string' &&
    typeof source.sourceKind === 'string' &&
    typeof source.status === 'string' &&
    typeof source.contentHash === 'string' &&
    typeof source.updatedAt === 'string' &&
    source.schemaVersion === knowledgeSchemaVersion
}

export function isKnowledgeResult(value: unknown): value is KnowledgeResult {
  if (!value || typeof value !== 'object') return false
  const result = value as Partial<KnowledgeResult>
  return typeof result.documentId === 'string' &&
    typeof result.title === 'string' &&
    typeof result.relativePath === 'string' &&
    typeof result.snippet === 'string' &&
    typeof result.score === 'number' &&
    result.schemaVersion === knowledgeSchemaVersion
}

export const KnowledgeEventNames = {
  SourcesChanged: 'cortexos:knowledge:sources:changed',
  RetrievalChanged: 'cortexos:knowledge:retrieval:changed',
  IngestionChanged: 'cortexos:knowledge:ingestion:changed',
} as const

export type KnowledgeEventName = (typeof KnowledgeEventNames)[keyof typeof KnowledgeEventNames]

export type KnowledgeState<T> =
  | { status: 'idle' }
  | { status: 'loading' }
  | { status: 'success'; items: T[] }
  | { status: 'error'; code: string; message: string }
