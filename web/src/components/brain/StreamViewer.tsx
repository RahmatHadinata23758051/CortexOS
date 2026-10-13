import { useEffect, useRef } from 'react'
import type { BrainStreamChunk } from '../../types'

export type BrainStreamViewerProps = {
  output: string
  chunks: BrainStreamChunk[]
  isRunning: boolean
}

export function BrainStreamViewer({ output, chunks, isRunning }: BrainStreamViewerProps) {
  const containerRef = useRef<HTMLDivElement>(null)

  // Auto-scroll to bottom as tokens arrive unless user manually scrolls up
  useEffect(() => {
    if (isRunning && containerRef.current) {
      containerRef.current.scrollTop = containerRef.current.scrollHeight
    }
  }, [output, isRunning])

  return (
    <section className="brain-stream-viewer" aria-labelledby="stream-viewer-heading">
      <div className="stream-header">
        <h3 id="stream-viewer-heading">Reasoning Stream</h3>
        <div className="stream-meta">
          <span className="chunk-badge">{chunks.length} chunks</span>
          {isRunning && <span className="stream-indicator live">● streaming</span>}
        </div>
      </div>

      <div ref={containerRef} className="stream-content" tabIndex={0} role="region" aria-label="Streaming content">
        {output ? (
          <pre className="stream-markdown">
            <code>{output}</code>
          </pre>
        ) : (
          <div className="stream-placeholder">
            <span className="placeholder-icon" aria-hidden="true">💬</span>
            <p>{isRunning ? 'Generating response…' : 'Enter a prompt and run to see streaming reasoning tokens.'}</p>
          </div>
        )}
      </div>
    </section>
  )
}
