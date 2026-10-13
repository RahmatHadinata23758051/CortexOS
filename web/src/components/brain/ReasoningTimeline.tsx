import { useMemo } from 'react'
import type { BrainState } from '../../types'

export type ReasoningTimelineProps = {
  state: BrainState
  steps: number
  tokens: number
  durationMs: number
}

const phaseOrder: Array<{ key: BrainState; label: string }> = [
  { key: 'idle', label: 'Idle' },
  { key: 'planning', label: 'Planning' },
  { key: 'executing', label: 'Executing' },
  { key: 'streaming', label: 'Streaming' },
  { key: 'tool_calling', label: 'Tool Calling' },
  { key: 'completed', label: 'Completed' },
  { key: 'failed', label: 'Failed' },
  { key: 'canceled', label: 'Canceled' },
]

function formatDuration(ms: number): string {
  if (ms < 1000) return `${ms}ms`
  const s = Math.floor(ms / 1000)
  if (s < 60) return `${s}s`
  const m = Math.floor(s / 60)
  return `${m}m ${s % 60}s`
}

export function ReasoningTimeline({ state, steps, tokens, durationMs }: ReasoningTimelineProps) {
  const phases = useMemo(() =>
    phaseOrder.map((p) => ({
      ...p,
      status: p.key === state
        ? 'current'
        : phaseOrder.findIndex((x) => x.key === state) > phaseOrder.findIndex((x) => x.key === p.key)
          ? 'done'
          : 'pending',
    })),
    [state],
  )

  return (
    <section className="reasoning-timeline" aria-labelledby="timeline-heading">
      <h3 id="timeline-heading" className="timeline-title">Reasoning Phases</h3>

      <ol className="phase-list">
        {phases.map((phase) => (
          <li key={phase.key} className={`phase-item ${phase.status}`}>
            <span className="phase-dot" aria-hidden="true" />
            <span className="phase-label">{phase.label}</span>
            {phase.status === 'current' && <span className="phase-pulse" aria-label="current phase" />}
          </li>
        ))}
      </ol>

      <div className="timeline-stats">
        <dl className="stat-grid">
          <dt>Steps</dt>
          <dd>{steps}</dd>
          <dt>Tokens</dt>
          <dd>{tokens.toLocaleString()}</dd>
          <dt>Duration</dt>
          <dd>{formatDuration(durationMs)}</dd>
        </dl>
      </div>

      {state === 'failed' && (
        <div className="timeline-error" role="alert">
          <span className="error-icon" aria-hidden="true">⚠</span>
          <span>Reasoning failed — check stream for details</span>
        </div>
      )}

      {state === 'canceled' && (
        <div className="timeline-canceled" role="status">
          <span className="cancel-icon" aria-hidden="true">⏹</span>
          <span>Reasoning canceled by user</span>
        </div>
      )}
    </section>
  )
}