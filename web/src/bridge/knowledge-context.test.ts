import { describe, expect, it, vi } from 'vitest'
import {
  groundKnowledgeContext,
  defaultKnowledgeContextBudget,
  KnowledgeContextEventNames,
} from './knowledge-context'
import {
  knowledgeContextSchemaVersion,
  type KnowledgeContextBridge,
  type KnowledgeContextBundle,
  isKnowledgeContextBundle,
} from '../types'

describe('groundKnowledgeContext', () => {
  const sampleBundle: KnowledgeContextBundle = {
    items: [
      { id: 'k1', title: 'Arch Spec', sourceLabel: 'docs/arch.md', snippet: 'Deterministic plan', score: 0.95 },
    ],
    totalChars: 18,
    truncated: false,
    degraded: false,
    schemaVersion: knowledgeContextSchemaVersion,
  }

  it('queries bridge and returns validated bundle', async () => {
    const bridge: KnowledgeContextBridge = {
      GroundRoutingContext: vi.fn().mockResolvedValue(sampleBundle),
    }

    const res = await groundKnowledgeContext(bridge, 'proj-1', 'arch query')
    expect(res).toEqual(sampleBundle)
    expect(bridge.GroundRoutingContext).toHaveBeenCalledWith({
      projectId: 'proj-1',
      query: 'arch query',
      budget: defaultKnowledgeContextBudget(),
      schemaVersion: knowledgeContextSchemaVersion,
    })
  })

  it('rejects invalid inputs', async () => {
    const bridge: KnowledgeContextBridge = {
      GroundRoutingContext: vi.fn(),
    }
    await expect(groundKnowledgeContext(bridge, '', 'query')).rejects.toThrow('projectId is required')
    await expect(groundKnowledgeContext(bridge, 'proj-1', '   ')).rejects.toThrow('query is required')
  })

  it('validates schema typeguard', () => {
    expect(isKnowledgeContextBundle(sampleBundle)).toBe(true)
    expect(isKnowledgeContextBundle({ schemaVersion: 'wrong' })).toBe(false)
  })

  it('defines canonical event names', () => {
    expect(KnowledgeContextEventNames.Grounded).toBe('cortexos:knowledge:context:grounded')
  })
})
