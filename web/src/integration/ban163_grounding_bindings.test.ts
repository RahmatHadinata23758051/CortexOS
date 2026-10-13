import { describe, expect, it } from 'vitest'
import {
  knowledgeContextSchemaVersion,
  isKnowledgeContextBundle,
  isKnowledgeContextItem,
  type KnowledgeContextBundle,
  type KnowledgeContextBudget,
} from '../types'
import { defaultKnowledgeContextBudget } from '../bridge/knowledge-context'

describe('BAN-163 Knowledge Grounding Bindings and Redaction', () => {
  it('enforces schema version alignment with cortexos.knowledge.context.v1', () => {
    expect(knowledgeContextSchemaVersion).toBe('cortexos.knowledge.context.v1')
  })

  it('enforces budget limits within safe ceilings', () => {
    const budget: KnowledgeContextBudget = defaultKnowledgeContextBudget()
    expect(budget.maxItems).toBeLessThanOrEqual(20)
    expect(budget.maxSnippetChars).toBeLessThanOrEqual(2000)
    expect(budget.maxTotalChars).toBeLessThanOrEqual(8000)
    expect(budget.timeoutMs).toBeLessThanOrEqual(10000)
  })

  it('validates sanitized context items with no absolute paths or secret tokens', () => {
    const safeBundle: KnowledgeContextBundle = {
      items: [
        {
          id: 'doc-101',
          title: 'Architecture Rules',
          sourceLabel: 'docs/architecture.md',
          snippet: 'Use [PATH] for local storage and [REDACTED] for auth headers.',
          score: 0.98,
        },
        {
          id: 'doc-102',
          title: 'Security Standard',
          sourceLabel: 'specs/security.md',
          snippet: 'All bridge DTOs must omit secrets and process handles.',
          score: 0.85,
        },
      ],
      totalChars: 110,
      truncated: false,
      degraded: false,
      schemaVersion: knowledgeContextSchemaVersion,
    }

    expect(isKnowledgeContextBundle(safeBundle)).toBe(true)
    for (const item of safeBundle.items) {
      expect(isKnowledgeContextItem(item)).toBe(true)
      expect(item.sourceLabel).not.toMatch(/^[a-zA-Z]:[\\/]/)
      expect(item.sourceLabel).not.toMatch(/^\/(?:etc|home|Users|var|opt)/)
      expect(item.snippet).not.toContain('apiKey=')
      expect(item.snippet).not.toContain('Bearer ')
    }
  })

  it('measures context serialization latency and payload size', () => {
    const items = Array.from({ length: 5 }, (_, i) => ({
      id: `doc-${i}`,
      title: `Document ${i}`,
      sourceLabel: `repo/docs/doc-${i}.md`,
      snippet: `This is a bounded snippet of context for testing performance and size budget ${i}.`,
      score: 1 - i * 0.1,
    }))

    const bundle: KnowledgeContextBundle = {
      items,
      totalChars: items.reduce((acc, it) => acc + it.snippet.length, 0),
      truncated: false,
      degraded: false,
      schemaVersion: knowledgeContextSchemaVersion,
    }

    const start = performance.now()
    const serialized = JSON.stringify(bundle)
    const elapsedMs = performance.now() - start

    expect(elapsedMs).toBeLessThan(50) // Sub-50ms serialization
    expect(serialized.length).toBeLessThan(4000) // Compact payload well under 4KB
  })
})
