import { type ReactNode } from 'react'

import { type InspectorEvidence } from '../../hooks/useInspector'

export type EvidenceViewerProps = {
  evidence: InspectorEvidence[]
  selectedIndex: number
  onSelect: (index: number) => void
  selected?: InspectorEvidence
}

export function EvidenceViewer({ evidence, selectedIndex, onSelect, selected }: EvidenceViewerProps) {
  return (
    <section className="evidence-viewer" aria-labelledby="evidence-title">
      <div className="panel-heading">
        <div>
          <span className="eyebrow">01 / EVIDENCE VIEWER</span>
          <h2 id="evidence-title">Bounded artifacts</h2>
        </div>
        <span className="evidence-count">{evidence.length} references</span>
      </div>

      {evidence.length === 0 ? (
        <div className="inspector-empty">
          <strong>No evidence attached</strong>
          <p>Inspector cannot accept a task without governed evidence.</p>
        </div>
      ) : (
        <>
          <div className="evidence-tabs" role="tablist" aria-label="Evidence references">
            {evidence.map((item, index) => (
              <button
                type="button"
                role="tab"
                aria-selected={index === selectedIndex}
                className={index === selectedIndex ? 'active' : ''}
                onClick={() => onSelect(index)}
                key={item.id}
              >
                <span>{String(index + 1).padStart(2, '0')}</span>
                {item.kind}
                <small>{item.id}</small>
              </button>
            ))}
          </div>

          {selected && (
            <div className="code-frame">
              <div className="code-toolbar">
                <span>
                  <i className="evidence-dot" />
                  {selected.language}
                </span>
                <span className="redaction-badge">⌁ {selected.redactions} redacted</span>
              </div>
              <pre aria-label="Redacted evidence code">
                <code>{renderHighlightedCode(selected.content)}</code>
              </pre>
              <footer>
                <span>{selected.id}</span>
                <strong>{selected.passed ? 'PASS / bounded and redacted' : 'REVIEW / failed gate'}</strong>
              </footer>
            </div>
          )}
        </>
      )}
    </section>
  )
}

function renderHighlightedCode(content: string): ReactNode {
  return content.split(/(\b(?:const|let|var|return|true|false|function|import|export|from|redacted)\b|[{}()[\]:,])/g).map((part, index) => {
    if (/^(const|let|var|return|true|false|function|import|export|from|redacted)$/.test(part)) {
      return <span className="syntax-key" key={index}>{part}</span>
    }
    if (/^[{}()[\]:,]$/.test(part)) {
      return <span className="syntax-punct" key={index}>{part}</span>
    }
    return part
  })
}
