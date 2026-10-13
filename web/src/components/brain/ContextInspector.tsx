import { useState } from 'react'

export type ContextSnippet = {
  id: string
  source: string
  title: string
  content: string
  score?: number
}

export type ContextInspectorProps = {
  snippets: ContextSnippet[]
  isOpen?: boolean
  onToggle?: () => void
}

export function ContextInspector({ snippets, isOpen = false, onToggle }: ContextInspectorProps) {
  const [expandedId, setExpandedId] = useState<string | null>(null)

  return (
    <aside
      className={`context-inspector ${isOpen ? 'open' : 'closed'}`}
      aria-label="Retrieved context snippets"
    >
      <div className="inspector-header">
        <div className="inspector-title-row">
          <h3>Context Snippets</h3>
          <span className="snippet-count">{snippets.length} loaded</span>
        </div>
        {onToggle && (
          <button
            type="button"
            className="toggle-button"
            onClick={onToggle}
            aria-label={isOpen ? 'Collapse context snippets' : 'Expand context snippets'}
          >
            {isOpen ? '✕' : '📂'}
          </button>
        )}
      </div>

      {isOpen && (
        <div className="inspector-body">
          {snippets.length === 0 ? (
            <p className="empty-message">No context documents loaded for this reasoning session.</p>
          ) : (
            <ul className="snippet-list">
              {snippets.map((snippet) => {
                const isExpanded = expandedId === snippet.id
                return (
                  <li key={snippet.id} className="snippet-item">
                    <button
                      type="button"
                      className="snippet-header"
                      onClick={() => setExpandedId(isExpanded ? null : snippet.id)}
                      aria-expanded={isExpanded}
                    >
                      <span className="snippet-title">{snippet.title}</span>
                      <span className="snippet-source">{snippet.source}</span>
                      {typeof snippet.score === 'number' && <span className="snippet-score">{snippet.score.toFixed(2)}</span>}
                      <span className="expand-indicator" aria-hidden="true">
                        {isExpanded ? '▲' : '▼'}
                      </span>
                    </button>
                    {isExpanded && (
                      <div className="snippet-content">
                        <pre>
                          <code>{snippet.content}</code>
                        </pre>
                      </div>
                    )}
                  </li>
                )
              })}
            </ul>
          )}
        </div>
      )}
    </aside>
  )
}
