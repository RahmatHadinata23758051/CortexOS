import { describe, expect, it, vi } from 'vitest'
import { loadKnowledgeSources, queryKnowledge } from './knowledge'
import { knowledgeSchemaVersion, type KnowledgeBridge } from '../types'

const source = { id: 'k1', projectId: 'p1', title: 'Guide', relativePath: 'docs/guide.md', sourceKind: 'vault_note', status: 'active', contentHash: 'sha256:abc', updatedAt: '2026-10-09T00:00:00Z', schemaVersion: knowledgeSchemaVersion }
const result = { documentId: 'd1', title: 'Guide', relativePath: 'docs/guide.md', snippet: 'A useful excerpt', score: 1.2, schemaVersion: knowledgeSchemaVersion }
const bridge: KnowledgeBridge = { ListKnowledgeSources: vi.fn().mockResolvedValue([source]), QueryKnowledge: vi.fn().mockResolvedValue([result]) }

describe('knowledge bridge', () => {
  it('loads and validates vault sources', async () => {
    await expect(loadKnowledgeSources(bridge, 'p1')).resolves.toEqual({ status: 'success', items: [source] })
    expect(bridge.ListKnowledgeSources).toHaveBeenCalledWith({ schemaVersion: knowledgeSchemaVersion, projectId: 'p1' })
  })
  it('queries bounded results with typed contract', async () => {
    await expect(queryKnowledge(bridge, 'p1', 'useful', 5)).resolves.toEqual({ status: 'success', items: [result] })
    expect(bridge.QueryKnowledge).toHaveBeenCalledWith({ schemaVersion: knowledgeSchemaVersion, projectId: 'p1', query: 'useful', limit: 5 })
  })
  it('returns structured errors', async () => {
    const failing = { ...bridge, QueryKnowledge: vi.fn().mockRejectedValue({ code: 'knowledge.error', message: 'unavailable' }) }
    await expect(queryKnowledge(failing, 'p1', 'query')).resolves.toEqual({ status: 'error', code: 'knowledge.error', message: 'unavailable' })
  })
})
