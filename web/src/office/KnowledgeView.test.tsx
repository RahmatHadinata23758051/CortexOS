import { describe, expect, it } from 'vitest'
import { renderToStaticMarkup } from 'react-dom/server'
import { KnowledgeView } from './KnowledgeView'

describe('KnowledgeView', () => {
  it('renders knowledge section with tabs', () => {
    const markup = renderToStaticMarkup(<KnowledgeView />)
    expect(markup).toContain('Vault knowledge browser')
    expect(markup).toContain('Browser')
    expect(markup).toContain('Search')
    expect(markup).toContain('Ingestion')
    expect(markup).toContain('Provenance')
  })

  it('renders project id input', () => {
    const markup = renderToStaticMarkup(<KnowledgeView />)
    expect(markup).toContain('Project ID')
    expect(markup).toContain('placeholder="project-id"')
  })

  it('renders search form with query input when search route is active', () => {
    const markup = renderToStaticMarkup(<KnowledgeView initialRoute="search" />)
    expect(markup).toContain('Search query')
    expect(markup).toContain('Search vault knowledge')
    expect(markup).toContain('Enter a query')
  })

  it('renders ingestion status view with progress pipeline', () => {
    const markup = renderToStaticMarkup(<KnowledgeView initialRoute="ingestion" />)
    expect(markup).toContain('Ingestion pipeline')
    expect(markup).toContain('Vault → Knowledge')
    expect(markup).toContain('Discover')
    expect(markup).toContain('Chunk')
    expect(markup).toContain('Redact')
    expect(markup).toContain('Validate')
    expect(markup).toContain('Index')
  })

  it('renders provenance visualization seam', () => {
    const markup = renderToStaticMarkup(<KnowledgeView initialRoute="provenance" />)
    expect(markup).toContain('Attribution chain')
    expect(markup).toContain('Provenance visualization')
    expect(markup).toContain('Chain of custody')
  })

  it('renders empty browser state when no project is selected', () => {
    const markup = renderToStaticMarkup(<KnowledgeView initialRoute="browser" />)
    expect(markup).toContain('Choose a project')
  })
})
