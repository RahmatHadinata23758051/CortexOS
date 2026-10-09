import { type InspectorDecision } from '../../hooks/useInspector'

export type DecisionPanelProps = {
  decision: InspectorDecision
  mergeState: 'idle' | 'merging' | 'merged'
  mergeEnabled: boolean
  note: string
  onNoteChange: (value: string) => void
  onDecision: (decision: 'accepted' | 'rejected' | 'changes_requested') => void
  onMerge: () => unknown
}

export function DecisionPanel({ decision, mergeState, mergeEnabled, note, onNoteChange, onDecision, onMerge }: DecisionPanelProps) {
  return (
    <section className="decision-panel" aria-labelledby="decision-title">
      <div className="panel-heading">
        <div>
          <span className="eyebrow">02 / DECISION</span>
          <h2 id="decision-title">Inspector verdict</h2>
        </div>
        <span className="decision-lock" aria-label="Orchestra-controlled">⌾</span>
      </div>
      <p className="decision-explainer">Choose a review outcome. Only <strong>Accept</strong> unlocks the Orchestra merge command.</p>
      <label className="decision-note">Review note<textarea value={note} onChange={(event) => onNoteChange(event.target.value)} placeholder="Record the reason for this decision…" rows={4} /></label>
      <div className="decision-actions">
        <button className="decision-accept" type="button" onClick={() => onDecision('accepted')} aria-pressed={decision === 'accepted'}>Accept <span>↗</span></button>
        <button className="decision-reject" type="button" onClick={() => onDecision('rejected')} aria-pressed={decision === 'rejected'}>Reject</button>
        <button className="decision-changes" type="button" onClick={() => onDecision('changes_requested')} aria-pressed={decision === 'changes_requested'}>Request changes</button>
      </div>
      <div className="merge-zone">
        <div><span className="eyebrow">ORCHESTRA MERGE</span><strong>{mergeState === 'merged' ? 'Merged into target' : mergeState === 'merging' ? 'Merging evidence…' : 'Ready only after acceptance'}</strong></div>
        <button className={`merge-button${mergeState === 'merged' ? ' merged' : ''}`} type="button" disabled={!mergeEnabled} onClick={onMerge}>{mergeState === 'merged' ? '✓ Merged' : mergeState === 'merging' ? 'Applying…' : 'Merge task'} <span aria-hidden="true">→</span></button>
      </div>
    </section>
  )
}
