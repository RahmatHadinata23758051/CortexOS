import { useState } from 'react'

import { AuditTrail } from './AuditTrail'
import { DecisionPanel } from './DecisionPanel'
import { EvidenceViewer } from './EvidenceViewer'
import { MergeDialog } from './MergeDialog'
import { useInspector } from '../../hooks/useInspector'
import { mergeOrchestraTask } from '../../bridge/orchestra'
import { type OrchestraBridge, type OrchestraTaskDetail, type OrchestraTaskSummary } from '../../types/orchestra'

export type InspectorReviewProps = {
  task: OrchestraTaskSummary
  detail: OrchestraTaskDetail
  /** Without a bridge the merge command is unavailable and the gate stays locked. */
  bridge?: OrchestraBridge
  onClose?: () => void
  /** Called after the backend confirms a merge so the board can re-read state. */
  onMerged?: () => void
}

export function InspectorReview({ task, detail, bridge, onClose, onMerged }: InspectorReviewProps) {
  const inspector = useInspector(detail, {
    mergeCommand: bridge ? (note) => mergeOrchestraTask(bridge, task.id, note) : undefined,
    onMerged,
  })
  const [selectedEvidence, setSelectedEvidence] = useState(0)
  const [mergeDialogOpen, setMergeDialogOpen] = useState(false)
  const selected = inspector.evidence[selectedEvidence]

  return <section className="inspector-review" aria-labelledby="inspector-title">
    <header className="inspector-review-header">
      <div className="tool-heading"><span className="eyebrow">CORTEXOS / INSPECTOR</span><h1 id="inspector-title">Inspector Review</h1></div>
      <div className="inspector-contract"><span>AUTHORITY</span><strong>ADR-0005</strong></div>
    </header>
    <div className="inspector-meta"><code>{task.id}</code><span className={`inspector-state state-${inspector.decision}`}>{decisionLabel(inspector.decision)}</span><span className="inspector-gate">MERGE GATE: {inspector.mergeEnabled || inspector.mergeState === 'merged' ? 'OPEN' : 'LOCKED'}</span></div>
    <div className="inspector-grid">
      <EvidenceViewer evidence={inspector.evidence} selectedIndex={selectedEvidence} onSelect={setSelectedEvidence} selected={selected} />
      <DecisionPanel decision={inspector.decision} mergeState={inspector.mergeState} mergeEnabled={inspector.mergeEnabled} note={inspector.note} onNoteChange={inspector.setNote} onDecision={inspector.decide} onMerge={() => setMergeDialogOpen(true)} />
    </div>
    <AuditTrail entries={inspector.audit} />
    {onClose && <button className="inspector-close" type="button" onClick={onClose}>Back to task board</button>}
    <MergeDialog
      isOpen={mergeDialogOpen}
      decision={inspector.decision}
      mergeState={inspector.mergeState}
      mergeEnabled={inspector.mergeEnabled}
      mergeError={inspector.mergeError}
      taskId={task.id}
      onMerge={inspector.merge}
      onClose={() => setMergeDialogOpen(false)}
    />
  </section>
}

function decisionLabel(decision: ReturnType<typeof useInspector>['decision']): string {
  return decision === 'changes_requested' ? 'Changes requested' : decision === 'pending' ? 'Awaiting decision' : decision[0].toUpperCase() + decision.slice(1)
}
