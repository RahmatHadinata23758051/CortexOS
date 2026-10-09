import { useMemo, useState } from 'react'
import type { TerminalLine, WorkerMessageType } from '../../hooks/useTerminal'

export type StreamViewerProps = {
  lines: TerminalLine[]
  onFilterChange?: (type: string) => void
}

const messageTypes: Array<{ type: WorkerMessageType | 'all'; label: string }> = [
  { type: 'all', label: 'All records' },
  { type: 'handshake', label: 'Handshake' },
  { type: 'task', label: 'Task' },
  { type: 'progress', label: 'Progress' },
  { type: 'evidence', label: 'Evidence' },
  { type: 'diagnostic', label: 'Diagnostic' },
  { type: 'heartbeat', label: 'Heartbeat' },
  { type: 'terminal', label: 'Terminal' },
  { type: 'cancel', label: 'Cancel' },
]

export function StreamViewer({ lines }: StreamViewerProps) {
  const [filterType, setFilterType] = useState<string>('all')
  const [searchQuery, setSearchQuery] = useState('')
  const [rawMode, setRawMode] = useState(false)
  const [expandedId, setExpandedId] = useState<string | null>(null)

  const filteredLines = useMemo(() => {
    return lines.filter((line) => {
      if (filterType !== 'all') {
        if (!line.message || line.message.type !== filterType) return false
      }
      if (!searchQuery.trim()) return true
      const q = searchQuery.toLowerCase()
      if (line.raw.toLowerCase().includes(q)) return true
      if (line.message) {
        if (line.message.type.toLowerCase().includes(q)) return true
        if (line.message.correlationId.toLowerCase().includes(q)) return true
        if (JSON.stringify(line.message.payload).toLowerCase().includes(q)) return true
      }
      return false
    })
  }, [lines, filterType, searchQuery])

  return (
    <section className="terminal-stream-viewer" aria-labelledby="stream-viewer-title">
      <div className="stream-toolbar">
        <div className="stream-toolbar-title">
          <span className="eyebrow">cortexos.worker.v1</span>
          <h3 id="stream-viewer-title">JSONL Wire Stream</h3>
        </div>
        <div className="stream-controls">
          <label className="stream-search">
            <span className="sr-only">Search JSONL stream</span>
            <input
              type="search"
              placeholder="Search stream records…"
              value={searchQuery}
              onChange={(e) => setSearchQuery(e.target.value)}
            />
          </label>
          <div className="stream-filters" role="group" aria-label="Filter stream records">
            {messageTypes.map((mt) => (
              <button
                key={mt.type}
                type="button"
                className={`stream-filter-btn${filterType === mt.type ? ' active' : ''}`}
                onClick={() => setFilterType(mt.type)}
              >
                {mt.label}
              </button>
            ))}
          </div>
          <button
            type="button"
            className="stream-toggle-mode"
            onClick={() => setRawMode((m) => !m)}
          >
            {rawMode ? 'Structured View' : 'Raw JSONL'}
          </button>
        </div>
      </div>

      <div className="stream-records" role="log" aria-live="polite">
        {lines.length === 0 ? (
          <div className="stream-empty">
            <span className="empty-glyph" aria-hidden="true">∅</span>
            <p>No JSONL worker messages received in this session.</p>
          </div>
        ) : filteredLines.length === 0 ? (
          <div className="stream-empty">
            <p>No messages matching filter &ldquo;{filterType}&rdquo; or query.</p>
          </div>
        ) : (
          filteredLines.map((line, idx) => (
            <StreamRecordCard
              key={line.id}
              line={line}
              index={idx + 1}
              rawMode={rawMode}
              expanded={expandedId === line.id}
              onToggleExpand={() => setExpandedId(expandedId === line.id ? null : line.id)}
            />
          ))
        )}
      </div>
    </section>
  )
}

function StreamRecordCard({
  line,
  index,
  rawMode,
  expanded,
  onToggleExpand,
}: {
  line: TerminalLine
  index: number
  rawMode: boolean
  expanded: boolean
  onToggleExpand: () => void
}) {
  if (line.error) {
    return (
      <article className="stream-card error-record">
        <div className="stream-card-meta">
          <span className="stream-seq">#{index}</span>
          <span className="stream-badge error">MALFORMED</span>
          <time>{new Date(line.receivedAt).toLocaleTimeString()}</time>
        </div>
        <p className="stream-error-desc">{line.error}</p>
        <pre className="stream-raw-code"><code>{line.raw}</code></pre>
      </article>
    )
  }

  const msg = line.message
  if (!msg) return null

  if (rawMode) {
    return (
      <article className="stream-card raw-record">
        <div className="stream-card-meta">
          <span className="stream-seq">#{msg.sequence}</span>
          <span className={`stream-badge ${msg.type}`}>{msg.type}</span>
          <span className="stream-corr">{msg.correlationId}</span>
          <time>{new Date(line.receivedAt).toLocaleTimeString()}</time>
        </div>
        <pre className="stream-raw-code"><code>{line.raw}</code></pre>
      </article>
    )
  }

  return (
    <article className={`stream-card type-${msg.type}`}>
      <div className="stream-card-header" onClick={onToggleExpand} role="button" tabIndex={0} onKeyDown={(e) => { if (e.key === 'Enter' || e.key === ' ') onToggleExpand() }}>
        <div className="stream-card-meta">
          <span className="stream-seq">#{msg.sequence}</span>
          <span className={`stream-badge badge-${msg.type}`}>{msg.type}</span>
          <span className="stream-corr">{msg.correlationId}</span>
          <time>{new Date(line.receivedAt).toLocaleTimeString()}</time>
        </div>
        <div className="stream-card-summary">
          <PayloadSummary type={msg.type} payload={msg.payload} />
          <button type="button" className="stream-expand-btn" aria-label={expanded ? 'Collapse payload' : 'Expand payload'}>
            {expanded ? '▲' : '▼'}
          </button>
        </div>
      </div>
      {expanded && (
        <div className="stream-card-body">
          <div className="payload-json-wrap">
            <span className="payload-label">Payload data</span>
            <pre className="payload-json">
              <code>{JSON.stringify(msg.payload, null, 2)}</code>
            </pre>
          </div>
        </div>
      )}
    </article>
  )
}

function PayloadSummary({ type, payload }: { type: WorkerMessageType; payload: Record<string, unknown> }) {
  switch (type) {
    case 'progress':
      return (
        <span className="summary-progress">
          Phase: <strong>{String(payload.phase ?? 'unknown')}</strong> ({Number(payload.percent ?? 0)}%)
          {payload.message ? ` — ${String(payload.message)}` : ''}
        </span>
      )
    case 'evidence':
      return (
        <span className="summary-evidence">
          Kind: <strong>{String(payload.kind ?? 'unknown')}</strong> [ID: {String(payload.evidenceId ?? '').slice(0, 8)}...]
        </span>
      )
    case 'diagnostic':
      return (
        <span className={`summary-diagnostic level-${String(payload.level ?? 'info')}`}>
          [{String(payload.level ?? 'info').toUpperCase()}] {String(payload.message ?? '')}
        </span>
      )
    case 'heartbeat':
      return (
        <span className="summary-heartbeat">
          Liveness tick {payload.healthy ? '✓ healthy' : '✗ unhealthy'}
        </span>
      )
    case 'terminal':
      return (
        <span className={`summary-terminal status-${String(payload.status ?? 'unknown')}`}>
          Claim: <strong>{String(payload.status ?? 'unknown')}</strong> (Untrusted evidence)
        </span>
      )
    case 'task':
      return (
        <span className="summary-task">
          Tool: <strong>{String(payload.toolName ?? 'unknown')}</strong> Task: {String(payload.taskId ?? '')}
        </span>
      )
    case 'cancel':
      return (
        <span className="summary-cancel">
          Reason: <strong>{String(payload.reason ?? 'Canceled')}</strong>
        </span>
      )
    default:
      return <span className="summary-default">{JSON.stringify(payload).slice(0, 60)}</span>
  }
}
