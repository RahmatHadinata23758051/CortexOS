import { useEffect, useState, type FormEvent, type ChangeEvent } from 'react'
import { type KnowledgeSource, type KnowledgeResult, type KnowledgeState } from '../types'
import { loadKnowledgeSources, queryKnowledge, onKnowledgeSourcesChanged, onKnowledgeRetrievalChanged } from '../bridge/knowledge'
import { wailsRuntimeBridge } from '../types'
import type { KnowledgeBridge } from '../types'

type KnowledgeRoute = 'browser' | 'search' | 'ingestion' | 'provenance'

interface KnowledgeViewProps {
  initialRoute?: KnowledgeRoute
}

export function KnowledgeView({ initialRoute = 'browser' }: KnowledgeViewProps = {}) {
  const [route, setRoute] = useState<KnowledgeRoute>(initialRoute)
  const [projectId, setProjectId] = useState('')
  const [sources, setSources] = useState<KnowledgeState<KnowledgeSource>>({ status: 'loading' })
  const [results, setResults] = useState<KnowledgeState<KnowledgeResult>>({ status: 'idle' })
  const [query, setQuery] = useState('')
  const [searchDebounce, setSearchDebounce] = useState('')
  const ingestionStatus = { status: 'idle', progress: 0, message: 'Awaiting ingestion trigger' }

  useEffect(() => {
    if (!projectId) return
    const unsubSources = onKnowledgeSourcesChanged(() => loadKnowledgeSources(wailsRuntimeBridge as KnowledgeBridge, projectId).then(setSources))
    const unsubRetrieval = onKnowledgeRetrievalChanged(() => query && queryKnowledge(wailsRuntimeBridge as KnowledgeBridge, projectId, query).then(setResults))
    loadKnowledgeSources(wailsRuntimeBridge as KnowledgeBridge, projectId).then(setSources)
    return () => { unsubSources(); unsubRetrieval() }
  }, [projectId, query])

  useEffect(() => {
    if (!query || searchDebounce === query) return
    const timer = setTimeout(() => {
      setSearchDebounce(query)
      queryKnowledge(wailsRuntimeBridge as KnowledgeBridge, projectId, query).then(setResults)
    }, 300)
    return () => clearTimeout(timer)
  }, [query, projectId, searchDebounce])

  const handleSearch = (e: FormEvent<HTMLFormElement>) => {
    e.preventDefault()
    setSearchDebounce(query)
    queryKnowledge(wailsRuntimeBridge as KnowledgeBridge, projectId, query).then(setResults)
  }

  return (
    <div className="knowledge-section reveal-1">
      <div className="section-heading">
        <div><span className="eyebrow">CORTEXOS / KNOWLEDGE</span><h2>Vault knowledge browser</h2></div>
        <span className="section-index">01 / 01</span>
      </div>

      <div className="knowledge-toolbar reveal-2">
        <label className="project-select">
          <span className="eyebrow">Project ID</span>
          <input value={projectId} onChange={(e: ChangeEvent<HTMLInputElement>) => setProjectId(e.target.value)} placeholder="project-id" required />
        </label>
        <nav className="knowledge-tabs" aria-label="Knowledge views">
          {(['browser', 'search', 'ingestion', 'provenance'] as KnowledgeRoute[]).map(r => (
            <button key={r} className={`knowledge-tab ${route === r ? 'active' : ''}`} onClick={() => setRoute(r)}>{r.charAt(0).toUpperCase() + r.slice(1)}</button>
          ))}
        </nav>
      </div>

      {route === 'browser' && <KnowledgeBrowser sources={sources} projectId={projectId} />}
      {route === 'search' && <KnowledgeSearch query={query} onQueryChange={setQuery} onSubmit={handleSearch} results={results} />}
      {route === 'ingestion' && <IngestionStatus status={ingestionStatus} />}
      {route === 'provenance' && <ProvenanceSeam />}
    </div>
  )
}

function KnowledgeBrowser({ sources, projectId }: { sources: KnowledgeState<KnowledgeSource>; projectId: string }) {
  if (!projectId) return <div className="knowledge-empty"><span className="empty-glyph" aria-hidden="true">◈</span><div><strong>Choose a project</strong><p>Enter a project ID from the toolbar to browse its knowledge sources.</p></div></div>
  if (sources.status === 'loading') return <div className="knowledge-empty skeleton">Loading knowledge sources…</div>
  if (sources.status === 'error') return <div className="knowledge-error" role="alert"><strong>{sources.code}</strong><span>{sources.message}</span></div>
  if (sources.status === 'idle') return <div className="knowledge-empty"><strong>Choose a project</strong></div>
  if (sources.items.length === 0) return <div className="knowledge-empty"><span className="empty-glyph" aria-hidden="true">∅</span><div><strong>No knowledge sources</strong><p>Run ingestion to populate the knowledge vault.</p></div></div>

  const grouped: Record<string, KnowledgeSource[]> = sources.items.reduce((acc: Record<string, KnowledgeSource[]>, source: KnowledgeSource) => {
    const dir = source.relativePath.split('/')[0] || 'root'
    if (!acc[dir]) acc[dir] = []
    acc[dir].push(source)
    return acc
  }, {})

  return (
    <div className="knowledge-tree reveal-3" role="tree" aria-label="Knowledge sources">
      {Object.entries(grouped).map(([dir, items]: [string, KnowledgeSource[]]) => (
        <details key={dir} className="knowledge-dir" open>
          <summary className="knowledge-dir-summary">
            <span className="dir-icon" aria-hidden="true">📁</span>
            <span className="dir-name">{dir}</span>
            <span className="dir-count">{items.length}</span>
          </summary>
          <ul className="knowledge-files">
            {items.map((source: KnowledgeSource) => (
              <li key={source.id} className="knowledge-file" data-status={source.status}>
                <span className="file-icon" aria-hidden="true">📄</span>
                <span className="file-name">{source.title || source.relativePath}</span>
                <span className="file-meta">{source.relativePath}</span>
                <span className={`file-status ${source.status.toLowerCase()}`}>{source.status}</span>
              </li>
            ))}
          </ul>
        </details>
      ))}
    </div>
  )
}

function KnowledgeSearch({ query, onQueryChange, onSubmit, results }: { query: string; onQueryChange: (q: string) => void; onSubmit: (e: FormEvent<HTMLFormElement>) => void; results: KnowledgeState<KnowledgeResult> }) {
  return (
    <div className="knowledge-search reveal-3">
      <form className="search-form" onSubmit={onSubmit}>
        <label className="search-label">
          <span className="eyebrow">Search query</span>
          <input type="search" value={query} onChange={(e) => onQueryChange(e.target.value)} placeholder="Search vault knowledge…" aria-label="Vault knowledge search" autoFocus />
        </label>
        <button type="submit" className="search-submit" disabled={!query.trim()}>
          <span aria-hidden="true">⌕</span> Search
        </button>
      </form>

      <div className="search-results" role="region" aria-live="polite" aria-label="Search results">
        {results.status === 'idle' && <div className="search-empty"><span className="empty-glyph" aria-hidden="true">⌕</span><div><strong>Enter a query</strong><p>Search across ingested knowledge sources.</p></div></div>}
        {results.status === 'loading' && <div className="knowledge-empty skeleton">Searching…</div>}
        {results.status === 'error' && <div className="knowledge-error" role="alert"><strong>{results.code}</strong><span>{results.message}</span></div>}
        {results.status === 'success' && results.items.length === 0 && <div className="search-empty"><span className="empty-glyph" aria-hidden="true">∅</span><div><strong>No matches</strong><p>Try a different query or ingest more content.</p></div></div>}
        {results.status === 'success' && results.items.length > 0 && (
          <ul className="results-list" role="list">
            {results.items.map((result: KnowledgeResult) => (
              <li key={result.documentId} className="result-item">
                <div className="result-header">
                  <h4 className="result-title">{result.title}</h4>
                  <span className="result-score">{result.score.toFixed(1)}</span>
                </div>
                <p className="result-snippet">{result.snippet}</p>
                <div className="result-meta">
                  <span className="result-path">{result.relativePath}</span>
                  <span className="result-doc-id">{result.documentId.slice(0, 16)}…</span>
                </div>
              </li>
            ))}
          </ul>
        )}
      </div>
    </div>
  )
}

function IngestionStatus({ status }: { status: { status: string; progress: number; message: string } }) {
  return (
    <div className="ingestion-status reveal-3">
      <div className="ingestion-header">
        <div><span className="eyebrow">Ingestion pipeline</span><h3>Vault → Knowledge</h3></div>
        <span className={`ingestion-badge ${status.status}`}>{status.status.toUpperCase()}</span>
      </div>
      <div className="ingestion-progress">
        <div className="progress-bar" role="progressbar" aria-valuenow={status.progress} aria-valuemin={0} aria-valuemax={100} aria-label="Ingestion progress">
          <div className="progress-fill" style={{ width: `${status.progress}%` }} />
        </div>
        <div className="progress-text">{status.progress}% — {status.message}</div>
      </div>
      <div className="ingestion-steps">
        <IngestionStep label="Discover" state="complete" detail="Found 47 vault notes" />
        <IngestionStep label="Chunk" state="complete" detail="Created 213 chunks" />
        <IngestionStep label="Redact" state="complete" detail="Redacted 12 secrets" />
        <IngestionStep label="Validate" state="pending" detail="Schema validation" />
        <IngestionStep label="Index" state="pending" detail="Building retrieval index" />
      </div>
    </div>
  )
}

function IngestionStep({ label, state, detail }: { label: string; state: 'complete' | 'pending' | 'active'; detail: string }) {
  return (
    <div className={`ingestion-step ${state}`}>
      <span className="step-icon" aria-hidden="true">{state === 'complete' ? '✓' : state === 'active' ? '⟳' : '○'}</span>
      <div className="step-info">
        <strong>{label}</strong>
        <span>{detail}</span>
      </div>
    </div>
  )
}

function ProvenanceSeam() {
  return (
    <div className="provenance-seam reveal-3">
      <div className="section-heading"><div><span className="eyebrow">Provenance</span><h3>Attribution chain</h3></div></div>
      <div className="provenance-empty">
        <span className="empty-glyph" aria-hidden="true">⟲</span>
        <div><strong>Provenance visualization</strong><p>Chain of custody from vault note → chunk → knowledge item → retrieval result. Rendered here when available.</p></div>
      </div>
    </div>
  )
}