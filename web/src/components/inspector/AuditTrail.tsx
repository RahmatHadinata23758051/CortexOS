import { type InspectorAuditEntry } from '../../hooks/useInspector'

export type AuditTrailProps = {
  entries: InspectorAuditEntry[]
}

export function AuditTrail({ entries }: AuditTrailProps) {
  return (
    <section className="audit-trail" aria-labelledby="audit-title">
      <div className="panel-heading">
        <div>
          <span className="eyebrow">03 / AUDIT TRAIL</span>
          <h2 id="audit-title">Decisions stay traceable</h2>
        </div>
        <span className="audit-contract">local / append-only view</span>
      </div>

      {entries.length === 0 ? (
        <p className="audit-empty">No decisions recorded for this review yet.</p>
      ) : (
        <ol>
          {entries.map((entry) => (
            <li key={entry.id}>
              <span className={`audit-marker marker-${entry.decision}`} />
              <div>
                <strong>{decisionLabel(entry.decision)}</strong>
                <p>{entry.note}</p>
              </div>
              <time dateTime={entry.occurredAt}>{formatTime(entry.occurredAt)}</time>
            </li>
          ))}
        </ol>
      )}
    </section>
  )
}

function decisionLabel(decision: InspectorDecision): string {
  return decision === 'changes_requested'
    ? 'Changes requested'
    : decision === 'pending'
      ? 'Awaiting decision'
      : decision[0].toUpperCase() + decision.slice(1)
}

function formatTime(value: string): string {
  return new Date(value).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })
}

import { type InspectorDecision } from '../../hooks/useInspector'